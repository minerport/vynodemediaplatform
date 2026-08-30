using System.Security.Cryptography;
using System.Net;
using System.Net.Http;
using System.Text;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using VyNode.Windows.Services;

namespace VyNode.Windows.Tests;

[TestClass]
public sealed class UpdateVerifierTests
{
    [TestMethod]
    public void SignedMetadataPassesAndTamperingFails()
    {
        using var signer = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var publicKey = Convert.ToBase64String(signer.ExportSubjectPublicKeyInfo());
        var body = Encoding.UTF8.GetBytes("{\"Channel\":\"stable\",\"Version\":\"15.1.0\",\"PackageUrl\":\"https://updates.vynode.app/windows/VyNode.msi\",\"Sha256\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\",\"MinimumClientVersion\":\"15.0.0\",\"PublishedAt\":\"2026-08-24T00:00:00Z\",\"SigningKeyId\":\"test-key\"}");
        var signature = signer.SignData(body, HashAlgorithmName.SHA256);
        var verifier = new UpdateVerifier(publicKey);
        Assert.AreEqual("15.1.0", verifier.Verify(body, signature).Version);
        body[20] ^= 1;
        Assert.ThrowsExactly<CryptographicException>(() => verifier.Verify(body, signature));
    }

    [TestMethod]
    public async Task PackageHashRejectsModifiedPayload()
    {
        var expected = Convert.ToHexString(SHA256.HashData("expected"u8));
        await Assert.ThrowsExactlyAsync<CryptographicException>(() => UpdateVerifier.VerifyPackageAsync(new MemoryStream("tampered"u8.ToArray()), expected, CancellationToken.None));
    }

    [TestMethod]
    public void MetadataFromWrongSigningKeyIsRejected()
    {
        using var trusted = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        using var untrusted = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var body = ValidManifest("15.1.0", "stable");
        var signature = untrusted.SignData(body, HashAlgorithmName.SHA256);
        var verifier = new UpdateVerifier(Convert.ToBase64String(trusted.ExportSubjectPublicKeyInfo()));
        Assert.ThrowsExactly<CryptographicException>(() => verifier.Verify(body, signature));
    }

    [TestMethod]
    public void StablePathRejectsDowngradeAndUnknownChannel()
    {
        Assert.IsFalse(UpdateVerifier.IsNewer("15.0.0", "14.9.9"));
        Assert.IsTrue(UpdateVerifier.IsNewer("15.0.0", "15.1.0"));
        using var signer = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var body = ValidManifest("15.1.0", "nightly");
        var verifier = new UpdateVerifier(Convert.ToBase64String(signer.ExportSubjectPublicKeyInfo()));
        Assert.ThrowsExactly<InvalidDataException>(() => verifier.Verify(body, signer.SignData(body, HashAlgorithmName.SHA256)));
    }

    [TestMethod]
    public async Task MatchingPackageHashIsAccepted()
    {
        var package = "valid package"u8.ToArray();
        var hash = Convert.ToHexString(SHA256.HashData(package));
        await UpdateVerifier.VerifyPackageAsync(new MemoryStream(package), hash, CancellationToken.None);
    }

    [TestMethod]
    public async Task OfflineAndSlowFeedAreNonBlocking()
    {
        using var signer = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var verifier = new UpdateVerifier(Convert.ToBase64String(signer.ExportSubjectPublicKeyInfo()));
        using var offlineHttp = new HttpClient(new DelegateHandler((_, _) => throw new HttpRequestException("offline")));
        var offline = new UpdateFeedClient(offlineHttp, verifier, TimeSpan.FromMilliseconds(50));
        Assert.IsNull(await offline.CheckAsync(new Uri("https://updates.vynode.app/manifest.json"), new Uri("https://updates.vynode.app/manifest.sig"), "15.0.0", "stable", CancellationToken.None));

        using var slowHttp = new HttpClient(new DelegateHandler(async (_, ct) => { await Task.Delay(TimeSpan.FromSeconds(5), ct); return new HttpResponseMessage(HttpStatusCode.OK); }));
        var slow = new UpdateFeedClient(slowHttp, verifier, TimeSpan.FromMilliseconds(30));
        var started = DateTime.UtcNow;
        Assert.IsNull(await slow.CheckAsync(new Uri("https://updates.vynode.app/manifest.json"), new Uri("https://updates.vynode.app/manifest.sig"), "15.0.0", "stable", CancellationToken.None));
        Assert.IsTrue(DateTime.UtcNow - started < TimeSpan.FromSeconds(1));
    }

    [TestMethod]
    public async Task SignedTestFeedRecognizesNewerRelease()
    {
        using var signer = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var metadata = ValidManifest("15.1.0", "stable");
        var signature = signer.SignData(metadata, HashAlgorithmName.SHA256);
        using var http = new HttpClient(new DelegateHandler((request, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new ByteArrayContent(request.RequestUri!.AbsolutePath.EndsWith(".sig", StringComparison.Ordinal) ? signature : metadata)
        })));
        var feed = new UpdateFeedClient(http, new UpdateVerifier(Convert.ToBase64String(signer.ExportSubjectPublicKeyInfo())), TimeSpan.FromSeconds(1));
        var result = await feed.CheckAsync(new Uri("https://updates.vynode.app/manifest.json"), new Uri("https://updates.vynode.app/manifest.sig"), "15.0.0", "stable", CancellationToken.None);
        Assert.AreEqual("15.1.0", result?.Version);
    }

    private static byte[] ValidManifest(string version, string channel) => Encoding.UTF8.GetBytes($"{{\"Channel\":\"{channel}\",\"Version\":\"{version}\",\"PackageUrl\":\"https://updates.vynode.app/windows/VyNode.msi\",\"Sha256\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\",\"MinimumClientVersion\":\"15.0.0\",\"PublishedAt\":\"2026-08-24T00:00:00Z\",\"SigningKeyId\":\"test-key\"}}");

    private sealed class DelegateHandler(Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> send) : HttpMessageHandler
    {
        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken) => send(request, cancellationToken);
    }
}
