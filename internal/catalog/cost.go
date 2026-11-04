package catalog

import (
	"bytes"
	"io/fs"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Tokens estimates tokens as chars/4, the same estimate checks/budget.mjs uses.
func Tokens(s string) int { return (len(s) + 2) / 4 }

// Frontmatter returns the YAML header of a Markdown file as name/description strings.
func Frontmatter(md []byte) (name, description string) {
	md = bytes.ReplaceAll(md, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(md, []byte("---\n")) {
		return "", ""
	}
	end := bytes.Index(md[4:], []byte("\n---"))
	if end < 0 {
		return "", ""
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	yaml.Unmarshal(md[4:4+end], &fm)
	return fm.Name, strings.TrimSpace(fm.Description)
}

// Cost is what an item adds to every session before the task starts: instruction text,
// listed skill descriptions and subagent descriptions. MCP tools load deferred and count 0.
func Cost(fsys fs.FS, it *Item) int {
	switch it.Kind {
	case KindInstructions:
		b, _ := fs.ReadFile(fsys, it.Vendored)
		return Tokens(string(b))
	case KindSkill:
		if it.ExplicitOnly {
			return 0
		}
		b, _ := fs.ReadFile(fsys, path.Join(it.Vendored, "SKILL.md"))
		_, desc := Frontmatter(b)
		return Tokens(it.ID + ": " + desc)
	case KindAgents:
		n := 0
		files, _ := fs.Glob(fsys, path.Join(it.Vendored, "*.md"))
		for _, f := range files {
			b, _ := fs.ReadFile(fsys, f)
			name, desc := Frontmatter(b)
			n += Tokens(name + ": " + desc)
		}
		return n
	}
	return 0
}
