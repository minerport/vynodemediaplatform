using System.Diagnostics;

namespace VyNode.Windows.Services;

public enum UpdateRuntimeState
{
    Unavailable,
    Checking,
    UpdateAvailable,
    Downloading,
    Verifying,
    ReadyToInstall,
    LaunchingInstaller,
    Error
}

public sealed record VerifiedUpdatePackage(UpdateManifest Manifest, string LocalPath);

public interface IInstallerLauncher
{
    void LaunchMsi(string absoluteMsiPath);
}

public sealed class WindowsMsiInstallerLauncher : IInstallerLauncher
{
    public static ProcessStartInfo CreateStartInfo(string absoluteMsiPath)
    {
        var start = new ProcessStartInfo
        {
            FileName = "msiexec.exe",
            UseShellExecute = true,
            Verb = "runas"
        };
        start.ArgumentList.Add("/i");
        start.ArgumentList.Add(absoluteMsiPath);
        return start;
    }

    public void LaunchMsi(string absoluteMsiPath) =>
        _ = Process.Start(CreateStartInfo(absoluteMsiPath)) ?? throw new InvalidOperationException("Windows Installer did not start.");
}

public sealed class UpdateInstallerHandoff(string managedPackageRoot, IInstallerLauncher launcher)
{
    private readonly string _managedRoot = Path.GetFullPath(managedPackageRoot);

    public async Task InstallAsync(VerifiedUpdatePackage package, CancellationToken ct)
    {
        var path = Path.GetFullPath(package.LocalPath);
        if (!Path.GetExtension(path).Equals(".msi", StringComparison.OrdinalIgnoreCase))
            throw new InvalidDataException("Only MSI update packages are supported.");
        var relative = Path.GetRelativePath(_managedRoot, path);
        if (Path.IsPathRooted(relative) || relative.Equals("..", StringComparison.Ordinal) ||
            relative.StartsWith($"..{Path.DirectorySeparatorChar}", StringComparison.Ordinal))
            throw new InvalidDataException("Update package is outside the managed update directory.");
        if (!File.Exists(path)) throw new FileNotFoundException("The verified update package is no longer available.", path);
        if ((File.GetAttributes(path) & FileAttributes.ReparsePoint) != 0)
            throw new InvalidDataException("Update package cannot be a reparse point.");

        await using (var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read))
            await UpdateVerifier.VerifyPackageAsync(stream, package.Manifest.Sha256, ct);

        launcher.LaunchMsi(path);
    }
}
