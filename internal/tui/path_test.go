package tui

import (
	"strings"
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
)

// makePathCourse builds a minimal course with n sequential exercise lessons.
func makePathCourse(n int) lesson.Course {
	lessons := make([]lesson.Lesson, n)
	for i := range lessons {
		lessons[i] = lesson.Lesson{
			ID:       "0" + string(rune('0'+i)) + "-lesson",
			Title:    "Lesson " + string(rune('A'+i)),
			Exercise: "int main(){}",
		}
	}
	return lesson.Course{
		ID:     "test-path",
		Title:  "Test Course",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{
			{ID: "main", Type: "exercises", Lessons: lessons},
		},
	}
}

func pathKey(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func pathUpdate(ps *PathScreen, key string) (*PathScreen, tea.Cmd) {
	m, cmd := ps.Update(pathKey(key))
	return m.(*PathScreen), cmd
}

// TestBuildPathEntries_Status verifies the completed/available/locked logic.
func TestBuildPathEntries_Status(t *testing.T) {
	c := makePathCourse(4)

	// No progress: first entry available, rest locked.
	entries := buildPathEntries(c, map[string]int{})
	if entries[0].status != pathAvailable {
		t.Errorf("entry[0] status = %v, want pathAvailable", entries[0].status)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].status != pathLocked {
			t.Errorf("entry[%d] status = %v, want pathLocked", i, entries[i].status)
		}
	}

	// First two completed: third available, fourth locked.
	progress := map[string]int{
		entries[0].lesson.ID: 1,
		entries[1].lesson.ID: 3,
	}
	entries = buildPathEntries(c, progress)
	if entries[0].status != pathCompleted {
		t.Errorf("entry[0] status = %v, want pathCompleted", entries[0].status)
	}
	if entries[1].status != pathCompleted {
		t.Errorf("entry[1] status = %v, want pathCompleted", entries[1].status)
	}
	if entries[2].status != pathAvailable {
		t.Errorf("entry[2] status = %v, want pathAvailable", entries[2].status)
	}
	if entries[3].status != pathLocked {
		t.Errorf("entry[3] status = %v, want pathLocked", entries[3].status)
	}

	// All completed.
	progress2 := map[string]int{}
	for _, e := range entries {
		progress2[e.lesson.ID] = 1
	}
	entries = buildPathEntries(c, progress2)
	for i, e := range entries {
		if e.status != pathCompleted {
			t.Errorf("all-done: entry[%d] status = %v, want pathCompleted", i, e.status)
		}
	}
}

// TestNewPathScreen_CursorStartsAtFirstIncomplete verifies the initial cursor.
func TestNewPathScreen_CursorStartsAtFirstIncomplete(t *testing.T) {
	c := makePathCourse(5)
	progress := map[string]int{}
	for _, s := range c.Sections {
		for i, l := range s.Lessons {
			if i < 2 {
				progress[l.ID] = 1
			}
		}
	}

	ps := NewPathScreen(c, progress, "", i18n.New("en"), 80, 24)
	if ps.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (first incomplete)", ps.cursor)
	}
}

// TestNewPathScreen_CursorStartsAtZeroWhenAllCompleted checks that when all
// entries are done the cursor still lands at a valid position.
func TestNewPathScreen_CursorStartsAtZeroWhenAllCompleted(t *testing.T) {
	c := makePathCourse(3)
	progress := map[string]int{}
	for _, s := range c.Sections {
		for _, l := range s.Lessons {
			progress[l.ID] = 1
		}
	}

	ps := NewPathScreen(c, progress, "", i18n.New("en"), 80, 24)
	if ps.cursor < 0 || ps.cursor >= len(ps.entries) {
		t.Errorf("cursor %d out of range [0, %d)", ps.cursor, len(ps.entries))
	}
}

func TestNewPathScreen_PrefersRequestedLessonID(t *testing.T) {
	c := makePathCourse(4)
	ps := NewPathScreen(c, map[string]int{}, "02-lesson", i18n.New("en"), 80, 24)
	if ps.cursor != 2 {
		t.Fatalf("cursor = %d, want 2 for preferred lesson", ps.cursor)
	}
}

// TestPathScreen_View_ContainsTitles checks that lesson titles appear in the rendered view.
func TestPathScreen_View_ContainsTitles(t *testing.T) {
	c := makePathCourse(3)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)
	view := ps.View()

	for _, s := range c.Sections {
		for _, l := range s.Lessons {
			if !strings.Contains(view, l.Title) {
				t.Errorf("view does not contain lesson title %q", l.Title)
			}
		}
	}
}

