using Windows.Security.Credentials;

namespace VyNode.Windows.Auth;

public sealed class SecureCredentialStore
{
    private const string ResourcePrefix = "VyNode.Media";
    private readonly PasswordVault _vault = new();

    public void SaveGlobal(string accountId, string refreshToken) => Save("connect", accountId, refreshToken);
    public void SaveServer(string accountId, string serverId, string refreshToken) => Save($"server:{serverId}", accountId, refreshToken);

    public string? ReadGlobal(string accountId) => Read("connect", accountId);
    public string? ReadServer(string accountId, string serverId) => Read($"server:{serverId}", accountId);

    public void ClearAccount(string accountId)
    {
        var userName = VaultUser(accountId);
        try
        {
            foreach (var credential in _vault.RetrieveAll().Where(x => x.Resource.StartsWith(ResourcePrefix, StringComparison.Ordinal) && x.UserName == userName).ToArray())
                _vault.Remove(credential);
        }
        catch { }
    }

    private void Save(string scope, string accountId, string secret)
    {
        var resource = $"{ResourcePrefix}:{scope}";
        var userName = VaultUser(accountId);
        try
        {
            var existing = _vault.Retrieve(resource, userName);
            _vault.Remove(existing);
        }
        catch { }
        _vault.Add(new PasswordCredential(resource, userName, secret));
    }

    private string? Read(string scope, string accountId)
    {
        try { var value = _vault.Retrieve($"{ResourcePrefix}:{scope}", VaultUser(accountId)); value.RetrievePassword(); return value.Password; }
        catch { return null; }
    }

    private static string VaultUser(string accountId) => accountId.StartsWith("vynode:", StringComparison.Ordinal) ? accountId : $"vynode:{accountId}";
}
