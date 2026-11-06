package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathHint returns the lines telling the user how to put binDir on PATH, or nil when it
// already is. agtk never edits shell startup files; it only prints the command.
func PathHint(binDir, pathEnv, shell, goos string) []string {
	for _, d := range filepath.SplitList(pathEnv) {
		if d == "" {
			continue
		}
		if filepath.Clean(d) == filepath.Clean(binDir) || (goos == "windows" && strings.EqualFold(filepath.Clean(d), filepath.Clean(binDir))) {
			return nil
		}
	}
	intro := binDir + " is not on PATH, so `agtk` is not found in new terminals. agtk does not edit your shell files. To add it:"
	if goos == "windows" {
		return []string{intro, `  [Environment]::SetEnvironmentVariable('Path', '` + binDir + `;' + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')   # PowerShell, then open a new terminal`}
	}
	switch filepath.Base(shell) {
	case "fish":
		return []string{intro, "  fish_add_path " + binDir}
	case "zsh":
		return []string{intro, `  echo 'export PATH="` + binDir + `:$PATH"' >> ~/.zshrc   # then open a new terminal`}
	case "bash":
		return []string{intro, `  echo 'export PATH="` + binDir + `:$PATH"' >> ~/.bashrc   # then open a new terminal`}
	}
	return []string{intro, `  export PATH="` + binDir + `:$PATH"   # put this in your shell's startup file`}
}

// pathHint is PathHint for the binary Apply installed, using this process's PATH and shell.
func (a *applier) pathHint() {
	if _, err := os.Stat(a.exe); err != nil {
		return // no hook in the profile, so agtk was not copied into the state folder
	}
	for _, line := range PathHint(filepath.Dir(a.exe), os.Getenv("PATH"), os.Getenv("SHELL"), runtime.GOOS) {
		a.say("%s", line)
	}
}
