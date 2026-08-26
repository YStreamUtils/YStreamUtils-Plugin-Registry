package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/google/go-github/v90/github"
)

type Permission string

type SourceConfig struct {
	Repository string `toml:"repository"`
	Owner      string `toml:"owner"`
	EntryPoint string `toml:"entry_point"`
}

type DocumentationConfig struct {
	Description string `toml:"description"`
}

type PluginManifest struct {
	Name          string              `toml:"name"`
	Version       string              `toml:"version"`
	Permissions   []Permission        `toml:"permissions"`
	Authors       []string            `toml:"authors"`
	Source        SourceConfig        `toml:"source"`
	Documentation DocumentationConfig `toml:"documentation"`
}

func main() {
	fmt.Println("=== Scanning Registry  ===")

	ctx := context.Background()
	var client *github.Client

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken != "" {
		client, _ = github.NewClient(github.WithAuthToken(githubToken))
		fmt.Println("[Linter] Initializing authenticated GitHub API client context.")
	} else {
		client, _ = github.NewClient(nil)
		fmt.Println("[Linter] Initializing unauthenticated GitHub API client context.")
	}
	seenNamespaces := make(map[string]string)
	manifestCount := 0

	err := filepath.Walk("plugins", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == "manifest.toml" {
			manifestCount++
			fmt.Printf("\n[Linter] Evaluating file boundary: %s\n", path)

			parts := strings.Split(filepath.ToSlash(path), "/")
			if len(parts) != 4 {
				return fmt.Errorf("Directory structure layout violation inside '%s'. Paths must exactly follow 'plugins/OwnerScope/PluginName/manifest.toml'", path)
			}

			ownerScope := parts[1]
			pluginDirName := parts[2]

			var manifest PluginManifest
			if _, err := toml.DecodeFile(path, &manifest); err != nil {
				return fmt.Errorf("TOML PARSING ERROR inside %s: %w", path, err)
			}

			if !strings.EqualFold(manifest.Source.Owner, ownerScope) {
				return fmt.Errorf("security boundary mismatch: Parent folder scope name '%s' must match manifest GitHub Owner target identity '%s' inside %s",
					ownerScope, manifest.Source.Owner, path)
			}

			if !strings.EqualFold(manifest.Name, pluginDirName) {
				return fmt.Errorf("directory name mismatch: Functional execution namespace '%s' must match folder folder name '%s'",
					manifest.Name, pluginDirName)
			}

			if len(manifest.Authors) == 0 {
				return fmt.Errorf("attribution error: At least one developer name must be declared inside the authors array allocation")
			}

			ns := strings.ToLower(manifest.Name)
			if originalPath, duplicate := seenNamespaces[ns]; duplicate {
				return fmt.Errorf("namespace hijacking collision: '%s' is already claimed by %s (Blocked execution path inside %s)", ns, originalPath, path)
			}
			seenNamespaces[ns] = path

			release, _, err := client.Repositories.GetLatestRelease(ctx, manifest.Source.Owner, manifest.Source.Repository)
			if err != nil {
				return fmt.Errorf("github api connectivity fault: Unable to fetch latest release tags for public repository %s/%s: %w", manifest.Source.Owner, manifest.Source.Repository, err)
			}

			hasWasm := false
			for _, asset := range release.Assets {
				if strings.HasSuffix(asset.GetName(), ".wasm") {
					hasWasm = true
					break
				}
			}

			if !hasWasm {
				return fmt.Errorf("release payload validation fault: Latest GitHub release '%s' for %s/%s lacks an accompanying compiled target .wasm file asset", release.GetTagName(), manifest.Source.Owner, manifest.Source.Repository)
			}

			fmt.Printf("✅ PASS: Registry profile %s/%s verified cleanly.\n", ownerScope, manifest.Name)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("\n❌ PULL REQUEST REJECTED BY CI: %v\n\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== Check Complete: Approved all %d plugin manifests. ===\n", manifestCount)
	os.Exit(0)
}
