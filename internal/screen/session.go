// Package screen runs the wizard one full screen at a time, like `p10k configure`: the
// terminal is cleared before each page, choices are single keypresses, and nothing
// scrolls back. Session is the key-driven state machine and pages are pure functions of
// its state and the terminal size; run.go holds the thin layer that talks to a terminal.
package screen

import (
	"errors"
	"io"
	"strings"

	"github.com/willlovecode/agent-config/internal/install"
	"github.com/willlovecode/agent-config/internal/policy"
	"github.com/willlovecode/agent-config/internal/profile"
	"github.com/willlovecode/agent-config/internal/scan"
	"github.com/willlovecode/agent-config/internal/wizard"
)

// ErrPlain asks the caller to run the scrolling wizard instead: the terminal is too
// small or cannot take escape sequences, or the user chose it.
var ErrPlain = errors.New("use the plain wizard")

type Config struct {
	Home      string
	Preflight func(discover bool) (*scan.Result, policy.Policy, error)
	Plan      func(p *profile.Profile, save bool) ([]install.Change, error)
	Apply     func(p *profile.Profile, save bool, out io.Writer) error
	Getenv    func(string) string // PATH and SHELL for the PATH hint; os.Getenv when nil
}

type kind int

const (
	kSystem kind = iota
	kSystemResult
	kQuestion
	kItems
	kReview
	kSave
	kConfirm
	kApplying
	kDone
	kLeft
)

// job is work that takes a moment. The page shows busy while it runs, so the driver
// draws first and then calls Run.
type job struct {
	busy   string
	stream bool // Run writes progress, so the driver gives it the terminal
	run    func(out io.Writer)
}

type Session struct {
	cfg   Config
	fresh func() *wizard.Wizard
	w     *wizard.Wizard
	env   Env
	kind  kind
	stage wizard.Stage // of the questions
	qi    int          // question within the stage
	start int          // first row shown; the layout rounds it down to a page
	msg   string
	view  int // item view

	save                bool
	checked, discovered bool

	detail       []string // set while the details of a row are shown
	detailName   string
	detailHeader string
	detailStart  int
	asking       bool // after "?": the next digit names the row

	plan    wizard.ReviewPlan
	planErr error
	log     string
	report  wizard.ApplyReport
	failed  error // apply

	job       *job
	done      bool
	installed bool
	plain     bool
	err       error
}

// NewSession starts at the first page. fresh makes the wizard again for Restart.
func NewSession(fresh func() *wizard.Wizard, cfg Config) *Session {
	if cfg.Getenv == nil {
		cfg.Getenv = func(string) string { return "" }
	}
	s := &Session{cfg: cfg, fresh: fresh, env: Env{Width: 80, Height: 24}}
	s.restart()
	return s
}

func (s *Session) SetEnv(e Env) { s.env = e }

// Done reports that the last page is showing and no key will do anything more.
func (s *Session) Done() bool { return s.done }

// Result says whether setup was applied, and why it stopped otherwise. ErrPlain means
// the caller should run the scrolling wizard.
func (s *Session) Result() (installed bool, err error) {
	if s.plain {
		return false, ErrPlain
	}
	return s.installed, s.err
}

// Pending is the job to run before the next key, if any.
func (s *Session) Pending() (stream bool, ok bool) {
	if s.job == nil {
		return false, false
	}
	return s.job.stream, true
}

// RunJob does the pending job; out receives progress for a streaming one.
func (s *Session) RunJob(out io.Writer) {
	if j := s.job; j != nil {
		j.run(out)
		s.job = nil
	}
}

func (s *Session) small() bool { return s.env.Width < minWidth || s.env.Height < minHeight }

func (s *Session) restart() {
	s.w = s.fresh()
	s.kind, s.stage, s.qi, s.start, s.msg, s.view = kSystem, 0, 0, -1, "", 0
	s.save, s.checked, s.discovered = true, false, false
	s.detail, s.asking = nil, false
	s.plan, s.planErr = wizard.ReviewPlan{}, nil
}

// to shows another page from its top.
func (s *Session) to(k kind) {
	s.kind, s.start, s.msg, s.detail, s.asking = k, -1, "", nil, false
}

func (s *Session) leave() {
	s.to(kLeft)
	s.done = true
}

func (s *Session) fail(err error) {
	s.err = err
	s.leave()
}

func (s *Session) enterQuestions(stage wizard.Stage, last bool) {
	s.to(kQuestion)
	s.stage, s.qi = stage, 0
	if last {
		s.qi = max(len(s.w.SetupQuestions(stage))-1, 0)
	}
}

