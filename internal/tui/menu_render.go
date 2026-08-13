package tui

import (
	"fmt"
	"strings"
	"time"

	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	"github.com/charmbracelet/lipgloss"
)

// menuStyles returns fresh item/selected style pairs for the given panel width
// and horizontal alignment. Called once per render instead of using package-level
// vars, because lipgloss Style.rules is a map — calling .Width()/.Align() on a
// package-level var permanently mutates it, leaking alignment across renders.
func menuStyles(w int, align lipgloss.Position) (item, selected lipgloss.Style) {
	item = lipgloss.NewStyle().Padding(0, 1).Width(w).Align(align)
	selected = lipgloss.NewStyle().Padding(0, 1).
		Background(lipgloss.Color(ColorBG)).
		Foreground(lipgloss.Color("255")).
		Width(w).Align(align)
	return
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func difficultyColor(d string) lipgloss.Color {
	switch strings.ToLower(d) {
	case "beginner":
		return lipgloss.Color(ColorSuccess)
	case "intermediate":
		return lipgloss.Color(ColorAmber)
	case "advanced":
		return lipgloss.Color(ColorError)
	default:
		return lipgloss.Color(ColorDim)
	}
}

func (m *MenuScreen) timeGreeting() string {
	h := time.Now().Hour()
	switch {
	case h < 4:
		return m.loc.T("menu.greeting.midnight")
	case h < 12:
		return m.loc.T("menu.greeting.morning")
	case h < 18:
		return m.loc.T("menu.greeting.afternoon")
	default:
		return m.loc.T("menu.greeting.evening")
	}
}

func truncRunes(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	return string(runes[:maxW-1]) + "…"
}

func padVis(s string, visW int) string {
	w := lipgloss.Width(s)
	if w >= visW {
		return s
	}
	return s + strings.Repeat(" ", visW-w)
}

func programmingLangLabel(lang string) string {
	if l := sandbox.GetLanguage(lang); l != nil {
		return l.DisplayName()
	}
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return ""
	}
	return strings.ToUpper(lang[:1]) + lang[1:]
}

