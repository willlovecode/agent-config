package hooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The case list is shared with checks/test-hooks.mjs: "D|command" must deny, "A|command" must allow.
func TestGitGuardCases(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "enforcement", "hooks", "test", "git-guard-cases.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		want, command, ok := strings.Cut(strings.TrimRight(sc.Text(), "\r"), "|")
		if !ok {
			continue
		}
		n++
		in := `{"tool_name":"Bash","tool_input":{"command":` + quote(command) + `}}`
		var out bytes.Buffer
		GitGuard(strings.NewReader(in), &out)
		got := "A"
		if out.Len() > 0 {
			got = "D"
		}
		if got != want {
			t.Errorf("want %s, got %s: %s", want, got, command)
		}
	}
	if n < 45 {
		t.Fatalf("only %d cases read", n)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestGitGuardBadInputDenies(t *testing.T) {
	var out bytes.Buffer
	GitGuard(strings.NewReader("not json"), &out)
	if !strings.Contains(out.String(), `"deny"`) {
		t.Fatal("unparsable input must deny")
	}
	out.Reset()
	GitGuard(strings.NewReader(`{"tool_input":{"command":["git","push"]}}`), &out)
	if out.Len() == 0 {
		t.Fatal("array command must be checked")
	}
	for _, in := range []string{
		`{"tool_input":{"command":["bash","-lc","git push"]}}`,
		`{"tool_input":{"command":["git","push",1]}}`,
		`{"tool_input":{"command":"git push origin main"}}`,
		`{"tool_input":{"command":"git\u000bpush"}}`,
		`{"tool_input":{"command":"git　reset --hard"}}`,
	} {
		out.Reset()
		GitGuard(strings.NewReader(in), &out)
		if out.Len() == 0 {
			t.Errorf("must deny: %s", in)
		}
	}
	out.Reset()
	GitGuard(strings.NewReader(`{"tool_input":{"file_path":"x"}}`), &out)
	if out.Len() != 0 {
		t.Fatal("non-shell call must pass")
	}
}

// A throwaway repo whose check fails when app.txt contains BAD.
func TestStopChecksStates(t *testing.T) {
	dir := t.TempDir()
	StateDir = t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	write := func(name, s string) { os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644) }
	git("init", "-q")
	write("app.txt", "ok\n")
	git("add", "-A")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init")

	check := `findstr BAD app.txt >nul && exit 1 || exit 0`
	if runtime.GOOS != "windows" {
		check = `! grep -q BAD app.txt`
	}
	stop := func(active bool) bool {
		in := `{"cwd":` + quote(dir) + `,"stop_hook_active":` + map[bool]string{true: "true", false: "false"}[active] + `}`
		var out bytes.Buffer
		StopChecks(strings.NewReader(in), &out, check, 60*time.Second)
		return strings.Contains(out.String(), `"decision":"block"`)
	}
	steps := []struct {
		name   string
		before func()
		active bool
		block  bool
	}{
		{"clean tree passes", nil, false, false},
		{"unchanged tree is cached", nil, false, false},
		{"failure blocks", func() { write("app.txt", "ok\nBAD\n") }, false, true},
		{"retry 1 still blocks", nil, true, true},
		{"retry 2 gives up", nil, true, false},
		{"fresh stop blocks again", nil, false, true},
		{"untracked file, passing", func() { write("app.txt", "ok\n"); write("new.txt", "x\n") }, false, false},
	}
	for _, s := range steps {
		if s.before != nil {
			s.before()
		}
		if got := stop(s.active); got != s.block {
			t.Errorf("%s: block=%v, want %v", s.name, got, s.block)
		}
	}
}
