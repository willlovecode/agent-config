package screen

import (
	"io"
	"os"

	"golang.org/x/term"

	"github.com/willlovecode/agent-config/internal/wizard"
)

// Terminal is the part of the program that touches the console. Tests replace it.
type Terminal interface {
	io.Writer
	Size() (width, height int)
	ReadKey() (Key, error)
	// Suspend gives the console back to cooked mode, for work that prints or runs
	// programs, and returns the function that takes raw mode again.
	Suspend() (resume func())
}

// Usable says whether the full-screen wizard can run: both ends are a terminal, and not
// a dumb one, which cannot clear the screen.
func Usable(in, out *os.File, getenv func(string) string) bool {
	if in == nil || out == nil || getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

// Color says whether to use bold and dim text: not with NO_COLOR set or a dumb terminal.
func Color(getenv func(string) string) bool {
	return getenv("NO_COLOR") == "" && getenv("TERM") != "dumb"
}

// Run shows the wizard on the terminal and returns whether anything was installed.
// ErrPlain means it cannot or should not run here: run the scrolling wizard instead.
func Run(fresh func() *wizard.Wizard, cfg Config, in, out *os.File) (bool, error) {
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	t, closeTerm, err := openTerminal(in, out)
	if err != nil {
		return false, ErrPlain
	}
	defer closeTerm()
	s := NewSession(fresh, cfg)
	drive(s, t, Color(cfg.Getenv))
	return s.Result()
}

// drive draws the session and feeds it keys until it is done.
func drive(s *Session, t Terminal, color bool) {
	for {
		w, h := t.Size() // read again for every page, so resizing works
		s.SetEnv(Env{Width: w, Height: h, Color: color})
		if s.plain {
			return
		}
		io.WriteString(t, s.Frame())
		if s.Done() {
			return
		}
		if stream, ok := s.Pending(); ok {
			resume := t.Suspend()
			s.RunJob(t)
			resume()
			if stream {
				io.WriteString(t, "\r\nPress any key to continue.")
				t.ReadKey()
			}
			continue
		}
		k, err := t.ReadKey()
		if err != nil {
			k = KeyEOF
		}
		s.Press(k)
	}
}
