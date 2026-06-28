package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/license"
	"ztutor/internal/sandbox"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── C Matrix Rain ─────────────────────────────────────────────────────────────

var cRainChars = []rune("0123456789abcdef*&{}();=+-<>!|^~#\\_?%@")

// rainGradient: index 0 = head (bright white), index 1+ = fading green trail.
var rainGradient = []lipgloss.Color{
	"231", // head — bright white
	"46",  // trail 1
	"40",
	"34",
	"28",
	"22",
	"22",
	"22",
	"238", // very dim tail
	"238",
	"238",
	"238",
	"238",
	"238",
	"238",
	"238",
}

// rainAnsiPfx pre-computes ANSI xterm-256 sequences for each gradient level.
var rainAnsiPfx []string

const ansiReset = "\033[0m"

// forceLTRText wraps a fragment in LTR marks so BiDi-aware terminals keep its
// internal ordering stable when embedded inside RTL text.
func forceLTRText(s string) string {
	return ltrMarker + s + ltrMarker
}

func init() {
	rainAnsiPfx = make([]string, len(rainGradient))
	for i, color := range rainGradient {
		rainAnsiPfx[i] = fmt.Sprintf("\033[38;5;%sm", string(color))
	}
}

type rainTickMsg struct{}

// rainTickCmd runs the rain at ~12 fps, independent of the mascot clock.
func rainTickCmd() tea.Cmd {
	return tea.Tick(83*time.Millisecond, func(time.Time) tea.Msg {
		return rainTickMsg{}
	})
}

type rainCol struct {
	head  float64
	speed float64
	trail int
	chars []rune
}

func newRainCol(h int) rainCol {
	size := h + 30
	chars := make([]rune, size)
	for i := range chars {
		chars[i] = cRainChars[rand.Intn(len(cRainChars))]
	}
	return rainCol{
		head:  -float64(rand.Intn(h + 1)),
		speed: 0.5 + rand.Float64()*1.0,
		trail: 5 + rand.Intn(11),
		chars: chars,
	}
}

func (c *rainCol) tick(h int) {
	c.head += c.speed
	hi := int(c.head)
	if rand.Intn(3) == 0 && hi >= 0 && hi < len(c.chars) {
		c.chars[hi] = cRainChars[rand.Intn(len(cRainChars))]
	}
	if c.head-float64(c.trail) > float64(h) {
		size := h + 30
		chars := make([]rune, size)
		for i := range chars {
			chars[i] = cRainChars[rand.Intn(len(cRainChars))]
		}
		c.head = -float64(rand.Intn(h/2 + 1))
		c.speed = 0.5 + rand.Float64()*1.0
		c.trail = 5 + rand.Intn(11)
		c.chars = chars
	}
}

func (c *rainCol) renderAt(row int) string {
	headRow := int(c.head)
	dist := headRow - row
	if dist < 0 || dist > c.trail {
		return " "
	}
	var ch rune
	if row >= 0 && row < len(c.chars) {
		ch = c.chars[row]
	} else {
		ch = ' '
	}
	idx := dist
	if idx >= len(rainAnsiPfx) {
		idx = len(rainAnsiPfx) - 1
	}
	return rainAnsiPfx[idx] + forceLTRText(string(ch)) + ansiReset
}

// ── Navigation message types ──────────────────────────────────────────────────

type NavigateToMenu struct{}

// NavigateBackToCourse returns to the currently active course view rather than
// the top-level course list. App resolves this using the active course ID.
type NavigateBackToCourse struct{}

type NavigateToConnectChoice struct{}

type NavigateToLicenseEntry struct{}
type NavigateToLicenseSummary struct{}

type licenseEntryDoneMsg struct{ licState *license.State }

type NavigateToLessonMsg struct {
	Lesson lesson.Lesson
}

type NavigateToExerciseMsg struct {
	Lesson lesson.Lesson
}

type NavigateToQuizMsg struct {
	Quiz lesson.Quiz
}

type lessonCompletedMsg struct {
	lessonID string
	stars    int
	goNext   bool
}

