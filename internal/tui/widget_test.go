package tui

import (
	"strings"
	"testing"
	"time"

	"ztutor/internal/i18n"

	"github.com/charmbracelet/lipgloss"
)

// ── HintWidget ───────────────────────────────────────────────────────────────

func TestHintWidget_Empty(t *testing.T) {
	w := newHintWidget(nil, i18n.New("en"))
	if w.Available() {
		t.Error("empty hints should not be available")
	}
	if w.IsVisible() {
		t.Error("empty hints should not be visible")
	}
	w.Next()
	if w.IsVisible() {
		t.Error("Next on empty hints should not make visible")
	}
	if w.HintsUsed() != 0 {
		t.Error("HintsUsed on empty should be 0")
	}
}

func TestHintWidget_Cycle(t *testing.T) {
	w := newHintWidget([]string{"hint 1", "hint 2"}, i18n.New("en"))
	if !w.Available() {
		t.Error("should be available")
	}
	if w.CurrentIndex() != -1 {
		t.Error("initial index should be -1")
	}
	if w.HintsUsed() != 0 {
		t.Error("initial HintsUsed should be 0")
	}

	w.Next()
	if w.CurrentIndex() != 0 {
		t.Errorf("after first Next: index = %d, want 0", w.CurrentIndex())
	}
	if !w.IsVisible() {
		t.Error("should be visible after Next")
	}
	if w.HintsUsed() != 1 {
		t.Errorf("HintsUsed = %d, want 1", w.HintsUsed())
	}
	if !stringsContains(w.View(), "hint 1") {
		t.Error("View should contain first hint")
	}

	w.Next()
	if w.CurrentIndex() != 1 {
		t.Errorf("after second Next: index = %d, want 1", w.CurrentIndex())
	}
	if w.HintsUsed() != 2 {
		t.Errorf("HintsUsed = %d, want 2", w.HintsUsed())
	}

	w.Next()
	if w.CurrentIndex() != 0 {
		t.Errorf("after third Next: index = %d, want 0 (wrap)", w.CurrentIndex())
	}
	if w.HintsUsed() != 2 {
		t.Errorf("HintsUsed should stay at 2 after wrapping: got %d", w.HintsUsed())
	}
}

func TestHintWidget_Hide(t *testing.T) {
	w := newHintWidget([]string{"hint 1"}, i18n.New("en"))
	w.Next()
	if !w.IsVisible() {
		t.Fatal("should be visible after Next")
	}
	w.Hide()
	if w.IsVisible() {
		t.Error("should not be visible after Hide")
	}
}

// ── TriviaWidget ─────────────────────────────────────────────────────────────

func TestTriviaWidget_Empty(t *testing.T) {
	w := newTriviaWidget(nil)
	if w.Available() {
		t.Error("empty trivia should not be available")
	}
	if w.Current() != "" {
		t.Error("empty trivia should return empty current")
	}
	w.Next()
	if w.Current() != "" {
		t.Error("Next on empty trivia should still return empty")
	}
}

func TestTriviaWidget_Cycle(t *testing.T) {
	w := newTriviaWidget([]string{"fact 1", "fact 2"})
	if !w.Available() {
		t.Error("should be available")
	}
	if w.Current() != "fact 1" {
		t.Errorf("initial current = %q, want %q", w.Current(), "fact 1")
	}
	w.Next()
	if w.Current() != "fact 2" {
		t.Errorf("after Next: current = %q, want %q", w.Current(), "fact 2")
	}
	w.Next()
	if w.Current() != "fact 1" {
		t.Errorf("after second Next: current = %q, want %q (wrap)", w.Current(), "fact 1")
	}
}

// ── TimerWidget ──────────────────────────────────────────────────────────────

func TestTimerWidget_StartStop(t *testing.T) {
	w := newTimerWidget()
	if w.IsVisible() {
		t.Error("should not be visible before Start")
	}
	w.Start()
	if !w.IsVisible() {
		t.Error("should be visible after Start")
	}
	d := w.Current()
	if d <= 0 {
		t.Error("Current should be positive after Start")
	}
	w.Stop()
	d2 := w.Current()
	if d2 <= 0 {
		t.Error("Current should be positive after Stop")
	}
}

func TestTimerWidget_Reset(t *testing.T) {
	w := newTimerWidget()
	w.Start()
	time.Sleep(10 * time.Millisecond)
	w.Stop()
	d := w.Current()
	if d <= 0 {
		t.Fatal("expected elapsed time")
	}
	w.Reset()
	if w.IsVisible() {
		t.Error("should not be visible after Reset")
	}
}

