package tui

import (
	"fmt"
	"strings"
	"time"

	"ztutor/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// introCompleteMsg signals that this intro is finished.
type introCompleteMsg struct{ courseID string } // courseID non-empty → course intro finished

type introChoice struct {
	text    string
	replies []DialogueBeat
}

// introBeatMeta holds intro-specific beat data not covered by DialogueBeat.
type introBeatMeta struct {
	isBoot  bool
	isTitle bool
	choices []introChoice
}

// buildBaseIntroBeats constructs the intro sequence from the given locale.
// Returns dialogue beats and their corresponding metadata arrays.
func buildBaseIntroBeats(loc *i18n.Locale) ([]DialogueBeat, []introBeatMeta) {
	return []DialogueBeat{
			{
				Text:      loc.T("intro.boot"),
				Speed:     5,
				AutoDelay: 500 * time.Millisecond,
			},
			{
				Text:      loc.T("intro.title"),
				Speed:     1,
				AutoDelay: 2200 * time.Millisecond,
			},
			{
				Text:      loc.T("intro.new_face"),
				Mood:      MoodCurious,
				Speed:     2,
				AutoDelay: 1800 * time.Millisecond,
			},
			{
				Text:      loc.T("intro.mochi_intro"),
				Mood:      MoodIdle,
				Speed:     2,
				AutoDelay: 2200 * time.Millisecond,
			},
			{
				// Choice beat — autoDelay ignored; DialogueChoice takes over after typing.
				Text: loc.T("intro.why_question"),
				Mood: MoodCurious,
			},
			{
				Text:      loc.T("intro.closing"),
				Mood:      MoodHappy,
				Speed:     2,
				AutoDelay: 0,
			},
		}, []introBeatMeta{
			{isBoot: true},
			{isTitle: true},
			{},
			{},
			{choices: []introChoice{
				{
					text: loc.T("intro.choice_understand"),
					replies: []DialogueBeat{
						{Text: loc.T("intro.reply_understand"), Mood: MoodHappy, Speed: 2, AutoDelay: 2200 * time.Millisecond},
					},
				},
				{
					text: loc.T("intro.choice_told"),
					replies: []DialogueBeat{
						{Text: loc.T("intro.reply_told"), Mood: MoodIdle, Speed: 2, AutoDelay: 2200 * time.Millisecond},
					},
				},
				{
					text: loc.T("intro.choice_challenge"),
					replies: []DialogueBeat{
						{Text: loc.T("intro.reply_challenge"), Mood: MoodFocused, Speed: 2, AutoDelay: 1800 * time.Millisecond},
					},
				},
			}},
			{},
		}
}

type IntroScreen struct {
	sized
	dialogueEngine
	beatMeta     []introBeatMeta // parallel to d.Beats; stores intro-specific metadata
	choiceIdx    int
	skipDialogue bool   // when true, complete after the last boot/title beat
	courseID     string // non-empty → this is a course-specific intro
	courseLang   string // needed to rebuild course beats on ^L
	courseTitle  string // needed to rebuild course beats on ^L
	customBeats  []string
	loc          *i18n.Locale
}

func NewIntroScreen(width, height int, skipDialogue bool, loc *i18n.Locale) *IntroScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	beats, metas := buildBaseIntroBeats(loc)
	if skipDialogue {
		var filtBeats []DialogueBeat
		var filtMetas []introBeatMeta
		for i, m := range metas {
			if m.isBoot || m.isTitle {
				filtBeats = append(filtBeats, beats[i])
				filtMetas = append(filtMetas, m)
			}
		}
		beats = filtBeats
		metas = filtMetas
		if len(beats) > 0 {
			last := &beats[len(beats)-1]
			if last.AutoDelay == 0 {
				last.AutoDelay = 1800 * time.Millisecond
			}
		}
	}
	return &IntroScreen{
		sized:          sized{Width: width, Height: height},
		dialogueEngine: dialogueEngine{Beats: beats, MaxLines: 6, rtlMascotAlign: lipgloss.Center},
		beatMeta:       metas,
		skipDialogue:   skipDialogue,
		loc:            loc,
	}
}

func (is *IntroScreen) SetMascotFrame(frame int) {
	is.dialogueEngine.Frame = frame
}