// showCourseIntroMsg is fired when a user enters a course for the first time.
// App intercepts it and shows a course-specific Mochi intro.
type showCourseIntroMsg struct{ course lesson.Course }

// ── Display item for Level 2 lists ────────────────────────────────────────────

type displayItem struct {
	title      string
	difficulty string
	isPremium  bool
	isLocked   bool
	kind       string // "lesson", "quiz", or "challenge"
	lesson     lesson.Lesson
	quiz       lesson.Quiz
	challenge  lesson.Challenge
}

// ── MenuScreen ────────────────────────────────────────────────────────────────

type MenuScreen struct {
	courses       []lesson.Course
	enrolled      map[string]bool
	canAccess     func(string) bool
	progress      map[string]int
	notifications []string
	username      string
	streak        int
	isAdmin       bool

	// Two-level navigation.
	viewLevel      string         // "courses" or "lessons"
	selectedCourse *lesson.Course // set when entering level 2
	sectionIndex   int            // active section tab in level 2

	// Level 1: course list cursor / offset.
	courseCursor int
	courseOffset int

	// Level 2: lesson/challenge list cursor / offset.
	lessonCursor int
	lessonOffset int
	displayItems []displayItem

	searchActive bool
	searchQuery  string
	searchInput  textinput.Model

	konamiBuffer []string
	konamiActive bool
	compatLine   string

	sized
	mascotFrame  int
	mascotHidden bool

	// checkCourseIntro returns true if the given courseID needs a first-run intro.
	checkCourseIntro func(string) bool

	loc *i18n.Locale
}

var konamiSeq = []string{"up", "up", "down", "down", "left", "right", "left", "right"}

var notifStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(ColorGold)).
	Bold(true)

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

// ── Constructor ───────────────────────────────────────────────────────────────

func NewMenuScreen(courses []lesson.Course, enrolled []string, canAccess func(string) bool, progress map[string]int, notifications []string, username string, streak int, isAdmin bool, checkCourseIntro func(string) bool, loc *i18n.Locale, width, height int) *MenuScreen {
	si := textinput.New()
	si.Placeholder = "search..."
	si.Prompt = ""
	si.CharLimit = 80

	enrolledSet := make(map[string]bool, len(enrolled))
	for _, id := range enrolled {
		enrolledSet[id] = true
	}

	m := &MenuScreen{
		courses:          courses,
		enrolled:         enrolledSet,
		canAccess:        canAccess,
		progress:         progress,
		notifications:    notifications,
		username:         username,
		streak:           streak,
		isAdmin:          isAdmin,
		searchInput:      si,
		sized:            sized{Width: width, Height: height},
		viewLevel:        "courses",
		checkCourseIntro: checkCourseIntro,
		loc:              loc,
	}
	if m.loc == nil {
		m.loc = i18n.New("en")
	}
	return m
}

// ── Builders ──────────────────────────────────────────────────────────────────

func (m *MenuScreen) matchingCourses(query string) []lesson.Course {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return m.courses
	}
	var out []lesson.Course
	for _, c := range m.courses {
		if strings.Contains(strings.ToLower(c.Title), q) {
			out = append(out, c)
		}
	}
	return out
}

