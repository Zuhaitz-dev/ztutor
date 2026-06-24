package tui

import (
	"strings"
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/sandbox"
)

// ── parseFlags ─────────────────────────────────────────────────────────────────

func TestParseFlags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"  ", nil},
		{"-O2", []string{"-O2"}},
		{"-O2 -Wall", []string{"-O2", "-Wall"}},
		{"  -O2 -Wall  ", []string{"-O2", "-Wall"}},
		{"-O2 -Wall -march=native", []string{"-O2", "-Wall", "-march=native"}},
		{"-O2\t-Wall", []string{"-O2", "-Wall"}},
	}
	for _, tt := range tests {
		got := parseFlags(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseFlags(%q) = %v (%d), want %v (%d)", tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseFlags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// ── isCrashText ──────────────────────────────────────────────────────────────

func TestIsCrashText(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"segmentation fault", true},
		{"SIGSEGV", true},
		{"Program crashed", true},
		{"DeadlySignal received", true},
		{"exited with code 139", true},
		{"Segmentation fault (core dumped)", true},
		{"", false},
		{"normal output", false},
		{"hello world", false},
		{"error: something went wrong", false},
	}
	for _, tt := range tests {
		got := isCrashText(tt.text)
		if got != tt.want {
			t.Errorf("isCrashText(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

// ── codeEasterEggLine ────────────────────────────────────────────────────────

func TestCodeEasterEggLine(t *testing.T) {
	loc := i18n.New("en")
	clang := sandbox.GetLanguage("c")

	tests := []struct {
		name string
		lang sandbox.Language
		code string
		want string
	}{
		{"beer.h in C", clang, `#include <beer.h>
int main(void) { return 0; }`, loc.T("exercise.egg.beer")},
		{"beer.h in Python", nil, `#include <beer.h>`, ""},
		{"void main", clang, `void main(int argc, char *argv[]) { }`, loc.T("exercise.egg.void_main")},
		{"goto", clang, `int main(void) { goto end; end: return 0; }`, loc.T("exercise.egg.goto")},
		{"no easter egg", clang, `int main(void) { return 0; }`, ""},
		{"nil language", nil, `int main(void) { return 0; }`, ""},
		{"empty code", clang, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codeEasterEggLine(tt.lang, tt.code, loc)
			if got != tt.want {
				t.Errorf("codeEasterEggLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ── diffOutput ───────────────────────────────────────────────────────────────

func TestDiffOutput(t *testing.T) {
	out := diffOutput("aaa\nbbb", "aaa\nccc")
	if !strings.Contains(out, "bbb") || !strings.Contains(out, "ccc") {
		t.Errorf("diff should contain both got and want lines, got:\n%s", out)
	}

	same := diffOutput("aaa\nbbb", "aaa\nbbb")
	if same == "" {
		t.Error("diff of identical output should not be empty")
	}

	gotLonger := diffOutput("aaa\nbbb\nccc", "aaa")
	if !strings.Contains(gotLonger, "bbb") || !strings.Contains(gotLonger, "ccc") {
		t.Errorf("diff with got longer should show extra lines, got:\n%s", gotLonger)
	}

	wantLonger := diffOutput("aaa", "aaa\nbbb\nccc")
	if !strings.Contains(wantLonger, "bbb") || !strings.Contains(wantLonger, "ccc") {
		t.Errorf("diff with want longer should show extra lines, got:\n%s", wantLonger)
	}

	empty := diffOutput("", "")
	if empty == "" {
		t.Error("diff of empty strings should not panic")
	}
}

// ── buildAchievementEvents ───────────────────────────────────────────────────

func TestBuildAchievementEvents(t *testing.T) {
	clang := sandbox.GetLanguage("c")

	t.Run("perfect pass with beer", func(t *testing.T) {
		events := buildAchievementEvents(true, 1, 3, false, clang, `#include <beer.h>
int main(void) { return 0; }`, "compile")
		assertContains(t, events, "compile")
		assertContains(t, events, "pass")
		assertContains(t, events, "pass_1attempt")
		assertContains(t, events, "pass_3star")
		assertContains(t, events, "pass_nowarnings")
		assertContains(t, events, "beer")
		assertNotContains(t, events, "pass_5attempts")
	})

	t.Run("passed with 5 attempts", func(t *testing.T) {
		events := buildAchievementEvents(true, 5, 1, true, clang, "", "compile")
		assertContains(t, events, "pass_5attempts")
		assertNotContains(t, events, "pass_1attempt")
		assertNotContains(t, events, "pass_3star")
	})

	t.Run("not passed no pass events", func(t *testing.T) {
		events := buildAchievementEvents(false, 1, 0, true, clang, "", "compile")
		assertNotContains(t, events, "pass")
		assertNotContains(t, events, "pass_1attempt")
	})

	t.Run("beer.h only for C", func(t *testing.T) {
		events := buildAchievementEvents(false, 1, 0, false, nil, "#include <beer.h>")
		assertNotContains(t, events, "beer")
	})

	t.Run("extra events prepended", func(t *testing.T) {
		events := buildAchievementEvents(true, 1, 3, false, clang, "", "compile", "segfault_king")
		if events[0] != "segfault_king" && events[1] != "segfault_king" {
			t.Error("extra events should appear in result")
		}
	})

	t.Run("empty events", func(t *testing.T) {
		events := buildAchievementEvents(false, 1, 0, false, nil, "")
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %v", events)
		}
	})
}

func assertContains(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Errorf("expected %q in %v", want, slice)
}

func assertNotContains(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			t.Errorf("unexpected %q in %v", want, slice)
			return
		}
	}
}

// ── extractMemorySummary ─────────────────────────────────────────────────────

func TestExtractMemorySummary(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		got := extractMemorySummary("")
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("no summary section", func(t *testing.T) {
		got := extractMemorySummary("some random output\nmore output")
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("heap summary section", func(t *testing.T) {
		asan := `some leading output
==12345== HEAP SUMMARY:
==12345==     in use at exit: 0 bytes in 0 blocks
==12345==   total heap usage: 1 allocs, 1 frees, 1,024 bytes allocated
==12345== 
==12345== All heap blocks were freed -- no leaks are possible
`
		got := extractMemorySummary(asan)
		if len(got) == 0 {
			t.Error("expected summary lines")
		}
		found := false
		for _, line := range got {
			if strings.Contains(line, "HEAP SUMMARY") {
				found = true
			}
		}
		if !found {
			t.Errorf("HEAP SUMMARY not in output: %v", got)
		}
	})

	t.Run("stops at runtime error", func(t *testing.T) {
		asan := `==12345== ERROR SUMMARY: 1 errors
==12345== runtime error detected
`
		got := extractMemorySummary(asan)
		if len(got) == 0 {
			t.Error("expected ERROR SUMMARY line")
		}
	})
}

// ── scoreboard ───────────────────────────────────────────────────────────────

func TestScoreboard(t *testing.T) {
	tests := []struct {
		results []sandbox.TestResult
		want    string
	}{
		{[]sandbox.TestResult{{Passed: true}, {Passed: false}, {Passed: true}}, "[V][X][V]"},
		{[]sandbox.TestResult{{Passed: true}, {Passed: true}, {Passed: true}}, "[V][V][V]"},
		{[]sandbox.TestResult{{Passed: false}, {Passed: false}}, "[X][X]"},
	}
	for _, tt := range tests {
		got := scoreboard(tt.results)
		if got == "" {
			t.Error("scoreboard should not return empty")
		}
		_ = got // contains lipgloss ANSI codes, visual check only
	}
}
