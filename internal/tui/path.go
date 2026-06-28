package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pathStatus describes whether a path node is reachable by the learner.
type pathStatus int

const (
	pathLocked    pathStatus = iota // prerequisites not met
	pathAvailable                   // ready to attempt
	pathCompleted                   // stars > 0
)

// pathEntry is a decorated item on the course path.
type pathEntry struct {
	lesson  lesson.Lesson
	section lesson.Section
	status  pathStatus
}

// PathScreen renders a game-like sequential node path for a course.
// Activated when the course declares layout: path in course.yaml.
type PathScreen struct {
	course   lesson.Course
	entries  []pathEntry
	progress map[string]int
	cursor   int
	frame    int

	viewport viewport.Model
	sized
	loc *i18n.Locale
}

// enterCoursePathMsg is fired by MenuScreen when entering a course with
// layout: path, so that App can build the PathScreen with access to
// app-level state (progress, locale, dimensions).
type enterCoursePathMsg struct{ course lesson.Course }

// ── Constructor ───────────────────────────────────────────────────────────────

func NewPathScreen(c lesson.Course, progress map[string]int, preferredLessonID string, loc *i18n.Locale, width, height int) *PathScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	entries := buildPathEntries(c, progress)

	vp := viewport.New(width, height-pathHeaderLines-pathFooterLines)

	ps := &PathScreen{
		course:   c,
		entries:  entries,
		progress: progress,
		viewport: vp,
		sized:    sized{Width: width, Height: height},
		loc:      loc,
	}
	ps.cursor = ps.cursorForLesson(preferredLessonID)
	ps.rebuildViewport()
	return ps
}

const (
	pathHeaderLines = 3 // title + separator + blank
	pathFooterLines = 2 // separator + key hints
)

// ── Path entry construction ───────────────────────────────────────────────────

func buildPathEntries(c lesson.Course, progress map[string]int) []pathEntry {
	var entries []pathEntry
	for _, sec := range c.Sections {
		if sec.Type == "challenges" || sec.Type == "quizzes" {
			continue
		}
		for _, l := range sec.Lessons {
			entries = append(entries, pathEntry{lesson: l, section: sec})
		}
	}
	for i := range entries {
		id := entries[i].lesson.ID
		if progress[id] > 0 {
			entries[i].status = pathCompleted
		} else if i == 0 || progress[entries[i-1].lesson.ID] > 0 {
			entries[i].status = pathAvailable
		} else {
			entries[i].status = pathLocked
		}
	}
	return entries
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

func (ps *PathScreen) Init() tea.Cmd { return nil }

func (ps *PathScreen) SetLocale(loc *i18n.Locale) {
	ps.loc = loc
	ps.rebuildViewport()
}

func (ps *PathScreen) SetMascotFrame(frame int) {
	ps.frame = frame
}

// SetCourse refreshes the underlying course data while preserving the current
// selection when possible. Used after course data is reloaded for a locale
// switch or other live refresh.
func (ps *PathScreen) SetCourse(c lesson.Course, progress map[string]int) {
	selectedLessonID := ""
	if ps.cursor >= 0 && ps.cursor < len(ps.entries) {
		selectedLessonID = ps.entries[ps.cursor].lesson.ID
	}
	ps.course = c
	ps.progress = progress
	ps.entries = buildPathEntries(c, progress)
	ps.cursor = 0
	if selectedLessonID != "" {
		ps.cursor = ps.cursorForLesson(selectedLessonID)
	}
	ps.rebuildViewport()
}

func (ps *PathScreen) cursorForLesson(preferredLessonID string) int {
	if preferredLessonID != "" {
		for i, e := range ps.entries {
			if e.lesson.ID == preferredLessonID {
				return i
			}
		}
	}
	// Default: first non-completed entry so the learner lands on the next
	// actionable node after reopening the course.
	for i, e := range ps.entries {
		if e.status != pathCompleted {
			return i
		}
	}
	return 0
}

func (ps *PathScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ps.HandleResize(msg)
		ps.viewport.Width = ps.pathPaneWidth()
		ps.viewport.Height = msg.Height - pathHeaderLines - pathFooterLines
		ps.rebuildViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case KeyBack, KeyBackAlt, KeyQuit:
			return ps, backCmd(NavigateToMenu{})

		case KeyUp, KeyUpVim:
			if ps.cursor > 0 {
				ps.cursor--
				ps.rebuildViewport()
			}
			return ps, nil

		case KeyDown, KeyDownVim:
			if ps.cursor < len(ps.entries)-1 {
				ps.cursor++
				ps.rebuildViewport()
			}
			return ps, nil

		case KeySelect:
			return ps.openCurrent()
		}
	}

	var cmd tea.Cmd
	ps.viewport, cmd = ps.viewport.Update(msg)
	return ps, cmd
}

