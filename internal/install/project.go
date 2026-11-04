package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/willlovecode/agent-config/internal/ojson"
)

// AddChecks wires the Stop check runner into one project for both tools. It writes
// .claude/settings.local.json and .codex/hooks.json in the project and git-excludes both,
// so nothing personal gets committed to the project's repo. Codex needs the project and
// the hook trusted (/hooks).
func AddChecks(home, project, command string, timeout int, w io.Writer) error {
	abs, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return fmt.Errorf("%s is not a folder", project)
	}
	st, err := ReadState(home)
	if err != nil || st.Hook == "" || !exists(st.Hook) {
		return fmt.Errorf("run `agtk apply` first (agtk is not installed in %s)", StateDir(home))
	}
	a := &applier{Options: Options{Home: home, Out: w}, exe: st.Hook}
	if a.s, err = newSession(StateDir(home)); err != nil {
		return err
	}
	t := strconv.Itoa(timeout)
	setStop := func(p string, group *ojson.Object) error {
		o, err := a.readJSON(p)
		if err != nil {
			return err
		}
		setHookGroup(o, "Stop", "stop-checks", group)
		return a.writeJSON(p, o)
	}
	err = setStop(filepath.Join(abs, ".claude", "settings.local.json"), ojson.Obj("hooks", []any{ojson.Obj(
		"type", "command", "command", a.exe, "args", []string{"hook", "stop-checks", command, t}, "timeout", timeout+30)}))
	if err != nil {
		return err
	}
	codexCmd := fmt.Sprintf(`"%s" hook stop-checks "%s" %s`, slash(a.exe), strings.ReplaceAll(command, `"`, `\"`), t)
	err = setStop(filepath.Join(abs, ".codex", "hooks.json"), ojson.Obj("hooks", []any{ojson.Obj(
		"type", "command", "command", codexCmd, "timeout", timeout+30)}))
	if err != nil {
		return err
	}
	if err := a.gitExclude(abs, ".claude/settings.local.json", ".codex/hooks.json"); err != nil {
		return err
	}
	fmt.Fprintf(w, "Stop checks for %s: %s (timeout %ds). Trust the hook in Codex with /hooks.\n", abs, command, timeout)
	if dir := a.s.finish(); dir != "" {
		fmt.Fprintf(w, "Backup: %s\n", dir)
	}
	return nil
}
