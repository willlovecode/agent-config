//go:build !windows && !linux

package policy

func readSystemPolicy() ([]byte, string, error) { return nil, "", nil }
