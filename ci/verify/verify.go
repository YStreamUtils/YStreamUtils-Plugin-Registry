package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/YStreamUtils/YStreamUtils-Plugin-Registry/ci/types"
	"github.com/google/go-github/v90/github"
	"golang.org/x/sync/errgroup"
)

func main() {
	fmt.Println("=== Scanning Registry Pipeline ===")

	ctx := context.Background()
	var client *github.Client
	var err error

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken != "" {
		client, err = github.NewClient(github.WithAuthToken(githubToken))
		if err != nil {
			panic(fmt.Sprintf("[Linter] Configuration failure: %v", err))
		}
		fmt.Println("[Linter] Initializing authenticated GitHub API client context.")
	} else {
		client, err = github.NewClient()
		if err != nil {
			panic(fmt.Sprintf("[Linter] Configuration failure: %v", err))
		}
		fmt.Println("[Linter] Initializing unauthenticated GitHub API client context.")
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Unable to look up current runtime compilation directory context")
	}

	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	pluginsPath := filepath.Join(projectRoot, "plugins")

	var targetDirs []string
	changedPluginsEnv := os.Getenv("CHANGED_PLUGINS")

	if changedPluginsEnv != "" {
		rawDirs := strings.Fields(changedPluginsEnv)
		for _, d := range rawDirs {
			absPath := filepath.Clean(filepath.Join(projectRoot, d))
			if strings.HasPrefix(absPath, pluginsPath) && absPath != pluginsPath {
				targetDirs = append(targetDirs, absPath)
			}
		}
		fmt.Printf("[Linter] Scoping execution context down to %d modified subdirectories.\n", len(targetDirs))
	}

	var manifestPaths []string

	if len(targetDirs) > 0 {
		for _, dir := range targetDirs {
			manifestFile := filepath.Join(dir, "manifest.json")
			if _, err := os.Stat(manifestFile); err == nil {
				manifestPaths = append(manifestPaths, manifestFile)
			}
		}
	} else {
		fmt.Println("[Linter] No targeted folder limits found. Scanning full tree...")
		err = filepath.Walk(pluginsPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && info.Name() == "manifest.json" {
				manifestPaths = append(manifestPaths, path)
			}
			return nil
		})
		if err != nil {
			fmt.Printf("❌ Discovery phase failure: %v\n", err)
			os.Exit(1)
		}
	}

	g, egCtx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	fmt.Printf("[Linter] Scheduling parallel verification jobs for %d manifests...\n", len(manifestPaths))

	for _, path := range manifestPaths {
		manifestPath := path

		g.Go(func() error {
			fmt.Printf("[Worker] Starting check on: %s\n", manifestPath)

			relPath, err := filepath.Rel(projectRoot, manifestPath)
			if err != nil {
				return fmt.Errorf("failed to resolve relative path context for %s: %w", manifestPath, err)
			}

			parts := strings.Split(filepath.ToSlash(relPath), "/")
			if len(parts) != 4 {
				return fmt.Errorf("directory structure layout violation inside '%s'. Paths must follow 'plugins/OwnerScope/PluginName/manifest.json'", relPath)
			}

			ownerScope := parts[1]
			pluginDirName := parts[2]

			jsonBytes, err := os.ReadFile(manifestPath)
			if err != nil {
				return fmt.Errorf("failed reading file path context: %w", err)
			}

			var manifest types.PluginManifest
			if err := json.Unmarshal(jsonBytes, &manifest); err != nil {
				return fmt.Errorf("JSON PARSING ERROR inside %s: %w", manifestPath, err)
			}

			if !strings.EqualFold(manifest.Source.Owner, ownerScope) {
				return fmt.Errorf("security boundary mismatch: Parent folder scope name '%s' must match manifest GitHub Owner target identity '%s' inside %s", ownerScope, manifest.Source.Owner, manifestPath)
			}

			if !strings.EqualFold(manifest.Name, pluginDirName) {
				return fmt.Errorf("directory name mismatch: Plugin name '%s' must match folder name '%s'", manifest.Name, pluginDirName)
			}

			if len(manifest.Authors) == 0 {
				return fmt.Errorf("at least one developer name must be declared inside the authors array allocation inside %s", manifestPath)
			}

			if strings.TrimSpace(manifest.EntryPoint) == "" {
				return fmt.Errorf("manifest must have an entrypoint target string defined inside %s", manifestPath)
			}

			release, _, err := client.Repositories.GetLatestRelease(egCtx, manifest.Source.Owner, manifest.Source.Repository)
			if err != nil {
				return fmt.Errorf("github api connectivity fault for %s/%s: %w", manifest.Source.Owner, manifest.Source.Repository, err)
			}

			expectedZipName := fmt.Sprintf("%s.zip", manifest.Name)
			var zipAsset *github.ReleaseAsset

			for _, asset := range release.Assets {
				if strings.EqualFold(asset.GetName(), expectedZipName) {
					zipAsset = asset
					break
				}
			}

			if zipAsset == nil {
				return fmt.Errorf("release payload validation fault: Latest GitHub release '%s' for %s/%s lacks the specific plugin archive: '%s'", release.GetTagName(), manifest.Source.Owner, manifest.Source.Repository, expectedZipName)
			}

			rc, _, err := client.Repositories.DownloadReleaseAsset(egCtx, manifest.Source.Owner, manifest.Source.Repository, zipAsset.GetID(), http.DefaultClient)
			if err != nil {
				return fmt.Errorf("failed creating download stream context for asset '%s': %w", zipAsset.GetName(), err)
			}
			defer rc.Close()

			zipBytes, err := io.ReadAll(rc)
			if err != nil {
				return fmt.Errorf("failed reading zip artifact payload buffer: %w", err)
			}

			zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
			if err != nil {
				return fmt.Errorf("downloaded asset file '%s' is an invalid or corrupted zip format payload: %w", zipAsset.GetName(), err)
			}

			hasEntrypoint := false
			hasTypeDefinitions := false
			hasManifestInsideZip := false

			for _, file := range zipReader.File {
				cleanPath := filepath.ToSlash(file.Name)

				if cleanPath == manifest.EntryPoint {
					hasEntrypoint = true
				}
				if cleanPath == "index.d.ts" {
					hasTypeDefinitions = true
				}
				if cleanPath == "manifest.json" {
					hasManifestInsideZip = true
				}
			}

			if !hasEntrypoint {
				return fmt.Errorf("release payload validation fault: Compressed file archive '%s' is missing the declared manifest entrypoint resource path: '%s'", zipAsset.GetName(), manifest.EntryPoint)
			}

			if !hasTypeDefinitions {
				return fmt.Errorf("release payload validation fault: Compressed file archive '%s' is missing the required Monaco engine auto-complete type file: 'index.d.ts'", zipAsset.GetName())
			}

			if !hasManifestInsideZip {
				return fmt.Errorf("release payload validation fault: Compressed file archive '%s' is missing its own 'manifest.json' reference file", zipAsset.GetName())
			}

			currentActiveVersion := "" 
			isNewPlugin := currentActiveVersion == ""

			if !isNewPlugin {
				oldParts := strings.Split(currentActiveVersion, ".")
				newParts := strings.Split(manifest.Version, ".")

				if len(oldParts) == 3 && len(newParts) == 3 {
					if oldParts[0] != newParts[0] {
						fmt.Printf("[Security] ⚠️ Major version bump detected (%s -> %s). Halting automerge.\n", currentActiveVersion, manifest.Version)
						_ = os.WriteFile(".require_manual_review", []byte("major_bump"), 0644)
					} else {
						fmt.Printf("[Linter] Safe patch/minor bump detected (%s -> %s). Eligible for automerge.\n", currentActiveVersion, manifest.Version)
					}
				}
			} else {
				fmt.Println("[Security] ⚠️ Brand new plugin registration detected. Halting automerge.")
				_ = os.WriteFile(".require_manual_review", []byte("new_plugin"), 0644)
			}

			fmt.Printf("✅ PASS: Verified %s/%s cleanly.\n", ownerScope, manifest.Name)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("\n❌ PULL REQUEST REJECTED BY CI CONCURRENCY ENGINE: %v\n\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== Check Complete: All targeted plugin manifests approved cleanly. ===\n")
	os.Exit(0)
}