func langLabels(langs []string, transform func(string) string) []string {
	if len(langs) == 0 {
		return nil
	}
	labels := make([]string, 0, len(langs))
	for _, lang := range langs {
		label := strings.TrimSpace(lang)
		if transform != nil {
			label = transform(label)
		}
		if label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

func renderBadgeLabels(labels []string, color lipgloss.Color, padded bool, compact bool) string {
	if len(labels) == 0 {
		return ""
	}
	joiner := "·"
	body := ""
	if compact && len(labels) > 1 {
		body = fmt.Sprintf("%s +%d", labels[0], len(labels)-1)
	} else {
		if padded {
			joiner = " · "
		}
		body = strings.Join(labels, joiner)
	}
	if padded {
		body = " " + body + " "
	}
	s := lipgloss.NewStyle().Foreground(color)
	return s.Render("{" + body + "}")
}

// uiLangBadges renders the available UI translation languages for a course as
// a compact muted badge, e.g. {en·es·ar}.  Returns empty string when langs is nil.
func uiLangBadges(langs []string) string {
	return renderBadgeLabels(langLabels(langs, nil), lipgloss.Color("244"), false, false)
}

func programmingLangBadges(langs []string) string {
	return renderBadgeLabels(langLabels(langs, programmingLangLabel), lipgloss.Color(ColorAccent), true, false)
}

func compactProgrammingLangBadges(langs []string) string {
	return renderBadgeLabels(langLabels(langs, programmingLangLabel), lipgloss.Color(ColorAccent), true, true)
}

func compactUILangBadges(langs []string) string {
	return renderBadgeLabels(langLabels(langs, nil), lipgloss.Color("244"), false, true)
}

type courseLineSegment struct {
	variants []string
	index    int
}

func (s *courseLineSegment) current() string {
	if len(s.variants) == 0 || s.index >= len(s.variants) {
		return ""
	}
	return s.variants[s.index]
}

func (s *courseLineSegment) degrade() bool {
	if s.index+1 >= len(s.variants) {
		return false
	}
	s.index++
	return true
}

func joinCourseLineParts(rtl bool, parts ...string) string {
	return joinInlineParts(rtl, parts...)
}

func sectionCounts(c *lesson.Course) (lessons, interviews, quizzes, challenges int) {
	for _, s := range c.Sections {
		switch s.Type {
		case "interviews":
			interviews += len(s.Lessons)
		case "quizzes":
			quizzes += len(s.Quizzes)
		case "challenges":
			challenges += len(s.Challenges)
		default:
			lessons += len(s.Lessons)
		}
	}
	return
}

// courseProgressCounts returns how many lessons/challenges in the course the
// user has completed (stars > 0) out of the total available.
func courseProgressCounts(c *lesson.Course, progress map[string]int) (done, total int) {
	for _, s := range c.Sections {
		switch s.Type {
		case "challenges":
			total += len(s.Challenges)
			for _, ch := range s.Challenges {
				if progress[ch.ID] > 0 {
					done++
				}
			}
		case "quizzes":
			total += len(s.Quizzes)
			for _, q := range s.Quizzes {
				if progress[q.ID] > 0 {
					done++
				}
			}
		default:
			total += len(s.Lessons)
			for _, l := range s.Lessons {
				if progress[l.ID] > 0 {
					done++
				}
			}
		}
	}
	return
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *MenuScreen) View() string {
	var b strings.Builder

	// alignR right-aligns s to the full terminal width when the locale is RTL.
	rtl := m.loc.IsRTL()
	alignR := func(s string) string {
		if !rtl {
			return s
		}
		return lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(s)
	}

	// Title — terminal-style header with subtle pulse.
	titleColors := []lipgloss.Color{"212", "213", "213", "212"}
	tc := titleColors[m.mascotFrame%len(titleColors)]
	pTitle := lipgloss.NewStyle().Bold(true).Foreground(tc).Render("ztutor // navigator")
	subTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)).Render("course graph :: lesson index :: progress trace")
	b.WriteString(alignR(pTitle))
	b.WriteString("\n")
	b.WriteString(alignR(subTitle))
	b.WriteString("\n")
	headerSep := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Render(strings.Repeat("─", max(12, m.Width)))
	b.WriteString(alignR(headerSep))
	b.WriteString("\n\n")

	T := m.loc.T

	// Greeting + streak — build as one string so it can be right-aligned as a unit.
	greetStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorBody))
	greetLine := greetStyle.Render(m.timeGreeting() + ", " + m.username + "!")
	if m.streak > 0 {
		label := T("menu.streak", m.streak)
		var sc lipgloss.Style
		switch {
		case m.streak >= 14:
			sc = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true)
		case m.streak >= 7:
			sc = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Bold(true)
		case m.streak >= 3:
			sc = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold))
		default:
			sc = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
		}
		greetLine += "   " + sc.Render(label)
	}
	b.WriteString(alignR(greetLine) + "\n")

	// Stats: count lessons across all sections.
	completed, totalStars, total := 0, 0, 0
	for _, c := range m.courses {
		for _, sec := range c.Sections {
			if sec.Type == "challenges" {
				continue
			}
			total += len(sec.Lessons)
			for _, l := range sec.Lessons {
				if s := m.progress[l.ID]; s > 0 {
					completed++
					totalStars += s
				}
			}
		}
	}
	statsLine := T("menu.lessons_complete", completed, total)
	if totalStars > 0 {
		statsLine += fmt.Sprintf("  ★ %d", totalStars)
	}
	b.WriteString(alignR(dim(statsLine)) + "\n")

	// Language-aware cycling snippet.
	b.WriteString(alignR(codeStyle("$ "+m.currentSnippet())) + "\n")

	// Spaced-repetition hint.
	if s := m.suggestedLesson(); s != nil {
		stars := m.progress[s.ID]
		filledS := strings.Repeat("★", stars)
		emptyS := strings.Repeat("☆", s.MaxStars()-stars)
		starStr := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render(filledS + emptyS)
		reviewStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber))
		reviewLine := reviewStyle.Render(T("menu.review_label")) + " " + dim(s.Title) + " " + starStr + dim("  "+T("menu.review_hint"))
		b.WriteString(alignR(reviewLine) + "\n")
	}

	// Achievement notifications.
	if len(m.notifications) > 0 {
		b.WriteString("\n")
		for _, n := range m.notifications {
			b.WriteString(alignR(notifStyle.Render("[!] "+n)) + "\n")
		}
	}
	b.WriteString("\n")

	// Search bar.
	if m.searchActive {
		searchPrefix := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render("[/]")
		b.WriteString(alignR(searchPrefix + " " + m.searchInput.View()))
	} else {
		b.WriteString(alignR(dim("[/] " + T("menu.search_hint"))))
	}
	b.WriteString("\n\n")

	// Render content based on view level.
	if m.viewLevel == "courses" {
		return m.renderCourseView(&b)
	}
	return m.renderLessonView(&b)
}

