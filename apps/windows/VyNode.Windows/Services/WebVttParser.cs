using System.Globalization;

namespace VyNode.Windows.Services;

public sealed record SubtitleCue(double Start, double End, string Text);

public static class WebVttParser
{
    public static IReadOnlyList<SubtitleCue> Parse(string text)
    {
        var lines = text.Replace("\r", string.Empty).Split('\n');
        var cues = new List<SubtitleCue>();
        for (var index = 0; index < lines.Length; index++)
        {
            var timing = lines[index].Trim();
            if (!timing.Contains(" --> ", StringComparison.Ordinal)) continue;
            var range = timing.Split(" --> ", StringSplitOptions.TrimEntries);
            if (range.Length != 2 || !TryParseTime(range[0], out var start) || !TryParseTime(range[1].Split(' ')[0], out var end)) continue;
            var body = new List<string>();
            while (++index < lines.Length && !string.IsNullOrWhiteSpace(lines[index])) body.Add(lines[index].Trim());
            if (body.Count > 0) cues.Add(new SubtitleCue(start, end, string.Join(Environment.NewLine, body)));
        }
        return cues;
    }

    private static bool TryParseTime(string value, out double seconds)
    {
        seconds = 0;
        var parts = value.Replace(',', '.').Split(':');
        if (parts.Length is < 2 or > 3) return false;
        if (!double.TryParse(parts[^1], NumberStyles.Float, CultureInfo.InvariantCulture, out var trailing)) return false;
        if (!int.TryParse(parts[^2], out var minutes)) return false;
        var hours = 0;
        if (parts.Length == 3 && !int.TryParse(parts[0], out hours)) return false;
        seconds = hours * 3600 + minutes * 60 + trailing;
        return true;
    }
}
