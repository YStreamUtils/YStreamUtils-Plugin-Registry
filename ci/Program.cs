using ci;
using Octokit;

Console.WriteLine("=== Scanning Registry Pipeline ===");

if (args.Length == 0)
{
    Console.WriteLine("Error: Missing execution flag. Use --verify, --sync, or --publish.");
    Environment.Exit(1);
}

var command = args[0];

var githubToken = Environment.GetEnvironmentVariable("GITHUB_TOKEN");
GitHubClient client;

if (!string.IsNullOrWhiteSpace(githubToken))
{
    client = new GitHubClient(new ProductHeaderValue("YStreamUtils-Registry-CI"))
    {
        Credentials = new Credentials(githubToken)
    };
    Console.WriteLine("[Linter] Initializing authenticated GitHub API client context.");
}
else
{
    client = new GitHubClient(new ProductHeaderValue("YStreamUtils-Registry-CI"));
    Console.WriteLine("[Linter] Initializing unauthenticated GitHub API client context.");
}

switch (command)
{
    case "--verify":
        await Verify.ExecuteAsync(client);
        break;

    case "--sync":
        await Sync.ExecuteAsync(client);
        break;

    case "--publish":
        Publish.Execute();
        break;

    default:
        Console.WriteLine($"Unknown action flag: {command}");
        Environment.Exit(1);
        break;
}