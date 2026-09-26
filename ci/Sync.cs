using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;
using Octokit;
using YStreamUtils.SDK.Plugin;

namespace ci;

public static class Sync
{
    private static readonly HttpClient HttpClient = new();

    public static async Task ExecuteAsync(GitHubClient client)
    {
        Console.WriteLine("[Syncer] Initiating automated upstream sync scan...");
        
        var projectRoot = Directory.GetCurrentDirectory();
        var pluginsPath = Path.Combine(projectRoot, "plugins");

        if (!Directory.Exists(pluginsPath))
        {
            Console.WriteLine("[Syncer] No plugins folder found. Exiting.");
            Environment.Exit(0);
        }

        var manifestPaths = Directory.GetFiles(pluginsPath, "manifest.json", SearchOption.AllDirectories);

        var hasUpdates = false;
        var manualReviewRequired = false;

        var options = new JsonSerializerOptions 
        { 
            PropertyNameCaseInsensitive = true, 
            WriteIndented = true 
        };

        foreach (var manifestPath in manifestPaths)
        {
            var jsonText = await File.ReadAllTextAsync(manifestPath);
            var localManifest = JsonSerializer.Deserialize<PluginManifest>(jsonText, options);

            if (localManifest?.Source == null || string.IsNullOrWhiteSpace(localManifest.Name)) continue;

            try
            {
                var latestRelease = await client.Repository.Release.GetLatest(localManifest.Source.Owner, localManifest.Source.Repository);
                var expectedDllName = $"{localManifest.Name}.dll";
                var dllAsset = latestRelease.Assets.FirstOrDefault(a => string.Equals(a.Name, expectedDllName, StringComparison.OrdinalIgnoreCase));

                if (dllAsset == null) continue;

                var dllBytes = await HttpClient.GetByteArrayAsync(dllAsset.BrowserDownloadUrl);
                using var stream = new MemoryStream(dllBytes);

                var loadContext = new AssemblyLoadContext("SyncValidationContext", isCollectible: true);
                PluginManifestAttribute? upstreamAttr;
                try
                {
                    var assembly = loadContext.LoadFromStream(stream);
                    upstreamAttr = assembly.GetCustomAttribute<PluginManifestAttribute>();
                }
                finally
                {
                    loadContext.Unload();
                }

                if (upstreamAttr == null) continue;

                if (upstreamAttr.Version != localManifest.Version)
                {
                    Console.WriteLine($"[Update Found] {localManifest.Name}: {localManifest.Version} -> {upstreamAttr.Version}");

                    var updatedManifest = upstreamAttr.ToManifest();
                    var updatedJson = JsonSerializer.Serialize(updatedManifest, options);

                    await File.WriteAllTextAsync(manifestPath, updatedJson);
                    hasUpdates = true;

                    var oldVersion = localManifest.Version.Split('.');
                    var newVersion = upstreamAttr.Version.Split('.');

                    if (oldVersion.Length == 3 && newVersion.Length == 3 && oldVersion[0] != newVersion[0])
                    {
                        Console.WriteLine($"[Security] Major version bump detected for {localManifest.Name}. Flagging for manual review.");
                        manualReviewRequired = true;
                    }
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[Error] Failed scanning upstream updates for {localManifest.Name}: {ex.Message}");
            }
        }

        if (hasUpdates)
        {
            if (manualReviewRequired)
            {
                Console.WriteLine("[Syncer] Sync complete. Updates written. Manual review REQUIRED.");
                Environment.Exit(11); // Updates + Manual Review
            }
            else
            {
                Console.WriteLine("[Syncer] Sync complete. Updates written. Eligible for Auto-Merge.");
                Environment.Exit(10); // Updates + Safe Auto-Merge
            }
        }
        else
        {
            Console.WriteLine("[Syncer] Sync complete. All plugin registrations are up to date.");
            Environment.Exit(0); // Zero changes
        }
    }
}