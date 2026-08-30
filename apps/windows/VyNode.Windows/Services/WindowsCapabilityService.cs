using System.Runtime.InteropServices;
using VyNode.Windows.Models;

namespace VyNode.Windows.Services;

public static class WindowsCapabilityService
{
    // Baseline Media Foundation codecs available on supported Windows editions.
    // Optional Store codec extensions (HEVC/AV1) are deliberately not advertised.
    public static PlaybackCapabilityProfile Current() => new(
        1,
        "VyNode Windows",
        "16.0.0",
        "WINDOWS",
        Environment.OSVersion.Version.ToString(),
        RuntimeInformation.OSArchitecture.ToString(),
        ["mp4", "m4v", "mov"],
        ["h264"],
        ["aac", "mp3"],
        ["webvtt", "vtt", "srt"],
        3840,
        2160,
        8,
        [],
        true,
        true);
}
