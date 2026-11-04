package hooks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// shellCommand runs s in cmd.exe the way Node's shell option does, with the command line
// passed through unchanged (Go's own argument quoting would mangle its quotes).
func shellCommand(ctx context.Context, s string) *exec.Cmd {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	c := exec.CommandContext(ctx, comspec)
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /s /c "` + s + `"`}
	return c
}
