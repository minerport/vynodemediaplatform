using System.Reflection;
using System.Security.Cryptography;

namespace VyNode.Windows.Services;

public sealed record UpdateTrustConfiguration(
    string KeyId,
    string PublicKeySpkiBase64,
    Uri MetadataUri,
    Uri SignatureUri,
    string Channel)
{
    public string Fingerprint
    {
        get
        {
            var key = Convert.FromBase64String(PublicKeySpkiBase64);
            return Convert.ToHexString(SHA256.HashData(key));
        }
    }

    public UpdateVerifier CreateVerifier() => new(PublicKeySpkiBase64);

    public static UpdateTrustConfiguration? Load(
        Assembly assembly,
        bool allowDevelopmentOverride = false,
        Func<string, string?>? environment = null)
    {
        var metadata = assembly.GetCustomAttributes<AssemblyMetadataAttribute>()
            .Where(x => x.Value is not null)
            .ToDictionary(x => x.Key, x => x.Value!, StringComparer.Ordinal);
        metadata.TryGetValue("VyNode.UpdatePublicKeyId", out var keyId);
        metadata.TryGetValue("VyNode.UpdatePublicKeySpki", out var key);
        metadata.TryGetValue("VyNode.UpdateMetadataUrl", out var metadataUrl);
        metadata.TryGetValue("VyNode.UpdateSignatureUrl", out var signatureUrl);

        if (allowDevelopmentOverride)
        {
            environment ??= Environment.GetEnvironmentVariable;
            keyId = environment("VYNODE_UPDATE_TEST_KEY_ID") ?? keyId;
            key = environment("VYNODE_UPDATE_TEST_PUBLIC_KEY_SPKI") ?? key;
            metadataUrl = environment("VYNODE_UPDATE_TEST_METADATA_URL") ?? metadataUrl;
            signatureUrl = environment("VYNODE_UPDATE_TEST_SIGNATURE_URL") ?? signatureUrl;
        }

        if (string.IsNullOrWhiteSpace(keyId) || string.IsNullOrWhiteSpace(key)) return null;
        if (!Uri.TryCreate(metadataUrl, UriKind.Absolute, out var metadataUri) || metadataUri.Scheme != Uri.UriSchemeHttps) return null;
        if (!Uri.TryCreate(signatureUrl, UriKind.Absolute, out var signatureUri) || signatureUri.Scheme != Uri.UriSchemeHttps) return null;
        try
        {
            using var verifier = ECDsa.Create();
            verifier.ImportSubjectPublicKeyInfo(Convert.FromBase64String(key), out _);
        }
        catch (Exception ex) when (ex is FormatException or CryptographicException)
        {
            return null;
        }
        return new UpdateTrustConfiguration(keyId, key, metadataUri, signatureUri, "stable");
    }
}
