package scan

import (
	"math"
	"strconv"
	"strings"
)

// memsizeGB turns the bytes `sysctl -n hw.memsize` prints (macOS) into whole GB; 0 if unreadable.
func memsizeGB(out string) float64 {
	n, err := strconv.ParseUint(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0
	}
	return math.Round(float64(n) / (1 << 30))
}