func (ps *PathScreen) openCurrent() (tea.Model, tea.Cmd) {
	if ps.cursor >= len(ps.entries) {
		return ps, nil
	}
	e := ps.entries[ps.cursor]
	if e.status == pathLocked {
		return ps, nil
	}
	return ps, backCmd(NavigateToLessonMsg{Lesson: e.lesson})
}

func (ps *PathScreen) View() string {
	body := ps.viewport.View()
	if ps.showDetailPanel() {
		sep := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Render("│")
		detail := ps.renderDetailPanel()
		if ps.loc.IsRTL() {
			body = lipgloss.JoinHorizontal(lipgloss.Top, detail, sep, body)
		} else {
			body = lipgloss.JoinHorizontal(lipgloss.Top, body, sep, detail)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		ps.renderHeader(),
		body,
		ps.renderFooter(),
	)
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (ps *PathScreen) rebuildViewport() {
	ps.viewport.Width = ps.pathPaneWidth()
	ps.viewport.Height = ps.Height - pathHeaderLines - pathFooterLines
	ps.viewport.SetContent(ps.buildContent())
	ps.scrollToCursor()
}

func (ps *PathScreen) showDetailPanel() bool {
	return ps.Width >= 96
}

func (ps *PathScreen) detailPanelWidth() int {
	if !ps.showDetailPanel() {
		return 0
	}
	w := ps.Width / 3
	if w < 30 {
		w = 30
	}
	if w > 42 {
		w = 42
	}
	return w
}

func (ps *PathScreen) pathPaneWidth() int {
	if !ps.showDetailPanel() {
		return ps.Width
	}
	return ps.Width - ps.detailPanelWidth() - 1
}

// nodeBoxWidth returns the total width of a node box (borders included),
// capped so it never overflows the terminal.
func (ps *PathScreen) nodeBoxWidth() int {
	const preferred = 52
	const minWidth = 28
	// 2 chars for the cursor glyph, 4 chars of outer margin
	available := ps.pathPaneWidth() - 6
	if available < minWidth {
		return minWidth
	}
	if preferred < available {
		return preferred
	}
	return available
}

// nodeInnerWidth returns the content width inside the node borders.
func (ps *PathScreen) nodeInnerWidth() int {
	return ps.nodeBoxWidth() - 2
}

// buildContent assembles the full scrollable path as a single string.
func (ps *PathScreen) buildContent() string {
	var sb strings.Builder
	sb.WriteString("\n")

	multiSec := ps.hasMultipleSections()
	for i, e := range ps.entries {
		if multiSec && i > 0 && e.section.ID != ps.entries[i-1].section.ID {
			sb.WriteString(ps.renderSectionDivider(e.section.Title))
		}
		sb.WriteString(ps.renderNode(e, i == ps.cursor))
		if i < len(ps.entries)-1 {
			sb.WriteString(ps.renderConnector(e.status))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func (ps *PathScreen) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorAccent)).
		Bold(true)
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorDim))
	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBorder))

	done := 0
	for _, e := range ps.entries {
		if e.status == pathCompleted {
			done++
		}
	}
	total := len(ps.entries)

	title := titleStyle.Render(ps.course.Title)
	bar := pathProgressBar(done, total, 16)
	fraction := dimStyle.Render(fmt.Sprintf("%d / %d", done, total))
	progress := bar + "  " + fraction

	titleW := lipgloss.Width(title)
	progressW := lipgloss.Width(progress)
	gap := ps.Width - titleW - progressW - 2
	if gap < 1 {
		gap = 1
	}

	var line1 string
	if ps.loc.IsRTL() {
		line1 = progress + strings.Repeat(" ", gap) + title
	} else {
		line1 = title + strings.Repeat(" ", gap) + progress
	}
	sep := sepStyle.Render(strings.Repeat("─", ps.Width))

	return line1 + "\n" + sep + "\n"
}

