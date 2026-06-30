package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"ztutor/internal/db"
	"ztutor/internal/i18n"
	"ztutor/internal/license"

	tea "github.com/charmbracelet/bubbletea"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func testDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func testLocale() *i18n.Locale {
	return i18n.New("en")
}

func assertViewNonEmpty(t *testing.T, name string, view string) {
	t.Helper()
	if view == "" {
		t.Errorf("%s: View() returned empty string", name)
	}
	view = stripANSI(view)
	if strings.TrimSpace(view) == "" {
		t.Errorf("%s: View() returned only ANSI codes / whitespace", name)
	}
}

// ── TestAdminDashboard_View ──────────────────────────────────────────────────

func TestAdminDashboard_View(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)
	loc := testLocale()

	m := newAdminDashboard(database, nil, loc, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "dashboard", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Admin") {
		t.Error("dashboard view missing expected 'Admin' content")
	}
}

func TestAdminDashboard_View_WithLicense(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)
	loc := testLocale()

	lic := &license.State{
		Licensed:        true,
		Licensee:        "Test School",
		MaxStudents:     50,
		UnlockedCourses: []string{"c1"},
		HasMultiUser:    true,
	}

	m := newAdminDashboard(database, lic, loc, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "dashboard with license", view)
	plain := stripANSI(view)
	// Licensed dashboard should show licensee name and features.
	if !strings.Contains(plain, "Test School") {
		t.Error("dashboard should show licensee name 'Test School'")
	}
}

func TestRainCol_RenderAt_WrapsGlyphWithLTRMarkers(t *testing.T) {
	col := rainCol{
		head:  0,
		speed: 1,
		trail: 5,
		chars: []rune{'<'},
	}

	got := col.renderAt(0)
	if !strings.Contains(got, ltrMarker+"<"+ltrMarker) {
		t.Fatalf("renderAt should wrap rain glyphs with LTR markers, got %q", got)
	}
}

func TestAdminDashboard_View_ArabicLicenseeUsesLTRIsolation(t *testing.T) {
	database := testDB(t)
	loc := i18n.New("ar")

	lic := &license.State{
		Licensed:        true,
		Licensee:        "Test School",
		MaxStudents:     50,
		UnlockedCourses: []string{"c1"},
		HasMultiUser:    true,
	}

	m := newAdminDashboard(database, lic, loc, 80, 24)
	view := m.View()

	if !strings.Contains(view, forceLTRText("Test School")) {
		t.Fatalf("arabic dashboard should isolate licensee as LTR, got %q", view)
	}
	if !strings.Contains(view, forceLTRText("premium, multi-user")) {
		t.Fatalf("arabic dashboard should isolate feature list as LTR, got %q", view)
	}
}

func TestAdminDashboard_View_FlashError(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)
	loc := testLocale()

	m := newAdminDashboardWithErr(database, nil, loc, "something broke", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "dashboard with error", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "something broke") {
		t.Error("dashboard view missing flash error message")
	}
}

func TestAdminApp_FirstRunFreeModeUsesLearnerSetup(t *testing.T) {
	database := testDB(t)

	app := NewAdminApp("starter", database, nil, t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "achievements.yaml"), 80, 24)
	view := stripANSI(app.View())

	if !strings.Contains(view, "Learner username:") {
		t.Fatalf("free first-run should use learner setup copy, got:\n%s", view)
	}
	if strings.Contains(view, "Admin username:") {
		t.Fatalf("free first-run should not use admin setup copy, got:\n%s", view)
	}
}

func TestLicenseSummaryScreen_View(t *testing.T) {
	lic := &license.State{
		Licensed:              true,
		Licensee:              "Acme School",
		UnlockedCourses:       []string{"c1"},
		HasInterviewQuestions: true,
	}

	screen := NewLicenseSummaryScreen(testLocale(), lic, nil, 80, 24)
	view := stripANSI(screen.View())

	for _, want := range []string{"License Summary", "Premium", "Acme School", "1 course(s)", "interviews"} {
		if !strings.Contains(view, want) {
			t.Fatalf("license summary missing %q, got:\n%s", want, view)
		}
	}
}