func TestTimerWidget_Toggle(t *testing.T) {
	w := newTimerWidget()
	w.Toggle()
	if !w.IsVisible() {
		t.Error("Toggle from hidden should make visible")
	}
	w.Toggle()
	if w.IsVisible() {
		t.Error("second Toggle should hide")
	}
}

// ── ProgressWidget ───────────────────────────────────────────────────────────

func TestProgressWidget_Empty(t *testing.T) {
	w := newProgressWidget()
	if w.Available() {
		t.Error("should not be available without results")
	}
	if w.IsVisible() {
		t.Error("should not be visible initially")
	}
}

func TestProgressWidget_Render(t *testing.T) {
	w := newProgressWidget()
	w.SetResult(3, 1)
	if !w.Available() {
		t.Error("should be available after SetResult")
	}
	if !w.IsVisible() {
		t.Error("should be visible after SetResult")
	}
	view := w.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestProgressWidget_Hide(t *testing.T) {
	w := newProgressWidget()
	w.SetResult(3, 2)
	w.Hide()
	if w.IsVisible() {
		t.Error("should not be visible after Hide")
	}
}

// ── ReferenceWidget ──────────────────────────────────────────────────────────

func TestReferenceWidget_Empty(t *testing.T) {
	w := newReferenceWidget(nil, i18n.New("en"))
	if w.Available() {
		t.Error("empty refs should not be available")
	}
}

func TestReferenceWidget_Toggle(t *testing.T) {
	w := newReferenceWidget([]string{"https://example.com"}, i18n.New("en"))
	if !w.Available() {
		t.Error("should be available")
	}
	w.Toggle()
	if !w.IsVisible() {
		t.Error("should be visible after Toggle")
	}
	w.Toggle()
	if w.IsVisible() {
		t.Error("second Toggle should hide")
	}
}

func TestReferenceWidget_Hide(t *testing.T) {
	w := newReferenceWidget([]string{"man 3 printf"}, i18n.New("en"))
	w.Toggle()
	w.Hide()
	if w.IsVisible() {
		t.Error("should not be visible after Hide")
	}
}

// ── rtlAlignBlock ────────────────────────────────────────────────────────────

func TestRtlAlignBlock_English(t *testing.T) {
	result := rtlAlignBlock("hello\nworld", 40)
	if !stringsContains(result, "hello") || !stringsContains(result, "world") {
		t.Errorf("rtlAlignBlock should preserve content, got %q", result)
	}
}

func TestRtlAlignBlock_SingleLine(t *testing.T) {
	result := rtlAlignBlock("test", 20)
	height := lipgloss.Height(result)
	if height < 1 {
		t.Errorf("rtlAlignBlock of single line should have height >= 1, got %d", height)
	}
}

func TestRtlAlignBlock_Empty(t *testing.T) {
	result := rtlAlignBlock("", 40)
	// empty input may be right-padded with spaces or returned empty
	if len(result) > 0 && strings.TrimSpace(result) != "" {
		t.Errorf("rtlAlignBlock of empty string should be empty or spaces, got %q", result)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsSub(s, substr)
}

func containsSub(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ── extractStackTrace ────────────────────────────────────────────────────────

func TestExtractStackTrace_AsanFormat(t *testing.T) {
	raw := `==12345==ERROR: AddressSanitizer: heap-use-after-free
    #0 0x4a2b3f in add (main.c:5)
    #1 0x4a2b60 in main (main.c:10)
    #2 0x7f3c2d (unknown module)`
	frames := extractStackTrace(raw)
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d: %+v", len(frames), frames)
	}
	if frames[0].num != 0 || frames[0].funcName != "add" || frames[0].file != "main.c" || frames[0].line != 5 {
		t.Errorf("frame 0: %+v", frames[0])
	}
	if frames[1].num != 1 || frames[1].funcName != "main" || frames[1].file != "main.c" || frames[1].line != 10 {
		t.Errorf("frame 1: %+v", frames[1])
	}
}

func TestExtractStackTrace_Empty(t *testing.T) {
	frames := extractStackTrace("no stack trace here\njust normal output")
	if len(frames) != 0 {
		t.Errorf("expected 0 frames, got %d", len(frames))
	}
}

// Regression: ASAN emits multiple stack traces for some errors (e.g. the
// access-site trace followed by an allocation-site trace). The parser was
// collecting all #N lines across the entire output, producing duplicate frames.
func TestExtractStackTrace_StopsAtSecondTrace(t *testing.T) {
	raw := `==ERROR: AddressSanitizer: SEGV
    #0 0x400d8 in main (main.c:8)
    #1 0x7fab1 in __libc_start_call_main (/lib64/libc.so.6+0x3680)
Address 0x0 is a null pointer.
SUMMARY: AddressSanitizer: SEGV
Shadow bytes around the buggy address:
    #0 0x400e9 in main (main.c:9)
    #1 0x7fab2 in __libc_start_call_main (/lib64/libc.so.6+0x3681)`
	frames := extractStackTrace(raw)
	// Only the first stack trace should be parsed; the second starts at the
	// second #0 and must be ignored.
	for _, f := range frames {
		if f.address == "0x400e9" {
			t.Errorf("second stack trace frame leaked into results: %+v", f)
		}
	}
	if len(frames) == 0 {
		t.Error("expected at least one frame from the first stack trace")
	}
	if frames[0].num != 0 || frames[0].file != "main.c" {
		t.Errorf("first frame unexpected: %+v", frames[0])
	}
}

// Regression: frames from shared libraries include "(path+offset) (BuildId:...)"
// which caused TrimSuffix to leave ") (BuildId:..." as part of the file name.
func TestExtractStackTrace_StripsLibraryBuildId(t *testing.T) {
	raw := `    #0 0x400d8 in main (main.c:8)
    #1 0x7ffacb60a680 in __libc_start_call_main (/lib64/libc.so.6+0x3680) (BuildId: abc123)`
	frames := extractStackTrace(raw)
	if len(frames) < 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	f := frames[1]
	// file should be the library path without BuildId contamination
	if strings.Contains(f.file, "BuildId") || strings.Contains(f.file, ")") {
		t.Errorf("library frame file contains BuildId or paren: %q", f.file)
	}
}

// ── extractHeapStats ─────────────────────────────────────────────────────────

func TestExtractHeapStats_WithLeaks(t *testing.T) {
	raw := `==12345== HEAP SUMMARY:
==12345==     in use at exit: 100 bytes in 1 blocks
==12345==   total heap usage: 2 allocs, 1 frees, 200 bytes allocated
==12345== 100 bytes in 1 blocks are definitely lost
==12345== 0 bytes in 0 blocks are indirectly lost`
	h := extractHeapStats(raw)
	if h == nil {
		t.Fatal("expected heap stats")
	}
	if h.defLeaked != 100 {
		t.Errorf("defLeaked = %d, want 100", h.defLeaked)
	}
	if h.totalAllocs != 2 {
		t.Errorf("totalAllocs = %d, want 2", h.totalAllocs)
	}
	if h.totalBytes != 200 {
		t.Errorf("totalBytes = %d, want 200", h.totalBytes)
	}
}

func TestExtractHeapStats_Clean(t *testing.T) {
	raw := `==12345== HEAP SUMMARY:
==12345==     in use at exit: 0 bytes in 0 blocks
==12345==   total heap usage: 5 allocs, 5 frees, 500 bytes allocated
==12345== All heap blocks were freed -- no leaks are possible`
	h := extractHeapStats(raw)
	if h == nil {
		t.Fatal("expected heap stats")
	}
	if h.defLeaked != 0 {
		t.Errorf("defLeaked = %d, want 0", h.defLeaked)
	}
}

func TestExtractHeapStats_NoSection(t *testing.T) {
	h := extractHeapStats("no heap info here")
	if h != nil {
		t.Error("expected nil for no heap section")
	}
}

// ── StructInspectorWidget.parseFromCode ──────────────────────────────────────

func TestStructInspector_ParseFromCode(t *testing.T) {
	code := `struct Point {
    int x;
    char *name;
};

int main(void) {
    return 0;
}`
	w := newStructInspectorWidget(i18n.New("en"))
	w.parseFromCode(code)
	if len(w.structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(w.structs))
	}
	s := w.structs[0]
	if s.name != "Point" {
		t.Errorf("struct name = %q, want Point", s.name)
	}
	if len(s.fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(s.fields))
	}
	if s.fields[0].typ != "int" || s.fields[0].name != "x" {
		t.Errorf("field 0: %s %s", s.fields[0].typ, s.fields[0].name)
	}
	if s.fields[1].typ != "char *" || s.fields[1].name != "name" {
		t.Errorf("field 1: %s %s", s.fields[1].typ, s.fields[1].name)
	}
}

func TestStructInspector_NoStructs(t *testing.T) {
	w := newStructInspectorWidget(i18n.New("en"))
	w.parseFromCode("int main(void) { return 0; }")
	if len(w.structs) != 0 {
		t.Errorf("expected 0 structs, got %d", len(w.structs))
	}
}

func TestStructInspector_MultipleStructs(t *testing.T) {
	code := `struct A { int x; };
struct B { char *s; double d; };`
	w := newStructInspectorWidget(i18n.New("en"))
	w.parseFromCode(code)
	if len(w.structs) != 2 {
		t.Fatalf("expected 2 structs, got %d", len(w.structs))
	}
}
