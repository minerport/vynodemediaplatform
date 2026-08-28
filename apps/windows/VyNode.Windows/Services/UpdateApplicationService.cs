using System.Reflection;
using System.Security.Cryptography;
using System.ComponentModel;

namespace VyNode.Windows.Services;

public sealed class UpdateApplicationService
{
    private readonly HttpClient _http;
    private readonly UpdateTrustConfiguration? _trust;
    private readonly string _packageRoot;
    private readonly IInstallerLauncher _launcher;

    public UpdateRuntimeState State { get; private set; }
    public VerifiedUpdatePackage? ReadyPackage { get; private set; }
    public string? SafeMessage { get; private set; }
    public UpdateTrustConfiguration? Trust => _trust;

    public UpdateApplicationService(HttpClient http, UpdateTrustConfiguration? trust, string packageRoot, IInstallerLauncher launcher)
    {
        _http = http;
        _trust = trust;
        _packageRoot = Path.GetFullPath(packageRoot);
        _launcher = launcher;
        State = trust is null ? UpdateRuntimeState.Unavailable : UpdateRuntimeState.UpdateAvailable;
        SafeMessage = trust is null ? "Updates are unavailable in this build. VyNode remains fully usable." : null;
    }

    public static UpdateApplicationService CreateDefault()
    {
        var allowDevelopmentOverride = false;
#if DEBUG
        allowDevelopmentOverride = true;
#endif
        var trust = UpdateTrustConfiguration.Load(Assembly.GetExecutingAssembly(), allowDevelopmentOverride);
        var root = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "VyNode", "Desktop", "Updates");
        return new UpdateApplicationService(new HttpClient(), trust, root, new WindowsMsiInstallerLauncher());
    }

    public async Task<VerifiedUpdatePackage?> CheckAndDownloadAsync(string currentVersion, CancellationToken ct)
    {
        ReadyPackage = null;
        if (_trust is null)
        {
            State = UpdateRuntimeState.Unavailable;
            SafeMessage = "Updates are unavailable in this build. VyNode remains fully usable.";
            return null;
        }
        try
        {
            State = UpdateRuntimeState.Checking;
            var feed = new UpdateFeedClient(_http, _trust.CreateVerifier(), TimeSpan.FromSeconds(8));
            var manifest = await feed.CheckAsync(_trust.MetadataUri, _trust.SignatureUri, currentVersion, _trust.Channel, ct);
            if (manifest is null)
            {
                State = UpdateRuntimeState.UpdateAvailable;
                SafeMessage = "VyNode is up to date, or the update service is temporarily unavailable.";
                return null;
            }
            State = UpdateRuntimeState.Downloading;
            Directory.CreateDirectory(_packageRoot);
            var version = Version.Parse(manifest.Version);
            var target = Path.Combine(_packageRoot, $"VyNode-Desktop-{version}.msi");
            var temporary = target + ".download";
            await using (var source = await _http.GetStreamAsync(manifest.PackageUrl, ct))
            await using (var destination = new FileStream(temporary, FileMode.Create, FileAccess.Write, FileShare.None))
                await source.CopyToAsync(destination, ct);
            State = UpdateRuntimeState.Verifying;
            await using (var stream = new FileStream(temporary, FileMode.Open, FileAccess.Read, FileShare.Read))
                await UpdateVerifier.VerifyPackageAsync(stream, manifest.Sha256, ct);
            File.Move(temporary, target, true);
            ReadyPackage = new VerifiedUpdatePackage(manifest, target);
            State = UpdateRuntimeState.ReadyToInstall;
            SafeMessage = $"VyNode {manifest.Version} is ready to install.";
            return ReadyPackage;
        }
        catch (Exception ex) when (ex is HttpRequestException or IOException or CryptographicException or InvalidDataException or FormatException)
        {
            State = UpdateRuntimeState.Error;
            SafeMessage = "VyNode could not prepare this update. The current app remains available.";
            return null;
        }
    }

    public async Task<bool> InstallReadyUpdateAsync(CancellationToken ct)
    {
        if (State != UpdateRuntimeState.ReadyToInstall || ReadyPackage is null)
        {
            State = UpdateRuntimeState.Error;
            SafeMessage = "No verified update is ready to install.";
            return false;
        }
        try
        {
            State = UpdateRuntimeState.LaunchingInstaller;
            await new UpdateInstallerHandoff(_packageRoot, _launcher).InstallAsync(ReadyPackage, ct);
            return true;
        }
        catch (Exception ex) when (ex is IOException or CryptographicException or InvalidDataException or UnauthorizedAccessException or InvalidOperationException or Win32Exception)
        {
            State = UpdateRuntimeState.Error;
            SafeMessage = "The update package changed or is unavailable. Download it again before installing.";
            return false;
        }
    }
}