// ── TestAdminStudentList_View ────────────────────────────────────────────────

func TestAdminStudentList_View(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw1", db.RoleStudent)
	_ = database.CreateUser("bob", "pw2", db.RoleStudent)

	m := newAdminStudentList(database, nil, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "student list", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "alice") {
		t.Error("student list view missing 'alice'")
	}
	if !strings.Contains(plain, "bob") {
		t.Error("student list view missing 'bob'")
	}
}

func TestAdminStudentList_View_Empty(t *testing.T) {
	database := testDB(t)

	m := newAdminStudentList(database, nil, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "empty student list", view)
}

// ── TestAdminAddStudent_Flow ─────────────────────────────────────────────────

func TestAdminAddStudent_Flow(t *testing.T) {
	database := testDB(t)

	m := newAdminAddStudent(database, nil, nil, 80, 24)

	// Type a username into the text input.
	m.input.SetValue("testuser")

	// Press enter to create the student.
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := model.(*adminAddStudentModel)

	// The password should be shown (showPending = true).
	if !m2.showPending {
		t.Error("expected showPending to be true after creating user")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd after enter, got %T", cmd)
	}

	// Verify the user was actually created.
	user, err := database.GetUser("testuser")
	if err != nil {
		t.Fatalf("GetUser after create: %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("username = %q, want testuser", user.Username)
	}
	if user.Role != db.RoleStudent {
		t.Errorf("role = %q, want student", user.Role)
	}

	// Press any key to navigate back (since showPending is true).
	_, cmd2 := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd2 == nil {
		t.Error("expected adminStudentAddedMsg cmd when any key pressed after creation")
	}
}

func TestAdminAddStudent_EmptyName(t *testing.T) {
	database := testDB(t)

	m := newAdminAddStudent(database, nil, nil, 80, 24)

	// Press enter with empty username.
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := model.(*adminAddStudentModel)

	if m2.msg == "" {
		t.Error("expected error message for empty username")
	}
}

func TestAdminAddStudent_LicenseSeatLimit(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("existing", "pw", db.RoleStudent)

	lic := &license.State{
		Licensed:    true,
		MaxStudents: 1,
	}

	m := newAdminAddStudent(database, lic, nil, 80, 24)
	m.input.SetValue("newuser")

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := model.(*adminAddStudentModel)

	if m2.msg == "" {
		t.Error("expected license seat limit error message")
	}
	if !strings.Contains(stripANSI(m2.msg), "max") {
		t.Errorf("license limit message should mention limit: %q", m2.msg)
	}
}

// ── TestAdminStudentDetail_View ──────────────────────────────────────────────

func TestAdminStudentDetail_View(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminStudentDetail("alice", database, "", "en", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "student detail", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "alice") {
		t.Error("student detail view missing 'alice'")
	}
}

func TestAdminStudentDetail_View_DisabledUser(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("bob", "pw", db.RoleStudent)
	_ = database.SetUserEnabled("bob", false)

	m := newAdminStudentDetail("bob", database, "", "en", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "disabled student detail", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "disabled") {
		t.Error("disabled user detail should show disabled indicator")
	}
}

// ── TestAdminCourseList_View ─────────────────────────────────────────────────

func TestAdminCourseList_View(t *testing.T) {
	database := testDB(t)

	m := newAdminCourseList(database, "", "en", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "course list", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Courses") {
		t.Error("course list view missing 'Courses'")
	}
}

// ── TestAdminLessonCreate_View ───────────────────────────────────────────────

func TestAdminLessonCreate_View(t *testing.T) {
	m := newAdminLessonCreate(t.TempDir(), 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "lesson create wizard", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Create Lesson") {
		t.Error("lesson create view missing 'Create Lesson'")
	}
}

func TestAdminLessonCreate_AdvanceStep(t *testing.T) {
	m := newAdminLessonCreate(t.TempDir(), 80, 24)

	// Set ID and title for valid metadata.
	m.idInput.SetValue("test-lesson")
	m.titleInput.SetValue("Test Lesson")

	// advanceStep should succeed and move to stepContent.
	cmd := m.advanceStep()
	if cmd != nil {
		t.Logf("advanceStep returned cmd (ok)")
	}
	if m.step != stepContent {
		t.Errorf("after advanceStep: step = %d, want stepContent(1)", m.step)
	}
}

func TestAdminLessonCreate_AdvanceStep_NoID(t *testing.T) {
	m := newAdminLessonCreate(t.TempDir(), 80, 24)

	// Advance without setting ID.
	_ = m.advanceStep()
	if m.msg == "" {
		t.Error("expected error message when advancing without ID")
	}
	if m.step != stepMeta {
		t.Error("step should remain at stepMeta when ID is missing")
	}
}

func TestAdminLessonCreate_AllStepsView(t *testing.T) {
	m := newAdminLessonCreate(t.TempDir(), 80, 24)
	m.idInput.SetValue("test-lesson")
	m.titleInput.SetValue("Test Lesson")

	steps := []lessonWizardStep{stepMeta, stepContent, stepExercise, stepExpected, stepTutorial, stepHints, stepSave}
	for _, step := range steps {
		m.step = step
		m.focusCurrentStep()
		view := m.View()
		assertViewNonEmpty(t, "lesson create step", view)
	}
}

// ── TestAdminAchievements_View ───────────────────────────────────────────────

func TestAdminAchievements_View(t *testing.T) {
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")
	m := newAdminAchievements(achFile, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "achievements", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Achievements") {
		t.Error("achievements view missing 'Achievements'")
	}
}

func TestAdminAchievements_View_Create(t *testing.T) {
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")
	m := newAdminAchievements(achFile, 80, 24)

	// Set mode to create.
	m.mode = achCreate
	view := m.View()

	assertViewNonEmpty(t, "achievements create view", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "New Achievement") {
		t.Error("achievements create view missing 'New Achievement'")
	}
}

// ── TestAdminExport_View ─────────────────────────────────────────────────────

func TestAdminExport_View(t *testing.T) {
	database := testDB(t)

	m := newAdminExport(database, nil, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "export", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Export") {
		t.Error("export view missing 'Export'")
	}
}

// ── TestAdminLessonImport_View ───────────────────────────────────────────────

func TestAdminLessonImport_View(t *testing.T) {
	m := newAdminLessonImport(t.TempDir(), 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "lesson import", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Import") {
		t.Error("lesson import view missing 'Import'")
	}
}

// ── TestAdminApp_Navigation ──────────────────────────────────────────────────

func TestAdminApp_Navigation(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	// Initial view should be the dashboard.
	view := app.View()
	assertViewNonEmpty(t, "admin app initial dashboard", view)

	// Press 's' on dashboard → returns NavigateToStudents cmd.
	model, cmd1 := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = model.(*AdminApp)
	if cmd1 == nil {
		t.Fatal("expected NavigateToStudents cmd from 's' key")
	}
	// Execute the cmd to get the navigation message; feed it back.
	nav1 := cmd1()
	model, _ = app.Update(nav1)
	app2 := model.(*AdminApp)
	view2 := app2.View()
	assertViewNonEmpty(t, "admin app student list", view2)
	plain2 := stripANSI(view2)
	if !strings.Contains(plain2, "admin") {
		t.Error("student list view should contain 'admin' username")
	}

	// Press 'b' on student list → returns NavigateToDashboard cmd.
	model3, cmd3 := app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	_ = model3
	if cmd3 == nil {
		t.Fatal("expected NavigateToDashboard cmd from 'b' key")
	}
	nav2 := cmd3()
	model4, _ := model3.Update(nav2)
	app4 := model4.(*AdminApp)
	view3 := app4.View()
	assertViewNonEmpty(t, "admin app back to dashboard", view3)

	// Press 'q' to quit.
	_, cmd5 := app4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd5 == nil {
		t.Error("expected tea.Quit cmd when pressing 'q' on dashboard")
	}
}

func TestAdminApp_StudentsNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)
	_ = database.CreateUser("charlie", "pw", db.RoleStudent)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	// Navigate to student list via message.
	model, _ := app.Update(NavigateToAdminStudents{})
	app2 := model.(*AdminApp)

	view := app2.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "charlie") {
		t.Error("student list after navigation should contain 'charlie'")
	}
}