func (m *MenuScreen) buildDisplayItems(query string) []displayItem {
	q := strings.ToLower(strings.TrimSpace(query))
	if m.selectedCourse == nil {
		return nil
	}
	if m.sectionIndex < 0 || m.sectionIndex >= len(m.selectedCourse.Sections) {
		return nil
	}
	sec := m.selectedCourse.Sections[m.sectionIndex]
	courseFree := !m.selectedCourse.EnrollmentRequired || m.canAccess(m.selectedCourse.ID)

	var items []displayItem
	switch sec.Type {
	case "challenges":
		for _, ch := range sec.Challenges {
			if q != "" {
				hay := strings.ToLower(ch.Title + " " + ch.Difficulty + " " + strings.Join(ch.Tags, " "))
				if !strings.Contains(hay, q) {
					continue
				}
			}
			locked := !courseFree
			items = append(items, displayItem{
				title:      ch.Title,
				difficulty: ch.Difficulty,
				kind:       "challenge",
				challenge:  ch,
				isLocked:   locked,
			})
		}
	case "quizzes":
		for _, quiz := range sec.Quizzes {
			if q != "" {
				hay := strings.ToLower(quiz.Title + " " + quiz.Description + " " + quiz.Difficulty + " " + strings.Join(quiz.Tags, " "))
				if !strings.Contains(hay, q) {
					continue
				}
			}
			locked := !courseFree
			items = append(items, displayItem{
				title:      quiz.Title,
				difficulty: quiz.Difficulty,
				kind:       "quiz",
				quiz:       quiz,
				isLocked:   locked,
			})
		}
	default:
		for _, l := range sec.Lessons {
			if q != "" {
				hay := strings.ToLower(l.Title + " " + l.Difficulty + " " + strings.Join(l.Tags, " "))
				if !strings.Contains(hay, q) {
					continue
				}
			}
			isPrem := l.IsPremium
			locked := isPrem && !courseFree
			items = append(items, displayItem{
				title:      l.Title,
				difficulty: l.Difficulty,
				isPremium:  isPrem,
				isLocked:   locked,
				kind:       "lesson",
				lesson:     l,
			})
		}
	}
	return items
}

// ── suggestedLesson ───────────────────────────────────────────────────────────

func (m *MenuScreen) suggestedLesson() *lesson.Lesson {
	var best *lesson.Lesson
	bestStars := 4
	for _, c := range m.courses {
		for _, sec := range c.Sections {
			if sec.Type == "challenges" {
				continue
			}
			for i := range sec.Lessons {
				l := &sec.Lessons[i]
				s := m.progress[l.ID]
				if s > 0 && s < l.MaxStars() && s < bestStars {
					bestStars = s
					best = l
				}
			}
		}
	}
	return best
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *MenuScreen) Init() tea.Cmd { return nil }

func (m *MenuScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		if m.searchActive {
			return m.updateSearchMode(msg)
		}
		if cmd, ok := m.handleGlobalKey(msg); ok {
			return m, cmd
		}
		switch m.viewLevel {
		case "courses":
			return m.updateCourseKeys(msg)
		case "lessons":
			return m.updateLessonKeys(msg)
		}
	}
	return m, nil
}

// handleGlobalKey processes keys common to both levels. Returns (cmd, true) if handled.
func (m *MenuScreen) handleGlobalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case KeyBack, KeyQuit:
		return tea.Quit, true
	case KeyAchieve:
		return backCmd(NavigateToAchievements{}), true
	case KeyRanks:
		return backCmd(NavigateToLeaderboard{}), true
	case KeySettings:
		return backCmd(NavigateToSettings{}), true
	case KeyCredits:
		return backCmd(NavigateToCredits{}), true
	case KeyMochi:
		m.mascotHidden = !m.mascotHidden
		return nil, true
	case KeyLanguage:
		next := m.loc.Next()
		return backCmd(changeLangMsg{lang: next.Lang()}), true
	case KeyAdmin:
		if m.isAdmin {
			return backCmd(launchAdminMsg{}), true
		}
	case KeyReview:
		if s := m.suggestedLesson(); s != nil {
			return backCmd(NavigateToLessonMsg{Lesson: *s}), true
		}
	}
	return nil, false
}

func (m *MenuScreen) updateCourseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeySearch:
		m.searchActive = true
		m.searchInput.Placeholder = "search courses..."
		m.searchInput.Focus()
		return m, textinput.Blink
	case KeyBackAlt:
		// Already at the top level; esc is a no-op here.
		return m, nil
	case KeyUp:
		m.moveCourseCursor(-1)
		if m.trackKonami(KeyUp) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeyUpVim:
		m.moveCourseCursor(-1)
	case KeyDown:
		m.moveCourseCursor(1)
		if m.trackKonami(KeyDown) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeyDownVim:
		m.moveCourseCursor(1)
	case KeyLeft:
		if m.trackKonami(KeyLeft) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeyRight:
		if m.trackKonami(KeyRight) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeySelect:
		courses := m.matchingCourses(m.searchQuery)
		if m.courseCursor < len(courses) {
			return m.enterCourse(courses[m.courseCursor])
		}
	}
	return m, nil
}

