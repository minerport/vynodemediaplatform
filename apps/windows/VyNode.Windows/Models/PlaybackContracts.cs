using System.Text.Json.Serialization;

namespace VyNode.Windows.Models;

public sealed record PlaybackCapabilityProfile(
    int SchemaVersion,
    string ClientName,
    string ClientVersion,
    string Platform,
    string PlatformVersion,
    string DeviceModel,
    IReadOnlyList<string> SupportedContainers,
    IReadOnlyList<string> SupportedVideoCodecs,
    IReadOnlyList<string> SupportedAudioCodecs,
    IReadOnlyList<string> SubtitleFormats,
    int MaximumVideoWidth,
    int MaximumVideoHeight,
    int MaximumAudioChannels,
    IReadOnlyList<string> HdrCapabilities,
    bool DirectPlaySupport,
    bool FragmentedMp4Support);

public sealed record PlaybackStartRequest(
    string LogicalType,
    string LogicalId,
    bool Resume,
    PlaybackCapabilityProfile Capabilities,
    string? RequestedVersionId = null,
    string? SelectedAudioTrackId = null,
    string? SelectedSubtitleTrackId = null,
    string? QualityId = null,
    double StartPosition = 0,
    string? PlaybackContextId = null,
    string? ContextType = null);

public sealed record PlaybackTrack(string Id, string Kind, string Codec, string? Language, string? Title, int Channels, bool Default, bool Forced, bool Commentary, bool HearingImpaired, bool Usable, string? Reason, string? Source);
public sealed record PlaybackVersion(string Id, string Container, string VideoCodec, IReadOnlyList<string>? AudioCodecs, IReadOnlyList<PlaybackTrack>? AudioTracks, IReadOnlyList<PlaybackTrack>? SubtitleTracks, int Width, int Height, long Bitrate, string? Hdr, string? Resolution, string? Label, bool Available);
public sealed record PlaybackQuality(string Id, string Label, int MaxWidth, int MaxHeight, long TargetVideoBitrate, long MaxVideoBitrate, long AudioBitrate);
public sealed record PlaybackMarker(string Id, string LogicalType, string LogicalId, [property: JsonPropertyName("type")] string MarkerType, string Source, double Start, double End, double? Confidence);
public sealed record PlaybackNavigationItem(string LogicalId, string ShowId, string ShowTitle, string Title, int SeasonNumber, int EpisodeNumber, bool Available);
public sealed record PlaybackNavigation(PlaybackNavigationItem? Previous, PlaybackNavigationItem? Next, bool Autoplay, int CountdownSeconds);
public sealed record PlaybackDecision(string Mode);
public sealed record PlaybackSession(
    string Id,
    string LogicalType,
    string LogicalId,
    PlaybackVersion SelectedVersion,
    PlaybackDecision Decision,
    string State,
    double Position,
    double Duration,
    double ResumePosition,
    string? MediaUrl,
    string? HlsUrl,
    IReadOnlyList<PlaybackQuality>? AvailableQualities,
    string? SubtitleUrl,
    PlaybackTrack? SelectedAudioTrack,
    PlaybackTrack? SelectedSubtitleTrack,
    IReadOnlyList<PlaybackMarker>? Markers,
    PlaybackNavigation? Navigation,
    string? PlaybackContextId);

public sealed record PlaybackProgressUpdate(string State, double Position, double Duration);
public sealed record PlaybackRoute(SessionContext Context, string LogicalType, string LogicalId, string Title, bool Resume = true, double StartPosition = 0);