func (ps *PathScreen) renderFooter() string {
	T := ps.loc.T
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Render(strings.Repeat("─", ps.Width))
	bar := helpBar(T("help.navigate"), T("path.help.open"), T("help.esc_back"))

	if ps.loc.IsRTL() {
		bar = lipgloss.NewStyle().Width(ps.Width).Align(lipgloss.Right).Render(bar)
	} else {
		barW := lipgloss.Width(bar)
		leftPad := (ps.Width - barW) / 2
		if leftPad < 0 {
			leftPad = 0
		}
		bar = strings.Repeat(" ", leftPad) + bar
	}

	return sep + "\n" + bar
}

func (ps *PathScreen) renderDetailPanel() string {
	w := ps.detailPanelWidth()
	h := ps.Height - pathHeaderLines - pathFooterLines
	if w <= 0 || h <= 0 {
		return ""
	}
	if ps.cursor < 0 || ps.cursor >= len(ps.entries) {
		return strings.Repeat(" ", w)
	}

	e := ps.entries[ps.cursor]
	l := e.lesson
	T := ps.loc.T
	rtl := ps.loc.IsRTL()

	align := lipgloss.Left
	if rtl {
		align = lipgloss.Right
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	tagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorHex)).
		Background(lipgloss.Color(ColorBG)).
		Padding(0, 1)
	diffStyle := lipgloss.NewStyle().Foreground(difficultyColor(l.Difficulty))
	metaValueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))

	innerW := w - 2
	if innerW < 10 {
		innerW = w
	}

	sepLine := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Render(strings.Repeat("─", innerW))

	var statusBadgeStyle lipgloss.Style
	statusText := T("path.status.locked")
	switch e.status {
	case pathCompleted:
		statusBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)
		statusText = T("path.status.completed")
	case pathAvailable:
		statusBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))
		statusText = T("path.status.ready")
	default:
		statusBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	}
	mode := T("path.type.exercise")
	if l.IsReadOnly() {
		mode = T("path.type.reading")
	} else if len(l.Files) > 0 {
		mode = T("path.type.multifile")
	}

	var parts []string
	parts = append(parts, titleStyle.Width(innerW).Align(align).Render(l.Title))
	parts = append(parts, ps.renderMetaGrid(innerW, [][2]string{
		{T("path.label.status"), statusBadgeStyle.Render(statusText)},
		{T("path.label.type"), metaValueStyle.Render(mode)},
		{T("path.label.section"), e.section.Title},
		{T("path.label.node"), pathNodeIndex(l.ID)},
	}, labelStyle, lipgloss.NewStyle()))

	// Score row for lessons with recorded progress.
	if ss := ps.progress[l.ID]; ss > 0 {
		scoreLine := labelStyle.Render(T("path.label.score")+": ") + starsStyle(ss, l.MaxStars())
		parts = append(parts, scoreLine)
	}
	if l.Difficulty != "" {
		parts = append(parts, labelStyle.Render(T("path.label.difficulty")+": ")+diffStyle.Render(l.Difficulty))
	}
	if len(l.Tags) > 0 {
		tags := make([]string, len(l.Tags))
		for i, t := range l.Tags {
			tags[i] = tagStyle.Render(t)
		}
		parts = append(parts, labelStyle.Render(T("path.label.tags")+":"), strings.Join(tags, " "))
	}
	if len(l.Companies) > 0 {
		comps := make([]string, len(l.Companies))
		for i, c := range l.Companies {
			comps[i] = tagStyle.Render(c)
		}
		parts = append(parts, labelStyle.Render(T("path.label.companies")+":"), strings.Join(comps, " "))
	}

	var statBits []string
	if len(l.Tests) > 0 {
		statBits = append(statBits, T("path.stat.tests", len(l.Tests)))
	}
	if len(l.Files) > 0 {
		statBits = append(statBits, T("path.stat.files", len(l.Files)))
	}
	if len(l.Hints) > 0 {
		statBits = append(statBits, T("path.stat.hints", len(l.Hints)))
	}
	if len(l.Trivia) > 0 {
		statBits = append(statBits, T("path.stat.trivia", len(l.Trivia)))
	}
	if len(l.References) > 0 {
		statBits = append(statBits, T("path.stat.refs", len(l.References)))
	}
	if len(l.Tutorial) > 0 {
		statBits = append(statBits, T("path.stat.tutorial", len(l.Tutorial)))
	}
	if len(e.section.AvailableTools) > 0 {
		statBits = append(statBits, T("path.stat.tools", strings.Join(e.section.AvailableTools, ", ")))
	}
	if len(statBits) > 0 || pathLessonSummary(l.Content) != "" {
		parts = append(parts, sepLine)
	}
	if len(statBits) > 0 {
		parts = append(parts, mutedStyle.Width(innerW).Align(align).Render(strings.Join(statBits, "  ·  ")))
	}

	summary := pathLessonSummary(l.Content)
	if summary != "" {
		parts = append(parts, "", labelStyle.Render(T("path.label.summary")+":"), bodyStyle.Width(innerW).Align(align).Render(summary))
	}
	if len(l.References) > 0 {
		parts = append(parts, "", labelStyle.Render(T("path.label.first_ref")+":"), mutedStyle.Width(innerW).Align(align).Render(pathCompactLine(l.References[0], 120)))
	}
	if len(parts) == 0 {
		parts = append(parts, bodyStyle.Render(T("path.label.no_lesson")))
	}

	content := strings.Join(parts, "\n")
	mascotH := mascotPanelHeight
	if h < mascotH+8 {
		mascotH = 0
	}
	infoH := h
	if mascotH > 0 {
		infoH = h - mascotH - 1
	}

	box := lipgloss.NewStyle().
		Width(w).
		Height(infoH).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1).
		Render(content)
	if mascotH == 0 {
		return box
	}
	mascot := renderMascotPanel(w, "Mochi", ps.mascotLine(e), ps.mascotMood(e), ps.frame, ps.loc.IsRTL())
	return lipgloss.JoinVertical(lipgloss.Left, box, mascot)
}