func (is *IntroScreen) Init() tea.Cmd {
	if len(is.Beats) == 0 {
		return backCmd(introCompleteMsg{courseID: is.courseID})
	}
	return is.TypeTick()
}

// advanceOrDone calls Advance and, in skipDialogue mode, fires introCompleteMsg
// when we were already on the last beat and couldn't advance.
func (is *IntroScreen) advanceOrDone() tea.Cmd {
	prev := is.BeatIdx
	is.Advance()
	if is.skipDialogue && is.BeatIdx == prev {
		return backCmd(introCompleteMsg{courseID: is.courseID})
	}
	return is.TypeTick()
}

func (is *IntroScreen) finishCurrentBeat() tea.Cmd {
	meta := is.beatMeta[is.BeatIdx]
	if len(meta.choices) > 0 {
		is.CharIdx = len([]rune(is.Beats[is.BeatIdx].Text))
		is.Phase = DialogueChoice
		return nil
	}
	return is.dialogueEngine.FinishBeat()
}

func (is *IntroScreen) selectChoice(idx int) tea.Cmd {
	meta := is.beatMeta[is.BeatIdx]
	if idx < 0 || idx >= len(meta.choices) {
		return nil
	}
	replies := meta.choices[idx].replies
	// Splice reply beats in immediately after the current beat.
	tail := make([]DialogueBeat, len(is.Beats[is.BeatIdx+1:]))
	copy(tail, is.Beats[is.BeatIdx+1:])
	is.Beats = append(is.Beats[:is.BeatIdx+1], replies...)
	is.Beats = append(is.Beats, tail...)
	// Splice empty meta for reply beats (replies are plain dialogue, no choices/boot/title).
	emptyMeta := make([]introBeatMeta, len(replies))
	metaTail := make([]introBeatMeta, len(is.beatMeta[is.BeatIdx+1:]))
	copy(metaTail, is.beatMeta[is.BeatIdx+1:])
	is.beatMeta = append(is.beatMeta[:is.BeatIdx+1], emptyMeta...)
	is.beatMeta = append(is.beatMeta, metaTail...)
	is.choiceIdx = 0
	is.Advance()
	return is.TypeTick()
}

func (is *IntroScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(is.Beats) == 0 {
		return is, backCmd(introCompleteMsg{courseID: is.courseID})
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		is.HandleResize(msg)

	case dialogueTypeTick:
		if is.Phase != DialogueTyping {
			return is, nil
		}
		beat := is.Beats[is.BeatIdx]
		total := len([]rune(beat.Text))
		spd := beat.Speed
		if spd <= 0 {
			spd = 2
		}
		is.CharIdx += spd
		if is.CharIdx >= total {
			return is, is.finishCurrentBeat()
		}
		return is, is.TypeTick()

	case dialogueAdvanceTick:
		if msg.BeatIdx != is.BeatIdx || is.Phase != DialogueWaiting {
			return is, nil
		}
		return is, is.advanceOrDone()

	case tea.KeyMsg:
		if msg.String() == KeyQuit {
			return is, tea.Quit
		}

		if msg.String() == KeyLanguage {
			next := is.loc.Next()
			is.loc = next
			switch {
			case is.courseID != "":
				is.Beats, is.beatMeta = courseIntroBeats(is.courseLang, is.courseTitle, is.customBeats, next)
			case is.skipDialogue:
				baseBeats, baseMetas := buildBaseIntroBeats(next)
				is.Beats = nil
				is.beatMeta = nil
				for i, m := range baseMetas {
					if m.isBoot || m.isTitle {
						is.Beats = append(is.Beats, baseBeats[i])
						is.beatMeta = append(is.beatMeta, m)
					}
				}
				if len(is.Beats) > 0 && is.Beats[len(is.Beats)-1].AutoDelay == 0 {
					is.Beats[len(is.Beats)-1].AutoDelay = 1800 * time.Millisecond
				}
			default:
				is.Beats, is.beatMeta = buildBaseIntroBeats(next)
			}
			if is.BeatIdx >= len(is.Beats) {
				is.BeatIdx = len(is.Beats) - 1
			}
			is.CharIdx = 0
			is.Phase = DialogueTyping
			is.choiceIdx = 0
			return is, tea.Batch(
				backCmd(changeLangMsg{lang: next.Lang()}),
				is.TypeTick(),
			)
		}

		switch is.Phase {
		case DialogueTyping:
			return is, is.finishCurrentBeat()

		case DialogueWaiting:
			return is, is.advanceOrDone()

		case DialogueHolding:
			return is, backCmd(introCompleteMsg{courseID: is.courseID})

		case DialogueChoice:
			meta := is.beatMeta[is.BeatIdx]
			switch msg.String() {
			case KeyUp, KeyUpVim:
				if is.choiceIdx > 0 {
					is.choiceIdx--
				}
			case KeyDown, KeyDownVim:
				if is.choiceIdx < len(meta.choices)-1 {
					is.choiceIdx++
				}
			case KeySelect, KeySelectAlt:
				return is, is.selectChoice(is.choiceIdx)
			case KeyBack:
				return is, backCmd(introCompleteMsg{courseID: is.courseID})
			}
		}
	}
	return is, nil
}