func (m *MenuScreen) renderCourseView(b *strings.Builder) string {
	courses := m.matchingCourses(m.searchQuery)
	ph := m.panelHeight()

	rtl := m.loc.IsRTL()
	alignR := func(s string) string {
		if !rtl {
			return s
		}
		return lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(s)
	}

	if len(courses) == 0 {
		if m.searchActive && m.searchQuery != "" {
			b.WriteString(alignR(dim(m.loc.T("menu.no_match", m.searchQuery))))
		} else {
			b.WriteString(alignR(dim(m.loc.T("menu.no_courses"))))
		}
		b.WriteString("\n\n")
		footer := m.renderFooter(m.Width)
		pad := m.Height - lipgloss.Height(b.String()) - lipgloss.Height(footer)
		if pad > 0 {
			b.WriteString(strings.Repeat("\n", pad))
		}
		b.WriteString(footer)
		return b.String()
	}

	panelW := m.Width
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorDim)).
		Render(fmt.Sprintf("[courses:%02d] [cursor:%02d]", len(courses), m.courseCursor+1))
	b.WriteString(alignR(header))
	b.WriteString("\n")
	b.WriteString(alignR(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Render(strings.Repeat("─", max(12, m.Width)))))
	b.WriteString("\n")

	align := lipgloss.Left
	if rtl {
		align = lipgloss.Right
	}
	itemSt, selSt := menuStyles(panelW, align)

	for i := m.courseOffset; i < len(courses) && i < m.courseOffset+ph; i++ {
		line := m.renderCourseLine(courses[i], panelW)
		if i == m.courseCursor {
			b.WriteString(selSt.Render(line))
		} else {
			b.WriteString(itemSt.Render(line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	footer := m.renderFooter(m.Width)
	pad := m.Height - lipgloss.Height(b.String()) - lipgloss.Height(footer)
	if pad > 0 {
		b.WriteString(strings.Repeat("\n", pad))
	}
	b.WriteString(footer)
	return b.String()
}

func (m *MenuScreen) renderCourseLine(c lesson.Course, maxWidth int) string {
	nl, ni, nq, nc := sectionCounts(&c)
	T := m.loc.T
	rtl := m.loc.IsRTL()

	// Build compact count chips.
	var chips []string
	if nl > 0 {
		chips = append(chips, dim(T("menu.chip_lessons", nl)))
	}
	if ni > 0 {
		chips = append(chips, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHex)).Render(T("menu.chip_interviews", ni)))
	}
	if nq > 0 {
		chips = append(chips, lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Render(T("menu.chip_quizzes", nq)))
	}
	if nc > 0 {
		chips = append(chips, lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render(T("menu.chip_challenges", nc)))
	}
	counts := strings.Join(chips, dim("  ·  "))

	done, total := courseProgressCounts(&c, m.progress)
	progressStr := ""
	if total > 0 {
		if done == total {
			progressStr = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true).Render(fmt.Sprintf("%d/%d", done, total))
		} else if done > 0 {
			progressStr = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render(fmt.Sprintf("%d/%d", done, total))
		} else {
			progressStr = dim(fmt.Sprintf("0/%d", total))
		}
	}

	progLangs := c.ProgrammingLanguages
	if len(progLangs) == 0 && c.Language != "" {
		progLangs = []string{c.Language}
	}
	segments := map[string]*courseLineSegment{
		"prog": {
			variants: []string{
				programmingLangBadges(progLangs),
				compactProgrammingLangBadges(progLangs),
				"",
			},
		},
		"ui": {
			variants: []string{
				uiLangBadges(c.UILanguages),
				compactUILangBadges(c.UILanguages),
				"",
			},
		},
		"counts":   {variants: []string{counts, ""}},
		"progress": {variants: []string{progressStr, ""}},
	}
	degradeOrder := []string{"counts", "progress", "ui", "prog"}
	titleText := c.Title
	titleStyle := lipgloss.NewStyle().Bold(true)
	fullTitleWidth := lipgloss.Width(titleText)
	minTitleWidth := min(12, fullTitleWidth)
	if minTitleWidth < 1 {
		minTitleWidth = 1
	}
	if maxWidth <= 0 {
		return joinCourseLineParts(
			rtl,
			titleStyle.Render(titleText),
			segments["prog"].current(),
			segments["ui"].current(),
			segments["counts"].current(),
			segments["progress"].current(),
		)
	}
	for {
		suffix := joinCourseLineParts(
			rtl,
			segments["prog"].current(),
			segments["ui"].current(),
			segments["counts"].current(),
			segments["progress"].current(),
		)
		availableTitle := maxWidth
		if suffix != "" {
			availableTitle -= lipgloss.Width(suffix) + 2
		}
		if availableTitle >= fullTitleWidth {
			return joinCourseLineParts(rtl, titleStyle.Render(titleText), suffix)
		}
		if availableTitle >= minTitleWidth {
			return joinCourseLineParts(rtl, titleStyle.Render(truncRunes(titleText, availableTitle)), suffix)
		}
		degraded := false
		for _, key := range degradeOrder {
			if segments[key].degrade() {
				degraded = true
				break
			}
		}
		if !degraded {
			if availableTitle < 1 {
				availableTitle = 1
			}
			return joinCourseLineParts(rtl, titleStyle.Render(truncRunes(titleText, availableTitle)), suffix)
		}
	}
}

