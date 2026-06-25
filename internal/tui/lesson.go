package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type LessonScreen struct {
	lesson   lesson.Lesson
	viewport viewport.Model
	answered bool
	sized
	loc *i18n.Locale
}

func NewLessonScreen(l lesson.Lesson, width, height int, loc *i18n.Locale) *LessonScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	vp := viewport.New(width-2, height-5)

	ls := &LessonScreen{
		lesson:   l,
		viewport: vp,
		sized:    sized{Width: width, Height: height},
		loc:      loc,
	}
	ls.viewport.SetContent(ls.renderContent())
	return ls
}

func (ls *LessonScreen) SetLocale(loc *i18n.Locale) {
	ls.loc = loc
	ls.viewport.SetContent(ls.renderContent())
}

func (ls *LessonScreen) Init() tea.Cmd { return nil }

func (ls *LessonScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case KeyBack, KeyBackAlt:
			return ls, backCmd(NavigateToMenu{})
		case "e", KeySelect:
			if ls.lesson.Exercise != "" || len(ls.lesson.Files) > 0 {
				return ls, backCmd(NavigateToExerciseMsg{Lesson: ls.lesson})
			}
		case KeyAchieve:
			if ls.lesson.Answer != "" {
				ls.answered = !ls.answered
				ls.viewport.SetContent(ls.renderContent())
				if !ls.answered {
					ls.viewport.GotoTop()
				}
			}

		case KeyScrollTop:
			ls.viewport.GotoTop()

		case KeyScrollBot:
			ls.viewport.GotoBottom()
		}

	case tea.WindowSizeMsg:
		ls.HandleResize(msg)
		ls.viewport.Width = msg.Width - 2
		ls.viewport.Height = msg.Height - 5
		ls.viewport.SetContent(ls.renderContent())
	}

	var cmd tea.Cmd
	ls.viewport, cmd = ls.viewport.Update(msg)
	return ls, cmd
}

func (ls *LessonScreen) View() string {
	rtl := ls.loc.IsRTL()
	var b strings.Builder

	// Title + company tags (interview mode).
	// Build the title line then right-align it independently in RTL so that
	// the glamour viewport below (which has its own layout) is not affected.
	titleSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	titleParts := []string{titleSt.Render(ls.lesson.Title)}

	if ls.lesson.IsPremium {
		premSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Bold(true)
		titleParts = append(titleParts, premSt.Render(ls.loc.T("lesson.premium_badge")))
	}

	if len(ls.lesson.Companies) > 0 {
		tagSt := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorHex)).
			Background(lipgloss.Color(ColorBG)).
			Padding(0, 1)
		tags := make([]string, len(ls.lesson.Companies))
		for i, c := range ls.lesson.Companies {
			tags[i] = tagSt.Render(c)
		}
		titleParts = append(titleParts, strings.Join(tags, " "))
	}
	titleLine := joinInlineParts(rtl, titleParts...)
	b.WriteString(rtlWrap(rtl, titleLine, ls.Width))
	b.WriteString("\n")

	// Difficulty + tags line (shown when present).
	if ls.lesson.Difficulty != "" || len(ls.lesson.Tags) > 0 {
		diffSt := lipgloss.NewStyle().Foreground(difficultyColor(ls.lesson.Difficulty))
		parts := []string{}
		if ls.lesson.Difficulty != "" {
			parts = append(parts, diffSt.Render(ls.lesson.Difficulty))
		}
		if len(ls.lesson.Tags) > 0 {
			parts = append(parts, dim(strings.Join(ls.lesson.Tags, "  ")))
		}
		b.WriteString(rtlWrap(rtl, joinInlineParts(rtl, parts...), ls.Width))
		b.WriteString("\n")
	}

	// Glamour-rendered content. Do NOT apply rtlWrap here: glamour manages its
	// own layout (indentation, code-block borders, word-wrap) and right-aligning
	// it line-by-line breaks those structures. Arabic text inside glamour renders
	// correctly at the terminal's bidi layer even without explicit right-alignment.
	b.WriteString(ls.viewport.View())
	b.WriteString("\n")

	// Help bar — right-align independently in RTL.
	items := []HelpAction{HA(ActionScroll), HA(ActionTopEnd)}
	if ls.lesson.Answer != "" {
		if ls.answered {
			items = append(items, HA(ActionHideAnswer))
		} else {
			items = append(items, HA(ActionRevealAnswer))
		}
	}
	if ls.lesson.Exercise != "" || len(ls.lesson.Files) > 0 {
		items = append(items, HA(ActionExercise))
	}
	items = append(items, HA(ActionBack))
	pct := int(ls.viewport.ScrollPercent() * 100)
	bar := actionHelpBar(ls.loc, items...)
	if bar != "" {
		bar += "  "
	}
	bar += dim(fmt.Sprintf("%d%%", pct))
	b.WriteString(rtlWrap(rtl, bar, ls.Width))

	return b.String()
}

func (ls *LessonScreen) renderContent() string {
	content := ls.lesson.Content

	// Append answer when revealed (interview mode).
	if ls.answered && ls.lesson.Answer != "" {
		content += "\n\n---\n\n" + ls.loc.T("lesson.section_answer") + "\n\n" + ls.lesson.Answer
	}

	// Show exactly one trivia fact (the first) when available and not in answer mode.
	if !ls.answered && len(ls.lesson.Trivia) > 0 {
		content += "\n\n---\n\n> " + ls.loc.T("lesson.trivia_prefix") + " " + ls.lesson.Trivia[0] + "\n"
	}

	if len(ls.lesson.References) > 0 {
		content += "\n\n" + ls.loc.T("lesson.section_see_also") + "\n\n"
		for _, r := range ls.lesson.References {
			content += "- " + r + "\n"
		}
	}

	// Glamour word wrap: viewport width minus glamour's own left margin (~4).
	wrapAt := ls.Width - 6
	if wrapAt < 40 {
		wrapAt = 40
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dracula"),
		glamour.WithWordWrap(wrapAt),
	)
	if err != nil {
		// Fallback: try dark style.
		renderer, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(wrapAt),
		)
		if err != nil {
			return ls.loc.T("lesson.render_error", err)
		}
	}

	out, err := renderer.Render(content)
	if err != nil {
		return ls.loc.T("lesson.render_error", err)
	}
	return out
}