// TestPathScreen_View_ContainsCourseTitle checks the header shows the course title.
func TestPathScreen_View_ContainsCourseTitle(t *testing.T) {
	c := makePathCourse(2)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)
	if !strings.Contains(ps.View(), c.Title) {
		t.Errorf("view does not contain course title %q", c.Title)
	}
}

func TestPathScreen_View_ShowsDetailPanelOnWideScreens(t *testing.T) {
	c := makePathCourse(1)
	c.Sections[0].Title = "Foundations"
	c.Sections[0].Lessons[0].Difficulty = "beginner"
	c.Sections[0].Lessons[0].Tags = []string{"intro", "stdio"}
	c.Sections[0].Lessons[0].References = []string{"K&R Chapter 1"}
	c.Sections[0].Lessons[0].Hints = []string{"hint one", "hint two"}
	c.Sections[0].AvailableTools = []string{"compile", "debug"}
	c.Sections[0].Lessons[0].Content = "# Lesson A\n\nThis lesson explains output streams and formatting."
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 120, 28)

	view := stripANSI(ps.View())
	if !strings.Contains(view, "Difficulty: beginner") {
		t.Fatalf("detail panel missing difficulty: %q", view)
	}
	if !strings.Contains(view, "Tags:") {
		t.Fatalf("detail panel missing tags: %q", view)
	}
	if !strings.Contains(view, "Summary:") {
		t.Fatalf("detail panel missing summary: %q", view)
	}
	if !strings.Contains(view, "Mochi") {
		t.Fatalf("detail panel missing Mochi: %q", view)
	}
	if !strings.Contains(view, "Section:") || !strings.Contains(view, "Foundations") {
		t.Fatalf("detail panel missing section metadata: %q", view)
	}
	if !strings.Contains(view, "2 hints") || !strings.Contains(view, "1 refs") {
		t.Fatalf("detail panel missing counts metadata: %q", view)
	}
	if !strings.Contains(view, "tools: compile,") || !strings.Contains(view, "debug") {
		t.Fatalf("detail panel missing tool metadata: %q", view)
	}
}

// TestPathScreen_Nav_MovesCursorDown checks that j/↓ advances the cursor.
func TestPathScreen_Nav_MovesCursorDown(t *testing.T) {
	c := makePathCourse(3)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)
	if ps.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", ps.cursor)
	}

	ps, _ = pathUpdate(ps, "j")
	if ps.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", ps.cursor)
	}

	ps, _ = pathUpdate(ps, "down")
	if ps.cursor != 2 {
		t.Errorf("cursor after ↓ = %d, want 2", ps.cursor)
	}

	// At last entry, further down is a no-op.
	ps, _ = pathUpdate(ps, "j")
	if ps.cursor != 2 {
		t.Errorf("cursor clamped at end: got %d, want 2", ps.cursor)
	}
}

// TestPathScreen_Nav_MovesCursorUp checks that k/↑ retreats the cursor.
func TestPathScreen_Nav_MovesCursorUp(t *testing.T) {
	c := makePathCourse(3)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)

	ps, _ = pathUpdate(ps, "j")
	ps, _ = pathUpdate(ps, "j")
	if ps.cursor != 2 {
		t.Fatalf("setup: cursor = %d, want 2", ps.cursor)
	}

	ps, _ = pathUpdate(ps, "k")
	if ps.cursor != 1 {
		t.Errorf("cursor after k = %d, want 1", ps.cursor)
	}

	ps, _ = pathUpdate(ps, "up")
	if ps.cursor != 0 {
		t.Errorf("cursor after ↑ = %d, want 0", ps.cursor)
	}

	// At first entry, further up is a no-op.
	ps, _ = pathUpdate(ps, "k")
	if ps.cursor != 0 {
		t.Errorf("cursor clamped at start: got %d, want 0", ps.cursor)
	}
}

// TestPathScreen_EscNavigatesToMenu checks that esc fires NavigateToMenu.
func TestPathScreen_EscNavigatesToMenu(t *testing.T) {
	c := makePathCourse(2)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)

	_, cmd := pathUpdate(ps, "esc")
	if cmd == nil {
		t.Fatal("esc should produce a cmd")
	}
	msg := cmd()
	if _, ok := msg.(NavigateToMenu); !ok {
		t.Errorf("esc cmd returned %T, want NavigateToMenu", msg)
	}
}

// TestPathScreen_QNavigatesToMenu checks that q also fires NavigateToMenu.
func TestPathScreen_QNavigatesToMenu(t *testing.T) {
	c := makePathCourse(2)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)

	_, cmd := pathUpdate(ps, "q")
	if cmd == nil {
		t.Fatal("q should produce a cmd")
	}
	if _, ok := cmd().(NavigateToMenu); !ok {
		t.Error("q cmd did not return NavigateToMenu")
	}
}

