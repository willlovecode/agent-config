package tui

import "golang.org/x/sys/windows"

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	getConsoleOutCP = kernel32.NewProc("GetConsoleOutputCP")
	setConsoleOutCP = kernel32.NewProc("SetConsoleOutputCP")
	restoreCodepage uintptr
)

// utf8Terminal switches the console to codepage 65001 (UTF-8) for the wizard's lifetime;
// Restore puts the old one back. If the switch fails, the wizard draws in ASCII.
func utf8Terminal() bool {
	cp, _, _ := getConsoleOutCP.Call()
	if cp == 65001 {
		return true
	}
	if ok, _, _ := setConsoleOutCP.Call(65001); ok == 0 {
		return false
	}
	restoreCodepage = cp
	return true
}

// Restore undoes the codepage switch made by Detect.
func Restore() {
	if restoreCodepage != 0 {
		setConsoleOutCP.Call(restoreCodepage)
		restoreCodepage = 0
	}
}
