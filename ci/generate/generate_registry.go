package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/YStreamUtils/YStreamUtils-Plugin-Registry/ci/types"
	"github.com/invopop/jsonschema"
)

func main() {
	fmt.Println("=== Compiling Unified Plugin Registry Distribution ===")

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Unable to look up current runtime compilation directory context")
	}

	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	pluginsPath := filepath.Join(projectRoot, "plugins")
	outputPath := filepath.Join(projectRoot, "public", "registry.json")
	schemaOutputPath := filepath.Join(projectRoot, "public", "schema.json")

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

		if !info.IsDir() && info.Name() == "manifest.json" {
			err := func() error {
				file, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open manifest at %s: %w", path, err)
				}
				defer file.Close()

				var manifest types.PluginManifest
				byteValue, err := io.ReadAll(file)
				if err != nil {
					return fmt.Errorf("failed to read file at %s: %w", path, err)
				}

				if err := json.Unmarshal(byteValue, &manifest); err != nil {
					return fmt.Errorf("failed to decode JSON at %s: %w", path, err)
				}

				if manifest.Name == "" {
					return fmt.Errorf("manifest missing name declaration at: %s", path)
				}

				registry.Plugins = append(registry.Plugins, manifest)
				fmt.Printf("[Bundler] Staged object index entry for plugin: %s\n", manifest.Name)
				return nil
			}()

			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("❌ COMPILATION ARTIFACT FAILURE: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		fmt.Printf("❌ FOLDER CREATION FAILURE: %v\n", err)
		os.Exit(1)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("❌ STORAGE BOUNDARY ARTIFACT FAILURE: Cannot create asset target: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(registry); err != nil {
		fmt.Printf("❌ ENCODER SERIALIZATION FAILURE: %v\n", err)
		os.Exit(1)
	}

	schemaFile, err := os.Create(schemaOutputPath)
	if err != nil {
		fmt.Printf("❌ SCHEMA FILE CREATION FAILURE: %v\n", err)
		os.Exit(1)
	}
	defer schemaFile.Close()

	reflectedSchema := jsonschema.Reflect(&types.PluginManifest{})

	schemaEncoder := json.NewEncoder(schemaFile)
	schemaEncoder.SetIndent("", "  ")
	if err := schemaEncoder.Encode(reflectedSchema); err != nil {
		fmt.Printf("❌ SCHEMA SERIALIZATION FAILURE: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n🚀 Build Complete! Successfully exported unified structure to %s\n", outputPath)
	os.Exit(0)
}
