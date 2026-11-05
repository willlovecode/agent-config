package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	agentconfig "github.com/willlovecode/agent-config"
	"github.com/willlovecode/agent-config/internal/catalog"
	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/plain"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/tui"
	"github.com/willlovecode/agent-config/internal/wizard"
)

// runWizard is `agtk` with no command: the DOS-style screens, line mode with --plain (or
// when stdin is not a terminal), or no questions at all with --answers.
func runWizard(args []string) error {
	fs := flag.NewFlagSet("agtk", flag.ExitOnError)
	ascii := fs.Bool("ascii", false, "draw frames with ASCII characters")
	plainMode := fs.Bool("plain", false, "ask line by line (screen readers, small terminals)")
	answers := fs.String("answers", "", "install from this answers file without questions")
	save := fs.Bool("save", false, "with --answers: also save the profile for `agtk apply`")
	fs.Parse(args)

	cat, err := catalog.Load(agentconfig.Assets)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	s := scan.Run()
	w := wizard.New(cat, agentconfig.Assets, s)
	h := home()
	opts := func(p *profile.Profile, out io.Writer) install.Options {
		return install.Options{Home: h, Assets: agentconfig.Assets, Catalog: cat, Profile: p, Tier: s.Tier,
			Exe: exe, Version: version, Out: out}
	}
	plan := func(p *profile.Profile) ([]install.Change, error) { return install.Plan(opts(p, io.Discard)) }
	apply := func(p *profile.Profile, save bool, out io.Writer) error {
		if save {
			if err := wizard.SaveProfile(p); err != nil {
				return err
			}
			fmt.Fprintf(out, "Saved your answers to %s\n", p.Path)
		}
		return install.Apply(opts(p, out))
	}

	if *answers != "" {
		p, err := profile.Load(*answers)
		if err != nil {
			return err
		}
		if err := w.Load(p); err != nil {
			return err
		}
		chosen, err := w.Profile()
		if err != nil {
			return err
		}
		return apply(chosen, *save, os.Stdout)
	}
	runPlain := func() error {
		_, err := plain.Run(w, plain.Config{Plan: plan, Apply: apply}, os.Stdin, os.Stdout)
		return err
	}
	if *plainMode || !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return runPlain()
	}
	d := tui.Detect(*ascii)
	res, err := tui.Run(w, tui.Config{Version: version, Display: d, Home: h, Plan: plan, Apply: apply})
	tui.Restore()
	if errors.Is(err, tui.ErrPlain) {
		return runPlain()
	}
	if err != nil {
		return err
	}
	// Leave the install log on the terminal once the screen is gone.
	fmt.Print(res.Log)
	if res.Err != nil {
		return res.Err
	}
	if !res.Installed {
		fmt.Println("Left setup. Nothing was written.")
	}
	return nil
}