func (is *IntroScreen) View() string {
	if len(is.Beats) == 0 {
		return ""
	}
	meta := is.beatMeta[is.BeatIdx]

	// Visible text for the current beat.
	beat := is.Beats[is.BeatIdx]
	runes := []rune(beat.Text)
	idx := is.CharIdx
	if idx > len(runes) {
		idx = len(runes)
	}
	visible := string(runes[:idx])

	// ── Boot sequence card ────────────────────────────────────────────────────
	if meta.isBoot {
		promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
		okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
		cursor := ""
		if is.Phase == DialogueTyping && is.Frame%2 == 0 {
			cursor = "_"
		}
		lines := strings.Split(visible, "\n")
		var c strings.Builder
		c.WriteString(promptStyle.Render("$ ztutor") + "\n\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "[OK]") {
				c.WriteString("  " + okStyle.Render("[OK]") + dimStyle.Render(line[4:]) + "\n")
			} else {
				isLast := i == len(lines)-1
				if isLast && is.Phase == DialogueTyping {
					c.WriteString("  " + line + cursor + "\n")
				} else {
					c.WriteString("  " + line + "\n")
				}
			}
		}
		c.WriteString("\n" + helpBar(is.loc.T("help.language")))
		return lipgloss.Place(is.Width, is.Height, lipgloss.Center, lipgloss.Center, c.String())
	}

	// ── Title card ────────────────────────────────────────────────────────────
	if meta.isTitle {
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
		subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

		borderColors := []lipgloss.Color{"212", "213", "212", "211"}
		bc := borderColors[is.Frame%len(borderColors)]
		boxStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(bc).
			Padding(1, 5)

		parts := strings.SplitN(beat.Text, "\n\n", 2)
		titleRunes := []rune(parts[0])
		titleVisible := string(titleRunes[:min(idx, len(titleRunes))])

		blinking := is.Phase == DialogueTyping && is.Frame%2 == 0
		titleCursor, subCursor := "", ""
		if blinking {
			if idx <= len(titleRunes) {
				titleCursor = "_"
			} else {
				subCursor = "_"
			}
		}

		if is.loc != nil && is.loc.IsRTL() {
			boxStyle = boxStyle.Align(lipgloss.Right)
		}
		var c strings.Builder
		c.WriteString(titleStyle.Render(titleVisible+titleCursor) + "\n")
		if len(parts) > 1 && idx > len(titleRunes)+2 {
			subRunes := []rune(parts[1])
			subIdx := idx - len(titleRunes) - 2
			if subIdx > len(subRunes) {
				subIdx = len(subRunes)
			}
			c.WriteString("\n" + subtitleStyle.Render(string(subRunes[:subIdx])+subCursor))
		}
		if is.Phase == DialogueHolding {
			c.WriteString("\n\n" + helpBar(is.loc.T("help.any_key"), is.loc.T("help.language")))
		}
		return lipgloss.Place(is.Width, is.Height, lipgloss.Center, lipgloss.Center,
			boxStyle.Render(c.String()))
	}

	// ── Dialogue beat ─────────────────────────────────────────────────────────

	loc := is.loc
	if loc == nil {
		loc = i18n.New("en")
	}
	rtl := loc.IsRTL()
	boxW := is.dialogueEngine.boxW(is.Width)

	var content strings.Builder

	// Mascot sprite.
	content.WriteString(is.dialogueEngine.RenderMascot(loc, is.Width))
	content.WriteString("\n\n")

	// Text box.
	content.WriteString(is.dialogueEngine.RenderTextBox(loc, is.Width))
	content.WriteString("\n\n")

	// Bottom section: choices or hint bar.
	arrow := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render("❯")
	if rtl {
		arrow = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render("❮")
	}
	if is.Phase == DialogueChoice {
		selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true)
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
		for i, ch := range meta.choices {
			if i == is.choiceIdx {
				if rtl {
					content.WriteString(rtlWrap(rtl, selStyle.Render(ch.text)+"  "+arrow, boxW+2) + "\n")
				} else {
					content.WriteString("  " + arrow + "  " + selStyle.Render(ch.text) + "\n")
				}
			} else {
				content.WriteString(rtlWrap(rtl, dimStyle.Render(ch.text), boxW+2) + "\n")
			}
		}
		content.WriteString("\n" + rtlWrap(rtl, helpBar(loc.T("help.choose"), loc.T("help.confirm"), loc.T("help.language"), loc.T("help.skip_intro")), boxW+2))
	} else {
		var bar string
		switch is.Phase {
		case DialogueWaiting:
			bar = helpBar(loc.T("help.any_key_continue"), loc.T("help.language"), loc.T("help.skip_intro"))
		case DialogueHolding:
			bar = helpBar(loc.T("help.any_key"), loc.T("help.language"), loc.T("help.skip_intro"))
		default:
			bar = helpBar(loc.T("help.any_key_skip"), loc.T("help.language"), loc.T("help.skip_intro"))
		}
		content.WriteString(rtlWrap(rtl, bar, boxW+2))
	}

	return lipgloss.Place(is.Width, is.Height, lipgloss.Center, lipgloss.Center, content.String())
}