// TestPathScreen_EnterLockedDoesNothing checks that enter on a locked node is a no-op.
func TestPathScreen_EnterLockedDoesNothing(t *testing.T) {
	c := makePathCourse(3)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)

	// Move cursor to index 1 (locked with no progress).
	ps, _ = pathUpdate(ps, "j")
	if ps.entries[ps.cursor].status != pathLocked {
		t.Skipf("entry[%d] is not locked (status=%v); skip", ps.cursor, ps.entries[ps.cursor].status)
	}

	_, cmd := pathUpdate(ps, "enter")
	if cmd != nil {
		t.Errorf("enter on locked node returned non-nil cmd, want nil")
	}
}

// TestPathScreen_EnterAvailableExerciseSendsLessonNavigate checks that enter
// on an available exercise lesson opens the LessonScreen first, not the
// ExerciseScreen directly. The lesson screen then offers "e" to open the exercise.
func TestPathScreen_EnterAvailableExerciseSendsLessonNavigate(t *testing.T) {
	c := makePathCourse(2)
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)
	if ps.entries[ps.cursor].status != pathAvailable {
		t.Fatalf("expected cursor on available entry, got %v", ps.entries[ps.cursor].status)
	}

	_, cmd := pathUpdate(ps, "enter")
	if cmd == nil {
		t.Fatal("enter on available exercise should produce a cmd")
	}
	if _, ok := cmd().(NavigateToLessonMsg); !ok {
		t.Errorf("enter returned %T, want NavigateToLessonMsg (lesson screen first)", cmd())
	}
}

// TestPathScreen_EnterReadOnlySendsLessonNavigate checks that enter on a
// read-only (no-exercise) lesson fires NavigateToLessonMsg.
func TestPathScreen_EnterReadOnlySendsLessonNavigate(t *testing.T) {
	c := makePathCourse(1)
	c.Sections[0].Lessons[0].Exercise = "" // make it read-only
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 80, 24)

	_, cmd := pathUpdate(ps, "enter")
	if cmd == nil {
		t.Fatal("enter on available read-only lesson should produce a cmd")
	}
	if _, ok := cmd().(NavigateToLessonMsg); !ok {
		t.Errorf("enter returned %T, want NavigateToLessonMsg", cmd())
	}
}

