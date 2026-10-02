using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;
using Octokit;
using YStreamUtils.SDK.Plugin;

namespace ci;

public static class Verify
{
    private static readonly HttpClient HttpClient = new();

    private static readonly JsonSerializerOptions DefaultSerializerOptions =
        new() { PropertyNameCaseInsensitive = true };

    public static async Task ExecuteAsync(GitHubClient client)
    {
        var projectRoot = Directory.GetCurrentDirectory();
        var pluginsPath = Path.Combine(projectRoot, "plugins");

        if (!Directory.Exists(pluginsPath)) return;

        var manifestPaths = Directory.GetFiles(pluginsPath, "manifest.json", SearchOption.AllDirectories);
        if (manifestPaths.Length == 0) return;

        var semaphore = new SemaphoreSlim(10);

        var tasks = manifestPaths.Select(async manifestPath =>
        {
            await semaphore.WaitAsync();
            var loadContext = new AssemblyLoadContext("PluginValidationContext", isCollectible: true);
            
            try
            {
                var relPath = Path.GetRelativePath(projectRoot, manifestPath).Replace('\\', '/');
                var parts = relPath.Split('/');
                if (parts.Length != 4) throw new Exception("Directory layout structure violation.");

                var ownerScope = parts[1];

                var jsonText = await File.ReadAllTextAsync(manifestPath);
                var localManifest = JsonSerializer.Deserialize<PluginManifest>(jsonText, DefaultSerializerOptions);
                if (localManifest == null) throw new Exception("Corrupted manifest JSON.");

                var latestRelease = await client.Repository.Release.GetLatest(localManifest.Source.Owner, localManifest.Source.Repository);
                var expectedDllName = $"{localManifest.Name}.dll";
                var dllAsset = latestRelease.Assets.FirstOrDefault(a => string.Equals(a.Name, expectedDllName, StringComparison.OrdinalIgnoreCase));

                if (dllAsset == null)
                    throw new Exception($"Release error: Missing '{expectedDllName}' on GitHub.");

                var dllBytes = await HttpClient.GetByteArrayAsync(dllAsset.BrowserDownloadUrl);
                using var stream = new MemoryStream(dllBytes);

                var assembly = loadContext.LoadFromStream(stream);

                var attribute = assembly.GetCustomAttribute<PluginManifestAttribute>();

                if (attribute == null)
                    throw new Exception($"'{expectedDllName}' lacks the [assembly: PluginManifestAttribute] declaration.");

                if (!string.Equals(attribute.Name, localManifest.Name, StringComparison.OrdinalIgnoreCase))
                    throw new Exception($"Mismatch: Name in DLL ('{attribute.Name}') does not match JSON ('{localManifest.Name}')");

                if (!string.Equals(attribute.Version, localManifest.Version, StringComparison.OrdinalIgnoreCase))
                    throw new Exception($"Mismatch: Version in DLL ('{attribute.Version}') does not match JSON ('{localManifest.Version}')");

                Console.WriteLine($"PASS: Verified {ownerScope}/{localManifest.Name}");
            }
            finally
            {
                loadContext.Unload();
                semaphore.Release();
            }
        });

        try
        {
            await Task.WhenAll(tasks);
            Console.WriteLine("\n=== Validation Succeeded: All plugins approved. ===");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"\nREJECTED BY AUTOMATION PIPELINE: {ex.Message}");
            Environment.Exit(1);
        }
    }
}