func TestAdminApp_CoursesNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminCourses{})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "courses list after navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Courses") {
		t.Error("courses list view missing 'Courses'")
	}
}

func TestAdminApp_AchievementsNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminAchievements{})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "achievements after navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Achievements") {
		t.Error("achievements view missing 'Achievements'")
	}
}

func TestAdminApp_ExportNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminExport{})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "export after navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Export") {
		t.Error("export view missing 'Export'")
	}
}

func TestAdminApp_StudentDetailNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)
	_ = database.CreateUser("dave", "pw", db.RoleStudent)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminStudentDetail{Username: "dave"})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "student detail after navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "dave") {
		t.Error("student detail view missing 'dave'")
	}
}

func TestAdminApp_LessonCreateNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminLessonCreate{})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "lesson create after navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Create Lesson") {
		t.Error("lesson create view missing 'Create Lesson'")
	}
}

func TestAdminApp_LessonImportNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminLessonImport{})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "lesson import after navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Import") {
		t.Error("lesson import view missing 'Import'")
	}
}

func TestAdminApp_AddStudentNavigate(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminAddStudent{})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "add student after navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Student") {
		t.Error("add student view missing 'Student'")
	}
}

func TestAdminApp_DashboardBackFromStudents(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(NavigateToAdminStudents{})
	app2 := model.(*AdminApp)

	model2, _ := app2.Update(NavigateToAdminDashboard{})
	app3 := model2.(*AdminApp)

	view := app3.View()
	assertViewNonEmpty(t, "dashboard after back navigation", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Admin") {
		t.Error("dashboard view after back navigation missing 'Admin' content")
	}
}

