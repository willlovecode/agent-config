// Command agtk sets up coding agents from a vetted catalog. See docs/wizard-design.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	agentconfig "github.com/willlovecode/agent-config"
	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/check"
	"github.com/willlovecode/agent-config/internal/hooks"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
)

var version = "0.1.0-dev" // set by the release build

const usage = `agtk %s: sets up Claude Code, Codex and Gemini CLI / Antigravity.

  agtk [wizard] [--plain]          full-screen setup wizard, one keypress per answer
                                   (--plain, a pipe or a dumb terminal: scrolling prompts)
  agtk wizard --project DIR        include project-scoped skills in the wizard
  agtk --answers file [--save]     install from an answers file without questions (CI)
  agtk apply [--profile dir|file] [--tier A|B] [--skip-tools]
                                   install a saved profile (default: the last one applied)
  agtk undo                        restore the files the last apply or add-checks changed
  agtk status                      what is installed, and what is left to do
  agtk scan                        show what agtk sees on this machine (never stored)
  agtk recommend [--profile ...]   catalog recommendations for this machine and the profile's answers
  agtk project add-checks <dir> "<command>" [--timeout 300]
                                   run the project's checks before the agent may stop
  agtk check [repo]                catalog, baseline, profile and budget checks (CI)
  agtk hook git-guard              (called by Claude Code and Codex)
  agtk hook stop-checks "<command>" [timeoutSeconds]

  --skip-tools can also be used with the wizard and --answers.
`

func main() {
	if len(os.Args) < 2 || (strings.HasPrefix(os.Args[1], "-") && !strings.Contains("-h --help --version", os.Args[1])) {
		if err := runWizard(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "agtk:", err)
			os.Exit(1)
		}
		return
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "wizard":
		err = runWizard(args)
	case "hook":
		err = runHook(args)
	case "apply":
		err = runApply(args)
	case "undo":
		err = install.Undo(install.StateDir(home()), os.Stdout)
	case "status":
		err = runStatus()
	case "scan":
		printScan(scan.Run())
	case "recommend":
		err = runRecommend(args)
	case "project":
		err = runProject(args)
	case "check":
		root := "."
		if len(args) > 0 {
			root = args[0]
		}
		if n := check.Run(root, os.Stdout); n > 0 {
			err = fmt.Errorf("%d check(s) failed", n)
		}
	case "version", "--version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
	default:
		err = fmt.Errorf("unknown command %q (agtk help)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agtk:", err)
		os.Exit(1)
	}
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "agtk: no home folder:", err)
		os.Exit(1)
	}
	return h
}

func runHook(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("hook name missing")
	}
	switch args[0] {
	case "git-guard":
		hooks.GitGuard(os.Stdin, os.Stdout)
	case "stop-checks":
		var cmd string
		timeout := 300
		if len(args) > 1 {
			cmd = args[1]
		}
		if len(args) > 2 {
			if n, err := strconv.Atoi(args[2]); err == nil && n > 0 {
				timeout = n
			}
		}
		hooks.StopChecks(os.Stdin, os.Stdout, cmd, time.Duration(timeout)*time.Second)
	default:
		return fmt.Errorf("unknown hook %q", args[0])
	}
	return nil
}

