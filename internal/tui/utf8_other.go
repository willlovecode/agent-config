//go:build !windows

package tui

func utf8Terminal() bool { return utf8Locale() }

// Restore undoes terminal changes made by Detect (none outside Windows).
func Restore() {}