// step moves to the next or previous question, crossing from one stage to the next.
func (s *Session) step(dir int) {
	s.qi += dir
	switch n := len(s.w.SetupQuestions(s.stage)); {
	case s.qi >= 0 && s.qi < n:
		s.to(kQuestion)
	case dir > 0 && s.stage == wizard.StageProfile:
		s.enterQuestions(wizard.StageModels, false)
	case dir > 0:
		s.to(kItems)
	case s.stage == wizard.StageModels:
		s.enterQuestions(wizard.StageProfile, true)
	case s.discovered:
		s.to(kSystemResult)
	default:
		s.checked = false
		s.to(kSystem)
	}
}

func (s *Session) back() {
	switch s.kind {
	case kSystemResult:
		s.checked = false
		s.to(kSystem)
	case kQuestion:
		s.step(-1)
	case kItems:
		s.enterQuestions(wizard.StageModels, true)
	case kReview:
		s.to(kItems)
	case kSave:
		s.to(kReview)
	case kConfirm:
		s.to(kSave)
	}
}

func (s *Session) check(discover bool) {
	busy := ""
	if discover {
		busy = "Checking this machine..."
	}
	s.job = &job{busy: busy, run: func(io.Writer) {
		if s.cfg.Preflight != nil {
			result, rules, err := s.cfg.Preflight(discover)
			if err != nil {
				s.fail(err)
				return
			}
			s.w.SetScan(result)
			s.w.SetPolicy(rules)
		}
		s.checked, s.discovered = true, discover
		if discover {
			s.to(kSystemResult)
		} else {
			s.enterQuestions(wizard.StageProfile, false)
		}
	}}
}

func (s *Session) prepare() {
	s.job = &job{busy: "Preparing your plan...", run: func(io.Writer) {
		s.plan, s.planErr = wizard.ReviewPlan{}, nil
		if s.cfg.Plan != nil {
			changes, err := s.cfg.Plan(s.w.Profile(), s.save)
			s.plan, s.planErr = wizard.GroupChanges(changes), err
		}
		s.to(kConfirm)
	}}
}

func (s *Session) apply() {
	s.to(kApplying)
	s.job = &job{stream: true, run: func(out io.Writer) {
		var log strings.Builder
		var err error
		if s.cfg.Apply != nil {
			err = s.cfg.Apply(s.w.Profile(), s.save, io.MultiWriter(out, &log))
		}
		s.log, s.failed = log.String(), err
		s.report = wizard.SummarizeApply(s.log)
		if err != nil {
			s.err = err
		}
		s.installed = err == nil
		s.to(kDone)
		s.done = true
	}}
}

// Press handles one keypress.
func (s *Session) Press(k Key) {
	if s.done || s.job != nil {
		return
	}
	if k == KeyCtrlC || k == KeyEOF {
		s.leave()
		return
	}
	if s.small() {
		switch lower(k) {
		case 'w':
			s.plain, s.done = true, true
		case 'q':
			s.leave()
		}
		return
	}
	if s.asking {
		s.asking = false
		s.msg = ""
		s.describe(k)
		return
	}
	s.msg = ""
	p := s.page()
	lay := p.layout(s.env, s.start)
	k = lower(k)
	switch {
	case k == KeyEnter:
		switch {
		case p.enter != 0:
			k = Key(p.enter)
		case p.enterRow >= 0 && p.enterRow < len(p.rows):
			p.rows[p.enterRow].do()
			return
		default:
			return
		}
	case k >= '1' && k <= '9':
		if i := int(k - '1'); i < len(lay.shown) {
			p.rows[lay.shown[i]].do()
		}
		return
	case k == 'n' && lay.next >= 0:
		s.start = lay.next
		return
	case k == 'p' && lay.prev >= 0:
		s.start = lay.prev
		return
	}
	for _, c := range append(append([]cmd{}, p.cmds...), p.nav...) {
		if Key(c.key) == k && c.do != nil {
			c.do()
			return
		}
	}
}

func lower(k Key) Key {
	if k >= 'A' && k <= 'Z' {
		return k + 'a' - 'A'
	}
	return k
}

// describe shows the details of the row whose digit was pressed.
func (s *Session) describe(k Key) {
	p := s.page()
	lay := p.layout(s.env, s.start)
	if k < '1' || k > '9' || int(k-'1') >= len(lay.shown) {
		return
	}
	r := p.rows[lay.shown[int(k-'1')]]
	if r.info == nil {
		return
	}
	s.detail, s.detailName, s.detailHeader, s.detailStart, s.start = r.info(), r.name, p.header, lay.first, 0
}
