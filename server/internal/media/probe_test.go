package media

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFFprobeJSONParsingAndSafeFilename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-probe")
	body := `#!/bin/sh
printf '%s' '{"format":{"format_name":"matroska","duration":"12.5","bit_rate":"9000"},"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","width":3840,"height":2160,"pix_fmt":"yuv420p10le","color_transfer":"smpte2084","disposition":{"default":1}},{"index":1,"codec_type":"audio","codec_name":"truehd","channels":8,"sample_rate":"48000","tags":{"language":"eng"}},{"index":2,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","disposition":{"forced":1}}]}'
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := NewFFprobe(script, 2)
	got, err := p.Probe(context.Background(), filepath.Join(dir, "-odd name.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainerFormat != "matroska" || len(got.Streams) != 3 || got.Streams[0].Codec != "hevc" || got.Streams[1].Channels != 8 || !got.Streams[2].Forced {
		t.Fatalf("unexpected probe: %#v", got)
	}
}

func TestFFprobeMalformedAndBoundedError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "bad-probe")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%040000d' 1 >&2\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := NewFFprobe(script, 1).Probe(context.Background(), "file.mkv")
	if err == nil || len(err.Error()) > 33000 || !strings.Contains(err.Error(), "ffprobe failed") {
		t.Fatalf("unexpected error length=%d error=%v", len(err.Error()), err)
	}
}
