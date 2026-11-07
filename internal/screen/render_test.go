package screen

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// pages are reached by key presses from the first page. Items and Review are left out:
// they list the catalog, which changes more often than the screens do.
var pages = []struct {
	name, keys string
	small      bool // drawn at 50x12 whatever the test size
}{
	{"system", "", false},
	{"system-result", "y", false},
	{"profile-use", "y\r", false},
	{"work", "y\r\r", false},
	{"languages", "y\r\r\r", false},
	{"languages-page-2", "y\r\r\rn", false},
	{"tools", "y\r\r\r\r\r", false},
	{"tools-blocked", "y\r\r\r\r\r12\r", false},
	{"plans", "y\r\r\r\r\r\r", false},
	{"plans-details", "y\r\r\r\r\r\r?1", false},
	{"models", "y\r\r\r\r\r\r2\r", false},
	{"routing", "y\r\r\r\r\r\r2\r123\r", false},
	{"save", "y\r\r\r\r\r\r\r\r\r", false},
	{"confirm", "y\r\r\r\r\r\r\r\r\r\r", false},
	{"done", "y\r\r\r\r\r\r\r\r\r\r\ry", false},
	{"left", "q", false},
	{"too-small", "", true},
}

func TestGolden(t *testing.T) {
	for _, tc := range []struct {
		name  string
		env   Env
		color bool
	}{
		{"80x24", Env{Width: 80, Height: 24}, false},
		{"80x24-color", Env{Width: 80, Height: 24}, true},
		{"60x20", Env{Width: 60, Height: 20}, false},
		{"60x20-color", Env{Width: 60, Height: 20}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			for _, pg := range pages {
				s, _ := testSession(t, claude(), codex())
				s.cfg.Home = t.TempDir()
				env := tc.env
				env.Color = tc.color
				s.SetEnv(env)
				press(s, pg.keys)
				if pg.small {
					env.Width, env.Height = 50, 12
					s.SetEnv(env)
				}
				fmt.Fprintf(&b, "===== %s\n%s\n", pg.name, s.View())
			}
			path := filepath.Join("testdata", tc.name+".golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run go test ./internal/screen -update)", err)
			}
			if got := strings.ReplaceAll(string(want), "\r\n", "\n"); got != b.String() {
				t.Errorf("%s differs from the pages; run go test ./internal/screen -update and review the diff", path)
				gotLines, wantLines := strings.Split(b.String(), "\n"), strings.Split(got, "\n")
				for i := range min(len(gotLines), len(wantLines)) {
					if gotLines[i] != wantLines[i] {
						t.Errorf("first difference at line %d:\n got  %q\n want %q", i+1, gotLines[i], wantLines[i])
						break
					}
				}
			}
		})
	}
}

// Pages carry no colour escapes unless asked, and a coloured page is the plain one plus escapes.
func TestColorOnlyAddsEscapes(t *testing.T) {
	for _, pg := range pages {
		s, _ := testSession(t, claude(), codex())
		s.cfg.Home = t.TempDir()
		s.SetEnv(Env{Width: 80, Height: 24})
		press(s, pg.keys)
		plain := s.View()
		s.SetEnv(Env{Width: 80, Height: 24, Color: true})
		colored := s.View()
		if strings.Contains(plain, "\x1b") {
			t.Errorf("%s: escape in the plain page", pg.name)
		}
		strip := strings.NewReplacer("\x1b[1m", "", "\x1b[2m", "", "\x1b[0m", "").Replace(colored)
		if strip != plain {
			t.Errorf("%s: colour changes more than styles:\n%s\n---\n%s", pg.name, strip, plain)
		}
		for _, r := range plain {
			if r > '~' {
				t.Errorf("%s: non-ASCII %q in the page", pg.name, r)
				break
			}
		}
	}
}