func (ps *PathScreen) renderMetaGrid(width int, rows [][2]string, labelStyle, valueStyle lipgloss.Style) string {
	if len(rows) == 0 {
		return ""
	}
	colW := width / 2
	if colW < 12 {
		colW = width
	}
	renderCell := func(label, value string) string {
		cell := labelStyle.Render(label+": ") + valueStyle.Render(value)
		return lipgloss.NewStyle().Width(colW).Render(cell)
	}
	var lines []string
	for i := 0; i < len(rows); i += 2 {
		left := renderCell(rows[i][0], rows[i][1])
		right := ""
		if i+1 < len(rows) {
			right = renderCell(rows[i+1][0], rows[i+1][1])
		}
		lines = append(lines, left+right)
	}
	return strings.Join(lines, "\n")
}

// renderNode returns a 3-line string for one lesson node.
func (ps *PathScreen) renderNode(e pathEntry, selected bool) string {
	innerW := ps.nodeInnerWidth()
	boxW := ps.nodeBoxWidth()

	// Style configuration per state.
	var (
		borderColor lipgloss.Color
		markerColor lipgloss.Color
		textColor   lipgloss.Color
		marker      string
		topL, topR  string
		botL, botR  string
		hBar, vBar  string
	)

	switch e.status {
	case pathCompleted:
		borderColor = lipgloss.Color(ColorSuccess)
		markerColor = lipgloss.Color(ColorSuccess)
		textColor = lipgloss.Color(ColorBody)
		marker = "✓"
		topL, topR, botL, botR = "╔", "╗", "╚", "╝"
		hBar, vBar = "═", "║"
	case pathAvailable:
		if selected {
			borderColor = lipgloss.Color(ColorAccent)
			markerColor = lipgloss.Color(ColorAccent)
			textColor = lipgloss.Color("255")
			marker = "▸"
		} else {
			borderColor = lipgloss.Color(ColorBody)
			markerColor = lipgloss.Color(ColorBody)
			textColor = lipgloss.Color(ColorBody)
			marker = " "
		}
		topL, topR, botL, botR = "╔", "╗", "╚", "╝"
		hBar, vBar = "═", "║"
	case pathLocked:
		borderColor = lipgloss.Color(ColorFaded)
		markerColor = lipgloss.Color(ColorFaded)
		textColor = lipgloss.Color(ColorFaded)
		marker = "·"
		topL, topR, botL, botR = "┌", "┐", "└", "┘"
		hBar, vBar = "─", "│"
	}

	borderS := lipgloss.NewStyle().Foreground(borderColor)
	markerS := lipgloss.NewStyle().Foreground(markerColor).Bold(selected)
	textS := lipgloss.NewStyle().Foreground(textColor)

	idx := pathNodeIndex(e.lesson.ID)
	rtl := ps.loc.IsRTL()

	// Star decoration for completed nodes (5 vis chars: " ★★★ ").
	// Placed on the trailing side (right in LTR, left in RTL).
	var starsDecor string
	const starsDecorW = 5
	if e.status == pathCompleted {
		ss := ps.progress[e.lesson.ID]
		starsDecor = " " + starsStyle(ss, e.lesson.MaxStars()) + " "
	}

	// Content layout: " {marker}  {idx}  " (8 vis chars) + title + decoration.
	// marker and idx are single-width; use constant prefix width.
	const prefixW = 8
	suffixW := 2
	if starsDecor != "" {
		suffixW = starsDecorW
	}
	titleW := innerW - prefixW - suffixW
	if titleW < 4 {
		titleW = 4
	}
	title := truncRunes(e.lesson.Title, titleW)
	titlePadded := padVis(title, titleW)

	markerPart := markerS.Render(" " + marker + " ")
	idxPart := textS.Render(" " + idx + "  ")

	var midContent string
	if rtl {
		if starsDecor != "" {
			// Stars on left in RTL (trailing side for RTL readers).
			midContent = starsDecor + textS.Render(titlePadded) + idxPart + markerPart
		} else {
			midContent = textS.Render(titlePadded+"  ") + idxPart + markerPart
		}
	} else {
		if starsDecor != "" {
			midContent = markerPart + idxPart + textS.Render(titlePadded) + starsDecor
		} else {
			midContent = markerPart + idxPart + textS.Render(titlePadded+"  ")
		}
	}

	hLine := strings.Repeat(hBar, innerW)
	topBorder := borderS.Render(topL + hLine + topR)
	botBorder := borderS.Render(botL + hLine + botR)
	midLine := borderS.Render(vBar) + midContent + borderS.Render(vBar)

	// Center the (cursor + box) unit horizontally within the path pane.
	cursorW := 2
	leftPad := (ps.pathPaneWidth() - boxW - cursorW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	pad := strings.Repeat(" ", leftPad)

	cursorAccent := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	var sb strings.Builder
	if rtl {
		// In RTL the cursor glyph appears to the RIGHT of the node box.
		var cursorGlyph string
		if selected {
			cursorGlyph = cursorAccent.Render(" <")
		} else {
			cursorGlyph = "  "
		}
		sb.WriteString(pad + topBorder + "  " + "\n")
		sb.WriteString(pad + midLine + cursorGlyph + "\n")
		sb.WriteString(pad + botBorder + "  " + "\n")
	} else {
		var cursorGlyph string
		if selected {
			cursorGlyph = cursorAccent.Render("▶ ")
		} else {
			cursorGlyph = "  "
		}
		sb.WriteString(pad + "  " + topBorder + "\n")
		sb.WriteString(pad + cursorGlyph + midLine + "\n")
		sb.WriteString(pad + "  " + botBorder + "\n")
	}
	return sb.String()
}

// renderConnector returns 2 lines connecting adjacent nodes.
func (ps *PathScreen) renderConnector(above pathStatus) string {
	innerW := ps.nodeInnerWidth()
	cursorW := 2
	leftPad := (ps.pathPaneWidth() - ps.nodeBoxWidth() - cursorW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	// The connector sits under the center of the node's inner area.
	// In LTR the cursor glyph sits to the left of the box, pushing the box
	// right by cursorW. In RTL the cursor is on the right, so the box starts
	// directly at leftPad.
	connOffset := leftPad + 1 + innerW/2
	if !ps.loc.IsRTL() {
		connOffset += cursorW
	}

	var color lipgloss.Color
	var pipeChar string
	if above == pathCompleted {
		color = lipgloss.Color(ColorSuccess)
		pipeChar = "║"
	} else {
		color = lipgloss.Color(ColorFaded)
		pipeChar = "╎"
	}
	pipe := lipgloss.NewStyle().Foreground(color).Render(pipeChar)
	pad := strings.Repeat(" ", connOffset)

	return pad + pipe + "\n" + pad + pipe + "\n"
}

// renderSectionDivider renders a titled horizontal rule between sections.
func (ps *PathScreen) renderSectionDivider(title string) string {
	titleS := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSection)).
		Bold(true)
	lineS := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder))

	label := titleS.Render("  " + strings.ToUpper(title) + "  ")
	labelW := lipgloss.Width(label)
	sideW := (ps.pathPaneWidth() - labelW) / 2
	if sideW < 1 {
		sideW = 1
	}
	line := lineS.Render(strings.Repeat("─", sideW))
	return "\n" + line + label + line + "\n\n"
}

