package hooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Stop checks: a Stop hook for Claude Code and Codex that runs a project's checks before
// the agent may finish. The command runs in cmd.exe on Windows and sh elsewhere. Skips
// when HEAD and the uncommitted work are unchanged since the last pass. Blocks at most
// maxBlocks times in a row, then lets the agent stop and report.

const maxBlocks = 2

// StateDir holds the per-project pass/block state; tests point it elsewhere.
var StateDir = filepath.Join(os.TempDir(), "agtk-stop-checks")

func gitOut(cwd string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "git", args...)
	c.Dir = cwd
	out, err := c.Output()
	return string(out), err == nil
}

func fingerprint(cwd, checkCmd string) string {
	head, ok1 := gitOut(cwd, "rev-parse", "HEAD")
	diff, ok2 := gitOut(cwd, "diff", "HEAD")
	untracked, ok3 := gitOut(cwd, "ls-files", "--others", "--exclude-standard", "-z")
	if !ok1 || !ok2 || !ok3 {
		return ""
	}
	h := sha256.New()
	io.WriteString(h, checkCmd)
	io.WriteString(h, head)
	io.WriteString(h, diff)
	for _, f := range strings.Split(untracked, "\x00") {
		if f == "" {
			continue
		}
		io.WriteString(h, f)
		p := filepath.Join(cwd, f)
		if st, err := os.Stat(p); err == nil && st.Size() <= 5<<20 {
			if b, err := os.ReadFile(p); err == nil {
				h.Write(b)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

type stopState struct {
	Passed string `json:"passed,omitempty"`
	Blocks int    `json:"blocks"`
}

// StopChecks runs checkCmd for the project in the hook input's cwd and writes a block
// decision when it fails.
func StopChecks(in io.Reader, out io.Writer, checkCmd string, timeout time.Duration) {
	var input struct {
		Cwd            string `json:"cwd"`
		StopHookActive bool   `json:"stop_hook_active"`
	}
	raw, _ := io.ReadAll(in)
	json.Unmarshal(raw, &input)
	if checkCmd == "" {
		return
	}
	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	os.MkdirAll(StateDir, 0o755)
	sum := sha256.Sum256([]byte(strings.ToLower(cwd)))
	stateFile := filepath.Join(StateDir, hex.EncodeToString(sum[:])[:16]+".json")
	var st stopState
	if b, err := os.ReadFile(stateFile); err == nil {
		json.Unmarshal(b, &st)
	}
	save := func(s stopState) {
		b, _ := json.Marshal(s)
		os.WriteFile(stateFile, b, 0o644)
	}
	fp := fingerprint(cwd, checkCmd)
	if fp != "" && st.Passed == fp {
		return
	}
	// A fresh stop (not one we caused) resets the block counter.
	blocks := 0
	if input.StopHookActive {
		blocks = st.Blocks
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := shellCommand(ctx, checkCmd)
	c.Dir = cwd
	c.WaitDelay = 5 * time.Second // don't hang on grandchildren that keep the pipes open
	output, err := c.CombinedOutput()
	// A daemon left behind (Gradle, MSBuild) can hold the pipe after a passing run.
	if errors.Is(err, exec.ErrWaitDelay) && c.ProcessState.ExitCode() == 0 {
		err = nil
	}
	if err == nil {
		save(stopState{Passed: fp})
		return
	}
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(output), "\r\n", "\n")), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	why := fmt.Sprintf("exit %d", c.ProcessState.ExitCode())
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		why = fmt.Sprintf("timed out after %ds", int(timeout.Seconds()))
	}
	if blocks >= maxBlocks {
		save(stopState{})
		return // give up blocking; the agent has already been told twice
	}
	save(stopState{Blocks: blocks + 1})
	writeJSON(out, map[string]string{
		"decision": "block",
		"reason": fmt.Sprintf("Project checks failed (%s, %s). Fix them, or tell the user why they cannot pass.\n%s",
			checkCmd, why, strings.Join(lines, "\n")),
	})
}
