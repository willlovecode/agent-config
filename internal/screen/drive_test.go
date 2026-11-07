package screen

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/willlovecode/agent-config/internal/profile"
)

// fakeTerm is a terminal that records what it is sent and types the keys it is given.
type fakeTerm struct {
	keys      *bufio.Reader
	sizes     [][2]int // handed out one per Size call; the last one repeats
	sizeCalls int
	out       strings.Builder
	suspended int
	resumed   int
}

func (f *fakeTerm) Write(p []byte) (int, error) { return f.out.Write(p) }
func (f *fakeTerm) ReadKey() (Key, error)       { return readKey(f.keys) }
func (f *fakeTerm) Size() (int, int) {
	i := min(f.sizeCalls, len(f.sizes)-1)
	f.sizeCalls++
	return f.sizes[i][0], f.sizes[i][1]
}
func (f *fakeTerm) Suspend() func() {
	f.suspended++
	return func() { f.resumed++ }
}

func newFakeTerm(keys string, sizes ...[2]int) *fakeTerm {
	if len(sizes) == 0 {
		sizes = [][2]int{{80, 24}}
	}
	return &fakeTerm{keys: bufio.NewReader(strings.NewReader(keys)), sizes: sizes}
}

func TestDriveClearsBeforeEveryPageAndTakesSingleKeys(t *testing.T) {
	s, got := testSession(t, claude(), codex())
	// No Enter after the choice: "y" alone answers the first page.
	term := newFakeTerm("y" + strings.Repeat("\r", 10) + "y")
	drive(s, term, false)
	if installed, err := s.Result(); !installed || err != nil || got.calls != 1 {
		t.Fatalf("installed=%v err=%v calls=%d", installed, err, got.calls)
	}
	out := term.out.String()
	frames := strings.Split(out, clearScreen)
	if frames[0] != "" {
		t.Fatalf("something was drawn before the first clear: %q", frames[0])
	}
	frames = frames[1:]
	// 12 pages: ask, results, 7 questions, items, review, save, confirm, applying, done
	if len(frames) < 14 {
		t.Fatalf("%d frames:\n%s", len(frames), out)
	}
	for i, f := range frames {
		// Pages end lines with \r\n for a raw terminal; only progress output, written cooked, has bare ones.
		if strings.Contains(strings.ReplaceAll(f, "\r\n", ""), "\n") && !strings.Contains(f, "Done (tier A).") {
			t.Fatalf("frame %d has a bare newline, which a raw terminal would not return from: %q", i, f)
		}
		if strings.Contains(f, "\x1b") {
			t.Fatalf("frame %d carries escape sequences without colour: %q", i, f)
		}
	}
	if !strings.Contains(frames[0], "This is the agtk setup wizard") ||
		!strings.Contains(frames[len(frames)-1], "Setup complete") {
		t.Errorf("first and last frame:\n%s\n---\n%s", frames[0], frames[len(frames)-1])
	}
	// The apply output went to the terminal between pages, with a pause before the last one.
	if !strings.Contains(out, "Done (tier A).") || !strings.Contains(out, "Press any key to continue.") {
		t.Errorf("progress output missing:\n%s", out)
	}
	if term.suspended != term.resumed || term.suspended < 3 {
		t.Errorf("raw mode was left %d times and taken back %d times; the check, the plan and the apply each leave it", term.suspended, term.resumed)
	}
}

func TestDriveReadsTheSizeForEveryPage(t *testing.T) {
	s, _ := testSession(t, claude(), codex())
	term := newFakeTerm("yq", [2]int{80, 24}, [2]int{80, 24}, [2]int{50, 10}, [2]int{50, 10}, [2]int{100, 40})
	drive(s, term, false)
	if term.sizeCalls < 4 {
		t.Fatalf("size read %d times", term.sizeCalls)
	}
	// Page 3 (the results) was drawn at 50x10, which is too small; the key after it was ignored.
	if !strings.Contains(term.out.String(), "This terminal is too small") {
		t.Errorf("a resize must be noticed:\n%s", term.out.String())
	}
}

func TestDriveQuitAndEndOfInput(t *testing.T) {
	for _, keys := range []string{"q", "y\x03", "", "y\r"} {
		s, got := testSession(t, claude())
		term := newFakeTerm(keys)
		drive(s, term, false)
		installed, err := s.Result()
		if installed || err != nil || got.calls != 0 || !strings.Contains(term.out.String(), "Left setup. Nothing was written.") {
			t.Errorf("keys %q: installed=%v err=%v calls=%d\n%s", keys, installed, err, got.calls, term.out.String())
		}
	}
}

func TestDriveSmallTerminalHandsOverToPlainMode(t *testing.T) {
	s, got := testSession(t, claude())
	term := newFakeTerm("w", [2]int{40, 10})
	drive(s, term, false)
	if _, err := s.Result(); err != ErrPlain || got.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, got.calls)
	}
	if strings.Contains(term.out.String(), "Left setup") {
		t.Errorf("handing over is not leaving:\n%s", term.out.String())
	}
}

func TestDriveColorStylesTheTitle(t *testing.T) {
	s, _ := testSession(t, claude())
	term := newFakeTerm("q")
	drive(s, term, true)
	if !strings.Contains(term.out.String(), "\x1b[1m") || !strings.Contains(term.out.String(), "\x1b[0m") {
		t.Errorf("no styles with colour on: %q", term.out.String())
	}
}

func TestDriveStreamsApplyProgress(t *testing.T) {
	s, _ := testSession(t, claude())
	s.cfg.Apply = func(_ *profile.Profile, _ bool, out io.Writer) error {
		io.WriteString(out, "Skills\n  install  handoff\n")
		return nil
	}
	term := newFakeTerm("y" + strings.Repeat("\r", 10) + "y" + "x")
	drive(s, term, false)
	out := term.out.String()
	progress := strings.Index(out, "Skills\n  install  handoff")
	title, done := strings.Index(out, "Applying your setup"), strings.LastIndex(out, "Setup complete")
	if !(title >= 0 && title < progress && progress < done) {
		t.Errorf("order: applying page %d, progress %d, done page %d\n%s", title, progress, done, out)
	}
}
