package screen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/wizard"
)

const clearScreen = "\x1b[H\x1b[2J"

// View is the current page for the current size, with "\n" between lines.
func (s *Session) View() string {
	lay := s.page().layout(s.env, s.start)
	lines := make([]string, len(lay.lines))
	for i, l := range lay.lines {
		lines[i] = sgr(l, s.env.Color)
	}
	return strings.Join(lines, "\n")
}

// Frame is what the terminal gets: cleared, then the page from the top.
func (s *Session) Frame() string {
	return clearScreen + strings.ReplaceAll(s.View(), "\n", "\r\n")
}

func (s *Session) page() *page {
	p := s.build()
	p.message = s.msg
	return p
}

func (s *Session) build() *page {
	switch {
	case s.job != nil && !s.job.stream:
		return &page{title: []string{s.job.busy}, enterRow: -1, noPrompt: true}
	case s.small():
		return s.smallPage()
	case s.detail != nil:
		return s.detailPage()
	}
	switch s.kind {
	case kSystem:
		return s.systemPage()
	case kSystemResult:
		return s.systemResultPage()
	case kQuestion:
		return s.questionPage()
	case kItems:
		return s.itemsPage()
	case kReview:
		return s.reviewPage()
	case kSave:
		return s.savePage()
	case kConfirm:
		return s.confirmPage()
	case kApplying:
		return &page{header: step(wizard.StageApply), title: []string{"Applying your setup"},
			hint: []string{"Each changed file is backed up first."}, enterRow: -1, final: true}
	case kDone:
		return s.donePage()
	}
	return s.leftPage()
}

func step(stage wizard.Stage) string {
	steps := wizard.SetupStages()
	return fmt.Sprintf("agtk setup | step %d of %d | %s", slices.Index(steps, stage)+1, len(steps), stage)
}

// nav is back, restart and quit. The first page has nothing to go back to.
func (s *Session) nav(back bool) []cmd {
	var out []cmd
	if back {
		out = append(out, cmd{key: 'b', label: "Back.", do: s.back})
	}
	if s.kind != kSystem {
		out = append(out, cmd{key: 'r', label: "Restart from the beginning.", do: s.restart})
	}
	return append(out, cmd{key: 'q', label: "Quit and do nothing.", do: s.leave})
}

func (s *Session) systemPage() *page {
	return &page{
		header: step(wizard.StageSystem),
		title:  []string{"This is the agtk setup wizard. It will ask you a few questions and set up Claude Code, Codex and Gemini CLI / Antigravity."},
		hint:   []string{"Check this machine first? That looks at hardware, tools on PATH and managed hook settings. Nothing is stored or sent."},
		cmds: []cmd{
			{key: 'y', label: "Yes, check this machine.", do: func() { s.check(true) }},
			{key: 'n', label: "No, I will choose by hand.", do: func() { s.check(false) }},
		},
		nav: s.nav(false), enter: 'y', enterRow: -1,
	}
}

func (s *Session) systemResultPage() *page {
	lines := wizard.SystemLines(s.w.Scan)
	return &page{
		header: step(wizard.StageSystem), title: []string{"System check"}, hint: lines[:1],
		rows:     s.textRows(lines[1:], contentWidth(s.env)-6),
		cmds:     []cmd{{key: 'y', label: "Continue.", do: func() { s.enterQuestions(wizard.StageProfile, false) }}},
		nav:      s.nav(true),
		enter:    'y',
		enterRow: -1,
	}
}

// textRows turns lines of text into rows that page.
func (s *Session) textRows(lines []string, w int) []row {
	var out []row
	for _, l := range lines {
		for _, part := range wrap(l, w, 2) {
			out = append(out, row{text: part})
		}
	}
	return out
}

func hintOf(q *wizard.Question) []string {
	switch {
	case q.Hint != "":
		return strings.Split(q.Hint, "\n")
	case q.ID == "use":
		return []string{"Your profile sets which components and models are available."}
	}
	return []string{"These choices help agtk recommend components for your work."}
}