// TestPathNodeIndex checks the index label extraction from lesson IDs.
func TestPathNodeIndex(t *testing.T) {
	cases := []struct{ id, want string }{
		{"00-module-intro", "00"},
		{"01-hello-stderr", "01"},
		{"15-capstone-redis", "15"},
		{"nohyphen", "00"},
		{"", "00"},
	}
	for _, tc := range cases {
		got := pathNodeIndex(tc.id)
		if got != tc.want {
			t.Errorf("pathNodeIndex(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// TestPathProgressBar_Bounds checks edge-case inputs to pathProgressBar.
func TestPathProgressBar_Bounds(t *testing.T) {
	// Zero total must not panic.
	s := pathProgressBar(0, 0, 10)
	if s == "" {
		t.Error("pathProgressBar(0,0,10) returned empty string")
	}

	// Full completion.
	s = pathProgressBar(5, 5, 8)
	if !strings.Contains(s, "█") {
		t.Error("full completion bar should contain filled block")
	}
}

// TestCourseLayout_ParsedFromYAML verifies that layout: path in course.yaml
// propagates to the Course.Layout field.
func TestCourseLayout_ParsedFromYAML(t *testing.T) {
	courses, err := lesson.LoadCoursesLang("../../courses", "en")
	if err != nil {
		t.Fatalf("LoadCoursesLang: %v", err)
	}
	for _, c := range courses {
		if c.ID == "c-programming" {
			if c.Layout != lesson.CourseLayoutPath {
				t.Errorf("c-programming layout = %q, want %q", c.Layout, lesson.CourseLayoutPath)
			}
			return
		}
	}
	t.Fatal("c-programming course not found")
}

// TestPathScreen_NodesDoNotOverflowPathPane guards against the rendering bug
// where nodeBoxWidth used ps.Width instead of pathPaneWidth, causing node
// right borders to be clipped when the detail panel is visible.
func TestPathScreen_NodesDoNotOverflowPathPane(t *testing.T) {
	c := makePathCourse(3)
	// Wide enough to trigger the detail panel (threshold is 96).
	ps := NewPathScreen(c, map[string]int{}, "", i18n.New("en"), 120, 28)
	paneW := ps.pathPaneWidth()

	raw := stripANSI(ps.buildContent())
	for i, line := range strings.Split(raw, "\n") {
		if len([]rune(line)) > paneW {
			t.Errorf("line %d width %d exceeds path pane width %d: %q",
				i, len([]rune(line)), paneW, line)
		}
	}
}

// TestPathScreen_RTL_FooterIsTranslated checks the Arabic footer uses translated hints.
func TestPathScreen_RTL_FooterIsTranslated(t *testing.T) {
	c := makePathCourse(2)
	arLoc := i18n.New("ar")
	ps := NewPathScreen(c, map[string]int{}, "", arLoc, 80, 24)
	view := stripANSI(ps.View())
	// Arabic "open" action label from path.help.open
	if !strings.Contains(view, "فتح") {
		t.Errorf("RTL footer should contain Arabic 'open' label; view:\n%s", view)
	}
	// Arabic "back" label from help.esc_back
	if !strings.Contains(view, "رجوع") {
		t.Errorf("RTL footer should contain Arabic 'back' label; view:\n%s", view)
	}
}

// TestPathScreen_RTL_DetailPanelOnLeft checks that the detail panel renders
// to the left of the path node content in RTL mode.
// Strategy: JoinHorizontal produces lines that interleave both panels.
// On any line containing both Arabic text (detail panel) and a path node
// border character (╔/║/╚ from the double-line box), the Arabic text must
// appear BEFORE the node character — i.e. the detail panel is on the left.
func TestPathScreen_RTL_DetailPanelOnLeft(t *testing.T) {
	c := makePathCourse(2)
	c.Sections[0].Lessons[0].Difficulty = "beginner"
	arLoc := i18n.New("ar")
	// Wide enough to trigger the detail panel (threshold is 96).
	ps := NewPathScreen(c, map[string]int{}, "", arLoc, 120, 28)
	view := stripANSI(ps.View())

	found := false
	for _, line := range strings.Split(view, "\n") {
		// Find the first path node box character on this line.
		nodeIdx := -1
		for i, r := range []rune(line) {
			if r == '╔' || r == '║' || r == '╚' {
				nodeIdx = i
				break
			}
		}
		if nodeIdx < 0 {
			continue
		}
		// Find Arabic text on this line before the node character.
		runes := []rune(line)
		for i := 0; i < nodeIdx; i++ {
			if runes[i] >= 0x0600 && runes[i] <= 0x06FF {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Error("in RTL layout, no line found where Arabic detail panel text precedes the path node box — detail panel may not be on the left")
	}
}

// TestPathScreen_RTL_NodeMarkerOnRight checks that in RTL mode the cursor
// marker "< " appears to the RIGHT of the node box, not the left.
func TestPathScreen_RTL_NodeMarkerOnRight(t *testing.T) {
	c := makePathCourse(1)
	arLoc := i18n.New("ar")
	ps := NewPathScreen(c, map[string]int{}, "", arLoc, 80, 24)
	content := stripANSI(ps.buildContent())

	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "<") && strings.Contains(line, "║") {
			// The "<" cursor should appear AFTER the closing "║", not before the opening one.
			openIdx := strings.Index(line, "║")
			closeIdx := strings.LastIndex(line, "║")
			ltIdx := strings.Index(line, "<")
			if ltIdx >= 0 && ltIdx < closeIdx && ltIdx > openIdx {
				t.Errorf("RTL cursor '<' appears inside the node box on line: %q", line)
			}
			if ltIdx >= 0 && ltIdx < openIdx {
				t.Errorf("RTL cursor '<' appears before the node box (LTR position) on line: %q", line)
			}
		}
	}
}

// TestPathScreen_DetailPanel_UsesTranslatedLabels checks that the detail panel
// strings come from the locale, not hardcoded English.
func TestPathScreen_DetailPanel_UsesTranslatedLabels(t *testing.T) {
	c := makePathCourse(1)
	c.Sections[0].Lessons[0].Difficulty = "beginner"
	c.Sections[0].Lessons[0].Content = "Some lesson prose."
	esLoc := i18n.New("es")
	ps := NewPathScreen(c, map[string]int{}, "", esLoc, 120, 28)
	view := stripANSI(ps.View())

	// Spanish translations from the locale file.
	for _, want := range []string{"Dificultad", "Listo", "Ejercicio", "Resumen"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail panel missing Spanish label %q; view snippet:\n%.200s", want, view)
		}
	}
}

func TestPathLessonSummary_StripsMarkdownNoise(t *testing.T) {
	content := "# Title\n\n- First point\n\n## Subtitle\n\nThis is the real summary paragraph.\n\n> quoted note"
	got := pathLessonSummary(content)
	if strings.Contains(got, "#") {
		t.Fatalf("summary should not include markdown headings: %q", got)
	}
	if !strings.Contains(got, "This is the real summary paragraph.") {
		t.Fatalf("summary should prefer real paragraph prose: %q", got)
	}
	if strings.Contains(got, "First point") {
		t.Fatalf("summary should avoid leading bullet-list fallback when a paragraph exists: %q", got)
	}
}
