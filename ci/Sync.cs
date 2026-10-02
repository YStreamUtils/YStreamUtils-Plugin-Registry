using System.IO.Compression;
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
                Console.WriteLine("Searching for updates for plugin: {0}", localManifest.Name);
                var latestRelease = await client.Repository.Release.GetLatest(localManifest.Source.Owner, localManifest.Source.Repository);
                
                var expectedZipName = $"{localManifest.Name}.zip";
                var zipAsset = latestRelease.Assets.FirstOrDefault(a => string.Equals(a.Name, expectedZipName, StringComparison.OrdinalIgnoreCase));

                if (zipAsset == null) continue;
                Console.WriteLine("Processing Plugin Manifest from ZIP: {0}", localManifest.Name);

                var zipBytes = await HttpClient.GetByteArrayAsync(zipAsset.BrowserDownloadUrl);
                using var zipStream = new MemoryStream(zipBytes);
                await using var archive = new ZipArchive(zipStream, ZipArchiveMode.Read);

                var expectedDllName = $"{localManifest.Name}.dll";
                var dllEntry = archive.Entries.FirstOrDefault(e => string.Equals(e.Name, expectedDllName, StringComparison.OrdinalIgnoreCase));

                if (dllEntry == null)
                {
                    Console.WriteLine($"[Warning] Found zip asset, but it does not contain {expectedDllName}");
                    continue;
                }

                PluginManifestAttribute? upstreamAttr;
                await using (var dllStream = await dllEntry.OpenAsync())
                {
                    using var seekableDllStream = new MemoryStream();
                    await dllStream.CopyToAsync(seekableDllStream);
                    seekableDllStream.Position = 0;

                    var loadContext = new AssemblyLoadContext("SyncValidationContext", isCollectible: true);
                    try
                    {
                        var assembly = loadContext.LoadFromStream(seekableDllStream);
                        upstreamAttr = assembly.GetCustomAttribute<PluginManifestAttribute>();
                    }
                    finally
                    {
                        loadContext.Unload();
                    }
                }
                
                Console.WriteLine($"[Syncer] Plugin Manifest: {JsonSerializer.Serialize(upstreamAttr, options)}");

                if (upstreamAttr == null) continue;

                Console.WriteLine("Processing Upstream Plugin: {0}", upstreamAttr.Name);
                var localVersion = Version.Parse(localManifest.Version.TrimStart('v', 'V'));
                var upstreamVersion = Version.Parse(upstreamAttr.Version.TrimStart('v', 'V'));
                if (localVersion < upstreamVersion)
                {
                    Console.WriteLine($"[Update Found] {localManifest.Name}: {localManifest.Version} -> {upstreamAttr.Version}");

                    var updatedManifest = upstreamAttr.ToManifest();
                    var updatedJson = JsonSerializer.Serialize(updatedManifest, options);

                    await File.WriteAllTextAsync(manifestPath, updatedJson);
                    hasUpdates = true;
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[Error] Failed scanning upstream updates for {localManifest.Name}: {ex.Message}");
            }
        }

        if (hasUpdates)
        {
            Console.WriteLine("[Syncer] Sync complete. Updates written. Eligible for Auto-Merge.");
            Environment.Exit(1);
        }
        else
        {
            Console.WriteLine("[Syncer] Sync complete. All plugin registrations are up to date.");
            Environment.Exit(0);
        }
    }
}