func TestAdminApp_StudentToggle(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)
	_ = database.CreateUser("eve", "pw", db.RoleStudent)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(adminStudentToggleMsg{username: "eve", enabled: false})
	app2 := model.(*AdminApp)

	view := app2.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "eve") {
		t.Error("student list after toggle should still contain 'eve'")
	}

	user, err := database.GetUser("eve")
	if err != nil {
		t.Fatalf("GetUser after toggle: %v", err)
	}
	if user.Enabled {
		t.Error("eve should be disabled after toggle")
	}
}

func TestAdminApp_PasswordResetDoneMsg(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(adminPasswordResetDoneMsg{})
	app2 := model.(*AdminApp)

	view := app2.View()
	assertViewNonEmpty(t, "student list after password reset done", view)
}

func TestAdminApp_ChangeLangMsg(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(changeLangMsg{lang: "en"})
	_ = model

	if app.uiLang != "en" {
		t.Errorf("uiLang = %q, want en", app.uiLang)
	}
}

func TestAdminApp_WindowSizeMsg(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app2 := model.(*AdminApp)

	if app2.Width != 100 {
		t.Errorf("width = %d, want 100", app2.Width)
	}
	if app2.Height != 30 {
		t.Errorf("height = %d, want 30", app2.Height)
	}
}

func TestAdminApp_WindowSizeMsg_Zero(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	model, _ := app.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	app2 := model.(*AdminApp)

	if app2.Width != 80 {
		t.Errorf("width = %d, want 80 (unchanged)", app2.Width)
	}
}

func TestAdminApp_NavigateToStudentView(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	_, cmd := app.Update(navigateToStudentView{})

	if cmd == nil {
		t.Error("expected tea.Quit cmd for navigateToStudentView")
	}
	if !app.LaunchStudent {
		t.Error("expected LaunchStudent to be true")
	}
}

