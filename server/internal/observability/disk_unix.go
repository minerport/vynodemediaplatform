//go:build !windows

package observability

import "syscall"

func diskUsage(path string) (uint64, uint64, uint64, error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return 0, 0, 0, err
	}
	total := s.Blocks * uint64(s.Bsize)
	available := s.Bavail * uint64(s.Bsize)
	return total, total - s.Bfree*uint64(s.Bsize), available, nil
}