// scrollToCursor adjusts the viewport so the selected node is centered.
func (ps *PathScreen) scrollToCursor() {
	// Compute the Y line of the top of the cursor's node in the content string.
	// Leading newline: 1
	// Each node: 3 lines. Each connector: 2 lines.
	y := 1
	for i := 0; i < ps.cursor && i < len(ps.entries); i++ {
		y += 3
		if i < len(ps.entries)-1 {
			y += 2
		}
	}
	// Aim to put the middle row of the node in the center of the viewport.
	target := y + 1 - ps.viewport.Height/2
	if target < 0 {
		target = 0
	}
	ps.viewport.YOffset = target
}

func (ps *PathScreen) hasMultipleSections() bool {
	count := 0
	for _, s := range ps.course.Sections {
		if s.Type != "challenges" && s.Type != "quizzes" {
			count++
		}
	}
	return count > 1
}

func pathLessonSummary(content string) string {
	blocks := strings.Split(content, "\n\n")
	var fallback []string
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "#") {
			continue
		}
		single := strings.Join(strings.Fields(block), " ")
		if single == "" {
			continue
		}
		if !pathLooksLikeMetaBlock(single) {
			return pathCompactLine(single, 220)
		}
		clean := strings.TrimLeft(single, "-*>0123456789. ")
		clean = strings.TrimSpace(clean)
		if clean != "" {
			fallback = append(fallback, clean)
		}
	}
	if len(fallback) == 0 {
		return ""
	}
	return pathCompactLine(strings.Join(fallback, " "), 220)
}

