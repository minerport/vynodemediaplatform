using System.Net;
using System.Security.Cryptography;
using System.Text;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using VyNode.Windows.Services;

namespace VyNode.Windows.Tests;

[TestClass]
public sealed class UpdateRuntimeTests
{
    [TestMethod]
    public void MissingReleaseKeyFailsClosedAndDevelopmentOverrideIsExplicit()
    {
        var assembly = typeof(UpdateTrustConfiguration).Assembly;
        Assert.IsNull(UpdateTrustConfiguration.Load(assembly));

        using var signer = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var values = new Dictionary<string, string>
        {
            ["VYNODE_UPDATE_TEST_KEY_ID"] = "acceptance-2026",
            ["VYNODE_UPDATE_TEST_PUBLIC_KEY_SPKI"] = Convert.ToBase64String(signer.ExportSubjectPublicKeyInfo()),
            ["VYNODE_UPDATE_TEST_METADATA_URL"] = "https://127.0.0.1/manifest.json",
            ["VYNODE_UPDATE_TEST_SIGNATURE_URL"] = "https://127.0.0.1/manifest.sig"
        };
        string? Environment(string name) => values.GetValueOrDefault(name);
        Assert.IsNull(UpdateTrustConfiguration.Load(assembly, false, Environment));
        var configured = UpdateTrustConfiguration.Load(assembly, true, Environment);
        Assert.IsNotNull(configured);
        Assert.AreEqual("acceptance-2026", configured.KeyId);
        Assert.AreEqual(64, configured.Fingerprint.Length);
    }

    [TestMethod]
    public void InvalidConfiguredPublicKeyFailsClosed()
    {
        var values = new Dictionary<string, string>
        {
            ["VYNODE_UPDATE_TEST_KEY_ID"] = "invalid",
            ["VYNODE_UPDATE_TEST_PUBLIC_KEY_SPKI"] = "not-a-key",
            ["VYNODE_UPDATE_TEST_METADATA_URL"] = "https://127.0.0.1/manifest.json",
            ["VYNODE_UPDATE_TEST_SIGNATURE_URL"] = "https://127.0.0.1/manifest.sig"
        };
        Assert.IsNull(UpdateTrustConfiguration.Load(typeof(UpdateTrustConfiguration).Assembly, true, values.GetValueOrDefault));
    }

    [TestMethod]
    public async Task VerifiedMsiLaunchIntentUsesExactManagedPath()
    {
        await WithTempDirectory(async root =>
        {
            var package = Path.Combine(root, "VyNode.msi");
            await File.WriteAllTextAsync(package, "verified");
            var launcher = new RecordingLauncher();
            await new UpdateInstallerHandoff(root, launcher).InstallAsync(Verified(package), CancellationToken.None);
            Assert.AreEqual(Path.GetFullPath(package), launcher.LaunchedPath);
        });
    }

    [TestMethod]
    public void WindowsInstallerIntentUsesStructuredMsiexecArguments()
    {
        var path = @"C:\Managed Updates\VyNode Desktop.msi";
        var start = WindowsMsiInstallerLauncher.CreateStartInfo(path);
        Assert.AreEqual("msiexec.exe", start.FileName);
        Assert.AreEqual("runas", start.Verb);
        Assert.IsTrue(start.UseShellExecute);
        CollectionAssert.AreEqual(new[] { "/i", path }, start.ArgumentList.ToArray());
        Assert.IsFalse(start.ArgumentList.Any(x => x.Contains("cmd.exe", StringComparison.OrdinalIgnoreCase)));
    }

    [TestMethod]
    public async Task TamperAfterVerificationIsDenied()
    {
        await WithTempDirectory(async root =>
        {
            var package = Path.Combine(root, "VyNode.msi");
            await File.WriteAllTextAsync(package, "verified");
            var verified = Verified(package);
            await File.WriteAllTextAsync(package, "changed");
            var launcher = new RecordingLauncher();
            await Assert.ThrowsExactlyAsync<CryptographicException>(() => new UpdateInstallerHandoff(root, launcher).InstallAsync(verified, CancellationToken.None));
            Assert.IsNull(launcher.LaunchedPath);
        });
    }

