package tui

import (
	"strings"
	"testing"
	"time"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
)

func TestChallengeScreen_View(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:      "test-challenge",
		Title:   "Test Challenge",
		Content: "# Welcome\nWrite some code.",
		Exercise: `#include <stdio.h>
int main(void) {
	printf("42\n");
	return 0;
}`,
		Difficulty: "beginner",
		Points:     100,
		Tests: []lesson.TestCase{
			{Stdin: "", Expected: "42"},
		},
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	view := cs.View()
	if view == "" {
		t.Error("View() should not return empty")
	}
	if !strings.Contains(view, "Test Challenge") {
		t.Error("View should contain challenge title")
	}
	if !strings.Contains(view, "100") {
		t.Error("View should contain points")
	}
}

func TestChallengeScreen_View_RTL(t *testing.T) {
	loc := i18n.New("ar")
	ch := lesson.Challenge{
		ID:       "rtl-challenge",
		Title:    "RTL Test",
		Exercise: "// code here",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	view := cs.View()
	if view == "" {
		t.Error("View() should not return empty for RTL")
	}
}

func TestChallengeScreen_Submit(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "submit-test",
		Title:    "Submit Test",
		Exercise: `int main(void) { return 0; }`,
		Tests: []lesson.TestCase{
			{Stdin: "", Expected: ""},
		},
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	if cs.compiling {
		t.Error("compiling should start false")
	}
	if cs.submitted {
		t.Error("submitted should start false")
	}

	model, cmd := cs.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	cs = model.(*ChallengeScreen)
	if !cs.compiling {
		t.Error("compiling should be true after KeyRun")
	}
	if cmd == nil {
		t.Error("expected non-nil submit command after KeyRun")
	}
}

func TestChallengeScreen_Submit_BackAfterSubmit(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "back-test",
		Title:    "Back Test",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)
	cs.submitted = true

	model, cmd := cs.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model.(*ChallengeScreen)
	if cmd == nil {
		t.Error("expected NavigateToMenu command after submit+enter")
	}
}

func TestChallengeScreen_BackFromEditor(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "back-editor-test",
		Title:    "Back Test",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	_, cmd := cs.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if cmd == nil {
		t.Error("expected NavigateToMenu command after ctrl+q")
	}
}

func TestChallengeScreen_MochiToggle(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "mochi-test",
		Title:    "Mochi Test",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	if cs.mascotHidden {
		t.Error("mascot should be visible by default")
	}

	model, _ := cs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	cs = model.(*ChallengeScreen)
	if !cs.mascotHidden {
		t.Error("mascot should be hidden after mochi toggle")
	}

	model, _ = cs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	cs = model.(*ChallengeScreen)
	if cs.mascotHidden {
		t.Error("mascot should be visible after second mochi toggle")
	}
}

func TestChallengeScreen_TimeWindow(t *testing.T) {
	loc := i18n.New("en")

	future := time.Now().Add(2 * time.Hour)
	past := time.Now().Add(-1 * time.Hour)
	futureEnd := time.Now().Add(1 * time.Hour)
	pastEnd := time.Now().Add(-30 * time.Minute)

	tests := []struct {
		name     string
		startsAt time.Time
		endsAt   time.Time
		wantText string
	}{
		{
			name:     "opens in future",
			startsAt: future,
			endsAt:   time.Time{},
			wantText: "opens",
		},
		{
			name:     "active no end",
			startsAt: past,
			endsAt:   time.Time{},
			wantText: "",
		},
		{
			name:     "closes in future",
			startsAt: past,
			endsAt:   futureEnd,
			wantText: "closes",
		},
		{
			name:     "already closed",
			startsAt: past,
			endsAt:   pastEnd,
			wantText: "closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := lesson.Challenge{
				ID:       tt.name,
				Title:    tt.name,
				Exercise: "// code",
				StartsAt: tt.startsAt,
				EndsAt:   tt.endsAt,
			}

			lang := sandbox.GetLanguage("c")
			cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

			view := cs.View()
			if tt.wantText != "" && !strings.Contains(strings.ToLower(view), strings.ToLower(tt.wantText)) {
				t.Errorf("View for %q should contain %q, got:\n%s", tt.name, tt.wantText, view)
			}
		})
	}
}