// ── Level 2: Lesson / Challenge View ──────────────────────────────────────────

func (m *MenuScreen) renderLessonView(b *strings.Builder) string {
	if m.selectedCourse == nil {
		return b.String() + dim("No course selected.") + "\n"
	}

	rtl := m.loc.IsRTL()
	courseHdr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent)).Render("[course] " + m.selectedCourse.Title)
	if rtl {
		courseHdr = lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(courseHdr)
	}
	b.WriteString(courseHdr)
	b.WriteString("\n")
	metaHdr := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)).Render(
		fmt.Sprintf("[items:%02d] [cursor:%02d]", len(m.displayItems), m.lessonCursor+1),
	)
	if rtl {
		metaHdr = lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(metaHdr)
	}
	b.WriteString(metaHdr)
	b.WriteString("\n")
	tabs := m.renderSectionTabs()
	if rtl {
		tabs = lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(tabs)
	}
	b.WriteString(tabs)
	b.WriteString("\n")
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Render(strings.Repeat("─", max(12, m.Width)))
	if rtl {
		sep = lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(sep)
	}
	b.WriteString(sep)
	b.WriteString("\n")

	ph := m.panelHeight()
	items := m.displayItems
	arrow := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render("❯")
	if rtl {
		arrow = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render("❮")
	}

	align := lipgloss.Left
	if rtl {
		align = lipgloss.Right
	}
	itemSt, selSt := menuStyles(m.Width, align)

	for i := m.lessonOffset; i < len(items) && i < m.lessonOffset+ph; i++ {
		di := items[i]
		line := m.renderLessonLine(di)
		if i == m.lessonCursor {
			if rtl {
				b.WriteString(selSt.Render(line + " " + arrow))
			} else {
				b.WriteString(selSt.Render(" " + arrow + " " + line))
			}
		} else {
			if rtl {
				b.WriteString(itemSt.Render(line))
			} else {
				b.WriteString(itemSt.Render("   " + line))
			}
		}
		b.WriteString("\n")
	}

	if len(items) == 0 {
		var emptyMsg string
		if m.searchActive && m.searchQuery != "" {
			emptyMsg = dim(m.loc.T("menu.lesson_no_match", m.searchQuery))
		} else {
			emptyMsg = dim(m.loc.T("menu.no_items"))
		}
		if rtl {
			emptyMsg = lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(emptyMsg)
		}
		b.WriteString(emptyMsg + "\n")
	}

	b.WriteString("\n")

	footer := m.renderFooter(m.Width)
	pad := m.Height - lipgloss.Height(b.String()) - lipgloss.Height(footer)
	if pad > 0 {
		b.WriteString(strings.Repeat("\n", pad))
	}
	b.WriteString(footer)
	return b.String()
}

