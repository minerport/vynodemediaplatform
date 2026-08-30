using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace VyNode.Windows.Services;

public sealed record UpdateManifest(string Channel, string Version, string PackageUrl, string Sha256, string MinimumClientVersion, string PublishedAt, string SigningKeyId);

public sealed class UpdateVerifier
{
    private readonly byte[] _publicKeySpki;
    public UpdateVerifier(string publicKeySpkiBase64) => _publicKeySpki = Convert.FromBase64String(publicKeySpkiBase64);

    public UpdateManifest Verify(ReadOnlySpan<byte> canonicalMetadata, ReadOnlySpan<byte> signature)
    {
        using var key = ECDsa.Create();
        key.ImportSubjectPublicKeyInfo(_publicKeySpki, out _);
        if (!key.VerifyData(canonicalMetadata, signature, HashAlgorithmName.SHA256))
            throw new CryptographicException("Update metadata signature is invalid.");
        var manifest = JsonSerializer.Deserialize<UpdateManifest>(canonicalMetadata)
            ?? throw new InvalidDataException("Update metadata is empty.");
        if (manifest.Channel is not ("stable" or "beta")) throw new InvalidDataException("Update channel is invalid.");
        if (!Version.TryParse(manifest.Version, out _)) throw new InvalidDataException("Update version is invalid.");
        if (!Uri.TryCreate(manifest.PackageUrl, UriKind.Absolute, out var uri) || uri.Scheme != Uri.UriSchemeHttps)
            throw new InvalidDataException("Update package URL must use HTTPS.");
        if (manifest.Sha256.Length != 64 || !manifest.Sha256.All(Uri.IsHexDigit)) throw new InvalidDataException("Update package hash is invalid.");
        if (string.IsNullOrWhiteSpace(manifest.SigningKeyId) || manifest.SigningKeyId.Length > 128)
            throw new InvalidDataException("Update signing key ID is invalid.");
        return manifest;
    }

    public static async Task VerifyPackageAsync(Stream package, string expectedSha256, CancellationToken ct)
    {
        var actual = Convert.ToHexString(await SHA256.HashDataAsync(package, ct));
        if (!CryptographicOperations.FixedTimeEquals(Encoding.ASCII.GetBytes(actual), Encoding.ASCII.GetBytes(expectedSha256.ToUpperInvariant())))
            throw new CryptographicException("Update package hash does not match signed metadata.");
    }

    public static bool IsNewer(string current, string candidate) => Version.TryParse(current, out var a) && Version.TryParse(candidate, out var b) && b > a;
}

public sealed class UpdateFeedClient(HttpClient http, UpdateVerifier verifier, TimeSpan timeout)
{
    public async Task<UpdateManifest?> CheckAsync(Uri metadataUrl, Uri signatureUrl, string currentVersion, string channel, CancellationToken ct)
    {
        using var bounded = CancellationTokenSource.CreateLinkedTokenSource(ct);
        bounded.CancelAfter(timeout);
        try
        {
            var metadata = await http.GetByteArrayAsync(metadataUrl, bounded.Token);
            var signature = await http.GetByteArrayAsync(signatureUrl, bounded.Token);
            var manifest = verifier.Verify(metadata, signature);
            if (!manifest.Channel.Equals(channel, StringComparison.OrdinalIgnoreCase)) return null;
            return UpdateVerifier.IsNewer(currentVersion, manifest.Version) ? manifest : null;
        }
        catch (HttpRequestException) { return null; }
        catch (TaskCanceledException) when (!ct.IsCancellationRequested) { return null; }
    }
}
