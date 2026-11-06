package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPathHint(t *testing.T) {
	bin := "/home/u/.agtk/bin"
	if h := PathHint(bin, strings.Join([]string{"/usr/bin", bin + "/", "/bin"}, string(filepath.ListSeparator)), "/bin/bash", "linux"); h != nil {
		t.Errorf("already on PATH, got hint %v", h)
	}
	for shell, want := range map[string]string{
		"/usr/bin/fish": "fish_add_path /home/u/.agtk/bin",
		"/bin/zsh":      `export PATH="/home/u/.agtk/bin:$PATH"' >> ~/.zshrc`,
		"/bin/bash":     `>> ~/.bashrc`,
		"":              `export PATH="/home/u/.agtk/bin:$PATH"`,
	} {
		h := strings.Join(PathHint(bin, "/usr/bin", shell, "linux"), "\n")
		if !strings.Contains(h, want) {
			t.Errorf("shell %q: want %q in\n%s", shell, want, h)
		}
	}
	win := strings.Join(PathHint(`C:\Users\u\.agtk\bin`, `C:\Windows`, "", "windows"), "\n")
	if !strings.Contains(win, "SetEnvironmentVariable") || !strings.Contains(win, `C:\Users\u\.agtk\bin;`) {
		t.Errorf("windows hint: %s", win)
	}
}