func pathLooksLikeMetaBlock(s string) bool {
	if strings.Contains(s, "|") {
		return true
	}
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "> ") {
		return true
	}
	if strings.HasPrefix(s, "Lesson ") && strings.Contains(s, "Topic") {
		return true
	}
	return false
}

func pathCompactLine(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if limit <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return s
}

func (ps *PathScreen) mascotMood(e pathEntry) MascotMood {
	switch e.status {
	case pathCompleted:
		return MoodHappy
	case pathLocked:
		return MoodThinking
	default:
		if e.lesson.IsReadOnly() {
			return MoodCurious
		}
		return MoodFocused
	}
}

func (ps *PathScreen) mascotLine(e pathEntry) string {
	T := ps.loc.T
	switch e.status {
	case pathCompleted:
		return T("path.mochi.completed")
	case pathLocked:
		return T("path.mochi.locked")
	default:
		if e.lesson.IsReadOnly() {
			return T("path.mochi.readonly")
		}
		if len(e.lesson.Tags) > 0 {
			return T("path.mochi.focus_tags", strings.Join(e.lesson.Tags, ", "))
		}
		return T("path.mochi.focus")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// pathProgressBar renders a compact filled/empty progress bar.
func pathProgressBar(done, total, width int) string {
	if total == 0 || width == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded)).Render(strings.Repeat("░", width))
	}
	filled := done * width / total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	filledS := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(strings.Repeat("█", filled))
	emptyS := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded)).Render(strings.Repeat("░", width-filled))
	_ = bar
	return filledS + emptyS
}

// pathNodeIndex extracts the numeric prefix from a lesson ID.
// "01-hello-stderr" returns "01"; "hello" returns "00".
func pathNodeIndex(id string) string {
	if i := strings.Index(id, "-"); i > 0 {
		return id[:i]
	}
	return "00"
}
