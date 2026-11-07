package screen

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT lets the console interpret escape sequences, and returns how to undo it.
// Consoles that cannot (before Windows 10 1511) get an error, and the plain wizard.
func enableVT(out *os.File) (func(), error) {
	h := windows.Handle(out.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return nil, err
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return func() {}, nil
	}
	if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil, err
	}
	return func() { windows.SetConsoleMode(h, mode) }, nil
}