// loadProfile reads --profile, else the profile of the last apply.
func loadProfile(path string) (*profile.Profile, error) {
	if path == "" {
		st, err := install.ReadState(home())
		if os.IsNotExist(err) || (err == nil && st.Profile == "") {
			return nil, fmt.Errorf("choose a saved profile with --profile")
		}
		if err != nil {
			return nil, err
		}
		path = st.Profile
	}
	return profile.Load(path)
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	prof := fs.String("profile", "", "profile folder or YAML file")
	tier := fs.String("tier", "", "A (hooks) or B (rules only); default: from the scan")
	skip := fs.Bool("skip-tools", false, "don't run claude/npm commands (MCP servers, plugins, npm packages)")
	fs.Parse(args)
	p, err := loadProfile(*prof)
	if err != nil {
		return err
	}
	cat, err := catalog.Load(agentconfig.Assets)
	if err != nil {
		return err
	}
	rules, err := policy.System(cat)
	if err != nil {
		return err
	}
	t := strings.ToUpper(*tier)
	if t != "" && t != "A" && t != "B" {
		return fmt.Errorf("--tier must be A or B")
	}
	if t == "" || rules.Company(p.Answers) {
		s := scan.Discover()
		if t == "" || s.Tier == "B" {
			t = s.Tier
		}
		for _, why := range s.TierWhy {
			fmt.Println("Tier B because", why)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return install.Apply(install.Options{
		Home: home(), Assets: agentconfig.Assets, Catalog: cat, Profile: p, Policy: rules, Tier: t,
		Exe: exe, SkipTools: *skip, Version: version, Out: os.Stdout,
	})
}

func runStatus() error {
	h := home()
	st, err := install.ReadState(h)
	if os.IsNotExist(err) {
		fmt.Println("agtk has not applied a profile on this machine yet.")
		return nil
	}
	if err != nil {
		return err
	}
	name := st.Profile
	if name == "" {
		name = "(one-time setup)"
	}
	fmt.Printf("Profile:  %s\nTier:     %s\nApplied:  %s by agtk %s\nBinary:   %s\n",
		name, st.Tier, st.AppliedAt.Local().Format("2006-01-02 15:04"), st.Version, st.Hook)
	b, _ := os.ReadFile(filepath.Join(h, ".claude", "settings.json"))
	fmt.Printf("Claude Code git-guard hook: %v\n", strings.Contains(string(b), "git-guard"))
	b, _ = os.ReadFile(filepath.Join(h, ".codex", "hooks.json"))
	fmt.Printf("Codex git-guard hook:       %v (Codex runs it only once trusted: /hooks)\n", strings.Contains(string(b), "git-guard"))
	fmt.Printf("Backups:  %d (agtk undo restores the newest)\n", len(install.Backups(install.StateDir(h))))
	return nil
}

func printScan(s *scan.Result) {
	fmt.Printf("System:   %s/%s, %d CPU threads, %.0f GB RAM\n", s.OS, s.Arch, s.CPUs, s.RAMGB)
	fmt.Println("Programs:")
	for _, p := range s.Programs {
		v := p.Version
		if v == "" {
			v = "-"
		}
		if p.Note != "" {
			v += " (" + p.Note + ")"
		}
		fmt.Printf("  %-8s %s\n", p.Name, v)
	}
	if len(s.Policies) == 0 {
		fmt.Println("Policies: none found")
	}
	for _, p := range s.Policies {
		fmt.Println("Policy:  ", p)
	}
	fmt.Printf("Tier:     %s (%s)\n", s.Tier, map[string]string{"A": "hooks allowed", "B": "rules and permissions only"}[s.Tier])
	fmt.Println("Nothing of this scan is stored or sent.")
}

func runRecommend(args []string) error {
	fs := flag.NewFlagSet("recommend", flag.ExitOnError)
	prof := fs.String("profile", "", "profile folder or YAML file")
	fs.Parse(args)
	p, err := loadProfile(*prof)
	if err != nil {
		return err
	}
	cat, err := catalog.Load(agentconfig.Assets)
	if err != nil {
		return err
	}
	facts := p.Answers.Facts(scan.Run().Facts())
	rules, err := policy.System(cat)
	if err != nil {
		return err
	}
	chosen := map[string]bool{}
	for _, id := range p.Items {
		chosen[id] = true
	}
	for _, pi := range p.ProjectItems {
		for _, id := range pi.Items {
			chosen[id] = true
		}
	}
	total := 0
	for _, d := range cat.Recommend(facts) {
		if reason := rules.ItemBlock(p.Answers, d.Item); reason != "" {
			fmt.Printf("[-] %-18s %s\n", d.Item.ID, reason)
			continue
		}
		mark := "[ ]"
		if d.Recommended {
			mark = "[X]"
			total += catalog.Cost(agentconfig.Assets, d.Item)
		}
		note := ""
		wiredPerProject := d.Item.Kind == catalog.KindHook && d.Item.Scope == "project"
		if d.Recommended != chosen[d.Item.ID] && !wiredPerProject {
			note = "  <- profile differs"
		}
		fmt.Printf("%s %-18s %s%s\n", mark, d.Item.ID, d.Reason, note)
	}
	fmt.Printf("Always loaded by the recommended items: ~%d tokens (plus the profile's own instructions)\n", total)
	return nil
}

func runProject(args []string) error {
	if len(args) < 3 || args[0] != "add-checks" {
		return fmt.Errorf(`usage: agtk project add-checks <dir> "<command>" [--timeout 300]`)
	}
	fs := flag.NewFlagSet("add-checks", flag.ExitOnError)
	timeout := fs.Int("timeout", 300, "seconds the checks may take")
	fs.Parse(args[3:])
	return install.AddChecks(home(), args[1], args[2], *timeout, os.Stdout)
}
