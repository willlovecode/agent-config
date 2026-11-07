package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	agentconfig "github.com/willlovecode/agent-config"
	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/plain"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/screen"
	"github.com/willlovecode/agent-config/internal/wizard"
)

// runWizard starts interactive setup or applies an unattended answers file.
func runWizard(args []string) error { return wizardMain(args, os.Stdin, os.Stdout) }

func wizardMain(args []string, in, out *os.File) error {
	fs := flag.NewFlagSet("agtk", flag.ExitOnError)
	fs.Bool("ascii", false, "accepted for compatibility (prompts use ASCII)")
	plainMode := fs.Bool("plain", false, "scrolling prompts, Enter after each answer (automatic for pipes and dumb terminals)")
	answers := fs.String("answers", "", "install from this answers file without questions")
	save := fs.Bool("save", false, "with --answers: also save the profile for `agtk apply`")
	skipTools := fs.Bool("skip-tools", false, "skip the claude/npm commands (MCP, plugins, npm packages)")
	project := fs.String("project", "", "offer project-scoped skills for this folder")
	fs.Parse(args)

	cat, err := catalog.Load(agentconfig.Assets)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	preflight := func(discover bool) (*scan.Result, policy.Policy, error) {
		rules, err := policy.System(cat)
		if err != nil {
			return nil, rules, err
		}
		if discover {
			return scan.Discover(), rules, nil
		}
		return scan.PolicyOnly(), rules, nil
	}
	projectDir := ""
	if *project != "" {
		if *answers != "" {
			return fmt.Errorf("set project_items in the answers file when using --answers")
		}
		projectDir, err = filepath.Abs(*project)
		if err != nil {
			return err
		}
		if st, err := os.Stat(projectDir); err != nil || !st.IsDir() {
			return fmt.Errorf("--project needs an existing folder")
		}
	}
	newWizard := func() *wizard.Wizard {
		w := wizard.New(cat, agentconfig.Assets, &scan.Result{})
		w.Project = projectDir
		return w
	}
	h := home()
	opts := func(p *profile.Profile, save bool, out io.Writer) (install.Options, error) {
		rules, err := policy.System(cat)
		if err != nil {
			return install.Options{}, err
		}
		if save {
			path, err := wizard.ProfilePath()
			if err != nil {
				return install.Options{}, err
			}
			p = p.At(path)
		}
		return install.Options{Home: h, Assets: agentconfig.Assets, Catalog: cat, Profile: p, Policy: rules, Tier: scan.PolicyOnly().Tier,
			Exe: exe, SaveProfile: save, SkipTools: *skipTools, Version: version, Out: out}, nil
	}
	plan := func(p *profile.Profile, save bool) ([]install.Change, error) {
		options, err := opts(p, save, io.Discard)
		if err != nil {
			return nil, err
		}
		return install.Plan(options)
	}
	apply := func(p *profile.Profile, save bool, out io.Writer) error {
		options, err := opts(p, save, out)
		if err != nil {
			return err
		}
		return install.Apply(options)
	}

	if *answers != "" {
		p, err := profile.Load(*answers)
		if err != nil {
			return err
		}
		s, rules, err := preflight(true)
		if err != nil {
			return err
		}
		w := newWizard()
		w.SetScan(s)
		w.SetPolicy(rules)
		if p.Items == nil {
			if err := w.Load(p); err != nil {
				return err
			}
			chosen := w.Profile()
			chosen.Answers.Autonomy, chosen.Answers.Budget = p.Answers.Autonomy, p.Answers.Budget
			p.Answers, p.Items = chosen.Answers, chosen.Items
			// The source file does not contain the resolved choices for agtk apply.
			p.Path = ""
		}
		return apply(p, *save, out)
	}
	if !*plainMode && screen.Usable(in, out, os.Getenv) {
		cfg := screen.Config{Home: h, Preflight: preflight, Plan: plan, Apply: apply}
		if _, err := screen.Run(newWizard, cfg, in, out); !errors.Is(err, screen.ErrPlain) {
			return err
		}
	}
	cfg := plain.Config{Home: h, Preflight: preflight, Plan: plan, Apply: apply, LineInput: *plainMode}
	_, err = plain.Run(newWizard(), cfg, in, out)
	return err
}
