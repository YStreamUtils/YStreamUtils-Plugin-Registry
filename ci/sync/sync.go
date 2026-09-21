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
	"strings"

	"github.com/google/go-github/v90/github"
)

type Source struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
}

type Manifest struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	EntryPoint string `json:"entryPoint"`
	Source     Source `json:"source"`
}

func main() {
	ctx := context.Background()
	var client *github.Client
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken != "" {
		client, _ = github.NewClient(github.WithAuthToken(githubToken))
	} else {
		client, _ = github.NewClient()
	}

	pluginsPath := filepath.Clean("../plugins")
	hasUpdates := false

	err := filepath.Walk(pluginsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "manifest.json" {
			return err
		}

		jsonBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var currentManifest Manifest
		if err := json.Unmarshal(jsonBytes, &currentManifest); err != nil {
			return nil
		}

		owner := currentManifest.Source.Owner
		repo := currentManifest.Source.Repository
		if owner == "" || repo == "" {
			return nil
		}

		release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
		if err != nil {
			return nil
		}

		expectedZipName := fmt.Sprintf("%s.zip", currentManifest.Name)
		var zipAsset *github.ReleaseAsset
		for _, asset := range release.Assets {
			if strings.EqualFold(asset.GetName(), expectedZipName) {
				zipAsset = asset
				break
			}
		}
		if zipAsset == nil {
			return nil
		}

		rc, _, err := client.Repositories.DownloadReleaseAsset(ctx, owner, repo, zipAsset.GetID(), http.DefaultClient)
		if err != nil {
			return nil
		}
		defer rc.Close()

		zipBytes, err := io.ReadAll(rc)
		if err != nil {
			return nil
		}

		zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			return nil
		}

		var remoteManifestBytes []byte
		for _, file := range zipReader.File {
			if filepath.ToSlash(file.Name) == "manifest.json" {
				fc, err := file.Open()
				if err == nil {
					remoteManifestBytes, _ = io.ReadAll(fc)
					fc.Close()
				}
				break
			}
		}
		if len(remoteManifestBytes) == 0 {
			return nil
		}

		if !bytes.Equal(jsonBytes, remoteManifestBytes) {
			if err := os.WriteFile(path, remoteManifestBytes, 0644); err == nil {
				fmt.Printf("Syncing update for plugin: %s\n", currentManifest.Name)
				hasUpdates = true
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Scan pipeline error: %v\n", err)
		os.Exit(1)
	}

	if hasUpdates {
		os.WriteFile("../.has_updates_token", []byte("true"), 0644)
	}
	os.Exit(0)
}