func TestAdminApp_LaunchStudentUsername(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	if app.LaunchStudentUsername() != "admin" {
		t.Errorf("LaunchStudentUsername = %q, want admin", app.LaunchStudentUsername())
	}
}

func TestAdminApp_WantsRelaunch(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("admin", "pw", db.RoleAdmin)

	lessonsDir := t.TempDir()
	coursesDir := t.TempDir()
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")

	app := NewAdminApp("admin", database, nil, lessonsDir, coursesDir, achFile, 80, 24)

	if app.WantsRelaunch() {
		t.Error("WantsRelaunch should be false initially")
	}

	app.LaunchStudent = true
	if !app.WantsRelaunch() {
		t.Error("WantsRelaunch should be true after setting LaunchStudent")
	}
}

// ── Dashboard keyboard shortcut tests ────────────────────────────────────────

func TestAdminDashboard_KeySendsNavigateToStudents(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)
	loc := testLocale()

	m := newAdminDashboard(database, nil, loc, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for 's' key")
	}

	msg := cmd()
	if _, ok := msg.(NavigateToAdminStudents); !ok {
		t.Errorf("expected NavigateToAdminStudents, got %T", msg)
	}
}

func TestAdminDashboard_KeySendsQuit(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)
	loc := testLocale()

	m := newAdminDashboard(database, nil, loc, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for 'q' key")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestAdminDashboard_ShortcutKeys(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)
	loc := testLocale()

	tests := []struct {
		key     rune
		wantMsg interface{}
	}{
		{'1', NavigateToAdminStudents{}},
		{'s', NavigateToAdminStudents{}},
		{'2', NavigateToAdminAddStudent{}},
		{'a', NavigateToAdminAddStudent{}},
		{'3', navigateToStudentView{}},
		{'v', navigateToStudentView{}},
		{'4', NavigateToAdminCourses{}},
		{'c', NavigateToAdminCourses{}},
		{'8', NavigateToAdminAchievements{}},
		{'g', NavigateToAdminAchievements{}},
		{'9', NavigateToAdminExport{}},
		{'x', NavigateToAdminExport{}},
	}

	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			m := newAdminDashboard(database, nil, loc, 80, 24)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
			if cmd == nil {
				t.Fatalf("expected non-nil cmd for key %q", string(tt.key))
			}
			msg := cmd()
			// Compare types by checking type assertion.
			switch tt.wantMsg.(type) {
			case NavigateToAdminStudents:
				if _, ok := msg.(NavigateToAdminStudents); !ok {
					t.Errorf("expected NavigateToAdminStudents, got %T", msg)
				}
			case NavigateToAdminAddStudent:
				if _, ok := msg.(NavigateToAdminAddStudent); !ok {
					t.Errorf("expected NavigateToAdminAddStudent, got %T", msg)
				}
			case navigateToStudentView:
				if _, ok := msg.(navigateToStudentView); !ok {
					t.Errorf("expected navigateToStudentView, got %T", msg)
				}
			case NavigateToAdminCourses:
				if _, ok := msg.(NavigateToAdminCourses); !ok {
					t.Errorf("expected NavigateToAdminCourses, got %T", msg)
				}
			case NavigateToAdminAchievements:
				if _, ok := msg.(NavigateToAdminAchievements); !ok {
					t.Errorf("expected NavigateToAdminAchievements, got %T", msg)
				}
			case NavigateToAdminExport:
				if _, ok := msg.(NavigateToAdminExport); !ok {
					t.Errorf("expected NavigateToAdminExport, got %T", msg)
				}
			}
		})
	}
}

// ── Student list key tests ───────────────────────────────────────────────────

func TestAdminStudentList_CursorKeys(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw1", db.RoleStudent)
	_ = database.CreateUser("bob", "pw2", db.RoleStudent)

	m := newAdminStudentList(database, nil, 80, 24)

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m2 := model.(*adminStudentListModel)
	if m2.offset != 1 {
		t.Errorf("after 'j': offset = %d, want 1", m2.offset)
	}

	model2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m3 := model2.(*adminStudentListModel)
	if m3.offset != 0 {
		t.Errorf("after 'k': offset = %d, want 0", m3.offset)
	}
}

