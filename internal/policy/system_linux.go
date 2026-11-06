package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func readSystemPolicy() ([]byte, string, error) {
	const file = "/etc/agtk/company-policy.yaml"
	st, err := os.Lstat(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read company policy at %s: %w", file, err)
	}
	dir, err := os.Lstat(filepath.Dir(file))
	if err != nil || !dir.IsDir() || !st.Mode().IsRegular() || !rootOwned(dir) || !rootOwned(st) {
		return nil, "", fmt.Errorf("company policy at %s needs a root-owned file and directory with no group or other write access", file)
	}
	b, err := os.ReadFile(file)
	return b, file, err
}

func rootOwned(st os.FileInfo) bool {
	s, ok := st.Sys().(*syscall.Stat_t)
	return ok && s.Uid == 0 && st.Mode().Perm()&0o022 == 0
}
