package playback

import (
	"bufio"
	"bytes"
	"errors"
	"html"
	"os"
	"strings"
	"unicode/utf8"
)

const maxSubtitleSize = 2 << 20

func convertSRT(in []byte) ([]byte, error) {
	if len(in) > maxSubtitleSize || !utf8.Valid(in) {
		return nil, errors.New("invalid subtitle")
	}
	in = bytes.TrimPrefix(in, []byte{0xef, 0xbb, 0xbf})
	s := bufio.NewScanner(bytes.NewReader(in))
	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	for s.Scan() {
		line := strings.TrimSuffix(s.Text(), "\r")
		if strings.Contains(line, "-->") {
			line = strings.ReplaceAll(line, ",", ".")
			out.WriteString(line + "\n")
			continue
		}
		if strings.TrimSpace(line) == "" {
			out.WriteByte('\n')
			continue
		}
		allDigits := true
		for _, r := range strings.TrimSpace(line) {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			continue
		}
		out.WriteString(html.EscapeString(line))
		out.WriteByte('\n')
	}
	if s.Err() != nil {
		return nil, s.Err()
	}
	return []byte(out.String()), nil
}
func sanitizeVTT(in []byte) ([]byte, error) {
	if len(in) > maxSubtitleSize || !utf8.Valid(in) {
		return nil, errors.New("invalid subtitle")
	}
	lines := strings.Split(strings.ReplaceAll(string(in), "\r\n", "\n"), "\n")
	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	for i, line := range lines {
		if i == 0 && strings.HasPrefix(strings.TrimPrefix(line, "\ufeff"), "WEBVTT") {
			continue
		}
		if strings.Contains(line, "-->") || strings.TrimSpace(line) == "" || strings.HasPrefix(line, "NOTE ") {
			out.WriteString(line + "\n")
		} else {
			out.WriteString(html.EscapeString(line) + "\n")
		}
	}
	return []byte(out.String()), nil
}
func readSubtitle(path, format string) ([]byte, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if strings.EqualFold(format, "srt") {
		return convertSRT(b)
	}
	return sanitizeVTT(b)
}