func TestAdminStudentList_EnterNavigatesToDetail(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw1", db.RoleStudent)

	m := newAdminStudentList(database, nil, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd for enter key")
	}
	msg := cmd()
	detailMsg, ok := msg.(NavigateToAdminStudentDetail)
	if !ok {
		t.Errorf("expected NavigateToAdminStudentDetail, got %T", msg)
	}
	if detailMsg.Username != "alice" {
		t.Errorf("username = %q, want alice", detailMsg.Username)
	}
}

func TestAdminStudentList_BackKeys(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw1", db.RoleStudent)

	for _, key := range []rune{'b', 'q'} {
		t.Run(string(key), func(t *testing.T) {
			m := newAdminStudentList(database, nil, 80, 24)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
			if cmd == nil {
				t.Fatal("expected cmd for back key")
			}
			msg := cmd()
			if _, ok := msg.(NavigateToAdminDashboard); !ok {
				t.Errorf("expected NavigateToAdminDashboard, got %T", msg)
			}
		})
	}
}

// ── Course list key tests ────────────────────────────────────────────────────

func TestAdminCourseList_BackKey(t *testing.T) {
	database := testDB(t)

	m := newAdminCourseList(database, "", "en", 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil {
		t.Fatal("expected cmd for 'b' key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateToAdminDashboard); !ok {
		t.Errorf("expected NavigateToAdminDashboard, got %T", msg)
	}
}

// ── Achievements mode tests ──────────────────────────────────────────────────

func TestAdminAchievements_CreateMode(t *testing.T) {
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")
	m := newAdminAchievements(achFile, 80, 24)

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m2 := model.(*adminAchievementsModel)
	if m2.mode != achCreate {
		t.Error("expected achCreate mode after pressing 'n'")
	}

	model2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := model2.(*adminAchievementsModel)
	if m3.mode != achList {
		t.Error("expected achList mode after pressing esc")
	}
}

func TestAdminAchievements_BackKey(t *testing.T) {
	achFile := filepath.Join(t.TempDir(), "achievements.yaml")
	m := newAdminAchievements(achFile, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected cmd for 'q' key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateToAdminDashboard); !ok {
		t.Errorf("expected NavigateToAdminDashboard, got %T", msg)
	}
}

// ── Export key tests ─────────────────────────────────────────────────────────

func TestAdminExport_BackKey(t *testing.T) {
	database := testDB(t)
	m := newAdminExport(database, nil, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected cmd for 'q' key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateToAdminDashboard); !ok {
		t.Errorf("expected NavigateToAdminDashboard, got %T", msg)
	}
}

// ── Lesson import key tests ──────────────────────────────────────────────────

func TestAdminLessonImport_QuitKey(t *testing.T) {
	m := newAdminLessonImport(t.TempDir(), 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected cmd for 'q' key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateToAdminDashboard); !ok {
		t.Errorf("expected NavigateToAdminDashboard, got %T", msg)
	}
}

// ── Student detail key tests ─────────────────────────────────────────────────

func TestAdminStudentDetail_BackKey(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminStudentDetail("alice", database, "", "en", 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil {
		t.Fatal("expected cmd for 'b' key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateToAdminStudents); !ok {
		t.Errorf("expected NavigateToAdminStudents, got %T", msg)
	}
}

// ── Password reset model tests ───────────────────────────────────────────────

func TestAdminPasswordReset_View(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminPasswordReset("alice", database, nil, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "password reset", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "alice") {
		t.Error("password reset view missing username 'alice'")
	}
}

func TestAdminPasswordReset_DoneKey(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminPasswordReset("alice", database, nil, 80, 24)
	m.done = true

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil {
		t.Fatal("expected cmd when done and any key pressed")
	}
	msg := cmd()
	if _, ok := msg.(adminPasswordResetDoneMsg); !ok {
		t.Errorf("expected adminPasswordResetDoneMsg, got %T", msg)
	}
}

func TestAdminPasswordReset_GeneratePassword(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminPasswordReset("alice", database, nil, 80, 24)

	// Press enter with empty input (generates a random password).
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := model.(*adminPasswordResetModel)

	if !m2.done {
		t.Error("expected done=true after resetting password")
	}
	if m2.msg == "" {
		t.Error("expected success message after password reset")
	}
}

// ── Lesson picker view test ─────────────────────────────────────────────────

func TestAdminLessonPicker_View(t *testing.T) {
	lessonsDir := t.TempDir()
	m := newAdminLessonPicker(lessonsDir, 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "lesson picker", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Edit") {
		t.Error("lesson picker view missing 'Edit' content")
	}
}

// ── Lesson target picker view test ───────────────────────────────────────────

func TestAdminLessonTargetPicker_View(t *testing.T) {
	targets := []lessonTarget{
		{Label: "Test Section", Dir: "/tmp/test"},
	}
	m := newAdminLessonTargetPicker(targets, "create", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "lesson target picker", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Create Lesson") {
		t.Error("target picker view missing 'Create Lesson'")
	}
}

func TestAdminLessonTargetPicker_Empty(t *testing.T) {
	m := newAdminLessonTargetPicker(nil, "create", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "empty target picker", view)
}

// ── Student detail grant overlay ─────────────────────────────────────────────

func TestAdminStudentDetail_GrantOverlay(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminStudentDetail("alice", database, "", "en", 80, 24)
	m.grantMode = true

	view := m.View()
	assertViewNonEmpty(t, "grant overlay", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Grant") {
		t.Error("grant overlay missing 'Grant' content")
	}
}

// ── Student detail enrollment overlay ────────────────────────────────────────

func TestAdminStudentDetail_EnrollmentOverlay(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminStudentDetail("alice", database, "", "en", 80, 24)
	m.enrollMode = true

	view := m.View()
	assertViewNonEmpty(t, "enrollment overlay", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Enrollments") {
		t.Error("enrollment overlay missing 'Enrollments' content")
	}
}

// ── Lesson create viewSave test ──────────────────────────────────────────────

func TestAdminLessonCreate_ViewSave(t *testing.T) {
	m := newAdminLessonCreate(t.TempDir(), 80, 24)
	m.step = stepSave
	m.idInput.SetValue("test-lesson")
	m.titleInput.SetValue("Test Lesson")

	view := m.View()
	assertViewNonEmpty(t, "lesson create save", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "Save") {
		t.Error("lesson create save view missing 'Save'")
	}
}

// ── adminCourseDetail model tests ────────────────────────────────────────────

func TestAdminCourseDetail_View(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("alice", "pw", db.RoleStudent)

	m := newAdminCourseDetail(database, "c-intro", "C Intro", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "course detail", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "C Intro") {
		t.Error("course detail view missing 'C Intro'")
	}
	if !strings.Contains(plain, "alice") {
		t.Error("course detail view missing user 'alice'")
	}
}

// ── adminStudentDetail with admin role ───────────────────────────────────────

func TestAdminStudentDetail_View_AdminRole(t *testing.T) {
	database := testDB(t)
	_ = database.CreateUser("superadmin", "pw", db.RoleAdmin)

	m := newAdminStudentDetail("superadmin", database, "", "en", 80, 24)
	view := m.View()

	assertViewNonEmpty(t, "admin user detail", view)
	plain := stripANSI(view)
	if !strings.Contains(plain, "superadmin") {
		t.Error("admin detail view missing username")
	}
	if !strings.Contains(plain, "admin") {
		t.Error("admin detail view should show admin role badge")
	}
}

// ── Build lesson targets test ────────────────────────────────────────────────

func TestBuildLessonTargets_Empty(t *testing.T) {
	coursesDir := filepath.Join(t.TempDir(), "nocourses")
	lessonsDir := filepath.Join(t.TempDir(), "nolessons")
	targets := buildLessonTargets(coursesDir, lessonsDir)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for empty dirs, got %d", len(targets))
	}
}
