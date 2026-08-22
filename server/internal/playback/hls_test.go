package playback

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHLSArgumentsAndTraversal(t *testing.T) {
	h := NewHLS("ffmpeg", t.TempDir(), 1)
	r := HLSRequest{SessionID: "session", SourcePath: "/media/input.mkv", AudioStreamIndex: 1, Plan: PipelinePlan{Video: StreamPlan{TargetWidth: 1280, TargetHeight: 720, TargetBitrate: 4_000_000}, Audio: StreamPlan{Action: "TRANSCODE"}}}
	a := h.args(r, filepath.Join(t.TempDir(), "out"))
	joined := ""
	for _, x := range a {
		joined += "|" + x
	}
	if !containsArg(a, "libx264") || !containsArg(a, "fmp4") || !containsArg(a, "scale=1280:720:force_original_aspect_ratio=decrease:force_divisible_by=2,format=yuv420p") {
		t.Fatal(joined)
	}
	if _, e := h.File("session", "../master.m3u8"); e != ErrForbidden {
		t.Fatal(e)
	}
}

func TestHLSCapacityIsBounded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a POSIX executable")
	}
	root := t.TempDir()
	helper := filepath.Join(root, "ffmpeg-test")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nsleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	h := NewHLS(helper, root, 1)
	t.Cleanup(h.Close)
	plan := PipelinePlan{Video: StreamPlan{TargetWidth: 640, TargetHeight: 360, TargetBitrate: 1_000_000}}
	if err := h.Ensure(HLSRequest{SessionID: "one", SourcePath: "/input", AudioStreamIndex: -1, Plan: plan}); err != nil {
		t.Fatal(err)
	}
	if err := h.Ensure(HLSRequest{SessionID: "two", SourcePath: "/input", AudioStreamIndex: -1, Plan: plan}); err != ErrVideoCapacity {
		t.Fatalf("second transcode = %v, want %v", err, ErrVideoCapacity)
	}
}
func containsArg(a []string, w string) bool {
	for _, x := range a {
		if x == w {
			return true
		}
	}
	return false
}
func TestStaleCleanupPreservesUnrelated(t *testing.T) {
	root := t.TempDir()
	owned := filepath.Join(root, "vynode", "old")
	if e := os.MkdirAll(owned, 0700); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(filepath.Join(owned, ".vynode-owned"), []byte("x"), 0600)
	unrelated := filepath.Join(root, "vynode", "keep")
	_ = os.MkdirAll(unrelated, 0700)
	_ = os.WriteFile(filepath.Join(unrelated, "user.txt"), []byte("x"), 0600)
	_ = NewHLS("ffmpeg", root, 1)
	if _, e := os.Stat(owned); !os.IsNotExist(e) {
		t.Fatal("owned stale directory remains")
	}
	if _, e := os.Stat(filepath.Join(unrelated, "user.txt")); e != nil {
		t.Fatal("unrelated artifact removed")
	}
}
func TestHardwareReportsSoftwareWithoutClaimingDevices(t *testing.T) {
	x := detectHardware(FFmpegCapabilities{Encoders: []string{"libx264", "h264_nvenc"}})
	if !x[0].Available || x[0].Type != "SOFTWARE" {
		t.Fatal(x)
	}
	for _, b := range x[1:] {
		if b.Available {
			t.Fatalf("unvalidated hardware claimed: %+v", b)
		}
	}
}
