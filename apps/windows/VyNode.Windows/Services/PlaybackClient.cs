using System.Net.Http.Headers;
using System.Net.Http.Json;
using VyNode.Windows.Models;

namespace VyNode.Windows.Services;

public sealed class PlaybackClient
{
    public async Task<PlaybackSession> StartAsync(SessionContext context, PlaybackStartRequest input, CancellationToken ct)
    {
        using var http = Client(context.Endpoint);
        using var request = Authorized(HttpMethod.Post, "api/v1/playback/sessions", context.ServerAccessToken);
        request.Headers.Add("X-VyNode-Client", "native");
        request.Content = JsonContent.Create(input);
        using var response = await http.SendAsync(request, ct);
        return Absolutize(await ReadAsync<PlaybackSession>(response, ct), context.Endpoint);
    }

    public async Task UpdateAsync(SessionContext context, string sessionId, PlaybackProgressUpdate input, CancellationToken ct)
    {
        using var http = Client(context.Endpoint);
        using var request = Authorized(HttpMethod.Patch, $"api/v1/playback/sessions/{Uri.EscapeDataString(sessionId)}", context.ServerAccessToken);
        request.Content = JsonContent.Create(input);
        using var response = await http.SendAsync(request, ct);
        await EnsureAsync(response, ct);
    }

    public async Task StopAsync(SessionContext context, string sessionId, CancellationToken ct)
    {
        using var http = Client(context.Endpoint);
        using var response = await http.SendAsync(Authorized(HttpMethod.Delete, $"api/v1/playback/sessions/{Uri.EscapeDataString(sessionId)}", context.ServerAccessToken), ct);
        await EnsureAsync(response, ct);
    }

    public async Task StartOverAsync(SessionContext context, string kind, string id, CancellationToken ct)
    {
        using var http = Client(context.Endpoint);
        using var response = await http.SendAsync(Authorized(HttpMethod.Post, $"api/v1/playback/{kind.ToUpperInvariant()}/{Uri.EscapeDataString(id)}/start-over", context.ServerAccessToken), ct);
        await EnsureAsync(response, ct);
    }

    public async Task<string> GetSubtitleAsync(SessionContext context, string url, CancellationToken ct)
    {
        using var http = Client(context.Endpoint);
        using var request = Authorized(HttpMethod.Get, url, context.ServerAccessToken);
        using var response = await http.SendAsync(request, ct);
        if (!response.IsSuccessStatusCode) throw new InvalidOperationException($"Subtitle request failed ({(int)response.StatusCode}).");
        return await response.Content.ReadAsStringAsync(ct);
    }

    private static PlaybackSession Absolutize(PlaybackSession session, string endpoint) => session with
    {
        MediaUrl = Absolute(endpoint, session.MediaUrl),
        HlsUrl = Absolute(endpoint, session.HlsUrl),
        SubtitleUrl = Absolute(endpoint, session.SubtitleUrl)
    };

    private static string? Absolute(string endpoint, string? value) => string.IsNullOrWhiteSpace(value) ? null : new Uri(new Uri(endpoint.TrimEnd('/') + "/"), value).AbsoluteUri;
    private static HttpClient Client(string endpoint) => new() { BaseAddress = new Uri(endpoint.TrimEnd('/') + "/"), Timeout = TimeSpan.FromSeconds(20) };
    private static HttpRequestMessage Authorized(HttpMethod method, string path, string token) { var request = new HttpRequestMessage(method, path); request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token); return request; }
    private static async Task<T> ReadAsync<T>(HttpResponseMessage response, CancellationToken ct) { if (!response.IsSuccessStatusCode) throw new InvalidOperationException($"Playback request failed ({(int)response.StatusCode})."); return await response.Content.ReadFromJsonAsync<T>(cancellationToken: ct) ?? throw new InvalidOperationException("Playback server returned an empty response."); }
    private static async Task EnsureAsync(HttpResponseMessage response, CancellationToken ct) { if (response.IsSuccessStatusCode) return; var detail = await response.Content.ReadAsStringAsync(ct); throw new InvalidOperationException($"Playback request failed ({(int)response.StatusCode}): {detail}"); }
}