func (m *MenuScreen) renderSectionTabs() string {
	if m.selectedCourse == nil || len(m.selectedCourse.Sections) == 0 {
		return ""
	}

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("232")).
		Background(lipgloss.Color(ColorAccent)).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorDim)).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1)

	var tabs []string
	for i, sec := range m.selectedCourse.Sections {
		title := sec.Title
		if title == "" {
			title = sec.Type
		}
		if i == m.sectionIndex {
			tabs = append(tabs, activeStyle.Render("["+title+"]"))
		} else {
			tabs = append(tabs, inactiveStyle.Render("<"+title+">"))
		}
	}
	return strings.Join(tabs, " ")
}

func (m *MenuScreen) renderLessonLine(di displayItem) string {
	kindBadge := "[LES]"
	switch di.kind {
	case "quiz":
		kindBadge = "[QIZ]"
	case "challenge":
		kindBadge = "[CHL]"
	}
	kindStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHex))

	// Stars / completion indicator (3 visible chars).
	var starStr string
	switch di.kind {
	case "lesson":
		starStr = starsStyle(m.progress[di.lesson.ID], di.lesson.MaxStars())
	case "quiz":
		starStr = starsStyle(m.progress[di.quiz.ID], 3)
	default:
		starStr = dim("   ")
	}

	// Compact difficulty badge — fixed 5 visible chars "[beg]" / "[int]" / "[adv]".
	var diffBadge string
	switch strings.ToLower(di.difficulty) {
	case "beginner":
		diffBadge = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("[beg]")
	case "intermediate":
		diffBadge = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render("[int]")
	case "advanced":
		diffBadge = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render("[adv]")
	default:
		if di.kind == "challenge" {
			diffBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render("[chl]")
		} else if di.kind == "quiz" {
			diffBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Render("[qiz]")
		} else {
			diffBadge = dim("[---]")
		}
	}

	title := di.title

	return kindStyle.Render(kindBadge) + "  " + starStr + "  " + diffBadge + "  " + title
}

// renderFooter builds the mascot panel + helpbar string that is pinned to the
// bottom of every menu view. panelW is the width used for the mascot panel.
func (m *MenuScreen) renderFooter(panelW int) string {
	var b strings.Builder
	if !m.mascotHidden {
		b.WriteString(renderMascotPanel(panelW, "Mochi", m.mascotLine(), m.mascotMood(), m.mascotFrame, m.loc.IsRTL()))
		b.WriteString("\n")
	}
	b.WriteString(m.renderHelpBar())
	return b.String()
}

// ── Help Bar ─────────────────────────────────────────────────────────────────

func (m *MenuScreen) renderHelpBar() string {
	var hb []HelpAction
	if m.viewLevel == "courses" {
		hb = []HelpAction{HA(ActionNavigate), HA(ActionSearch), HA(ActionSelect)}
	} else {
		hb = []HelpAction{HA(ActionNavigate), HA(ActionSection), HA(ActionSearch), HA(ActionSelect), HA(ActionMenuBack)}
	}
	if m.suggestedLesson() != nil {
		hb = append(hb, HA(ActionReview))
	}
	hb = append(hb, HA(ActionAchievements), HA(ActionLeaderboard), HA(ActionSettings), HA(ActionCredits), HA(ActionMochi), HA(ActionLanguage))
	if m.isAdmin {
		hb = append(hb, HA(ActionAdmin))
	}
	hb = append(hb, HA(ActionQuit))
	bar := actionHelpBar(m.loc, hb...)
	if m.loc.IsRTL() {
		return lipgloss.NewStyle().Width(m.Width).Align(lipgloss.Right).Render(bar)
	}
	return bar
}
