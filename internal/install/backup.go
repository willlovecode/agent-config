package install

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// A backup session records, before the first change to each path in one apply, what was
// there: a copy of the file or folder, or the fact that nothing was. Undo replays it in
// reverse. Commands run against other programs (claude mcp add, npm install) are listed
// but cannot be undone.

type entry struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"` // file, dir, link, external
	Existed  bool   `json:"existed"`
	Backup   string `json:"backup,omitempty"` // name inside the session folder
	Target   string `json:"target,omitempty"` // where a link pointed
	Describe string `json:"describe,omitempty"`
}

type session struct {
	dir     string
	dry     bool    // record what would change, write nothing
	Entries []entry `json:"entries"`
	seen    map[string]bool
}

func newSession(stateDir string, dry bool) (*session, error) {
	if dry {
		return &session{dry: true, seen: map[string]bool{}}, nil
	}
	base := filepath.Join(stateDir, "backups", time.Now().UTC().Format("20060102T150405.000Z"))
	dir := base // UTC, so names sort in order across DST changes
	for i := 2; exists(dir) || exists(dir+".undone"); i++ {
		dir = fmt.Sprintf("%s-%03d", base, i)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &session{dir: dir, seen: map[string]bool{}}, nil
}

func (s *session) save() error {
	if s.dry {
		return nil
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, "manifest.json"), b, 0o644)
}

// remember backs up path once per session, before it is first changed.
func (s *session) remember(path, kind string) error {
	key := strings.ToLower(filepath.Clean(path))
	if s.seen[key] {
		return nil
	}
	s.seen[key] = true
	e := entry{Path: path, Kind: kind}
	if st, err := os.Lstat(path); err == nil && isLink(st) {
		e.Existed, e.Kind = true, "link"
		if e.Target, err = os.Readlink(path); err != nil {
			return err
		}
	} else if err == nil && s.dry {
		e.Existed = true
	} else if err == nil {
		e.Existed = true
		e.Backup = fmt.Sprintf("%03d-%s", len(s.Entries), filepath.Base(path))
		dst := filepath.Join(s.dir, e.Backup)
		if st.IsDir() {
			e.Kind = "dir"
			if err := copyDir(path, dst); err != nil {
				return err
			}
		} else if err := copyFile(path, dst); err != nil {
			return err
		}
	}
	s.Entries = append(s.Entries, e)
	return s.save()
}

// finish drops the session folder when nothing was changed; it returns the folder kept, or "".
func (s *session) finish() string {
	if s.dry {
		return ""
	}
	if len(s.Entries) == 0 {
		os.Remove(s.dir)
		return ""
	}
	return s.dir
}

func (s *session) external(describe string) error {
	s.Entries = append(s.Entries, entry{Kind: "external", Describe: describe})
	return s.save()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 && p != src {
			return fmt.Errorf("%s holds a link (%s); agtk does not back up links inside folders", src, rel)
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

// Backups lists the session folders, newest first, without the undone ones.
func Backups(stateDir string) []string {
	dirs, _ := filepath.Glob(filepath.Join(stateDir, "backups", "*"))
	var out []string
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(d, "manifest.json")); err == nil && !strings.HasSuffix(d, ".undone") {
			out = append(out, d)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

// Undo restores the newest backup session and marks it undone.
func Undo(stateDir string, w io.Writer) error {
	list := Backups(stateDir)
	if len(list) == 0 {
		return fmt.Errorf("no backup to restore in %s", filepath.Join(stateDir, "backups"))
	}
	dir := list[0]
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	var s session
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	fmt.Fprintf(w, "Restoring %s\n", dir)
	for i := len(s.Entries) - 1; i >= 0; i-- {
		e := s.Entries[i]
		switch {
		case e.Kind == "external":
			fmt.Fprintf(w, "  not undone (outside agtk's files): %s\n", e.Describe)
			continue
		case !e.Existed:
			if e.Kind == "dir" {
				err = os.RemoveAll(e.Path)
			} else {
				err = os.Remove(e.Path) // a link is removed, never its target
			}
			if os.IsNotExist(err) {
				err = nil
			}
			fmt.Fprintf(w, "  removed  %s\n", e.Path)
		case e.Kind == "link":
			if err = removePath(e.Path); err == nil {
				err = makeLink(e.Path, e.Target)
			}
			fmt.Fprintf(w, "  relinked %s -> %s\n", e.Path, e.Target)
		case e.Kind == "dir":
			if err = os.RemoveAll(e.Path); err == nil {
				err = copyDir(filepath.Join(dir, e.Backup), e.Path)
			}
			fmt.Fprintf(w, "  restored %s\n", e.Path)
		default:
			data, rerr := os.ReadFile(filepath.Join(dir, e.Backup))
			if rerr != nil {
				return rerr
			}
			err = replaceFile(e.Path, data, fileMode(e.Path))
			fmt.Fprintf(w, "  restored %s\n", e.Path)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", e.Path, err)
		}
	}
	// The state names this apply; with it undone, there is no applied profile to report.
	if st, err := ReadState(filepath.Dir(stateDir)); err == nil && filepath.Clean(st.Backup) == filepath.Clean(dir) {
		os.Remove(filepath.Join(stateDir, "state.json"))
	}
	return os.Rename(dir, dir+".undone")
}

// replaceFile writes data to path through a temporary file. A running program (agtk.exe
// itself, while a hook runs) cannot be overwritten on Windows but can be renamed aside.
func replaceFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".agtk-new"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		old := fmt.Sprintf("%s.old-%d", path, time.Now().UnixNano())
		if os.Rename(path, old) != nil {
			os.Remove(tmp)
			return err
		}
		return os.Rename(tmp, path)
	}
	return nil
}

// removePath removes a file, folder or link (never a link's target). A running program
// cannot be deleted on Windows, so it is renamed aside instead.
func removePath(p string) error {
	st, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.IsDir() && !isLink(st) {
		return os.RemoveAll(p)
	}
	if err := os.Remove(p); err != nil {
		if os.Rename(p, fmt.Sprintf("%s.old-%d", p, time.Now().UnixNano())) == nil {
			return nil
		}
		return err
	}
	return nil
}

// isLink reports a symlink or a Windows junction (Go reports junctions as irregular).
func isLink(st fs.FileInfo) bool { return st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

func fileMode(p string) os.FileMode {
	if strings.HasSuffix(strings.ToLower(p), ".exe") || filepath.Base(filepath.Dir(p)) == "bin" {
		return 0o755
	}
	return 0o644
}