func (m *MenuScreen) updateLessonKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeySearch:
		m.searchActive = true
		m.searchInput.Placeholder = "search lessons..."
		m.searchInput.Focus()
		return m, textinput.Blink
	case KeyBackB, KeyBackAlt:
		m.viewLevel = "courses"
		m.selectedCourse = nil
		m.sectionIndex = 0
		m.lessonCursor = 0
		m.lessonOffset = 0
		m.displayItems = nil
		return m, nil
	case KeySection:
		m.switchSection(1)
	case KeySectionPrev:
		m.switchSection(-1)
	case KeyUp:
		m.moveLessonCursor(-1)
		if m.trackKonami(KeyUp) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeyUpVim:
		m.moveLessonCursor(-1)
	case KeyDown:
		m.moveLessonCursor(1)
		if m.trackKonami(KeyDown) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeyDownVim:
		m.moveLessonCursor(1)
	case KeyLeft:
		if m.trackKonami(KeyLeft) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeyRight:
		if m.trackKonami(KeyRight) {
			return m, backCmd(achievementEventMsg{events: []string{"konami"}})
		}
	case KeySelect:
		if m.lessonCursor < len(m.displayItems) {
			return m.enterItem(m.displayItems[m.lessonCursor])
		}
	}
	return m, nil
}

func (m *MenuScreen) switchSection(dir int) {
	if m.selectedCourse == nil || len(m.selectedCourse.Sections) == 0 {
		return
	}
	m.sectionIndex = (m.sectionIndex + dir + len(m.selectedCourse.Sections)) % len(m.selectedCourse.Sections)
	m.lessonCursor = 0
	m.lessonOffset = 0
	m.displayItems = m.buildDisplayItems(m.searchQuery)
	if m.sectionIndex < len(m.selectedCourse.Sections) {
		sec := m.selectedCourse.Sections[m.sectionIndex]
		key := "mochi.section_" + sec.Type
		if val := m.loc.T(key); val != key {
			m.compatLine = val
		}
	}
}

func (m *MenuScreen) updateSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeyBackAlt:
		m.clearSearch()
		return m, nil
	case KeySelect:
		// Save the selection from the filtered view BEFORE clearSearch rebuilds
		// the list with an empty query (which would shift cursor to the wrong item).
		if m.viewLevel == "courses" {
			courses := m.matchingCourses(m.searchQuery)
			if m.courseCursor < len(courses) {
				selected := courses[m.courseCursor]
				m.clearSearch()
				return m.enterCourse(selected)
			}
		} else if m.lessonCursor < len(m.displayItems) {
			selected := m.displayItems[m.lessonCursor]
			m.clearSearch()
			return m.enterItem(selected)
		}
		m.clearSearch()
		return m, nil
	case KeyUp, KeyUpVim:
		m.searchMoveCursor(-1)
	case KeyDown, KeyDownVim:
		m.searchMoveCursor(1)
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = m.searchInput.Value()
		m.applySearchFilter()
		return m, cmd
	}
	return m, nil
}

func (m *MenuScreen) searchMoveCursor(dir int) {
	if m.viewLevel == "courses" {
		m.moveCourseCursor(dir)
	} else {
		m.moveLessonCursor(dir)
	}
}

func (m *MenuScreen) applySearchFilter() {
	if m.viewLevel == "courses" {
		courses := m.matchingCourses(m.searchQuery)
		if m.courseCursor >= len(courses) {
			m.courseCursor = max(0, len(courses)-1)
		}
		if m.courseOffset > m.courseCursor {
			m.courseOffset = m.courseCursor
		}
	} else {
		m.displayItems = m.buildDisplayItems(m.searchQuery)
		if m.lessonCursor >= len(m.displayItems) {
			m.lessonCursor = max(0, len(m.displayItems)-1)
			m.lessonOffset = max(0, m.lessonCursor-m.panelHeight()+1)
		}
	}
}

