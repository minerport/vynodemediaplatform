//go:build windows

package observability

import (
	"syscall"
	"unsafe"
)

func diskUsage(path string) (uint64, uint64, uint64, error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel.NewProc("GetDiskFreeSpaceExW")
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, 0, err
	}
	var available, total, free uint64
	r, _, e := proc.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if r == 0 {
		return 0, 0, 0, e
	}
	return total, total - free, available, nil
}
