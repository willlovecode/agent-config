package screen

import (
	"bufio"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

type consoleTerm struct {
	in, out *os.File
	r       *bufio.Reader
	state   *term.State
}

func (c *consoleTerm) Write(p []byte) (int, error) { return c.out.Write(p) }
func (c *consoleTerm) ReadKey() (Key, error)       { return readKey(c.r) }

func (c *consoleTerm) Size() (int, int) {
	if w, h, err := term.GetSize(int(c.out.Fd())); err == nil && w > 0 && h > 0 {
		return w, h
	}
	return 80, 24
}

func (c *consoleTerm) raw() error {
	state, err := term.MakeRaw(int(c.in.Fd()))
	if err == nil {
		c.state = state
	}
	return err
}

func (c *consoleTerm) cooked() {
	if c.state != nil {
		term.Restore(int(c.in.Fd()), c.state)
		c.state = nil
	}
}

func (c *consoleTerm) Suspend() func() {
	c.cooked()
	return func() { c.raw() }
}

// openTerminal turns on escape sequences (Windows) and raw mode. The returned function
// puts the terminal back; it also runs when a signal ends the process, and the caller
// defers it so panics restore the terminal too.
func openTerminal(in, out *os.File) (Terminal, func(), error) {
	restoreOut, err := enableVT(out)
	if err != nil {
		return nil, nil, err
	}
	c := &consoleTerm{in: in, out: out, r: bufio.NewReader(in)}
	if err := c.raw(); err != nil {
		restoreOut()
		return nil, nil, err
	}
	var once sync.Once
	restore := func() {
		once.Do(func() {
			c.cooked()
			restoreOut()
		})
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		if _, ok := <-sig; ok {
			restore()
			io.WriteString(out, "\r\n")
			os.Exit(130)
		}
	}()
	return c, func() {
		signal.Stop(sig)
		close(sig)
		restore()
	}, nil
}