func (m *MenuScreen) enterCourse(c lesson.Course) (tea.Model, tea.Cmd) {
	courseFree := !c.EnrollmentRequired || m.canAccess(c.ID)
	if c.EnrollmentRequired && !courseFree {
		m.compatLine = m.loc.T("menu.course_no_enrollment")
		return m, nil
	}
	if len(c.Sections) == 0 {
		m.compatLine = m.loc.T("menu.course_no_sections")
		return m, nil
	}
	// First visit: fire a course-specific intro only when the course opted in.
	if len(c.CourseIntro) > 0 && m.checkCourseIntro != nil && m.checkCourseIntro(c.ID) {
		return m, backCmd(showCourseIntroMsg{course: c})
	}
	if c.Layout == lesson.CourseLayoutPath {
		return m, backCmd(enterCoursePathMsg{course: c})
	}
	m.enterCourseDirectly(c)
	return m, nil
}

// enterCourseDirectly transitions the menu to lesson-list view for the given
// course without triggering the first-run intro check.
func (m *MenuScreen) enterCourseDirectly(c lesson.Course) {
	m.viewLevel = "lessons"
	sel := c
	m.selectedCourse = &sel
	m.sectionIndex = 0
	m.lessonCursor = 0
	m.lessonOffset = 0
	m.displayItems = m.buildDisplayItems(m.searchQuery)
}

// restoreCourse navigates back to lesson-list view for the course with the
// given ID after a language reload. It finds the course by ID in the
// current filtered list so translated course data is used.
func (m *MenuScreen) restoreCourse(id string) {
	courses := m.matchingCourses("")
	for i, c := range courses {
		if c.ID == id {
			m.courseCursor = i
			if m.courseOffset > i {
				m.courseOffset = i
			}
			m.enterCourseDirectly(c)
			return
		}
	}
}

func (m *MenuScreen) enterItem(item displayItem) (tea.Model, tea.Cmd) {
	if item.isLocked {
		m.compatLine = m.loc.T("menu.item_locked")
		return m, nil
	}
	if item.kind == "challenge" {
		lang := sandbox.GetLanguage(m.selectedCourse.Language)
		if lang == nil {
			lang = sandbox.GetLanguage("c")
		}
		return m, backCmd(NavigateToChallengeMsg{Challenge: item.challenge, CourseID: m.selectedCourse.ID, Lang: lang})
	}
	if item.kind == "quiz" {
		return m, backCmd(NavigateToQuizMsg{Quiz: item.quiz})
	}
	return m, backCmd(NavigateToLessonMsg{Lesson: item.lesson})
}

// ── Cursor movement ───────────────────────────────────────────────────────────

func (m *MenuScreen) moveCourseCursor(dir int) {
	courses := m.matchingCourses(m.searchQuery)
	n := len(courses)
	if n == 0 {
		return
	}
	m.courseCursor = (m.courseCursor + dir + n) % n
	ph := m.panelHeight()
	if m.courseCursor < m.courseOffset {
		m.courseOffset = m.courseCursor
	} else if m.courseCursor >= m.courseOffset+ph {
		m.courseOffset = m.courseCursor - ph + 1
	}
}

func (m *MenuScreen) moveLessonCursor(dir int) {
	n := len(m.displayItems)
	if n == 0 {
		return
	}
	m.lessonCursor = (m.lessonCursor + dir + n) % n
	ph := m.panelHeight()
	if m.lessonCursor < m.lessonOffset {
		m.lessonOffset = m.lessonCursor
	} else if m.lessonCursor >= m.lessonOffset+ph {
		m.lessonOffset = m.lessonCursor - ph + 1
	}
}

// ── Search ────────────────────────────────────────────────────────────────────

func (m *MenuScreen) clearSearch() {
	m.searchActive = false
	m.searchQuery = ""
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	if m.viewLevel == "courses" {
		courses := m.matchingCourses("")
		if m.courseCursor >= len(courses) {
			m.courseCursor = max(0, len(courses)-1)
		}
	} else {
		m.displayItems = m.buildDisplayItems("")
		if m.lessonCursor >= len(m.displayItems) {
			m.lessonCursor = max(0, len(m.displayItems)-1)
		}
	}
}

