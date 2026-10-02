using System.Reflection;
using System.Runtime.Loader;
using System.Text.Json;
using Octokit;
using YStreamUtils.SDK.Plugin;
using System.IO.Compression;

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
                var latestRelease =
                    await client.Repository.Release.GetLatest(localManifest.Source.Owner,
                        localManifest.Source.Repository);

                var expectedZipName = $"{localManifest.Name}.zip";
                var zipAsset = latestRelease.Assets.FirstOrDefault(a =>
                    string.Equals(a.Name, expectedZipName, StringComparison.OrdinalIgnoreCase));

                if (zipAsset == null) continue;
                Console.WriteLine("Processing Plugin Manifest from ZIP: {0}", localManifest.Name);

                var zipBytes = await HttpClient.GetByteArrayAsync(zipAsset.BrowserDownloadUrl);

                string? upstreamName = null;
                string? upstreamVersion = null;

                using (var zipStream = new MemoryStream(zipBytes))
                await using (var archive = new ZipArchive(zipStream, ZipArchiveMode.Read))
                {
                    var expectedDllName = $"{localManifest.Name}.dll";
                    var dllEntry = archive.Entries.FirstOrDefault(e =>
                        string.Equals(e.Name, expectedDllName, StringComparison.OrdinalIgnoreCase));

                    if (dllEntry == null)
                    {
                        Console.WriteLine($"[Warning] Found zip asset, but it does not contain {expectedDllName}");
                        continue;
                    }

                    var assemblyByteMap = new Dictionary<string, byte[]>(StringComparer.OrdinalIgnoreCase);
                    foreach (var entry in archive.Entries)
                    {
                        
                        if (entry.Name.Contains("YStreamUtils-PluginSDK", StringComparison.OrdinalIgnoreCase))
                        {
                            continue;
                        }
                        
                        if (!entry.Name.EndsWith(".dll", StringComparison.OrdinalIgnoreCase)) continue;
                        await using var entryStream = await entry.OpenAsync();
                        using var ms = new MemoryStream();
                        await entryStream.CopyToAsync(ms);
                        assemblyByteMap[entry.Name] = ms.ToArray();
                    }

                    if (!assemblyByteMap.TryGetValue(expectedDllName, out var targetDllBytes)) continue;
                    using var seekableDllStream = new MemoryStream(targetDllBytes);

                    var loadContext = new AssemblyLoadContext("SyncValidationContext", isCollectible: true);

                    Func<AssemblyLoadContext, AssemblyName, Assembly?> resolver = (context, assemblyName) =>
                    {
                        try
                        {
                            return AssemblyLoadContext.Default.LoadFromAssemblyName(assemblyName);
                        }
                        catch
                        {
                            // ignored
                        }

                        var targetDllKey = $"{assemblyName.Name}.dll";
                        return assemblyByteMap.TryGetValue(targetDllKey, out var dependencyBytes)
                            ? context.LoadFromStream(new MemoryStream(dependencyBytes))
                            : null;
                    };

                    loadContext.Resolving += resolver;

                    try
                    {
                        var assembly = loadContext.LoadFromStream(seekableDllStream);

                        var upstreamAttr = assembly.GetCustomAttribute<PluginManifestAttribute>();

                        if (upstreamAttr != null)
                        {
                            var manifest = upstreamAttr.ToManifest();
                            upstreamName = manifest.Name;
                            upstreamVersion = manifest.Version;
                        }

                        if (string.IsNullOrEmpty(upstreamName) || string.IsNullOrEmpty(upstreamVersion))
                        {
                            Console.WriteLine(
                                $"[Warning] Could not extract PluginManifestAttribute from {localManifest.Name}");
                            continue;
                        }

                        Console.WriteLine("Processing Upstream Plugin: {0}", upstreamName);
                        var localVer = Version.Parse(localManifest.Version.TrimStart('v', 'V'));
                        var upstreamVer = Version.Parse(upstreamVersion.TrimStart('v', 'V'));

                        if (localVer < upstreamVer)
                        {
                            Console.WriteLine(
                                $"[Update Found] {localManifest.Name}: {localManifest.Version} -> {upstreamVersion}");

                            var updatedJson = JsonSerializer.Serialize(upstreamVersion, options);
                            await File.WriteAllTextAsync(manifestPath, updatedJson);
                            hasUpdates = true;
                        }
                    }
                    finally
                    {
                        loadContext.Resolving -= resolver;
                        loadContext.Unload();
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