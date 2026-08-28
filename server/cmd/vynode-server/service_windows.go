//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

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
	return true, svc.Run(windowsServiceName, serviceHandler{})
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
