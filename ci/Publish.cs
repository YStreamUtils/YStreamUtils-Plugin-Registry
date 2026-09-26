using System.Text.Json;
using Json.Schema;
using Json.Schema.Generation;
using YStreamUtils.SDK.Plugin;
using YStreamUtils.SDK.Registry;

namespace ci;

public static class Publish
{
    public static void Execute()
    {
        Console.WriteLine("=== Compiling Plugin Registry ===");

        var projectRoot = Directory.GetCurrentDirectory();
        var pluginsPath = Path.Combine(projectRoot, "plugins");
        var outputDir = Path.Combine(projectRoot, "public");
        
        var outputPath = Path.Combine(outputDir, "registry.json");
        var schemaOutputPath = Path.Combine(outputDir, "schema.json");

        var registry = new RegistryDistribution
        {
            Name = "YStreamUtils Plugin Registry",
            Description = "The Official YStreamUtils plugin registry.",
            Source = new SourceConfig
            {
                Owner = "YStreamUtils",
                Repository = "YStreamUtils-Plugin-Registry"
            },
            Plugins = []
        };

        if (!Directory.Exists(pluginsPath))
        {
            Console.WriteLine("Compilation Error: 'plugins' folder missing.");
            Environment.Exit(1);
        }

        var manifestFiles = Directory.GetFiles(pluginsPath, "manifest.json", SearchOption.AllDirectories);
        var options = new JsonSerializerOptions { PropertyNameCaseInsensitive = true, WriteIndented = true };

        try
        {
            foreach (var path in manifestFiles)
            {
                var jsonText = File.ReadAllText(path);
                var manifest = JsonSerializer.Deserialize<PluginManifest>(jsonText, options);

                if (manifest == null)
                    throw new Exception($"Failed to decode JSON structure at: {path}");

                if (string.IsNullOrWhiteSpace(manifest.Name))
                    throw new Exception($"Manifest structure data error: missing name at {path}");

                registry.Plugins.Add(manifest);
                Console.WriteLine($"[Bundler] Staged object entry for plugin: {manifest.Name}");
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine($"COMPILATION ARTIFACT FAILURE: {ex.Message}");
            Environment.Exit(1);
        }

        if (!Directory.Exists(outputDir))
        {
            Directory.CreateDirectory(outputDir);
        }

        try
        {
            var registryJson = JsonSerializer.Serialize(registry, options);
            File.WriteAllText(outputPath, registryJson);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"ENCODER SERIALIZATION FAILURE: {ex.Message}");
            Environment.Exit(1);
        }

        try
        {
            var schemaBuilder = new JsonSchemaBuilder();
            var schema = schemaBuilder.FromType<PluginManifest>().Build();
            
            var schemaJson = JsonSerializer.Serialize(schema, options);
            File.WriteAllText(schemaOutputPath, schemaJson);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"SCHEMA SERIALIZATION FAILURE: {ex.Message}");
            Environment.Exit(1);
        }

        Console.WriteLine($"Build Complete! Successfully exported to {outputDir}");
        Environment.Exit(0);
    }
}