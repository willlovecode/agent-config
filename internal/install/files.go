package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/willlovecode/agent-config/internal/ojson"
)

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// writeFile changes a file through the backup session.
func (a *applier) writeFile(p string, data []byte) error {
	if old, err := os.ReadFile(p); err == nil && string(old) == string(data) {
		return nil
	}
	if err := a.s.remember(p, "file"); err != nil {
		return err
	}
	if a.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (a *applier) readJSON(p string) (*ojson.Object, error) {
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return ojson.New(), nil
	}
	if err != nil {
		return nil, err
	}
	o, err := ojson.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return o, nil
}

func (a *applier) writeJSON(p string, o *ojson.Object) error {
	b, err := ojson.Format(o)
	if err != nil {
		return err
	}
	return a.writeFile(p, b)
}

// copyFromAssets copies an embedded folder to dest (which must not exist).
func copyFromAssets(fsys fs.FS, src, dest string) error {
	return fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(p, src), "/")))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasPrefix(string(b), "#!") {
			mode = 0o755 // keep vendored scripts runnable on Linux
		}
		return os.WriteFile(target, b, mode)
	})
}

// treeHash is the 16-hex-digit content hash the skill pin files carry, computed the way
// skills/install-skills.ps1 does: SHA-256 of the concatenated per-file hashes.
func treeHash(fsys fs.FS, root string) (string, error) {
	var files []string
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	// Ordinal, ignoring case, on backslash paths: [StringComparer]::OrdinalIgnoreCase in PowerShell.
	key := func(p string) string { return strings.ToUpper(strings.ReplaceAll(p, "/", `\`)) }
	sort.Slice(files, func(i, j int) bool { return key(files[i]) < key(files[j]) })
	var all strings.Builder
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(b)
		all.WriteString(strings.ToUpper(hex.EncodeToString(sum[:])))
	}
	sum := sha256.Sum256([]byte(all.String()))
	return strings.ToUpper(hex.EncodeToString(sum[:]))[:16], nil
}

// link makes a directory link: a junction on Windows (no admin rights needed), a symlink elsewhere.
func (a *applier) link(linkPath, target string) error {
	if exists(linkPath) {
		return nil
	}
	if err := a.s.remember(linkPath, "link"); err != nil {
		return err
	}
	if a.DryRun {
		return nil
	}
	return makeLink(linkPath, target)
}

func makeLink(linkPath, target string) error {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "mklink", "/J", linkPath, target).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mklink /J %s: %v %s", linkPath, err, out)
		}
		return nil
	}
	return os.Symlink(target, linkPath)
}

var tomlTableRe = regexp.MustCompile(`^\s*\[`)

// setTOMLTopKeys sets each key in the top-level (pre-[table]) part of a TOML text,
// leaving every other line as it was. It returns the new text and what changed.
func setTOMLTopKeys(text string, keys []string, values map[string]string) (string, []string) {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if text == "" {
		lines = nil
	}
	end := len(lines)
	for i, l := range lines {
		if tomlTableRe.MatchString(l) {
			end = i
			break
		}
	}
	var changes []string
	for _, k := range keys {
		line := k + " = " + values[k]
		keyRe := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(k) + `\s*=`)
		found := false
		for i := 0; i < end; i++ {
			if keyRe.MatchString(lines[i]) {
				if lines[i] != line {
					changes = append(changes, strings.TrimSpace(lines[i])+" -> "+line)
				}
				lines[i], found = line, true
				break
			}
		}
		if !found {
			lines = append([]string{line}, lines...)
			end++
			changes = append(changes, "added "+line)
		}
	}
	return strings.Join(lines, nl), changes
}

// tomlTopValues reads `key = value` lines for the given keys from a TOML snippet.
func tomlTopValues(text string, keys ...string) ([]string, map[string]string) {
	var order []string
	vals := map[string]string{}
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if tomlTableRe.MatchString(l) {
			break
		}
		k, v, ok := strings.Cut(l, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || !contains(keys, k) {
			continue
		}
		if _, dup := vals[k]; !dup {
			order = append(order, k)
		}
		vals[k] = v
	}
	return order, vals
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func slash(p string) string { return path.Clean(filepath.ToSlash(p)) }
