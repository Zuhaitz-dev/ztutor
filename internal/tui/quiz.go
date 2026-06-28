package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type QuizScreen struct {
	quiz      lesson.Quiz
	index     int
	cursor    int
	selected  map[string]int
	submitted map[string]bool
	completed bool
	bestStars int
	sized
	loc *i18n.Locale
}

func NewQuizScreen(q lesson.Quiz, bestStars, width, height int, loc *i18n.Locale) *QuizScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	return &QuizScreen{
		quiz:      q,
		selected:  make(map[string]int),
		submitted: make(map[string]bool),
		bestStars: bestStars,
		sized:     sized{Width: width, Height: height},
		loc:       loc,
	}
}

func (qs *QuizScreen) Init() tea.Cmd { return nil }

func (qs *QuizScreen) SetLocale(loc *i18n.Locale) { qs.loc = loc }

func (qs *QuizScreen) SetQuiz(q lesson.Quiz) {
	qs.quiz = q
	if qs.index >= len(q.Questions) {
		qs.index = max(0, len(q.Questions)-1)
	}
	if cur := qs.current(); cur != nil && qs.cursor >= len(cur.Options) {
		qs.cursor = max(0, len(cur.Options)-1)
	}
}

func (qs *QuizScreen) SetMascotFrame(int) {}

func (qs *QuizScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		qs.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case KeyBack, KeyBackAlt, KeyBackEditor:
			return qs, backCmd(NavigateBackToCourse{})
		case KeyUp, KeyUpVim:
			qs.move(-1)
		case KeyDown, KeyDownVim:
			qs.move(1)
		case KeySelect, KeySelectAlt:
			return qs, qs.activate()
		}
	}
	return qs, nil
}

func (qs *QuizScreen) current() *lesson.QuizQuestion {
	if qs.index < 0 || qs.index >= len(qs.quiz.Questions) {
		return nil
	}
	return &qs.quiz.Questions[qs.index]
}

func (qs *QuizScreen) move(delta int) {
	q := qs.current()
	if q == nil || qs.submitted[q.ID] || len(q.Options) == 0 {
		return
	}
	qs.cursor = (qs.cursor + delta + len(q.Options)) % len(q.Options)
}

func (qs *QuizScreen) activate() tea.Cmd {
	q := qs.current()
	if q == nil {
		return backCmd(NavigateBackToCourse{})
	}
	if !qs.submitted[q.ID] {
		qs.selected[q.ID] = qs.cursor
		qs.submitted[q.ID] = true
		return nil
	}
	if qs.index+1 < len(qs.quiz.Questions) {
		qs.index++
		qs.cursor = qs.selected[qs.quiz.Questions[qs.index].ID]
		return nil
	}
	if !qs.completed {
		qs.completed = true
		return backCmd(lessonCompletedMsg{lessonID: qs.quiz.ID, stars: qs.stars()})
	}
	return backCmd(NavigateBackToCourse{})
}

func (qs *QuizScreen) score() (correct, total int) {
	total = len(qs.quiz.Questions)
	for _, q := range qs.quiz.Questions {
		sel, ok := qs.selected[q.ID]
		if !ok || sel < 0 || sel >= len(q.Options) {
			continue
		}
		if q.Options[sel].Correct {
			correct++
		}
	}
	return correct, total
}

func (qs *QuizScreen) stars() int {
	correct, total := qs.score()
	if total == 0 {
		return 1
	}
	switch {
	case correct == total:
		return 3
	case correct*2 >= total:
		return 2
	default:
		return 1
	}
}

