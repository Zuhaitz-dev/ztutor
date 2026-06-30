package tui

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ztutor/internal/db"
	"ztutor/internal/lesson"
	"ztutor/internal/license"

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

func TestConnectChoice_ShowsLicenseSummaryOptionWhenLicensed(t *testing.T) {
	screen := NewConnectChoiceScreen(testLocale(), 80, 24, "", true)
	view := stripANSI(screen.View())
	if !strings.Contains(view, "View current license") {
		t.Fatalf("licensed connect choice should show license summary option, got:\n%s", view)
	}
}

func TestConnectChoice_HidesLicenseSummaryOptionWhenUnlicensed(t *testing.T) {
	screen := NewConnectChoiceScreen(testLocale(), 80, 24, "", false)
	view := stripANSI(screen.View())
	if strings.Contains(view, "View current license") {
		t.Fatalf("unlicensed connect choice should hide license summary option, got:\n%s", view)
	}
}

func TestAppNavigateToLicenseSummary_ShowsSummary(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}
	lic := &license.State{Licensed: true, Licensee: "Acme"}
	app := NewApp("alice", t.TempDir(), t.TempDir(), database, lic, 80, 24, "default", nil, nil)

	app.Update(NavigateToLicenseSummary{})
	if _, ok := app.current.(*licenseSummaryScreen); !ok {
		t.Fatalf("current = %T, want *licenseSummaryScreen", app.current)
	}
}

func TestLicenseSummary_ShowsInstalledAndMissingCourses(t *testing.T) {
	lic := &license.State{
		Licensed:        true,
		Licensee:        "Acme",
		UnlockedCourses: []string{"c-programming", "redis-capstone"},
	}
	courses := []lesson.Course{
		{ID: "c-programming", Title: "C Programming"},
	}
	screen := NewLicenseSummaryScreen(testLocale(), lic, courses, 80, 24)
	view := stripANSI(screen.View())
	for _, want := range []string{
		"Installed now: 1 installed course(s)",
		"C Programming (c-programming)",
		"Not installed: 1 missing course(s)",
		"redis-capstone",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("license summary missing %q, got:\n%s", want, view)
		}
	}
}

func TestAppRedeemsPersonalLicenseAndReloadsIt(t *testing.T) {
	database := testDB(t)
	if err := database.CreateUser("alice", "", db.RoleStudent); err != nil {
		t.Fatalf("create user: %v", err)
	}

	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	license.SetPublicKey(pub)

	courseKey := make([]byte, 32)
	for i := range courseKey {
		courseKey[i] = byte(i + 1)
	}
	info := license.Info{
		Licensee:        "Campaign",
		LicenseID:       "lic-789",
		Username:        "alice",
		UnlockedCourses: []string{"c-module-02"},
		CourseKey:       hex.EncodeToString(courseKey),
		IssuedAt:        time.Now().Format(time.RFC3339),
	}
	signed, err := license.Sign(priv, info)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	app := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	msg := app.submitLicenseEntry(string(signed))()
	app.Update(msg)

	enrolled, err := database.ListEnrollments("alice")
	if err != nil {
		t.Fatalf("ListEnrollments: %v", err)
	}
	if len(enrolled) != 1 || enrolled[0] != "c-module-02" {
		t.Fatalf("enrollments = %v, want [c-module-02]", enrolled)
	}
	if app.lic == nil || !app.lic.CanAccessCourse("c-module-02") {
		t.Fatalf("app license did not unlock c-module-02: %+v", app.lic)
	}
	if got := hex.EncodeToString(app.lic.CourseKey); got != info.CourseKey {
		t.Fatalf("course key = %q, want %q", got, info.CourseKey)
	}

	app2 := NewApp("alice", t.TempDir(), t.TempDir(), database, nil, 80, 24, "default", nil, nil)
	if app2.lic == nil || !app2.lic.CanAccessCourse("c-module-02") {
		t.Fatalf("reloaded app license did not preserve entitlement: %+v", app2.lic)
	}
	if got := hex.EncodeToString(app2.lic.CourseKey); got != info.CourseKey {
		t.Fatalf("reloaded course key = %q, want %q", got, info.CourseKey)
	}
}

