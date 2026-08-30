//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
)

const windowsServiceName = "VyNodeMediaServer"

type serviceHandler struct{}

func (serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- run(ctx) }()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case code := <-done:
			if code != 0 {
				return false, uint32(code)
			}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				changes <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
			}
		}
	}
}

func runWindowsService() (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false, err
	}
	if os.Getenv("VYNODE_CONFIG_DIR") == "" {
		root := filepath.Join(os.Getenv("ProgramData"), "VyNode", "Media Server")
		_ = os.Setenv("VYNODE_CONFIG_DIR", root)
		_ = os.Setenv("VYNODE_TRANSCODE_DIR", filepath.Join(root, "transcode"))
		_ = os.Setenv("VYNODE_OPTIMIZED_DIR", filepath.Join(root, "optimized"))
		_ = os.Setenv("VYNODE_DOWNLOADS_DIR", filepath.Join(root, "downloads"))
	}
	if err := loadMediaToolOverrides(); err != nil {
		return true, err
	}
	if os.Getenv("VYNODE_WEB_DIR") == "" {
		executable, err := os.Executable()
		if err != nil {
			return true, fmt.Errorf("resolve installed Web Admin: %w", err)
		}
		webDir := filepath.Join(filepath.Dir(executable), "web")
		if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
			return true, fmt.Errorf("installed Web Admin unavailable: %w", err)
		}
		_ = os.Setenv("VYNODE_WEB_DIR", webDir)
	}
	if err := configureManagedMediaTools(); err != nil {
		return true, err
	}
	return true, svc.Run(windowsServiceName, serviceHandler{})
}

func loadMediaToolOverrides() error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\VyNode\Media Server`, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("open media server configuration: %w", err)
	}
	defer key.Close()
	for _, item := range []struct{ value, environment string }{
		{"ExternalFFmpegPath", "VYNODE_FFMPEG_PATH"},
		{"ExternalFFprobePath", "VYNODE_FFPROBE_PATH"},
	} {
		if os.Getenv(item.environment) != "" {
			continue
		}
		path, _, readErr := key.GetStringValue(item.value)
		if readErr == registry.ErrNotExist {
			continue
		}
		if readErr != nil {
			return fmt.Errorf("read %s: %w", item.value, readErr)
		}
		if path = filepath.Clean(path); !filepath.IsAbs(path) {
			return fmt.Errorf("%s must be an absolute path", item.value)
		}
		if _, statErr := os.Stat(path); statErr != nil {
			return fmt.Errorf("%s is unavailable: %w", item.value, statErr)
		}
		_ = os.Setenv(item.environment, path)
	}
	return nil
}

type managedMediaToolsManifest struct {
	Version       string `json:"version"`
	FFmpegSHA256  string `json:"ffmpegSha256"`
	FFprobeSHA256 string `json:"ffprobeSha256"`
}

func configureManagedMediaTools() error {
	needFFmpeg := os.Getenv("VYNODE_FFMPEG_PATH") == ""
	needFFprobe := os.Getenv("VYNODE_FFPROBE_PATH") == ""
	if !needFFmpeg {
		_ = os.Setenv("VYNODE_FFMPEG_SOURCE", "custom")
	}
	if !needFFprobe {
		_ = os.Setenv("VYNODE_FFPROBE_SOURCE", "custom")
	}
	if !needFFmpeg && !needFFprobe {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve server executable: %w", err)
	}
	root := filepath.Join(filepath.Dir(executable), "tools", "ffmpeg")
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return fmt.Errorf("managed media-tools manifest unavailable: %w", err)
	}
	var manifest managedMediaToolsManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("invalid managed media-tools manifest: %w", err)
	}
	if needFFmpeg {
		path := filepath.Join(root, "ffmpeg.exe")
		if err := verifyManagedTool(path, manifest.FFmpegSHA256); err != nil {
			return fmt.Errorf("managed FFmpeg integrity check failed: %w", err)
		}
		_ = os.Setenv("VYNODE_FFMPEG_PATH", path)
		_ = os.Setenv("VYNODE_FFMPEG_SOURCE", "managed")
	}
	if needFFprobe {
		path := filepath.Join(root, "ffprobe.exe")
		if err := verifyManagedTool(path, manifest.FFprobeSHA256); err != nil {
			return fmt.Errorf("managed FFprobe integrity check failed: %w", err)
		}
		_ = os.Setenv("VYNODE_FFPROBE_PATH", path)
		_ = os.Setenv("VYNODE_FFPROBE_SOURCE", "managed")
	}
	return nil
}

func verifyManagedTool(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if expected == "" || !equalFoldASCII(actual, expected) {
		return fmt.Errorf("SHA-256 mismatch for %s", filepath.Base(path))
	}
	return nil
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		left, right := a[index], b[index]
		if left >= 'A' && left <= 'Z' {
			left += 'a' - 'A'
		}
		if right >= 'A' && right <= 'Z' {
			right += 'a' - 'A'
		}
		if left != right {
			return false
		}
	}
	return true
}

func windowsLogWriter(configDir string) (io.Writer, func(), error) {
	root := configDir
	dir := filepath.Join(root, "logs")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, func() {}, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "server.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, func() {}, err
	}
	var once sync.Once
	return file, func() { once.Do(func() { _ = file.Close() }) }, nil
}