func (qs *QuizScreen) View() string {
	rtl := qs.loc.IsRTL()
	align := lipgloss.Left
	if rtl {
		align = lipgloss.Right
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent)).Width(qs.Width).Align(align)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)).Width(qs.Width).Align(align)
	var b strings.Builder

	b.WriteString(titleStyle.Render(qs.quiz.Title))
	b.WriteString("\n")
	if qs.quiz.Description != "" {
		b.WriteString(metaStyle.Render(qs.quiz.Description))
		b.WriteString("\n")
	}
	correct, total := qs.score()
	meta := qs.loc.T("quiz.progress", qs.index+1, len(qs.quiz.Questions), correct, total)
	if qs.bestStars > 0 {
		meta += "  " + qs.loc.T("exercise.stars.prev_best", strings.Repeat("★", qs.bestStars))
	}
	b.WriteString(metaStyle.Render(meta))
	b.WriteString("\n\n")

	q := qs.current()
	if q == nil {
		b.WriteString(dim(qs.loc.T("quiz.empty")))
	} else {
		b.WriteString(qs.renderQuestion(*q, rtl))
	}
	b.WriteString("\n\n")

	var actions []HelpAction
	if q != nil && qs.submitted[q.ID] {
		if qs.index+1 < len(qs.quiz.Questions) {
			actions = []HelpAction{HA(ActionNextQuestion), HA(ActionBack)}
		} else {
			actions = []HelpAction{HA(ActionConfirm), HA(ActionBack)}
		}
	} else {
		actions = []HelpAction{HA(ActionChoose), HA(ActionSubmitAnswer), HA(ActionBack)}
	}
	footer := actionHelpBar(qs.loc, actions...)
	if rtl {
		footer = lipgloss.NewStyle().Width(qs.Width).Align(lipgloss.Right).Render(footer)
	}

	pad := qs.Height - lipgloss.Height(b.String()) - lipgloss.Height(footer)
	if pad > 0 {
		b.WriteString(strings.Repeat("\n", pad))
	}
	b.WriteString(footer)
	return b.String()
}

func (qs *QuizScreen) renderQuestion(q lesson.QuizQuestion, rtl bool) string {
	align := lipgloss.Left
	if rtl {
		align = lipgloss.Right
	}
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorBody)).Width(qs.Width).Align(align)
	optionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Width(qs.Width).Align(align)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(lipgloss.Color(ColorCyan)).Width(qs.Width).Align(align)
	correctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true).Width(qs.Width).Align(align)
	wrongStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Width(qs.Width).Align(align)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)).Width(qs.Width).Align(align)

	var b strings.Builder
	b.WriteString(promptStyle.Render(fmt.Sprintf("%d. %s", qs.index+1, q.Prompt)))
	b.WriteString("\n\n")
	submitted := qs.submitted[q.ID]
	selected := qs.selected[q.ID]
	for i, opt := range q.Options {
		marker := " "
		if i == qs.cursor && !submitted {
			marker = ">"
		}
		if rtl {
			marker = "<"
			if i != qs.cursor || submitted {
				marker = " "
			}
		}
		label := fmt.Sprintf("%s %s. %s", marker, optionLetter(i), opt.Text)
		style := optionStyle
		if submitted {
			switch {
			case opt.Correct:
				style = correctStyle
				label = fmt.Sprintf("+ %s. %s", optionLetter(i), opt.Text)
			case i == selected:
				style = wrongStyle
				label = fmt.Sprintf("x %s. %s", optionLetter(i), opt.Text)
			default:
				style = dimStyle
			}
		} else if i == qs.cursor {
			style = selectedStyle
		}
		b.WriteString(style.Render(label))
		b.WriteString("\n")
	}
	if submitted {
		b.WriteString("\n")
		if selected >= 0 && selected < len(q.Options) && q.Options[selected].Correct {
			b.WriteString(correctStyle.Render(qs.loc.T("quiz.correct")))
		} else {
			b.WriteString(wrongStyle.Render(qs.loc.T("quiz.incorrect")))
		}
		if q.Explanation != "" {
			b.WriteString("\n")
			b.WriteString(dimStyle.Render(q.Explanation))
		}
	}
	return b.String()
}

func optionLetter(i int) string {
	if i >= 0 && i < 26 {
		return string(rune('A' + i))
	}
	return fmt.Sprintf("%d", i+1)
}
