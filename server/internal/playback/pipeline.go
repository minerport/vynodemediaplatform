package playback

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrCapacity = errors.New("playback capacity reached")
var ErrPipelineUnavailable = errors.New("ffmpeg unavailable")

type FFmpegCapabilities struct {
	Available bool     `json:"available"`
	Version   string   `json:"version,omitempty"`
	Muxers    []string `json:"muxers"`
	Encoders  []string `json:"encoders"`
	Decoders  []string `json:"decoders"`
	Active    int      `json:"activePipelines"`
	Maximum   int      `json:"maximumPipelines"`
}
type PipelineRequest struct {
	SessionID, InstanceID, SourcePath string
	Mode                              Mode
	Start                             float64
	AudioStreamIndex                  int
	TargetChannels                    int
}
type PipelineResult struct {
	Code      string
	Stderr    string
	StartedAt time.Time
}

type boundedBuffer struct {
	b   []byte
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if len(b.b) < b.max {
		left := b.max - len(b.b)
		if len(p) > left {
			p = p[:left]
		}
		b.b = append(b.b, p...)
	}
	return n, nil
}
func (b *boundedBuffer) String() string { return string(b.b) }

type FFmpegPipeline struct {
	path   string
	slots  chan struct{}
	mu     sync.Mutex
	active map[string]context.CancelFunc
	caps   FFmpegCapabilities
}

func NewFFmpeg(path string, maximum int) *FFmpegPipeline {
	if maximum < 1 {
		maximum = 2
	}
	if path == "" {
		path, _ = exec.LookPath("ffmpeg")
	}
	p := &FFmpegPipeline{path: path, slots: make(chan struct{}, maximum), active: map[string]context.CancelFunc{}}
	p.caps = p.detect()
	p.caps.Maximum = maximum
	return p
}
func (p *FFmpegPipeline) detect() FFmpegCapabilities {
	c := FFmpegCapabilities{Muxers: []string{}, Encoders: []string{}, Decoders: []string{}}
	if p.path == "" {
		return c
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, e := exec.CommandContext(ctx, p.path, "-hide_banner", "-version").Output()
	if e != nil {
		return c
	}
	c.Available = true
	line := strings.SplitN(string(out), "\n", 2)[0]
	c.Version = strings.TrimSpace(line)
	for _, x := range []struct {
		flag  string
		dst   *[]string
		names []string
	}{{"-muxers", &c.Muxers, []string{"mp4", "webvtt"}}, {"-encoders", &c.Encoders, []string{"aac", "webvtt"}}, {"-decoders", &c.Decoders, []string{"h264", "aac", "ac3", "subrip"}}} {
		o, _ := exec.CommandContext(ctx, p.path, "-hide_banner", x.flag).CombinedOutput()
		lower := strings.ToLower(string(o))
		for _, n := range x.names {
			if strings.Contains(lower, " "+n+" ") || strings.Contains(lower, " "+n+"\n") {
				*x.dst = append(*x.dst, n)
			}
		}
	}
	return c
}
func (p *FFmpegPipeline) Capabilities() FFmpegCapabilities {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.caps
	c.Active = len(p.active)
	return c
}
func (p *FFmpegPipeline) Available() bool {
	return p != nil && p.caps.Available && contains(p.caps.Muxers, "mp4")
}
func (p *FFmpegPipeline) Cancel(session string) {
	p.mu.Lock()
	cancel := p.active[session]
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (p *FFmpegPipeline) Close() {
	p.mu.Lock()
	cs := make([]context.CancelFunc, 0, len(p.active))
	for _, c := range p.active {
		cs = append(cs, c)
	}
	p.mu.Unlock()
	for _, c := range cs {
		c()
	}
}
func (p *FFmpegPipeline) Args(r PipelineRequest) []string {
	a := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if r.Start > 0 {
		a = append(a, "-ss", strconv.FormatFloat(r.Start, 'f', 3, 64))
	}
	a = append(a, "-i", r.SourcePath, "-map", "0:v:0")
	if r.AudioStreamIndex >= 0 {
		a = append(a, "-map", fmt.Sprintf("0:%d", r.AudioStreamIndex))
	}
	a = append(a, "-c:v", "copy")
	if r.Mode == AudioTranscode {
		bitrate := "192k"
		if r.TargetChannels > 2 {
			bitrate = "384k"
		}
		a = append(a, "-c:a", "aac", "-b:a", bitrate, "-ac", strconv.Itoa(r.TargetChannels))
	} else {
		a = append(a, "-c:a", "copy")
	}
	return append(a, "-sn", "-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof", "pipe:1")
}
func (p *FFmpegPipeline) Stream(ctx context.Context, r PipelineRequest, w io.Writer) (PipelineResult, error) {
	res := PipelineResult{StartedAt: time.Now()}
	if !p.Available() {
		res.Code = "FFMPEG_START_FAILED"
		return res, ErrPipelineUnavailable
	}
	p.mu.Lock()
	prior := p.active[r.SessionID]
	p.mu.Unlock()
	if prior != nil {
		prior()
	}
	if prior != nil {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case p.slots <- struct{}{}:
		case <-ctx.Done():
			return res, ctx.Err()
		case <-timer.C:
			res.Code = "PLAYBACK_CAPACITY_REACHED"
			return res, ErrCapacity
		}
	} else {
		select {
		case p.slots <- struct{}{}:
		default:
			res.Code = "PLAYBACK_CAPACITY_REACHED"
			return res, ErrCapacity
		}
	}
	defer func() { <-p.slots }()
	runctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.mu.Lock()
	p.active[r.SessionID] = cancel
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.active, r.SessionID); p.mu.Unlock() }()
	stderr := &boundedBuffer{max: 32768}
	cmd := exec.CommandContext(runctx, p.path, p.Args(r)...)
	configureProcess(cmd)
	cmd.Stdout = w
	cmd.Stderr = stderr
	err := cmd.Run()
	res.Stderr = redact(stderr.String(), r.SourcePath)
	if err != nil {
		if runctx.Err() != nil {
			res.Code = "PIPELINE_CANCELED"
			return res, runctx.Err()
		}
		res.Code = "FFMPEG_PROCESS_FAILED"
		return res, fmt.Errorf("%s", res.Code)
	}
	res.Code = "STOPPED"
	return res, nil
}
func redact(v, path string) string {
	return strings.TrimSpace(strings.ReplaceAll(v, path, "[media-path]"))
}
func (p *FFmpegPipeline) ConvertEmbeddedSubtitle(ctx context.Context, path string, index int) ([]byte, error) {
	if !p.Available() {
		return nil, ErrPipelineUnavailable
	}
	out := &boundedBuffer{max: 4 << 20}
	b := &boundedBuffer{max: 32768}
	cmd := exec.CommandContext(ctx, p.path, "-hide_banner", "-loglevel", "error", "-nostdin", "-i", path, "-map", fmt.Sprintf("0:%d", index), "-f", "webvtt", "pipe:1")
	cmd.Stdout = out
	cmd.Stderr = b
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("subtitle conversion failed")
	}
	return sanitizeVTT(out.b)
}
