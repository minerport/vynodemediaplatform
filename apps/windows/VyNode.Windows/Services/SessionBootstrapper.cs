using VyNode.Windows.Auth;
using VyNode.Windows.Models;

namespace VyNode.Windows.Services;

public sealed class SessionBootstrapper
{
    private readonly ConnectClient _connect = new();
    private readonly ServerClient _server = new();
    private readonly SecureCredentialStore _credentials = new();

    public async Task<SessionContext> ConnectAsync(GlobalContext global, LinkedServer selected, CancellationToken ct)
    {
        Exception? last = null;
        foreach (var endpoint in selected.Endpoints.OrderBy(EndpointPriority))
        {
            try
            {
                var identity = await _server.IdentityAsync(endpoint.Url, ct);
                if (!string.Equals(identity.InstanceId, selected.Id, StringComparison.Ordinal))
                    throw new InvalidOperationException("The endpoint returned a different server identity. No credential was sent.");
                var assertion = await _connect.AssertionAsync(global.Login.AccessToken, selected.Id, ct);
                var local = await _server.BootstrapAsync(endpoint.Url, assertion, global.Device, ct);
                _credentials.SaveServer(global.Login.User.Id, selected.Id, local.RefreshToken);
                return new SessionContext(global.Login.User, selected, endpoint.Url, local.AccessToken, global.Login.AccessToken, global.Servers, global.Device, local.User?.Role);
            }
            catch (Exception ex) { last = ex; }
        }
        throw new InvalidOperationException($"{selected.Name} is not currently reachable.", last);
    }

    private static int EndpointPriority(ServerEndpoint endpoint)
    {
        if (Uri.TryCreate(endpoint.Url, UriKind.Absolute, out var uri) && (uri.IsLoopback || uri.Host.Equals("localhost", StringComparison.OrdinalIgnoreCase))) return 0;
        return endpoint.Kind.Equals("local", StringComparison.OrdinalIgnoreCase) ? 1 : endpoint.Kind.Equals("secure", StringComparison.OrdinalIgnoreCase) ? 2 : 3;
    }
}
