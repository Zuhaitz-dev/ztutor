package tui

import (
	"testing"

	"ztutor/internal/db"
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAppLanguageChangeStaysOnLicenseEntry(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}

	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.Update(NavigateToLicenseEntry{})
	if _, ok := app.current.(*licenseEntryScreen); !ok {
		t.Fatalf("current = %T, want *licenseEntryScreen", app.current)
	}

	if _, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlL}); cmd != nil {
		t.Fatal("language change on license entry should stay in place without async command")
	}
	screen, ok := app.current.(*licenseEntryScreen)
	if !ok {
		t.Fatalf("current = %T, want *licenseEntryScreen", app.current)
	}
	if screen.loc.Lang() != "es" {
		t.Fatalf("license entry lang = %q, want es", screen.loc.Lang())
	}
}

func TestAppNewUser_StartsOnIntroScreen(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	if _, ok := app.current.(*IntroScreen); !ok {
		t.Fatalf("new user should start on IntroScreen, got %T", app.current)
	}
}

func TestAppIntroComplete_GoesToConnectChoice(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.Update(introCompleteMsg{})
	if _, ok := app.current.(*connectChoiceScreen); !ok {
		t.Fatalf("after main intro complete, should be on connectChoiceScreen, got %T", app.current)
	}
}

func TestAppCourseIntroComplete_GoesToMenuWithCourse(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := lesson.Course{
		ID:    "test-course",
		Title: "Test Course",
		Sections: []lesson.Section{{
			ID:      "lessons",
			Type:    "exercises",
			Lessons: []lesson.Lesson{{ID: "l1", Title: "Lesson 1"}},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{course}
	app.pendingCourseEntry = &course
	app.Update(introCompleteMsg{courseID: "test-course"})
	if _, ok := app.current.(*MenuScreen); !ok {
		t.Fatalf("after course intro complete, should be on MenuScreen, got %T", app.current)
	}
}

func TestAppNavigateBackToCourse_PathCourseRestoresPathScreen(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := lesson.Course{
		ID:     "path-course",
		Title:  "Path Course",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:      "lessons",
			Type:    "exercises",
			Lessons: []lesson.Lesson{{ID: "l1", Title: "Lesson 1", Exercise: "int main(){}"}},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{course}
	app.openCourse(course)
	app.current = NewLessonScreen(course.Sections[0].Lessons[0], 0, 80, 24, app.loc)

	app.Update(NavigateBackToCourse{})

	if _, ok := app.current.(*PathScreen); !ok {
		t.Fatalf("back from path course content should restore PathScreen, got %T", app.current)
	}
}

func TestAppLessonCompleted_PathCourseRestoresPathScreen(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := lesson.Course{
		ID:     "path-course",
		Title:  "Path Course",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:      "lessons",
			Type:    "exercises",
			Lessons: []lesson.Lesson{{ID: "l1", Title: "Lesson 1", Exercise: "int main(){}"}},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{course}
	app.openCourse(course)

	app.Update(lessonCompletedMsg{lessonID: "l1", stars: 3})

	if _, ok := app.current.(*PathScreen); !ok {
		t.Fatalf("completion in path course should restore PathScreen, got %T", app.current)
	}
	if app.progress["l1"] != 3 {
		t.Fatalf("progress[l1] = %d, want 3", app.progress["l1"])
	}
}

func TestAppNavigateBackToCourse_ListCourseRestoresLessonMenu(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := lesson.Course{
		ID:    "list-course",
		Title: "List Course",
		Sections: []lesson.Section{{
			ID:      "lessons",
			Type:    "exercises",
			Lessons: []lesson.Lesson{{ID: "l1", Title: "Lesson 1", Exercise: "int main(){}"}},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{course}
	app.openCourse(course)
	app.current = NewLessonScreen(course.Sections[0].Lessons[0], 0, 80, 24, app.loc)

	app.Update(NavigateBackToCourse{})

	menu, ok := app.current.(*MenuScreen)
	if !ok {
		t.Fatalf("back from list course content should restore MenuScreen, got %T", app.current)
	}
	if menu.viewLevel != "lessons" || menu.selectedCourse == nil || menu.selectedCourse.ID != "list-course" {
		t.Fatalf("restored menu should stay inside the active course, got viewLevel=%q selected=%v", menu.viewLevel, menu.selectedCourse)
	}
}

func TestAppRefreshCurrentScreenData_PathScreenReloadsCourseContent(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	initial := lesson.Course{
		ID:     "path-course",
		Title:  "English Title",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:    "lessons",
			Title: "Basics",
			Type:  "exercises",
			Lessons: []lesson.Lesson{
				{ID: "l1", Title: "First Lesson", Exercise: "int main(){}"},
			},
		}},
	}
	localized := lesson.Course{
		ID:     "path-course",
		Title:  "Titulo Traducido",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:    "lessons",
			Title: "Fundamentos",
			Type:  "exercises",
			Lessons: []lesson.Lesson{
				{ID: "l1", Title: "Leccion Uno", Exercise: "int main(){}"},
			},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{initial}
	app.openCourse(initial)

	app.courses = []lesson.Course{localized}
	app.refreshCurrentScreenData()

	ps, ok := app.current.(*PathScreen)
	if !ok {
		t.Fatalf("current = %T, want *PathScreen", app.current)
	}
	if ps.course.Title != "Titulo Traducido" {
		t.Fatalf("course title = %q, want localized title", ps.course.Title)
	}
	if len(ps.entries) != 1 || ps.entries[0].lesson.Title != "Leccion Uno" {
		t.Fatalf("path entries not refreshed with localized lesson title: %+v", ps.entries)
	}
}

func TestAppLessonCompleted_GoNextOpensNextLessonScreen(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := lesson.Course{
		ID:     "path-course",
		Title:  "Path Course",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:    "lessons",
			Type:  "exercises",
			Title: "Lessons",
			Lessons: []lesson.Lesson{
				{ID: "l1", Title: "Lesson 1", Exercise: "int main(){}"},
				{ID: "l2", Title: "Lesson 2", Exercise: "int main(){return 0;}"},
			},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{course}
	app.openCourse(course)

	app.Update(lessonCompletedMsg{lessonID: "l1", stars: 3, goNext: true})

	ls, ok := app.current.(*LessonScreen)
	if !ok {
		t.Fatalf("goNext should open LessonScreen, got %T", app.current)
	}
	if ls.lesson.ID != "l2" {
		t.Fatalf("next lesson id = %q, want l2", ls.lesson.ID)
	}
}

func TestAppLessonCompleted_GoNextDoesNotJumpAcrossCourses(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	courseA := lesson.Course{
		ID:     "course-a",
		Title:  "Course A",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:    "lessons",
			Type:  "exercises",
			Title: "Lessons",
			Lessons: []lesson.Lesson{
				{ID: "a1", Title: "A1", Exercise: "int main(){}"},
			},
		}},
	}
	courseB := lesson.Course{
		ID:    "course-b",
		Title: "Course B",
		Sections: []lesson.Section{{
			ID:    "lessons",
			Type:  "exercises",
			Title: "Lessons",
			Lessons: []lesson.Lesson{
				{ID: "b1", Title: "B1", Exercise: "int main(){}"},
			},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{courseA, courseB}
	app.openCourse(courseA)

	app.Update(lessonCompletedMsg{lessonID: "a1", stars: 3, goNext: true})

	if _, ok := app.current.(*PathScreen); !ok {
		t.Fatalf("when there is no next lesson in active course, should return to PathScreen, got %T", app.current)
	}
}

func TestAppNavigateBackToCourse_PathCourseKeepsSelectedNode(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := lesson.Course{
		ID:     "path-course",
		Title:  "Path Course",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:    "lessons",
			Type:  "exercises",
			Title: "Lessons",
			Lessons: []lesson.Lesson{
				{ID: "l1", Title: "Lesson 1", Exercise: "int main(){}"},
				{ID: "l2", Title: "Lesson 2", Exercise: "int main(){}"},
				{ID: "l3", Title: "Lesson 3", Exercise: "int main(){}"},
			},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{course}
	app.openCourse(course)

	app.Update(NavigateToLessonMsg{Lesson: course.Sections[0].Lessons[2]})
	app.Update(NavigateBackToCourse{})

	ps, ok := app.current.(*PathScreen)
	if !ok {
		t.Fatalf("current = %T, want *PathScreen", app.current)
	}
	if ps.cursor != 2 {
		t.Fatalf("cursor = %d, want 2 after returning from selected lesson", ps.cursor)
	}
}

func TestAppLessonCompleted_PathCourseAdvancesSelectedNode(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	course := lesson.Course{
		ID:     "path-course",
		Title:  "Path Course",
		Layout: lesson.CourseLayoutPath,
		Sections: []lesson.Section{{
			ID:    "lessons",
			Type:  "exercises",
			Title: "Lessons",
			Lessons: []lesson.Lesson{
				{ID: "l1", Title: "Lesson 1", Exercise: "int main(){}"},
				{ID: "l2", Title: "Lesson 2", Exercise: "int main(){}"},
				{ID: "l3", Title: "Lesson 3", Exercise: "int main(){}"},
			},
		}},
	}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.courses = []lesson.Course{course}
	app.openCourse(course)

	app.Update(lessonCompletedMsg{lessonID: "l1", stars: 3})

	ps, ok := app.current.(*PathScreen)
	if !ok {
		t.Fatalf("current = %T, want *PathScreen", app.current)
	}
	if ps.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after completing first lesson", ps.cursor)
	}
}

func TestAppSettingsSaveStaysOnSettings(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}

	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	app.Update(NavigateToSettings{})
	if _, ok := app.current.(*SettingsScreen); !ok {
		t.Fatalf("current = %T, want *SettingsScreen", app.current)
	}

	app.Update(settingsSavedMsg{key: "keymap", value: "vim"})
	if _, ok := app.current.(*SettingsScreen); !ok {
		t.Fatalf("current = %T, want *SettingsScreen after save", app.current)
	}
	if app.keymap != "vim" {
		t.Fatalf("app keymap = %q, want vim", app.keymap)
	}
	if got, _ := database.GetUserSetting("alice", "keymap"); got != "vim" {
		t.Fatalf("persisted keymap = %q, want vim", got)
	}
}