func checkbox(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func (s *Session) questionPage() *page {
	qs := s.w.SetupQuestions(s.stage)
	q := qs[min(s.qi, len(qs)-1)]
	rowW := contentWidth(s.env) - keyIndent
	p := &page{header: step(s.stage), title: []string{q.Prompt}, hint: hintOf(q), enterRow: -1, nav: s.nav(true)}
	for i, o := range q.Options {
		on := s.w.Has(q.ID, o.Value)
		r := row{group: o.Group, numbered: true, name: o.Label, info: func() []string { return s.w.SetupDetails(q, o) }}
		note := s.w.Note(q, o)
		if q.ID == "models" {
			note = "" // the account is already the group
		}
		if q.Multi {
			text := checkbox(on) + " " + o.Label
			if note != "" {
				text += "  (" + note + ")"
			}
			if left := rowW - width(text) - 3; left >= 12 {
				if detail := firstLine(o.Detail); detail != "" {
					text += dim(" - " + truncate(detail, left))
				}
			}
			r.text = text
			r.do = func() { s.w.Set(q, o.Value) }
		} else {
			var tags []string
			if on {
				tags = append(tags, "default")
				p.enterRow = i
			}
			if note != "" {
				tags = append(tags, note)
			}
			r.text = o.Label + "."
			if len(tags) > 0 {
				r.text += "  " + dim("("+strings.Join(tags, ", ")+")")
			}
			r.extra = s.preview(q, o, rowW)
			r.do = func() {
				s.w.Set(q, o.Value)
				s.confirm(q)
			}
		}
		p.rows = append(p.rows, r)
	}
	if q.Multi {
		p.cmds = []cmd{{key: 'y', label: "Continue.", do: func() { s.confirm(q) }}}
		p.enter = 'y'
	}
	p.cmds = append(p.cmds, s.describeCmd())
	return p
}

// confirm moves on from a question unless its answer is not allowed.
func (s *Session) confirm(q *wizard.Question) {
	if msg := s.w.ConfirmSetup(q); msg != "" {
		s.msg = msg
		return
	}
	s.step(1)
}

func (s *Session) describeCmd() cmd {
	return cmd{key: '?', label: "Describe a choice.", do: func() {
		s.asking = true
		s.msg = "Press the number of the choice to describe."
	}}
}

// preview is what a choice means, drawn under it: the lines of its details up to the
// first blank one, which for a routing preset are its routes.
func (s *Session) preview(q *wizard.Question, o wizard.Option, w int) []string {
	var lines []string
	for _, l := range s.w.SetupDetails(q, o) {
		if l == "" {
			break
		}
		lines = append(lines, l)
	}
	if routes := len(strings.Split(o.Detail, "\n")); q.ID == "routing" && len(lines) > routes && s.env.Height < 50 {
		// A short terminal gets the routes as one paragraph, without their fallbacks.
		for i, l := range lines[routes:] {
			lines[routes:][i], _, _ = strings.Cut(l, " -> ")
		}
		lines = append(lines[:routes:routes], strings.Join(lines[routes:], "; "))
	}
	var out []string
	for _, l := range lines {
		for _, part := range wrap(l, w-2, 2) {
			out = append(out, dim(part))
		}
	}
	return out
}

func (s *Session) itemsPage() *page {
	views := wizard.ItemViews()
	view := views[s.view%len(views)]
	all := s.w.Lines()
	selected := 0
	for _, l := range all {
		if l.On {
			selected++
		}
	}
	used, limit := s.w.Budget()
	rowW := contentWidth(s.env) - keyIndent
	p := &page{
		header: step(wizard.StageItems), title: []string{"Choose what to install."},
		hint:     []string{fmt.Sprintf("%d selected | view: %s | ~%d of %d context tokens", selected, strings.ToLower(view.Label), used, limit)},
		enterRow: -1, enter: 'y', nav: s.nav(true),
	}
	for _, l := range wizard.FilterLines(all, "", view.ID) {
		mark, tail := checkbox(l.On), fmt.Sprintf("  ~%d tok", l.Cost)
		if l.Blocked != "" {
			mark, tail = "[-]", "  blocked"
		}
		text := mark + " " + l.Item.ID + dim(tail)
		if left := rowW - width(text) - 3; left >= 12 {
			text += dim(" - " + truncate(l.Item.Summary, left))
		}
		p.rows = append(p.rows, row{group: wizard.ItemGroup(l.Item), text: text, numbered: true, name: l.Item.ID,
			info: func() []string { return wizard.ItemDetails(l) },
			do: func() {
				if reason := s.w.Toggle(l.Item.ID); reason != "" {
					s.msg = reason
				}
			}})
	}
	next := views[(s.view+1)%len(views)]
	p.cmds = []cmd{
		{key: 'y', label: "Continue to the review.", do: func() { s.to(kReview) }},
		{key: 'v', label: "Show " + strings.ToLower(next.Label) + " items.", do: func() { s.view, s.start = (s.view+1)%len(views), -1 }},
		s.describeCmd(),
	}
	return p
}

func (s *Session) reviewPage() *page {
	p := s.w.Profile()
	lines := s.w.ReviewDetails(p)
	return &page{
		header: step(wizard.StageReview), title: []string{"Review"}, hint: []string{wizard.SetupSummary(p)},
		rows:     s.textRows(lines, contentWidth(s.env)-6),
		cmds:     []cmd{{key: 'y', label: "Continue.", do: func() { s.to(kSave) }}},
		nav:      s.nav(true),
		enter:    'y',
		enterRow: -1,
	}
}

func (s *Session) savePage() *page {
	enter := 'y'
	if !s.save {
		enter = 'n'
	}
	choose := func(save bool) func() {
		return func() {
			s.save = save
			s.prepare()
		}
	}
	return &page{
		header: step(wizard.StageReview), title: []string{"Save your choices as a profile for agtk apply?"},
		hint: []string{"A saved profile lets you repeat this setup later with `agtk apply`."},
		cmds: []cmd{
			{key: 'y', label: "Yes, save the profile.", do: choose(true)},
			{key: 'n', label: "No, only apply it now.", do: choose(false)},
		},
		nav: s.nav(true), enter: enter, enterRow: -1,
	}
}

// show is a path relative to the home folder as ~/..., else unchanged.
func (s *Session) show(path string) string {
	if s.cfg.Home != "" {
		if rel, err := filepath.Rel(s.cfg.Home, path); err == nil && filepath.IsLocal(rel) {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return path
}

func (s *Session) confirmPage() *page {
	p := &page{header: step(wizard.StageReview), enterRow: -1, nav: s.nav(true)}
	if s.planErr != nil {
		p.title = []string{"The plan could not be prepared"}
		p.hint = []string{s.planErr.Error(), "Fix the issue, then try again."}
		p.cmds = []cmd{{key: 't', label: "Try again.", do: s.prepare}}
		p.enter = 't'
		return p
	}
	p.title = []string{"Ready to apply?"}
	p.hint = []string{"This is exactly what will be written. Close the clients you are configuring first. Each changed file is backed up."}
	w := contentWidth(s.env) - 6
	if len(s.plan.Files)+len(s.plan.Commands) == 0 {
		p.rows = append(p.rows, row{text: "No file or command changes in this plan."})
	}
	add := func(heading string, items []string) {
		p.rows = append(p.rows, row{text: bold(heading)})
		for _, item := range items {
			for _, part := range wrap(item, w, 10) {
				p.rows = append(p.rows, row{text: part})
			}
		}
	}
	var files, commands []string
	for _, c := range s.plan.Files {
		files = append(files, fmt.Sprintf("  %-7s %s", c.Action, s.show(c.What)))
	}
	for _, c := range s.plan.Commands {
		commands = append(commands, "  "+c.What)
	}
	add(fmt.Sprintf("Files (%d new, %d to edit)", s.plan.New, s.plan.Edited), files)
	add(fmt.Sprintf("Commands (%d may run)", len(s.plan.Commands)), commands)
	if len(commands) > 0 {
		p.rows = append(p.rows, row{text: dim("Manage command changes in the tool that runs them.")})
	}
	save := "No"
	if s.save {
		save = "Yes"
	}
	p.rows = append(p.rows, row{text: "Save profile for agtk apply: " + save})
	p.cmds = []cmd{{key: 'y', label: "Yes, apply.", do: s.apply}}
	return p
}

func (s *Session) detailPage() *page {
	return &page{
		header: s.detailHeader, title: []string{s.detailName},
		rows:     s.textRows(s.detail, contentWidth(s.env)-6),
		cmds:     []cmd{{key: 'b', label: "Back to the list.", do: func() { s.detail, s.start = nil, s.detailStart }}},
		nav:      []cmd{{key: 'q', label: "Quit and do nothing.", do: s.leave}},
		enter:    'b',
		enterRow: -1,
	}
}

func (s *Session) smallPage() *page {
	return &page{
		title: []string{"This terminal is too small"},
		hint: []string{
			fmt.Sprintf("agtk needs a window of at least %dx%d, and this one is %dx%d.", minWidth, minHeight, s.env.Width, s.env.Height),
			"Make it bigger and press any key, or use the plain wizard, which scrolls.",
		},
		cmds:     []cmd{{key: 'w', label: "Use the plain wizard."}},
		nav:      []cmd{{key: 'q', label: "Quit and do nothing."}},
		enterRow: -1,
	}
}

func (s *Session) leftPage() *page {
	p := &page{final: true, enterRow: -1}
	if s.err != nil {
		p.static = []string{"Setup stopped: " + s.err.Error()}
	}
	p.static = append(p.static, "Left setup. Nothing was written.")
	return p
}

func (s *Session) donePage() *page {
	p := &page{header: step(wizard.StageApply), final: true, enterRow: -1}
	if s.failed != nil {
		p.title = []string{"Setup failed"}
		p.hint = []string{s.failed.Error()}
		if s.report.CanUndo(s.cfg.Home) {
			p.static = []string{"  " + wizard.UndoHint}
		}
		return p
	}
	w := contentWidth(s.env) - 6
	p.title = []string{"Setup complete"}
	if s.report.Backup != "" {
		p.static = append(p.static, "  Backup: "+s.report.Backup)
		if s.report.CanUndo(s.cfg.Home) {
			p.static = append(p.static, "  "+wizard.UndoHint)
		}
	}
	p.static = append(p.static, "", "  "+bold("Next steps"))
	n := 0
	item := func(text string) {
		n++
		prefix := fmt.Sprintf("  %d. ", n)
		for i, part := range wrap(text, w, 0) {
			if i > 0 {
				prefix = strings.Repeat(" ", len(prefix))
			}
			p.static = append(p.static, prefix+part)
		}
	}
	item("Start new sessions in the clients you configured.")
	if slices.Contains(s.w.Values("tools"), "gemini") {
		item("Gemini CLI and Antigravity both read ~/.gemini/GEMINI.md. Start a new session in either one.")
	}
	for _, next := range s.report.Next {
		item(next)
	}
	if hint := s.pathHint(); len(hint) > 0 {
		item(hint[0])
		for _, l := range hint[1:] {
			p.static = append(p.static, "       "+strings.TrimLeft(l, " ")) // verbatim: wrapping would break the command
		}
	}
	return p
}

// pathHint is how to put agtk's own folder on PATH, when the binary was installed there and it is missing.
func (s *Session) pathHint() []string {
	if s.cfg.Home == "" {
		return nil
	}
	name := "agtk"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe := filepath.Join(install.StateDir(s.cfg.Home), "bin", name)
	if _, err := os.Stat(exe); err != nil {
		return nil
	}
	return install.PathHint(filepath.Dir(exe), s.cfg.Getenv("PATH"), s.cfg.Getenv("SHELL"), runtime.GOOS)
}
