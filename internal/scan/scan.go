// Package scan looks at the machine: OS, memory, installed tools and company policies
// that limit hooks. The result lives in memory only; it is never written or sent.
package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/willlovecode/agent-config/internal/catalog"
)

// Program is one tool looked up on PATH.
type Program struct {
	Name, Fact string // command name, fact name for rules
	Args       []string
	Version    string // "" when not found
	Note       string
}

type Result struct {
	OS, Arch string
	CPUs     int
	RAMGB    float64
	Programs []*Program
	Policies []string // managed policy files found, with what they do
	Tier     string   // A: hooks allowed, B: rules and permissions only
	TierWhy  []string
}

var versionRe = regexp.MustCompile(`\d+(\.\d+)+`)

func programs() []*Program {
	p := func(name, fact string, args ...string) *Program {
		if len(args) == 0 {
			args = []string{"--version"}
		}
		return &Program{Name: name, Fact: fact, Args: args}
	}
	return []*Program{
		p("claude", "claude_code"), p("codex", "codex"), p("git", "git"), p("node", "node"),
		p("npm", "npm"), p("java", "jdk", "-version"), p("mvn", "maven"), p("gradle", "gradle"),
		p("python3", "python"), p("python", "python"), p("uv", "uv"), p("go", "go", "version"),
		p("docker", "docker"),
	}
}

// Run scans the machine. It starts no network connections.
func Run() *Result {
	r := &Result{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU(), RAMGB: ramGB()}
	r.Programs = programs()
	var wg sync.WaitGroup
	for _, p := range r.Programs {
		wg.Add(1)
		go func() { defer wg.Done(); probe(p) }()
	}
	wg.Wait()
	r.policies()
	return r
}

func probe(p *Program) {
	path, err := exec.LookPath(p.Name)
	if err != nil {
		return
	}
	// The Microsoft Store python stub opens the Store instead of running.
	if strings.Contains(strings.ToLower(path), `\windowsapps\`) && strings.HasPrefix(p.Name, "python") {
		p.Note = "Microsoft Store stub, not a real install"
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, path, p.Args...).CombinedOutput()
	if v := versionRe.FindString(string(out)); v != "" {
		p.Version = v
	} else {
		p.Note = "found, version unknown"
	}
}

func (r *Result) policies() {
	r.Tier = "A"
	var claudeFiles []string
	if r.OS == "windows" {
		claudeFiles = []string{
			filepath.Join(os.Getenv("ProgramFiles"), "ClaudeCode", "managed-settings.json"),
			filepath.Join(os.Getenv("ProgramData"), "ClaudeCode", "managed-settings.json"),
		}
	} else {
		claudeFiles = []string{"/etc/claude-code/managed-settings.json"}
		if _, err := os.Stat("/etc/codex/requirements.toml"); err == nil {
			r.Policies = append(r.Policies, "/etc/codex/requirements.toml (Codex requirements)")
		}
	}
	for _, f := range claudeFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var ms struct {
			DisableAllHooks       bool `json:"disableAllHooks"`
			AllowManagedHooksOnly bool `json:"allowManagedHooksOnly"`
		}
		json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &ms)
		note := f + " (Claude Code managed settings)"
		if ms.DisableAllHooks || ms.AllowManagedHooksOnly {
			r.Tier = "B"
			r.TierWhy = append(r.TierWhy, "Claude Code managed settings block user hooks ("+f+")")
			note += ": blocks user hooks"
		}
		r.Policies = append(r.Policies, note)
	}
}

// Facts turns the scan into rule facts: os, arch, cpus, ram_gb, tier, and one version
// string per program fact (claude_code, codex, jdk, node, python, ...).
func (r *Result) Facts() catalog.Facts {
	f := catalog.Facts{"os": r.OS, "arch": r.Arch, "cpus": float64(r.CPUs), "ram_gb": r.RAMGB, "tier": r.Tier}
	for _, p := range r.Programs {
		if s, _ := f[p.Fact].(string); s == "" {
			f[p.Fact] = p.Version
		}
	}
	return f
}

// Found reports the version of a program fact, or "".
func (r *Result) Found(fact string) string {
	for _, p := range r.Programs {
		if p.Fact == fact && p.Version != "" {
			return p.Version
		}
	}
	return ""
}
