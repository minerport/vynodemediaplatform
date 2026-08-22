package playback

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFFmpegArgumentsAreStructuredAndCopyVideo(t *testing.T) {
	p := &FFmpegPipeline{}
	path := filepath.Join("media", "- odd ' 名.mkv")
	a := p.Args(PipelineRequest{SourcePath: path, Mode: AudioTranscode, Start: 20.5, AudioStreamIndex: 3, TargetChannels: 6})
	joined := strings.Join(a, "|")
	if !contains(a, path) || !strings.Contains(joined, "-c:v|copy") || !strings.Contains(joined, "-c:a|aac") || !strings.Contains(joined, "-ss|20.500") {
		t.Fatal(a)
	}
}
func TestSubtitleConversionEscapesUntrustedText(t *testing.T) {
	in := []byte("1\n00:00:01,000 --> 00:00:02,000\n<script>alert(1)</script>\n\n")
	out, e := convertSRT(in)
	if e != nil || !strings.HasPrefix(string(out), "WEBVTT") || strings.Contains(string(out), "<script>") || !strings.Contains(string(out), "00:00:01.000") {
		t.Fatalf("%s %v", out, e)
	}
}
func TestPipelineCancellationAndCapacity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper; Windows is cross-built")
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "ffmpeg helper")
	if e := os.WriteFile(helper, []byte("#!/bin/sh\nsleep 30\n"), 0700); e != nil {
		t.Fatal(e)
	}
	p := &FFmpegPipeline{path: helper, slots: make(chan struct{}, 1), active: map[string]context.CancelFunc{}, caps: FFmpegCapabilities{Available: true, Muxers: []string{"mp4"}}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, e := p.Stream(ctx, PipelineRequest{SessionID: "one", SourcePath: "safe", Mode: DirectStream, AudioStreamIndex: -1}, io.Discard)
		done <- e
	}()
	deadline := time.Now().Add(2 * time.Second)
	for p.Capabilities().Active == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	_, e := p.Stream(context.Background(), PipelineRequest{SessionID: "two", SourcePath: "safe", Mode: DirectStream, AudioStreamIndex: -1}, io.Discard)
	if !errors.Is(e, ErrCapacity) {
		t.Fatalf("capacity=%v", e)
	}
	cancel()
	select {
	case e = <-done:
		if e == nil {
			t.Fatal("cancel returned nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("process leaked")
	}
	if p.Capabilities().Active != 0 {
		t.Fatal("active process leaked")
	}
}

func TestSeekReplacesPipelineInSameSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper")
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "ffmpeg")
	if e := os.WriteFile(helper, []byte("#!/bin/sh\nsleep 30\n"), 0700); e != nil {
		t.Fatal(e)
	}
	p := &FFmpegPipeline{path: helper, slots: make(chan struct{}, 1), active: map[string]context.CancelFunc{}, caps: FFmpegCapabilities{Available: true, Muxers: []string{"mp4"}}}
	first := make(chan error, 1)
	go func() {
		_, e := p.Stream(context.Background(), PipelineRequest{SessionID: "same", SourcePath: "x", Mode: DirectStream, AudioStreamIndex: -1}, io.Discard)
		first <- e
	}()
	deadline := time.Now().Add(time.Second)
	for p.Capabilities().Active == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	second := make(chan error, 1)
	go func() {
		_, e := p.Stream(ctx, PipelineRequest{SessionID: "same", SourcePath: "x", Mode: DirectStream, Start: 20, AudioStreamIndex: -1}, io.Discard)
		second <- e
	}()
	select {
	case e := <-first:
		if e == nil {
			t.Fatal("old pipeline was not canceled")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("old pipeline did not exit")
	}
	deadline = time.Now().Add(time.Second)
	for p.Capabilities().Active == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if p.Capabilities().Active != 1 {
		t.Fatal("replacement did not start")
	}
	cancel()
	select {
	case <-second:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement leaked")
	}
}
