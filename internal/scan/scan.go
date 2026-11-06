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
	path       string
}

func (p *Program) Available() bool { return p.path != "" || p.Version != "" }

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
		p("claude", "claude_code"), p("codex", "codex"), p("gemini", "gemini"), p("agy", "gemini"), p("git", "git"), p("node", "node"),
		p("npm", "npm"), p("java", "jdk", "-version"), p("mvn", "maven"), p("gradle", "gradle"),
		p("python3", "python"), p("python", "python"), p("uv", "uv"), p("go", "go", "version"),
		p("docker", "docker"),
	}
}

// Run discovers programs and asks the detected commands for their versions.
func Run() *Result {
	r := Discover()
	var wg sync.WaitGroup
	for _, p := range r.Programs {
		if !p.Available() {
			continue
		}
		wg.Add(1)
		go func() { defer wg.Done(); probe(p) }()
	}
	wg.Wait()
	return r
}

// Discover inspects the machine without starting any of the detected programs.
func Discover() *Result {
	r := PolicyOnly()
	r.CPUs, r.RAMGB = runtime.NumCPU(), ramGB()
	r.Programs = programs()
	for _, p := range r.Programs {
		locate(p)
	}
	return r
}

// PolicyOnly checks the managed hook settings needed to build an install plan.
func PolicyOnly() *Result {
	r := &Result{OS: runtime.GOOS, Arch: runtime.GOARCH}
	r.policies()
	return r
}

func locate(p *Program) {
	path, err := exec.LookPath(p.Name)
	if err != nil {
		return
	}
	// The Microsoft Store python stub opens the Store instead of running.
	if strings.Contains(strings.ToLower(path), `\windowsapps\`) && strings.HasPrefix(p.Name, "python") {
		p.Note = "Microsoft Store stub, not a real install"
		return
	}
	p.path = path
}

func probe(p *Program) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, p.path, p.Args...).CombinedOutput()
	if v := versionRe.FindString(string(out)); v != "" {
		p.Version = v
	} else {
		p.Note = "found, version unknown"
	}
}

func (r *Result) policies() {
	r.Tier = "A"
	var claudeFiles []string
	codexRequirements := "/etc/codex/requirements.toml"
	if r.OS == "windows" {
		codexRequirements = ""
		if data := os.Getenv("ProgramData"); data != "" {
			codexRequirements = filepath.Join(data, "OpenAI", "Codex", "requirements.toml")
		}
		claudeFiles = []string{
			filepath.Join(os.Getenv("ProgramFiles"), "ClaudeCode", "managed-settings.json"),
			filepath.Join(os.Getenv("ProgramData"), "ClaudeCode", "managed-settings.json"),
		}
	} else {
		claudeFiles = []string{"/etc/claude-code/managed-settings.json"}
	}
	if codexRequirements != "" {
		if _, err := os.Stat(codexRequirements); err == nil {
			r.Policies = append(r.Policies, codexRequirements+" (Codex requirements)")
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
// string or a presence flag per program fact (claude_code, codex, jdk, node, python, ...).
func (r *Result) Facts() catalog.Facts {
	f := catalog.Facts{"os": r.OS, "arch": r.Arch, "cpus": float64(r.CPUs), "ram_gb": r.RAMGB, "tier": r.Tier}
	for _, p := range r.Programs {
		if _, hasVersion := f[p.Fact].(string); hasVersion {
			continue
		}
		if p.Version != "" {
			f[p.Fact] = p.Version
		} else {
			found, _ := f[p.Fact].(bool)
			f[p.Fact] = found || p.Available()
		}
	}
	return f
}

// Find returns a detected program for a fact, preferring one with a known version.
func (r *Result) Find(fact string) *Program {
	var found *Program
	for _, p := range r.Programs {
		if p.Fact != fact || !p.Available() {
			continue
		}
		if p.Version != "" {
			return p
		}
		found = p
	}
	return found
}
