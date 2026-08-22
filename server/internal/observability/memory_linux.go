//go:build linux

package observability

import (
	"os"
	"strconv"
	"strings"
)

func platformMemory() (rss, total, available uint64) {
	if b, e := os.ReadFile("/proc/self/status"); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "VmRSS:") {
				f := strings.Fields(line)
				if len(f) > 1 {
					n, _ := strconv.ParseUint(f[1], 10, 64)
					rss = n * 1024
				}
			}
		}
	}
	if b, e := os.ReadFile("/proc/meminfo"); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 2 {
				continue
			}
			n, _ := strconv.ParseUint(f[1], 10, 64)
			switch strings.TrimSuffix(f[0], ":") {
			case "MemTotal":
				total = n * 1024
			case "MemAvailable":
				available = n * 1024
			}
		}
	}
	return
}
