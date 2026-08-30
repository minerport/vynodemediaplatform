using System.Net.Http.Headers;
using System.Net.Http.Json;
using VyNode.Windows.Models;

namespace VyNode.Windows.Services;

public sealed class ConnectClient
{
    private readonly HttpClient _http;
    public ConnectClient()
    {
        var configured = Environment.GetEnvironmentVariable("VYNODE_CONNECT_URL") ?? "https://connect.vynodehub.com";
        _http = new HttpClient { BaseAddress = new Uri(configured.TrimEnd('/') + "/"), Timeout = TimeSpan.FromSeconds(15) };
    }

    public async Task<LoginResponse> LoginAsync(string username, string password, DeviceInput device, CancellationToken ct)
    {
        using var request = new HttpRequestMessage(HttpMethod.Post, "api/v1/account/login") { Content = JsonContent.Create(new { username, password, deviceName = device.Name, platform = device.Platform, clientName = device.ClientName, clientVersion = device.ClientVersion, platformVersion = device.PlatformVersion }) };
        request.Headers.Add("X-VyNode-Client", "native");
        using var response = await _http.SendAsync(request, ct);
        return await ReadAsync<LoginResponse>(response, ct);
    }

    public async Task<LoginResponse> RefreshAsync(string refreshToken, CancellationToken ct)
    {
        using var request = new HttpRequestMessage(HttpMethod.Post, "api/v1/account/refresh") { Content = JsonContent.Create(new { refreshToken }) };
        request.Headers.Add("X-VyNode-Client", "native");
        using var response = await _http.SendAsync(request, ct);
        return await ReadAsync<LoginResponse>(response, ct);
    }

    public async Task LogoutAsync(string accessToken, CancellationToken ct)
    {
        using var request = Authorized(HttpMethod.Post, "api/v1/account/logout", accessToken);
        using var response = await _http.SendAsync(request, ct);
        if (!response.IsSuccessStatusCode) throw new InvalidOperationException($"VyNode request failed ({(int)response.StatusCode}).");
    }

    public async Task<IReadOnlyList<LinkedServer>> ServersAsync(string accessToken, CancellationToken ct)
    {
        using var request = Authorized(HttpMethod.Get, "api/v1/servers", accessToken);
        using var response = await _http.SendAsync(request, ct);
        return await ReadAsync<List<LinkedServer>>(response, ct);
    }

    public async Task<string> AssertionAsync(string accessToken, string serverId, CancellationToken ct)
    {
        using var request = Authorized(HttpMethod.Post, $"api/v1/servers/{Uri.EscapeDataString(serverId)}/assertion", accessToken);
        using var response = await _http.SendAsync(request, ct);
        return (await ReadAsync<ServerAssertion>(response, ct)).Value;
    }

    private static HttpRequestMessage Authorized(HttpMethod method, string path, string token)
    {
        var request = new HttpRequestMessage(method, path);
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        request.Headers.Add("X-VyNode-Client", "native");
        return request;
    }

    private static async Task<T> ReadAsync<T>(HttpResponseMessage response, CancellationToken ct)
    {
        if (!response.IsSuccessStatusCode) throw new InvalidOperationException($"VyNode request failed ({(int)response.StatusCode}).");
        return await response.Content.ReadFromJsonAsync<T>(cancellationToken: ct) ?? throw new InvalidOperationException("VyNode returned an empty response.");
    }
}