// ── Konami ────────────────────────────────────────────────────────────────────

func (m *MenuScreen) trackKonami(key string) bool {
	m.konamiBuffer = append(m.konamiBuffer, key)
	if len(m.konamiBuffer) > len(konamiSeq) {
		m.konamiBuffer = m.konamiBuffer[len(m.konamiBuffer)-len(konamiSeq):]
	}
	if len(m.konamiBuffer) < len(konamiSeq) {
		return false
	}
	for i, k := range konamiSeq {
		if m.konamiBuffer[i] != k {
			return false
		}
	}
	if !m.konamiActive {
		m.konamiActive = true
		m.konamiBuffer = nil
		m.notifications = append(m.notifications, "CHEAT CODE ACCEPTED — Mochi has been watching the whole time.")
		return true
	}
	return false
}

// ── Layout helpers ────────────────────────────────────────────────────────────

func (m *MenuScreen) panelHeight() int {
	mascotH := mascotPanelHeight
	if m.mascotHidden {
		mascotH = 0
	}
	overhead := 11 + mascotH
	if len(m.notifications) > 0 {
		overhead += len(m.notifications) + 1
	}
	if m.suggestedLesson() != nil {
		overhead++
	}
	if m.viewLevel == "lessons" {
		overhead++ // section tab bar
	}
	h := m.Height - overhead
	if h < 3 {
		h = 3
	}
	return h
}

func (m *MenuScreen) SetMascotFrame(frame int) {
	m.mascotFrame = frame
}

func (m *MenuScreen) mascotMood() MascotMood {
	if m.konamiActive {
		return MoodHappy
	}
	if m.searchActive {
		return MoodCurious
	}
	if len(m.notifications) > 0 {
		return MoodHappy
	}
	if m.viewLevel == "lessons" {
		return MoodFocused
	}
	return MoodIdle
}

func (m *MenuScreen) mascotLine() string {
	if m.compatLine != "" {
		line := m.compatLine
		m.compatLine = ""
		return line
	}
	if m.konamiActive {
		return m.loc.T("mochi.konami")
	}
	if m.searchActive {
		if strings.TrimSpace(m.searchQuery) == "" {
			if m.viewLevel == "courses" {
				return m.loc.T("mochi.search_empty_courses")
			}
			return m.loc.T("mochi.search_empty_lessons")
		}
		if m.viewLevel == "courses" {
			return m.loc.T("mochi.search_found_courses", m.searchQuery)
		}
		return m.loc.T("mochi.search_found_lessons", len(m.displayItems), m.searchQuery)
	}
	if len(m.notifications) > 0 {
		return m.loc.T("mochi.notifications")
	}
	if h := time.Now().Hour(); h < 4 {
		return m.loc.T("mochi.midnight")
	}
	if m.viewLevel == "lessons" {
		if m.lessonCursor < len(m.displayItems) {
			di := m.displayItems[m.lessonCursor]
			if di.kind == "lesson" {
				if stars := m.progress[di.lesson.ID]; stars > 0 {
					return m.loc.T("mochi.lesson_stars", stars, di.title)
				}
			}
			return m.loc.T("mochi.lesson_ready", di.title)
		}
		return m.loc.T("mochi.lesson_pick")
	}
	courses := m.matchingCourses(m.searchQuery)
	if m.courseCursor < len(courses) {
		c := courses[m.courseCursor]
		done, total := courseProgressCounts(&c, m.progress)
		if done > 0 && total > 0 {
			return m.loc.T("mochi.course_progress", c.Title, done, total)
		}
		l, i, q, ch := sectionCounts(&c)
		var parts []string
		if l > 0 {
			parts = append(parts, m.loc.T("menu.chip_lessons", l))
		}
		if i > 0 {
			parts = append(parts, m.loc.T("menu.chip_interviews", i))
		}
		if q > 0 {
			parts = append(parts, m.loc.T("menu.chip_quizzes", q))
		}
		if ch > 0 {
			parts = append(parts, m.loc.T("menu.chip_challenges", ch))
		}
		return fmt.Sprintf("%s — %s", c.Title, strings.Join(parts, ", "))
	}
	return m.loc.T("mochi.course_pick")
}

