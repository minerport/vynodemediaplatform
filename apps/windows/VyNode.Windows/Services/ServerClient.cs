using System.Net.Http.Headers;
using System.Net.Http.Json;
using VyNode.Windows.Models;

namespace VyNode.Windows.Services;

public sealed class ServerClient
{
    public async Task<ServerSession> BootstrapAsync(string endpoint, string assertion, DeviceInput device, CancellationToken ct)
    {
        using var http = Client(endpoint);
        using var request = new HttpRequestMessage(HttpMethod.Post, "api/v1/connect/exchange") { Content = JsonContent.Create(new { assertion, device }) };
        request.Headers.Add("X-VyNode-Client", "native");
        using var response = await http.SendAsync(request, ct);
        return await ReadAsync<ServerSession>(response, ct);
    }

    public async Task<ServerIdentity> IdentityAsync(string endpoint, CancellationToken ct)
    {
        using var http = Client(endpoint);
        using var response = await http.GetAsync("api/v1/connection-info", ct);
        return await ReadAsync<ServerIdentity>(response, ct);
    }

    public Task<HomeResponse> HomeAsync(string endpoint, string token, CancellationToken ct) => GetAsync<HomeResponse>(endpoint, token, "api/v1/home", ct);
    public Task<MovieList> MoviesAsync(string endpoint, string token, CancellationToken ct) => GetAsync<MovieList>(endpoint, token, "api/v1/movies?limit=200", ct);
    public Task<ShowList> ShowsAsync(string endpoint, string token, CancellationToken ct) => GetAsync<ShowList>(endpoint, token, "api/v1/shows?limit=200", ct);
    public Task<SearchResults> SearchAsync(string endpoint, string token, string query, CancellationToken ct) => GetAsync<SearchResults>(endpoint, token, $"api/v1/search?q={Uri.EscapeDataString(query)}", ct);
    public Task<Movie> MovieAsync(string endpoint, string token, string id, CancellationToken ct) => GetAsync<Movie>(endpoint, token, $"api/v1/movies/{Uri.EscapeDataString(id)}", ct);
    public Task<Show> ShowAsync(string endpoint, string token, string id, CancellationToken ct) => GetAsync<Show>(endpoint, token, $"api/v1/shows/{Uri.EscapeDataString(id)}", ct);
    public async Task<IReadOnlyList<Artwork>> MovieArtworkAsync(string endpoint, string token, string id, CancellationToken ct) => (await GetAsync<ArtworkList>(endpoint, token, $"api/v1/movies/{Uri.EscapeDataString(id)}/artwork", ct)).Artwork ?? [];
    public async Task<IReadOnlyList<Artwork>> ShowArtworkAsync(string endpoint, string token, string id, CancellationToken ct) => (await GetAsync<ArtworkList>(endpoint, token, $"api/v1/shows/{Uri.EscapeDataString(id)}/artwork", ct)).Artwork ?? [];
    public Task<Progress> ProgressAsync(string endpoint, string token, string kind, string id, CancellationToken ct) => GetAsync<Progress>(endpoint, token, $"api/v1/playback/{kind.ToUpperInvariant()}/{Uri.EscapeDataString(id)}/progress", ct);

    public async Task<byte[]> ArtworkContentAsync(string endpoint, string token, string artworkId, CancellationToken ct)
    {
        using var http = Client(endpoint);
        using var request = Authorized(HttpMethod.Get, $"api/v1/artwork/{Uri.EscapeDataString(artworkId)}/content", token);
        using var response = await http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, ct);
        if (!response.IsSuccessStatusCode) throw new InvalidOperationException("Artwork is unavailable.");
        return await response.Content.ReadAsByteArrayAsync(ct);
    }

    public async Task<ServerSession> RefreshAsync(string endpoint, string refreshToken, CancellationToken ct)
    {
        using var http = Client(endpoint);
        using var request = new HttpRequestMessage(HttpMethod.Post, "api/v1/auth/refresh") { Content = JsonContent.Create(new { refreshToken }) };
        request.Headers.Add("X-VyNode-Client", "native");
        using var response = await http.SendAsync(request, ct);
        return await ReadAsync<ServerSession>(response, ct);
    }

    public async Task<(ServerSession Session, GlobalUser User)> LoginAsync(string endpoint, string username, string password, DeviceInput device, CancellationToken ct)
    {
        using var http = Client(endpoint);
        using var request = new HttpRequestMessage(HttpMethod.Post, "api/v1/auth/login") { Content = JsonContent.Create(new { username, password, device }) };
        request.Headers.Add("X-VyNode-Client", "native");
        using var response = await http.SendAsync(request, ct);
        var login = await ReadAsync<LocalLoginResponse>(response, ct);
        return (new ServerSession(login.AccessToken, login.RefreshToken), login.User);
    }

    private sealed record LocalLoginResponse(string AccessToken, string RefreshToken, GlobalUser User);

    private async Task<T> GetAsync<T>(string endpoint, string token, string path, CancellationToken ct)
    {
        using var http = Client(endpoint);
        using var request = Authorized(HttpMethod.Get, path, token);
        using var response = await http.SendAsync(request, ct);
        return await ReadAsync<T>(response, ct);
    }

    private static HttpRequestMessage Authorized(HttpMethod method, string path, string token)
    {
        var request = new HttpRequestMessage(method, path);
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        return request;
    }

    private static HttpClient Client(string endpoint) => new() { BaseAddress = new Uri(endpoint.TrimEnd('/') + "/"), Timeout = TimeSpan.FromSeconds(12) };
    private static async Task<T> ReadAsync<T>(HttpResponseMessage response, CancellationToken ct)
    {
        if (!response.IsSuccessStatusCode) throw new InvalidOperationException($"Server request failed ({(int)response.StatusCode}).");
        return await response.Content.ReadFromJsonAsync<T>(cancellationToken: ct) ?? throw new InvalidOperationException("Server returned an empty response.");
    }
}
