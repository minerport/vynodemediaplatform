package playback

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var hlsName = regexp.MustCompile(`^(master\.m3u8|init\.mp4|segment-[0-9]{6}\.m4s)$`)

type HLSRequest struct {
	SessionID        string
	SourcePath       string
	AudioStreamIndex int
	Plan             PipelinePlan
	Progress         func(encodedSeconds, speed float64, outputBytes int64)
	Done             func(error)
}
type hlsRun struct {
	cancel   context.CancelFunc
	done     chan struct{}
	dir      string
	released bool
}
type HLSManager struct {
	path, root string
	slots      chan struct{}
	mu         sync.Mutex
	active     map[string]*hlsRun
}

func NewHLS(path, root string, maximum int) *HLSManager {
	if maximum < 1 {
		maximum = 1
	}
	if path == "" {
		path, _ = exec.LookPath("ffmpeg")
	}
	h := &HLSManager{path: path, root: filepath.Join(root, "vynode"), slots: make(chan struct{}, maximum), active: map[string]*hlsRun{}}
	_ = h.cleanupStale()
	return h
}
func (h *HLSManager) cleanupStale() error {
	entries, err := os.ReadDir(h.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d := filepath.Join(h.root, e.Name())
		if _, x := os.Stat(filepath.Join(d, ".vynode-owned")); x == nil {
			_ = os.RemoveAll(d)
		}
	}
	return nil
}
func (h *HLSManager) Active() (int, int) { return len(h.slots), cap(h.slots) }
func (h *HLSManager) Ensure(r HLSRequest) error {
	h.mu.Lock()
	if h.active[r.SessionID] != nil {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()
	select {
	case h.slots <- struct{}{}:
	default:
		return ErrVideoCapacity
	}
	if h.path == "" {
		<-h.slots
		return ErrPipelineUnavailable
	}
	dir := filepath.Join(h.root, r.SessionID)
	if filepath.Dir(dir) != h.root {
		<-h.slots
		return ErrValidation
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		<-h.slots
		return ErrTranscodeStorage
	}
	if err := os.WriteFile(filepath.Join(dir, ".vynode-owned"), []byte("vynode\n"), 0600); err != nil {
		<-h.slots
		return ErrTranscodeStorage
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &hlsRun{cancel: cancel, done: make(chan struct{}), dir: dir}
	h.mu.Lock()
	h.active[r.SessionID] = run
	h.mu.Unlock()
	args := h.args(r, dir)
	cmd := exec.CommandContext(ctx, h.path, args...)
	cmd.Dir = dir
	configureProcess(cmd)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		h.finish(r.SessionID, run)
		return err
	}
	if err = cmd.Start(); err != nil {
		h.finish(r.SessionID, run)
		return err
	}
	go func() {
		s := bufio.NewScanner(stderr)
		diagnostic := &boundedBuffer{max: 32768}
		var encoded, speed float64
		var outputBytes int64
		for s.Scan() {
			line := s.Text()
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				_, _ = diagnostic.Write([]byte(line + "\n"))
				continue
			}
			switch key {
			case "out_time_us":
				microseconds, _ := strconv.ParseInt(value, 10, 64)
				encoded = float64(microseconds) / 1_000_000
			case "speed":
				if parsed, parseErr := strconv.ParseFloat(strings.TrimSuffix(value, "x"), 64); parseErr == nil {
					speed = parsed
				}
			case "total_size":
				outputBytes, _ = strconv.ParseInt(value, 10, 64)
			case "progress":
				outputBytes = h.outputSize(dir)
				if r.Progress != nil {
					r.Progress(encoded, speed, outputBytes)
				}
			}
		}
		err := cmd.Wait()
		if err != nil {
			safe := redact(diagnostic.String(), r.SourcePath)
			safe = redact(safe, dir)
			if safe != "" {
				err = fmt.Errorf("ffmpeg failed: %s", safe)
			}
		}
		outputBytes = h.outputSize(dir)
		if r.Progress != nil {
			r.Progress(encoded, speed, outputBytes)
		}
		if ctx.Err() != nil {
			err = nil
		}
		if r.Done != nil {
			r.Done(err)
		}
		h.finish(r.SessionID, run)
	}()
	return nil
}

func (h *HLSManager) outputSize(dir string) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !hlsName.MatchString(entry.Name()) {
			continue
		}
		if info, err := entry.Info(); err == nil {
			total += info.Size()
		}
	}
	return total
}
func (h *HLSManager) args(r HLSRequest, dir string) []string {
	a := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-i", r.SourcePath, "-map", "0:v:0"}
	if r.AudioStreamIndex >= 0 {
		a = append(a, "-map", fmt.Sprintf("0:%d", r.AudioStreamIndex))
	}
	if r.Plan.Video.Action == "COPY" {
		a = append(a, "-c:v", "copy")
	} else {
		p := r.Plan.Video
		bitrate := p.TargetBitrate
		if bitrate <= 0 {
			bitrate = 4_000_000
		}
		max := bitrate * 5 / 4
		scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2,format=yuv420p", p.TargetWidth, p.TargetHeight)
		a = append(a, "-vf", scale, "-c:v", "libx264", "-preset", "veryfast", "-b:v", strconv.FormatInt(bitrate, 10), "-maxrate", strconv.FormatInt(max, 10), "-bufsize", strconv.FormatInt(max*2, 10), "-g", "96", "-keyint_min", "96", "-sc_threshold", "0")
	}
	if r.Plan.Audio.Action == "COPY" && strings.EqualFold(r.Plan.Audio.SourceCodec, "aac") {
		a = append(a, "-c:a", "copy")
	} else {
		a = append(a, "-c:a", "aac", "-b:a", "192k")
	}
	return append(a, "-sn", "-progress", "pipe:2", "-f", "hls", "-hls_segment_type", "fmp4", "-hls_time", "4", "-hls_list_size", "0", "-hls_playlist_type", "event", "-hls_flags", "independent_segments+temp_file", "-hls_fmp4_init_filename", "init.mp4", "-hls_segment_filename", filepath.Join(dir, "segment-%06d.m4s"), filepath.Join(dir, "master.m3u8"))
}
func (h *HLSManager) finish(id string, r *hlsRun) {
	h.mu.Lock()
	if !r.released {
		<-h.slots
		r.released = true
	}
	h.mu.Unlock()
	close(r.done)
}
func (h *HLSManager) File(session, name string) (string, error) {
	if !hlsName.MatchString(name) {
		return "", ErrForbidden
	}
	h.mu.Lock()
	r := h.active[session]
	h.mu.Unlock()
	if r == nil {
		return "", ErrExpired
	}
	p := filepath.Join(r.dir, name)
	deadline := time.Now().Add(15 * time.Second)
	for {
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
			return p, nil
		}
		if time.Now().After(deadline) {
			return "", ErrUnavailable
		}
		time.Sleep(100 * time.Millisecond)
	}
}
func (h *HLSManager) Cancel(id string) {
	h.mu.Lock()
	r := h.active[id]
	h.mu.Unlock()
	if r == nil {
		return
	}
	h.mu.Lock()
	if h.active[id] == r {
		delete(h.active, id)
	}
	h.mu.Unlock()
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(3 * time.Second):
	}
	if _, e := os.Stat(filepath.Join(r.dir, ".vynode-owned")); e == nil {
		_ = os.RemoveAll(r.dir)
	}
}
func (h *HLSManager) Close() {
	h.mu.Lock()
	ids := make([]string, 0, len(h.active))
	for id := range h.active {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	for _, id := range ids {
		h.Cancel(id)
	}
}