func TestReadLicenseValue_MissingPathReturnsHelpfulError(t *testing.T) {
	_, err := readLicenseValue("/definitely/missing/license.key")
	if err == nil || !strings.Contains(err.Error(), "license file not found") {
		t.Fatalf("readLicenseValue error = %v, want helpful file-not-found error", err)
	}
}

func TestReadLicenseValue_ResolvesDataDirFallback(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	path := filepath.Join(dataHome, "ztutor", "license.key")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	want := []byte("license-bytes")
	if err := os.WriteFile(path, want, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := readLicenseValue("license.key")
	if err != nil {
		t.Fatalf("readLicenseValue: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("readLicenseValue = %q, want %q", got, want)
	}
}

func TestResolveLicensePath_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "license.key")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, ok := resolveLicensePath(path)
	if !ok || got != path {
		t.Fatalf("resolveLicensePath = (%q, %v), want (%q, true)", got, ok, path)
	}
}

func TestResolveLicensePath_HomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "license.key")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, ok := resolveLicensePath("license.key")
	if !ok || got != path {
		t.Fatalf("resolveLicensePath = (%q, %v), want (%q, true)", got, ok, path)
	}
}

func TestResolveLicensePath_RawJSONIsNotPath(t *testing.T) {
	got, ok := resolveLicensePath(`{"sig":"abc","payload":{}}`)
	if ok || got != "" {
		t.Fatalf("resolveLicensePath = (%q, %v), want empty/false", got, ok)
	}
}

func TestMergeLicenseStates(t *testing.T) {
	base := &license.State{
		Licensed:        true,
		Licensee:        "Base",
		LicenseID:       "base-id",
		Username:        "alice",
		MaxStudents:     10,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
		UnlockedCourses: []string{"c-programming", "redis"},
		CourseKey:       []byte{1, 2, 3},
		HasMultiUser:    true,
	}
	extra := &license.State{
		Licensed:              true,
		Licensee:              "Extra",
		LicenseID:             "extra-id",
		Email:                 "alice@example.com",
		MaxStudents:           20,
		ExpiresAt:             time.Now().Add(48 * time.Hour),
		UnlockedCourses:       []string{"redis", "python"},
		CourseKey:             []byte{9, 9, 9},
		HasAdminUI:            true,
		HasInterviewQuestions: true,
	}

	merged := mergeLicenseStates(base, extra)
	if merged.Licensee != "Base" || merged.LicenseID != "base-id" || merged.Username != "alice" {
		t.Fatalf("merged identity fields = %+v", merged)
	}
	if merged.Email != "alice@example.com" {
		t.Fatalf("merged.Email = %q, want alice@example.com", merged.Email)
	}
	if merged.MaxStudents != 10 {
		t.Fatalf("merged.MaxStudents = %d, want 10", merged.MaxStudents)
	}
	if !merged.ExpiresAt.Equal(extra.ExpiresAt) {
		t.Fatalf("merged.ExpiresAt = %v, want %v", merged.ExpiresAt, extra.ExpiresAt)
	}
	if !merged.HasMultiUser || !merged.HasAdminUI || !merged.HasInterviewQuestions {
		t.Fatalf("merged feature flags = %+v", merged)
	}
	if !reflect.DeepEqual(merged.CourseKey, []byte{1, 2, 3}) {
		t.Fatalf("merged.CourseKey = %v, want base key", merged.CourseKey)
	}
	if !reflect.DeepEqual(merged.UnlockedCourses, []string{"c-programming", "redis", "python"}) {
		t.Fatalf("merged.UnlockedCourses = %v", merged.UnlockedCourses)
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
