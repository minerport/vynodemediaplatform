//go:build !linux

package observability

func platformMemory() (uint64, uint64, uint64) { return 0, 0, 0 }