    [TestMethod]
    public async Task MissingWrongPathAndWrongTypeNeverLaunch()
    {
        await WithTempDirectory(async root =>
        {
            var launcher = new RecordingLauncher();
            var handoff = new UpdateInstallerHandoff(root, launcher);
            await Assert.ThrowsExactlyAsync<FileNotFoundException>(() => handoff.InstallAsync(Verified(Path.Combine(root, "missing.msi")), CancellationToken.None));

            var outside = Path.Combine(Path.GetDirectoryName(root)!, "outside.msi");
            await File.WriteAllTextAsync(outside, "verified");
            try
            {
                await Assert.ThrowsExactlyAsync<InvalidDataException>(() => handoff.InstallAsync(Verified(outside), CancellationToken.None));
            }
            finally { File.Delete(outside); }

            var script = Path.Combine(root, "VyNode.ps1");
            await File.WriteAllTextAsync(script, "verified");
            await Assert.ThrowsExactlyAsync<InvalidDataException>(() => handoff.InstallAsync(Verified(script), CancellationToken.None));
            Assert.IsNull(launcher.LaunchedPath);
        });
    }

    [TestMethod]
    public async Task DownloadVerifyReadyAndPreLaunchRehashAreEnforced()
    {
        await WithTempDirectory(async root =>
        {
            using var signer = ECDsa.Create(ECCurve.NamedCurves.nistP256);
            var packageBytes = Encoding.UTF8.GetBytes("acceptance msi");
            var hash = Convert.ToHexString(SHA256.HashData(packageBytes));
            var metadata = Encoding.UTF8.GetBytes($"{{\"Channel\":\"stable\",\"Version\":\"15.0.1\",\"PackageUrl\":\"https://updates.test/VyNode.msi\",\"Sha256\":\"{hash}\",\"MinimumClientVersion\":\"15.0.0\",\"PublishedAt\":\"2026-08-24T00:00:00Z\"}}");
            var signature = signer.SignData(metadata, HashAlgorithmName.SHA256);
            using var http = new HttpClient(new FixtureHandler(request =>
                request.RequestUri!.AbsolutePath.EndsWith(".sig", StringComparison.Ordinal) ? signature :
                request.RequestUri.AbsolutePath.EndsWith(".json", StringComparison.Ordinal) ? metadata : packageBytes));
            var trust = new UpdateTrustConfiguration("test", Convert.ToBase64String(signer.ExportSubjectPublicKeyInfo()), new Uri("https://updates.test/manifest.json"), new Uri("https://updates.test/manifest.sig"), "stable");
            var launcher = new RecordingLauncher();
            var service = new UpdateApplicationService(http, trust, root, launcher);
            var ready = await service.CheckAndDownloadAsync("15.0.0", CancellationToken.None);
            Assert.IsNotNull(ready);
            Assert.AreEqual(UpdateRuntimeState.ReadyToInstall, service.State);

            await File.AppendAllTextAsync(ready.LocalPath, "tampered");
            Assert.IsFalse(await service.InstallReadyUpdateAsync(CancellationToken.None));
            Assert.AreEqual(UpdateRuntimeState.Error, service.State);
            Assert.IsNull(launcher.LaunchedPath);
        });
    }

    private static VerifiedUpdatePackage Verified(string path)
    {
        var bytes = File.Exists(path) ? File.ReadAllBytes(path) : "missing"u8.ToArray();
        var hash = Convert.ToHexString(SHA256.HashData(bytes));
        return new VerifiedUpdatePackage(new UpdateManifest("stable", "15.0.1", "https://updates.test/VyNode.msi", hash, "15.0.0", "2026-08-24T00:00:00Z"), path);
    }

    private static async Task WithTempDirectory(Func<string, Task> test)
    {
        var root = Path.Combine(Path.GetTempPath(), $"vynode-update-{Guid.NewGuid():N}");
        Directory.CreateDirectory(root);
        try { await test(root); }
        finally { Directory.Delete(root, true); }
    }

    private sealed class RecordingLauncher : IInstallerLauncher
    {
        public string? LaunchedPath { get; private set; }
        public void LaunchMsi(string absoluteMsiPath) => LaunchedPath = absoluteMsiPath;
    }

    private sealed class FixtureHandler(Func<HttpRequestMessage, byte[]> content) : HttpMessageHandler
    {
        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken) =>
            Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new ByteArrayContent(content(request)) });
    }
}
