//go:build darwin

package scan

import (
	"context"
	"os/exec"
	"time"
)

func ramGB() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	return memsizeGB(string(out))
}