// ── Course-specific intro ─────────────────────────────────────────────────────

// NewCourseIntroScreen builds an IntroScreen with beats tailored to the course.
func NewCourseIntroScreen(courseID, courseLang, title string, customBeats []string, loc *i18n.Locale, width, height int) *IntroScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	beats, metas := courseIntroBeats(courseLang, title, customBeats, loc)
	return &IntroScreen{
		sized:          sized{Width: width, Height: height},
		dialogueEngine: dialogueEngine{Beats: beats, MaxLines: 6, rtlMascotAlign: lipgloss.Center},
		beatMeta:       metas,
		courseID:       courseID,
		courseLang:     courseLang,
		courseTitle:    title,
		customBeats:    customBeats,
		loc:            loc,
	}
}

// introLangKey normalises a sandbox language ID to the short key used in locale files.
func introLangKey(lang string) string {
	switch lang {
	case "python":
		return "py"
	case "c++", "cpp":
		return "c"
	}
	return lang
}

func courseIntroBeats(courseLang, title string, customBeats []string, loc *i18n.Locale) ([]DialogueBeat, []introBeatMeta) {
	if len(customBeats) == 0 {
		// Fall back to locale-keyed beats for the course language.
		shortLang := introLangKey(courseLang)
		for i := 1; i <= 4; i++ {
			key := fmt.Sprintf("intro.%s.beat%d", shortLang, i)
			val := loc.T(key)
			if val == key {
				break
			}
			customBeats = append(customBeats, val)
		}
	}
	if len(customBeats) == 0 {
		// Ultimate fallback: generic default beats.
		customBeats = append(customBeats, loc.T("intro.default.beat1", title))
		for i := 2; i <= 4; i++ {
			key := fmt.Sprintf("intro.default.beat%d", i)
			val := loc.T(key)
			if val == key {
				break
			}
			customBeats = append(customBeats, val)
		}
	}
	if len(customBeats) == 0 {
		return nil, nil
	}
	beats := make([]DialogueBeat, len(customBeats))
	for i, text := range customBeats {
		autoDelay := 2000 * time.Millisecond
		if i == len(customBeats)-1 {
			autoDelay = 0
		}
		beats[i] = DialogueBeat{Text: text + "\n", Mood: MoodCurious, Speed: 2, AutoDelay: autoDelay}
	}
	metas := make([]introBeatMeta, len(customBeats))
	return beats, metas
}