func TestChallengeScreen_WindowSizeMsg(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "resize-test",
		Title:    "Resize Test",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	model, _ := cs.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	cs = model.(*ChallengeScreen)
	if cs.Width != 120 {
		t.Errorf("width = %d, want 120", cs.Width)
	}
	if cs.Height != 60 {
		t.Errorf("height = %d, want 60", cs.Height)
	}
}

func TestChallengeScreen_SetLocale(t *testing.T) {
	ch := lesson.Challenge{
		ID:       "locale-test",
		Title:    "Locale Test",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, nil)

	if cs.loc == nil {
		t.Fatal("locale should not be nil after construction")
	}

	esLoc := i18n.New("es")
	cs.SetLocale(esLoc)
	if cs.loc.Lang() != "es" {
		t.Errorf("locale lang = %q, want 'es'", cs.loc.Lang())
	}
	if cs.companionLine == "" {
		t.Error("companionLine should not be empty after SetLocale")
	}
}

func TestChallengeScreen_SetMascotFrame(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "frame-test",
		Title:    "Frame Test",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	cs.SetMascotFrame(2)
	if cs.mascotFrame != 2 {
		t.Errorf("mascotFrame = %d, want 2", cs.mascotFrame)
	}
}

func TestChallengeScoreboard(t *testing.T) {
	results := []sandbox.TestResult{
		{Num: 1, Passed: true},
		{Num: 2, Passed: false},
		{Num: 3, Passed: true},
	}
	out := challengeScoreboard(results)
	if out == "" {
		t.Error("scoreboard should not be empty")
	}
	if !strings.Contains(out, "✓") || !strings.Contains(out, "✗") {
		t.Error("scoreboard should contain ✓ and ✗")
	}
}

func TestFmtDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{59 * time.Minute, "59m"},
		{1 * time.Hour, "1h0m"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		got := fmtDuration(tt.d)
		if got != tt.want {
			t.Errorf("fmtDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestChallengeScreen_ChallengeTestResultMsg_AllPass(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "msg-test",
		Title:    "Message Test",
		Exercise: "// code",
		Tests: []lesson.TestCase{
			{Stdin: "", Expected: "42"},
			{Stdin: "", Expected: "43"},
		},
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	results := []sandbox.TestResult{
		{Num: 1, Passed: true, Got: "42", Want: "42"},
		{Num: 2, Passed: true, Got: "43", Want: "43"},
	}
	msg := challengeTestResultMsg{
		compileResult: &sandbox.Result{},
		testResults:   results,
	}

	model, _ := cs.Update(msg)
	cs = model.(*ChallengeScreen)

	if !cs.submitted {
		t.Error("submitted should be true after result msg")
	}
	if !cs.passed {
		t.Error("passed should be true when all tests pass")
	}
	if cs.output == "" {
		t.Error("output should not be empty")
	}
}

func TestChallengeScreen_ChallengeTestResultMsg_CompileError(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "compile-err-test",
		Title:    "Compile Error",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	msg := challengeTestResultMsg{
		compileResult: &sandbox.Result{Error: "syntax error"},
	}

	model, _ := cs.Update(msg)
	cs = model.(*ChallengeScreen)

	if cs.compiling {
		t.Error("compiling should be false after compile error")
	}
	if cs.output == "" {
		t.Error("output should show compile error")
	}
}

func TestChallengeScreen_ChallengeTestResultMsg_RuntimeError(t *testing.T) {
	loc := i18n.New("en")
	ch := lesson.Challenge{
		ID:       "runtime-err-test",
		Title:    "Runtime Error",
		Exercise: "// code",
	}

	lang := sandbox.GetLanguage("c")
	cs := NewChallengeScreen(ch, "test-course", lang, sandbox.NewSuccessExecutor(), 80, 40, loc)

	msg := challengeTestResultMsg{
		compileResult: &sandbox.Result{},
		testResults: []sandbox.TestResult{
			{Num: 1, Passed: false, Error: "segfault", ExitCode: 139},
		},
	}

	model, _ := cs.Update(msg)
	cs = model.(*ChallengeScreen)

	if !cs.submitted {
		t.Error("submitted should be true after result")
	}
	if cs.passed {
		t.Error("passed should be false when test fails")
	}
}
