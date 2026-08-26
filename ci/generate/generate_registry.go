package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"ystreamutils-plugin-registry/types"

	"github.com/BurntSushi/toml"
)

func main() {
	fmt.Println("=== Compiling Unified Plugin Registry Distribution ===")

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Unable to look up current runtime compilation directory context")
	}

	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	pluginsPath := filepath.Join(projectRoot, "plugins")
	outputPath := filepath.Join(projectRoot, "public", "registry.toml")

	registry := types.RegistryDistribution{
		Name: "YStreamutils Plugin Registry",
		Source: types.SourceConfig{
			Owner:      "YStreamutils",
			Repository: "YStreamutils-Plugin-Registry",
		},
		Plugins: []types.PluginManifest{},
	}

	err := filepath.Walk(pluginsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == "manifest.toml" {
			var manifest types.PluginManifest
			if _, err := toml.DecodeFile(path, &manifest); err != nil {
				return fmt.Errorf("failed to decode TOML at %s: %w", path, err)
			}

			if manifest.Name == "" {
				return fmt.Errorf("manifest missing name declaration at: %s", path)
			}

			registry.Plugins = append(registry.Plugins, manifest)
			fmt.Printf("[Bundler] Staged object index entry for plugin: %s\n", manifest.Name)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("❌ COMPILATION ARTIFACT FAILURE: %v\n", err)
		os.Exit(1)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("❌ STORAGE BOUNDARY ARTIFACT FAILURE: Cannot create asset target: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	encoder := toml.NewEncoder(outFile)
	if err := encoder.Encode(registry); err != nil {
		fmt.Printf("❌ ENCODER SERIALIZATION FAILURE: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n🚀 Build Complete! Successfully exported unified structure to %s\n", outputPath)
	os.Exit(0)
}
