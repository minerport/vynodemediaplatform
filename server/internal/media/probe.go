package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type MediaProbe interface {
	Probe(context.Context, string) (ProbeResult, error)
	Available() bool
	Version(context.Context) string
}

type FFprobe struct {
	path    string
	timeout time.Duration
	sem     chan struct{}
}

func NewFFprobe(configured string, concurrency int) *FFprobe {
	path := configured
	if path == "" {
		path, _ = exec.LookPath("ffprobe")
	}
	return &FFprobe{path: path, timeout: 45 * time.Second, sem: make(chan struct{}, concurrency)}
}
func (p *FFprobe) Available() bool { return p.path != "" }
func (p *FFprobe) Version(ctx context.Context) string {
	if !p.Available() {
		return ""
	}
	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, p.path, "-version").Output()
	if err != nil {
		return ""
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	if len(line) > 160 {
		line = line[:160]
	}
	return line
}

type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(v []byte) (int, error) {
	n := len(v)
	remain := b.max - b.buf.Len()
	if remain > 0 {
		if len(v) > remain {
			v = v[:remain]
		}
		_, _ = b.buf.Write(v)
	}
	return n, nil
}
func (b *limitedBuffer) Bytes() []byte  { return b.buf.Bytes() }
func (b *limitedBuffer) String() string { return b.buf.String() }
func (p *FFprobe) Probe(ctx context.Context, path string) (ProbeResult, error) {
	if !p.Available() {
		return ProbeResult{}, errors.New("ffprobe unavailable")
	}
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return ProbeResult{}, ctx.Err()
	}
	c, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	cmd := exec.CommandContext(c, p.path, "-v", "error", "-show_format", "-show_streams", "-of", "json", "-i", path)
	out, errout := &limitedBuffer{max: 8 << 20}, &limitedBuffer{max: 32 << 10}
	cmd.Stdout = out
	cmd.Stderr = errout
	if err := cmd.Run(); err != nil {
		if c.Err() != nil {
			return ProbeResult{}, c.Err()
		}
		return ProbeResult{}, fmt.Errorf("ffprobe failed: %s", strings.TrimSpace(errout.String()))
	}
	var raw struct {
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			Index          int               `json:"index"`
			CodecType      string            `json:"codec_type"`
			CodecName      string            `json:"codec_name"`
			Profile        string            `json:"profile"`
			Level          int               `json:"level"`
			Width          int               `json:"width"`
			Height         int               `json:"height"`
			Channels       int               `json:"channels"`
			PixFmt         string            `json:"pix_fmt"`
			RFrameRate     string            `json:"r_frame_rate"`
			FieldOrder     string            `json:"field_order"`
			ChannelLayout  string            `json:"channel_layout"`
			SampleRate     string            `json:"sample_rate"`
			BitRate        string            `json:"bit_rate"`
			ColorPrimaries string            `json:"color_primaries"`
			ColorTransfer  string            `json:"color_transfer"`
			ColorSpace     string            `json:"color_space"`
			ColorRange     string            `json:"color_range"`
			Tags           map[string]string `json:"tags"`
			Disposition    map[string]int    `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return ProbeResult{}, fmt.Errorf("invalid ffprobe JSON")
	}
	result := ProbeResult{ContainerFormat: raw.Format.Name}
	result.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	result.Bitrate, _ = strconv.ParseInt(raw.Format.BitRate, 10, 64)
	for _, x := range raw.Streams {
		s := Stream{Index: x.Index, Type: strings.ToUpper(x.CodecType), Codec: x.CodecName, Profile: x.Profile, Level: x.Level, Width: x.Width, Height: x.Height, PixelFormat: x.PixFmt, FrameRate: x.RFrameRate, ScanType: x.FieldOrder, Channels: x.Channels, ChannelLayout: x.ChannelLayout, ColorPrimaries: x.ColorPrimaries, ColorTransfer: x.ColorTransfer, ColorSpace: x.ColorSpace, ColorRange: x.ColorRange, Language: x.Tags["language"], Title: x.Tags["title"], Default: x.Disposition["default"] == 1, Forced: x.Disposition["forced"] == 1, HearingImpaired: x.Disposition["hearing_impaired"] == 1, Commentary: x.Disposition["comment"] == 1}
		s.Bitrate, _ = strconv.ParseInt(x.BitRate, 10, 64)
		s.SampleRate, _ = strconv.Atoi(x.SampleRate)
		if strings.Contains(x.PixFmt, "10") {
			s.BitDepth = 10
		}
		result.Streams = append(result.Streams, s)
	}
	return result, nil
}

var _ io.Writer = (*limitedBuffer)(nil)
