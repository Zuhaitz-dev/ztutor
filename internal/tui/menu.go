package tui

import (
	"fmt"
	"strings"
	"time"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const ansiReset = "\033[0m"

// ── Navigation message types ──────────────────────────────────────────────────

type NavigateToMenu struct{}

// NavigateBackToCourse returns to the currently active course view rather than
// the top-level course list. App resolves this using the active course ID.
type NavigateBackToCourse struct{}

type NavigateToConnectChoice struct{}

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
	kind       string // "lesson", "quiz", or "challenge"
	lesson     lesson.Lesson
	quiz       lesson.Quiz
	challenge  lesson.Challenge
}

// ── MenuScreen ────────────────────────────────────────────────────────────────

type MenuScreen struct {
	courses       []lesson.Course
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

// ── Constructor ───────────────────────────────────────────────────────────────

func NewMenuScreen(courses []lesson.Course, progress map[string]int, notifications []string, username string, streak int, isAdmin bool, checkCourseIntro func(string) bool, loc *i18n.Locale, width, height int) *MenuScreen {
	si := textinput.New()
	si.Placeholder = "search..."
	si.Prompt = ""
	si.CharLimit = 80

	m := &MenuScreen{
		courses:          courses,
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
			items = append(items, displayItem{
				title:      ch.Title,
				difficulty: ch.Difficulty,
				kind:       "challenge",
				challenge:  ch,
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
			items = append(items, displayItem{
				title:      quiz.Title,
				difficulty: quiz.Difficulty,
				kind:       "quiz",
				quiz:       quiz,
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
			items = append(items, displayItem{
				title:      l.Title,
				difficulty: l.Difficulty,
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
