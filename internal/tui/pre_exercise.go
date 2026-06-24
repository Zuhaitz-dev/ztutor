package tui

import (
	"strings"
	"time"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type startExerciseMsg struct{ lesson lesson.Lesson }

type PreExerciseScreen struct {
	lesson lesson.Lesson
	sized
	dialogueEngine
	loc *i18n.Locale
}

// tutorialBeats converts a lesson's Tutorial strings into DialogueBeat entries.
func tutorialBeats(lines []string) []DialogueBeat {
	beats := make([]DialogueBeat, len(lines))
	for i, line := range lines {
		mood := MoodIdle
		switch {
		case i == 0:
			mood = MoodCurious
		case i == len(lines)-1:
			mood = MoodHappy
		}
		var delay time.Duration
		if i < len(lines)-1 {
			delay = 2200 * time.Millisecond
		}
		beats[i] = DialogueBeat{
			Text:      line,
			Mood:      mood,
			Speed:     2,
			AutoDelay: delay,
		}
	}
	return beats
}

func NewPreExerciseScreen(l lesson.Lesson, width, height int, loc *i18n.Locale) *PreExerciseScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	return &PreExerciseScreen{
		lesson:         l,
		sized:          sized{Width: width, Height: height},
		dialogueEngine: dialogueEngine{Beats: tutorialBeats(l.Tutorial), MaxLines: 4, rtlMascotAlign: lipgloss.Right},
		loc:            loc,
	}
}

func (ps *PreExerciseScreen) SetLocale(loc *i18n.Locale) {
	ps.loc = loc
}

func (ps *PreExerciseScreen) SetMascotFrame(frame int) {
	ps.dialogueEngine.Frame = frame
}

func (ps *PreExerciseScreen) Init() tea.Cmd { return ps.TypeTick() }

func (ps *PreExerciseScreen) finishCurrentBeat() tea.Cmd {
	return ps.dialogueEngine.FinishBeat()
}

func (ps *PreExerciseScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ps.HandleResize(msg)

	case dialogueTypeTick:
		if ps.Phase != DialogueTyping {
			return ps, nil
		}
		beat := ps.Beats[ps.BeatIdx]
		total := len([]rune(beat.Text))
		ps.CharIdx += beat.Speed
		if ps.CharIdx >= total {
			return ps, ps.finishCurrentBeat()
		}
		return ps, ps.TypeTick()

	case dialogueAdvanceTick:
		if msg.BeatIdx != ps.BeatIdx {
			return ps, nil
		}
		ps.Advance()
		return ps, ps.TypeTick()

	case tea.KeyMsg:
		switch msg.String() {
		case KeyQuit:
			return ps, tea.Quit
		case KeyBack:
			l := ps.lesson
			return ps, backCmd(startExerciseMsg{lesson: l})
		default:
			switch ps.Phase {
			case DialogueTyping:
				return ps, ps.finishCurrentBeat()
			case DialogueWaiting:
				ps.Advance()
				return ps, ps.TypeTick()
			case DialogueHolding:
				l := ps.lesson
				return ps, backCmd(startExerciseMsg{lesson: l})
			}
		}
	}
	return ps, nil
}

func (ps *PreExerciseScreen) View() string {
	rtl := ps.loc.IsRTL()
	boxW := ps.dialogueEngine.boxW(ps.Width)

	var content strings.Builder

	// Lesson title header.
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	header := dimStyle.Render(ps.loc.T("pre_exercise.label")) + " " + titleStyle.Render(ps.lesson.Title)
	content.WriteString(rtlWrap(rtl, header, boxW+2) + "\n\n")

	// Mascot sprite.
	content.WriteString(ps.dialogueEngine.RenderMascot(ps.loc, ps.Width))
	content.WriteString("\n\n")

	// Text box.
	content.WriteString(ps.dialogueEngine.RenderTextBox(ps.loc, ps.Width))
	content.WriteString("\n\n")

	// Hint.
	T := ps.loc.T
	var bar string
	switch ps.Phase {
	case DialogueWaiting:
		bar = helpBar(T("help.any_key_continue"), T("help.q_skip"))
	case DialogueHolding:
		bar = helpBar(T("help.any_key_start"), T("help.q_skip"))
	default:
		bar = helpBar(T("help.any_key_skip"), T("help.q_skip"))
	}
	content.WriteString(rtlWrap(rtl, bar, boxW+2))

	return lipgloss.Place(ps.Width, ps.Height, lipgloss.Center, lipgloss.Center, content.String())
}
