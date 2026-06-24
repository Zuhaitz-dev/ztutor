package tui

import (
	"fmt"
	"strings"
	"time"

	editormod "ztutor/internal/editor"
	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NavigateToChallengeMsg struct {
	Challenge lesson.Challenge
	CourseID  string
	Lang      sandbox.Language
}

type challengeTestResultMsg struct {
	compileResult *sandbox.Result
	testResults   []sandbox.TestResult
	err           error
}

type ChallengeScreen struct {
	challenge lesson.Challenge
	courseID  string
	lang      sandbox.Language
	executor  sandbox.Executor
	editor    *editormod.CodeEditor
	output    string
	submitted bool
	compiling bool

	attempts int
	passed   bool

	sized
	mascotFrame   int
	mascotHidden  bool
	companionLine string
	loc           *i18n.Locale
}

func NewChallengeScreen(ch lesson.Challenge, courseID string, lang sandbox.Language, executor sandbox.Executor, width, height int, loc *i18n.Locale) *ChallengeScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	highlight := "c"
	if lang != nil {
		highlight = lang.Name()
	}
	editorH := height/2 - 4
	if editorH < 3 {
		editorH = 3
	}
	ed := editormod.New(ch.Exercise, width-6, editorH, highlight)
	ed.Focus()

	line := loc.T("challenge.mochi.welcome")
	if ch.Points > 0 {
		line = line + " " + loc.T("challenge.mochi.points", ch.Points)
	}

	return &ChallengeScreen{
		challenge:     ch,
		courseID:      courseID,
		lang:          lang,
		executor:      executor,
		editor:        ed,
		companionLine: line,
		sized:         sized{Width: width, Height: height},
		loc:           loc,
	}
}

func (cs *ChallengeScreen) SetLocale(loc *i18n.Locale) {
	cs.loc = loc
	cs.companionLine = loc.T("challenge.mochi.welcome")
}

func (cs *ChallengeScreen) Init() tea.Cmd {
	return nil
}

func (cs *ChallengeScreen) SetMascotFrame(frame int) {
	cs.mascotFrame = frame
}

func (cs *ChallengeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cs.HandleResize(msg)

	case tea.KeyMsg:
		if cs.submitted {
			switch msg.String() {
			case KeySelect, KeyBack, KeyBackAlt, KeySelectAlt:
				return cs, backCmd(NavigateToMenu{})
			case KeyRun:
				// re-submit
			case KeyMochi:
				cs.mascotHidden = !cs.mascotHidden
			}
			return cs, nil
		}

		switch msg.String() {
		case KeyBackEditor, KeyQuit:
			return cs, backCmd(NavigateToMenu{})
		case KeyRun:
			if !cs.compiling {
				cs.compiling = true
				cs.attempts++
				cs.companionLine = cs.loc.T("challenge.mochi.computing")
				cs.output = ""
				return cs, cs.submitCmd()
			}
		case KeyMochi:
			cs.mascotHidden = !cs.mascotHidden
		}

		newEd, cmd := cs.editor.Update(msg)
		cs.editor = newEd
		return cs, cmd

	case challengeTestResultMsg:
		cs.compiling = false
		if msg.err != nil {
			cs.output = exErrorStyle.Render(cs.loc.T("exercise.result.error", msg.err))
			return cs, nil
		}
		if msg.compileResult.Error != "" {
			cs.output = exErrorStyle.Render(msg.compileResult.Error)
			return cs, nil
		}

		passed, total := 0, len(msg.testResults)
		for _, r := range msg.testResults {
			if r.Passed {
				passed++
			}
		}
		cs.passed = passed == total

		var b strings.Builder
		board := challengeScoreboard(msg.testResults)
		if cs.passed {
			b.WriteString(exSuccessStyle.Render(cs.loc.T("exercise.result.all_passed", total)) + "  " + board)
			cs.companionLine = cs.loc.T("challenge.mochi.accepted")
		} else {
			b.WriteString(exErrorStyle.Render(cs.loc.T("exercise.result.tests_failed", passed, total)) + "  " + board)
			for _, r := range msg.testResults {
				if r.Passed {
					continue
				}
				b.WriteString("\n\n" + cs.loc.T("exercise.result.test_label", r.Num))
				if r.Error != "" {
					b.WriteString("\n" + exErrorStyle.Render(cs.loc.T("exercise.result.runtime_error", r.Error)))
				} else if r.ExitCode != 0 {
					b.WriteString("\n" + exErrorStyle.Render(cs.loc.T("exercise.result.runtime_exit", r.ExitCode)))
				} else {
					b.WriteString("\n" + dim(cs.loc.T("exercise.result.diff_hint")) + "\n")
					b.WriteString(diffOutput(r.Got, r.Want))
				}
			}
		}
		cs.submitted = true
		cs.output = b.String()
		return cs, nil
	}

	return cs, nil
}