// ── Language-aware snippet cycling ───────────────────────────────────────────

var langSnippets = map[string][]string{
	"c": {
		"#include <stdio.h>",
		"int main(void) { ... }",
		"malloc(sizeof(T) * n)",
		"char *p = &buf[0];",
		"gcc -Wall -Wextra",
		"valgrind --leak-check",
		"gdb -q ./prog",
		"assert(ptr != NULL);",
	},
	"python": {
		"def hello():",
		"  print('hello')",
		"for i in range(n):",
		"import sys",
		"with open(f) as fp:",
		"x = [i**2 for i in range(10)]",
		"if __name__ == '__main__':",
		"try: ... except: pass",
	},
	"go": {
		"package main",
		"func main() {",
		`fmt.Println("hello")`,
		"go func() { ... }()",
		"defer f.Close()",
		"if err != nil { ... }",
		"var wg sync.WaitGroup",
		"make(chan struct{})",
	},
}

var genericSnippets = []string{
	"learn by doing",
	"read the error",
	"test your assumptions",
	"break it on purpose",
	"write the test first",
	"simplify and retry",
}

func (m *MenuScreen) currentLang() string {
	if m.viewLevel == "lessons" && m.selectedCourse != nil {
		return m.selectedCourse.Language
	}
	courses := m.matchingCourses(m.searchQuery)
	if m.courseCursor < len(courses) {
		return courses[m.courseCursor].Language
	}
	return ""
}

func (m *MenuScreen) currentSnippet() string {
	lang := m.currentLang()
	snippets, ok := langSnippets[lang]
	if !ok || len(snippets) == 0 {
		snippets = genericSnippets
	}
	return snippets[(m.mascotFrame/6)%len(snippets)]
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
			if rtl {
				b.WriteString(selSt.Render(line))
			} else {
				b.WriteString(selSt.Render(line))
			}
		} else {
			if rtl {
				b.WriteString(itemSt.Render(line))
			} else {
				b.WriteString(itemSt.Render(line))
			}
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

	courseFree := !c.EnrollmentRequired || m.canAccess(c.ID)
	courseLocked := c.EnrollmentRequired && !courseFree

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

	var tags []string
	if c.HasFreemium && !courseLocked {
		tags = append(tags, lipgloss.NewStyle().Foreground(lipgloss.Color("178")).Render(T("menu.tag_freemium")))
	}
	if courseLocked {
		tags = append(tags, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(T("menu.tag_locked")))
	}
	if c.Encrypted {
		if c.TotalLessons == 0 && c.TotalQuizzes == 0 && c.TotalChallenges == 0 {
			tags = append(tags, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(T("menu.tag_missing")))
		} else {
			tags = append(tags, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Render(T("menu.tag_encrypted")))
		}
	}
	tagStr := strings.Join(tags, " ")

	done, total := courseProgressCounts(&c, m.progress)
	progressStr := ""
	if total > 0 && !courseLocked {
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
		"tags":     {variants: []string{tagStr, ""}},
	}
	degradeOrder := []string{"counts", "tags", "progress", "ui", "prog"}
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
			segments["tags"].current(),
		)
	}
	for {
		suffix := joinCourseLineParts(
			rtl,
			segments["prog"].current(),
			segments["ui"].current(),
			segments["counts"].current(),
			segments["progress"].current(),
			segments["tags"].current(),
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
	if di.isLocked {
		title = dim(title)
	}

	// Optional suffix: premium lock or locked indicator.
	suffix := ""
	if di.isLocked {
		suffix = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(m.loc.T("menu.tag_locked"))
	} else if di.isPremium {
		suffix = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render("[P]")
	}

	return kindStyle.Render(kindBadge) + "  " + starStr + "  " + diffBadge + "  " + title + suffix
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
