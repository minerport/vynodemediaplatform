package playback

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in test of the real server HLS manager against packaged executables.
// Uses synthetic media only; does not require an account or production database.
func TestPackagedMediaRuntime(t *testing.T) {
	ffmpeg, ffprobe := os.Getenv("VYNODE_TEST_FFMPEG"), os.Getenv("VYNODE_TEST_FFPROBE")
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("set explicit packaged tool paths for runtime acceptance")
	}
	if !filepath.IsAbs(ffmpeg) || !filepath.IsAbs(ffprobe) {
		t.Fatal("packaged executable paths must be absolute")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	input := filepath.Join(t.TempDir(), "synthetic input [spaces].mp4")
	if err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "3", "-c:v", "libx264", "-c:a", "aac", input).Run(); err != nil {
		t.Fatalf("synthetic input generation: %v", err)
	}
	for _, mode := range []string{"remux", "audio-transcode", "video-transcode"} {
		t.Run(mode, func(t *testing.T) {
			h := NewHLS(ffmpeg, t.TempDir(), 1)
			defer h.Close()
			plan := PipelinePlan{Video: StreamPlan{Action: "COPY"}, Audio: StreamPlan{Action: "COPY", SourceCodec: "aac"}}
			if mode != "remux" {
				plan.Audio.Action = "TRANSCODE"
			}
			if mode == "video-transcode" {
				plan.Video = StreamPlan{Action: "TRANSCODE", TargetWidth: 320, TargetHeight: 180, TargetBitrate: 500000}
			}
			done := make(chan error, 1)
			if err := h.Ensure(HLSRequest{SessionID: "runtime", SourcePath: input, AudioStreamIndex: 1, Plan: plan, Done: func(err error) { done <- err }}); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("media pipeline timeout")
			}
			playlist, err := h.File("runtime", "master.m3u8")
			if err != nil {
				t.Fatal(err)
			}
			if err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_streams", playlist).Run(); err != nil {
				t.Fatalf("packaged FFprobe cannot inspect generated playlist: %v", err)
			}
			if err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-nostdin", "-ss", "1", "-i", playlist, "-t", "1", "-f", "null", "-").Run(); err != nil {
				t.Fatalf("generated HLS seek/decode failed: %v", err)
			}
		})
	}
}