func (cs *ChallengeScreen) submitCmd() tea.Cmd {
	return func() tea.Msg {
		inputs := make([]sandbox.TestInput, len(cs.challenge.Tests))
		for i, tc := range cs.challenge.Tests {
			inputs[i] = sandbox.TestInput{
				Stdin:    tc.Stdin,
				Args:     parseFlags(tc.Args),
				Expected: tc.Expected,
			}
		}
		compileRes, results, err := cs.executor.RunAllTests(cs.lang, map[string]string{cs.lang.SourceFileName(): cs.editor.Value()}, "", nil, inputs)
		return challengeTestResultMsg{compileResult: compileRes, testResults: results, err: err}
	}
}

func challengeScoreboard(results []sandbox.TestResult) string {
	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	var b strings.Builder
	for _, r := range results {
		if r.Passed {
			b.WriteString(passStyle.Render("[✓]"))
		} else {
			b.WriteString(failStyle.Render("[✗]"))
		}
	}
	return b.String()
}

func (cs *ChallengeScreen) View() string {
	l := NewTerminalLayout(cs.Width, cs.Height)

	l.AddFixed("header", nil, func(w int) string {
		title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent)).Render(cs.challenge.Title)
		if cs.challenge.Points > 0 {
			title += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render(cs.loc.T("challenge.points_label", cs.challenge.Points))
		}
		return title + "\n"
	})

	l.AddFixed("time", func() bool {
		return !cs.challenge.StartsAt.IsZero()
	}, func(w int) string {
		var line string
		if cs.challenge.StartsAt.After(time.Now()) {
			diff := time.Until(cs.challenge.StartsAt).Round(time.Minute)
			line = dim(cs.loc.T("challenge.opens_in", fmtDuration(diff)))
		} else if !cs.challenge.EndsAt.IsZero() && cs.challenge.EndsAt.Before(time.Now()) {
			line = dim(cs.loc.T("challenge.closed"))
		} else if !cs.challenge.EndsAt.IsZero() {
			diff := time.Until(cs.challenge.EndsAt).Round(time.Minute)
			line = dim(cs.loc.T("challenge.closes_in", fmtDuration(diff)))
		}
		return line + "\n"
	})

	l.AddFlex("editor", 3, nil, func(w, h int) string {
		if h < 3 {
			h = 3
		}
		edW := w - 6
		if edW < 20 {
			edW = 20
		}
		cs.editor.SetSize(edW, h)
		return cs.editor.View()
	})

	l.AddFixed("status", func() bool {
		return cs.compiling || cs.output != ""
	}, func(w int) string {
		if cs.compiling {
			return dim(cs.loc.T("challenge.testing")) + "\n"
		}
		return exOutputStyle.Width(w-4).Render(cs.output) + "\n"
	})

	l.AddFixed("mascot", func() bool {
		return !cs.mascotHidden
	}, func(w int) string {
		mood := MoodIdle
		if cs.compiling {
			mood = MoodThinking
		} else if cs.passed {
			mood = MoodHappy
		} else if cs.submitted {
			mood = MoodWorried
		}
		return renderMascotPanel(w, "Mochi", cs.companionLine, mood, cs.mascotFrame, cs.loc.IsRTL()) + "\n"
	})

	l.AddFixed("helpbar", nil, func(w int) string {
		var row1 []string
		if cs.submitted {
			row1 = []string{cs.loc.T("challenge.help.back")}
		} else {
			row1 = []string{cs.loc.T("challenge.help.submit"), cs.loc.T("exercise.help.back"), cs.loc.T("help.mochi")}
		}
		return helpBar(row1...)
	})

	result := l.Render()
	return rtlWrap(cs.loc.IsRTL(), result, cs.Width)
}

func fmtDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

