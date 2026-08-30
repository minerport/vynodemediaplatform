using System.Text.Json;
using VyNode.Windows.Models;

namespace VyNode.Windows.Services;

public sealed record CachedClientState(GlobalUser User, LinkedServer Server, IReadOnlyList<LinkedServer> Servers, string Endpoint, DeviceInput Device);
public sealed record CachedGlobalState(GlobalUser User, DeviceInput Device);

public sealed class LocalStateStore
{
    private static readonly string DirectoryPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "VyNode", "Desktop");
    private static readonly string FilePath = Path.Combine(DirectoryPath, "session.json");
    private static readonly string GlobalFilePath = Path.Combine(DirectoryPath, "global-session.json");
    private static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web) { WriteIndented = true };

    public async Task SaveAsync(SessionContext session, CancellationToken ct = default)
    {
        Directory.CreateDirectory(DirectoryPath);
        var state = new CachedClientState(session.User, session.Server, session.LinkedServers ?? [session.Server], session.Endpoint, session.Device ?? DeviceIdentity.Describe());
        await File.WriteAllTextAsync(FilePath, JsonSerializer.Serialize(state, Json), ct);
    }

    public async Task<CachedClientState?> ReadAsync(CancellationToken ct = default)
    {
        try { return JsonSerializer.Deserialize<CachedClientState>(await File.ReadAllTextAsync(FilePath, ct), Json); }
        catch (FileNotFoundException) { return null; }
        catch (JsonException) { return null; }
    }

    public async Task SaveGlobalAsync(GlobalUser user, DeviceInput device, CancellationToken ct = default)
    {
        Directory.CreateDirectory(DirectoryPath);
        await File.WriteAllTextAsync(GlobalFilePath, JsonSerializer.Serialize(new CachedGlobalState(user, device), Json), ct);
    }

    public async Task<CachedGlobalState?> ReadGlobalAsync(CancellationToken ct = default)
    {
        try { return JsonSerializer.Deserialize<CachedGlobalState>(await File.ReadAllTextAsync(GlobalFilePath, ct), Json); }
        catch (FileNotFoundException) { return null; }
        catch (JsonException) { return null; }
    }

    public void Clear()
    {
        if (File.Exists(FilePath)) File.Delete(FilePath);
        if (File.Exists(GlobalFilePath)) File.Delete(GlobalFilePath);
    }
}
