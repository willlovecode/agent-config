//go:build !windows && !darwin

package scan

import (
	"fmt"
	"math"
	"os"
	"strings"
)

func ramGB() float64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		var kb float64
		if _, err := fmt.Sscanf(line, "MemTotal: %f kB", &kb); err == nil {
			return math.Round(kb / (1 << 20))
		}
	}
	return 0
}
