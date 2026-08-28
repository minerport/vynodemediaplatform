using System.Text.Json.Serialization;

namespace VyNode.Windows.Models;

public sealed record DeviceInput(string Name, string Platform, string ClientName, string ClientVersion, string PlatformVersion);
public sealed record LoginResponse(string AccessToken, string RefreshToken, [property: JsonPropertyName("account")] GlobalUser User);
public sealed record GlobalUser(string Id, string Username, string? DisplayName, string? Role = null);
public sealed record LinkedServer(string Id, string Name, string Relationship, IReadOnlyList<ServerEndpoint> Endpoints);
public sealed record ServerEndpoint(string Url, string Kind, string? VerifiedAt);
public sealed record ServerAssertion([property: JsonPropertyName("assertion")] string Value);
public sealed record ServerSession(string AccessToken, string RefreshToken, GlobalUser? User = null);
public sealed record ServerIdentity([property: JsonPropertyName("serverId")] string InstanceId, [property: JsonPropertyName("serverName")] string Name, [property: JsonPropertyName("apiVersion")] string Version);
public sealed record HomeResponse(IReadOnlyList<HomeRow> Rows);
public sealed record HomeRow(string Id, string Type, string Title, IReadOnlyList<MediaItem> Items);
public sealed record MediaItem(string Id, [property: JsonPropertyName("type")] string Kind, string Title, string? Subtitle, string? ArtworkId, int Year, double Rating, int Position);
public sealed record MovieList(IReadOnlyList<Movie> Movies);
public sealed record ShowList(IReadOnlyList<Show> Shows);
public sealed record SearchResults(IReadOnlyList<Movie>? Movies, IReadOnlyList<Show>? Shows);
public sealed record Movie(string Id, string Title, string Overview, int Year, int RuntimeMinutes, double Rating, string? ContentRating, IReadOnlyList<string>? Genres, IReadOnlyList<MediaVersion>? Versions = null);
public sealed record MediaVersion(string Id, string Label, string Resolution, string Codec, string Hdr);
public sealed record Show(string Id, string Title, string Overview, int Year, double Rating, IReadOnlyList<string>? Genres, IReadOnlyList<Season>? Seasons = null);
public sealed record Season(string Id, string Title, string Overview, int SeasonNumber, IReadOnlyList<Episode> Episodes);
public sealed record Episode(string Id, string Title, string Overview, int EpisodeNumber, int RuntimeMinutes, bool Available);
public sealed record Artwork(string Id, string Type, bool Selected, bool Cached, int Width, int Height);
public sealed record ArtworkList(IReadOnlyList<Artwork> Artwork);
public sealed record Progress(double Position, double Duration, bool Watched);

public sealed record SessionContext(GlobalUser User, LinkedServer Server, string Endpoint, string ServerAccessToken, string? GlobalAccessToken = null, IReadOnlyList<LinkedServer>? LinkedServers = null, DeviceInput? Device = null, string? LocalRole = null);
public sealed record GlobalContext(LoginResponse Login, IReadOnlyList<LinkedServer> Servers, DeviceInput Device);
