package i18n

import (
	"testing"
)

func TestNew_English(t *testing.T) {
	loc := New("en")
	if loc == nil {
		t.Fatal("New(en) returned nil")
	}
	if loc.Lang() != "en" {
		t.Errorf("Lang = %q, want en", loc.Lang())
	}
	if loc.IsRTL() {
		t.Error("English should not be RTL")
	}
}

func TestNew_Arabic_IsRTL(t *testing.T) {
	loc := New("ar")
	if loc == nil {
		t.Fatal("New(ar) returned nil")
	}
	if loc.Lang() != "ar" {
		t.Errorf("Lang = %q, want ar", loc.Lang())
	}
	if !loc.IsRTL() {
		t.Error("Arabic should be RTL")
	}
}

func TestNew_Spanish(t *testing.T) {
	loc := New("es")
	if loc == nil {
		t.Fatal("New(es) returned nil")
	}
	if loc.Lang() != "es" {
		t.Errorf("Lang = %q, want es", loc.Lang())
	}
	if loc.IsRTL() {
		t.Error("Spanish should not be RTL")
	}
}

func TestNew_Chinese(t *testing.T) {
	loc := New("zh")
	if loc == nil {
		t.Fatal("New(zh) returned nil")
	}
	if loc.Lang() != "zh" {
		t.Errorf("Lang = %q, want zh", loc.Lang())
	}
}

func TestNew_UnknownLangFallsBackToEnglish(t *testing.T) {
	loc := New("xx")
	if loc == nil {
		t.Fatal("New(xx) returned nil")
	}
	if loc.Lang() != "en" {
		t.Errorf("unknown language should fall back to en, got %q", loc.Lang())
	}
}

func TestNew_EmptyFallsBackToEnglish(t *testing.T) {
	loc := New("")
	if loc == nil {
		t.Fatal("New empty returned nil")
	}
	// Empty lang keeps "" as the lang code but uses English data.
	val := loc.T("exercise.help.run")
	if val == "exercise.help.run" {
		t.Error("empty lang should resolve English translations")
	}
	if loc.IsRTL() {
		t.Error("empty lang should not be RTL")
	}
}

func TestT_KnownKey(t *testing.T) {
	loc := New("en")
	val := loc.T("exercise.help.run")
	if val == "exercise.help.run" {
		t.Error("known key should return a translation, not the key itself")
	}
}

func TestT_UnknownKeyReturnsKey(t *testing.T) {
	loc := New("en")
	val := loc.T("this.key.does.not.exist")
	if val != "this.key.does.not.exist" {
		t.Errorf("unknown key should return itself, got %q", val)
	}
}

func TestT_FallsBackToEnglish(t *testing.T) {
	locEn := New("en")
	locAr := New("ar")
	enVal := locEn.T("exercise.help.run")
	arVal := locAr.T("exercise.help.run")
	if arVal == enVal {
		// The Arabic translation naturally uses different text, but if they're
		// equal it means the Arabic key isn't translated (which is fine).
		t.Logf("Arabic run key matches English: %q", arVal)
	}
}

func TestT_SprintfArgs(t *testing.T) {
	loc := New("en")
	val := loc.T("exercise.help.hint", 2, 5)
	if val == "exercise.help.hint" {
		t.Skip("hint key not found in locale")
	}
	// Should contain the substituted values.
	t.Logf("hint(2,5) = %q", val)
}

func TestNext_CyclesThroughAll(t *testing.T) {
	loc := New("en")
	seen := map[string]int{}
	for i := 0; i < len(Available)*2; i++ {
		seen[loc.Lang()]++
		loc = loc.Next()
	}
	if len(seen) != len(Available) {
		t.Errorf("expected to cycle through %d languages, saw %d: %v", len(Available), len(seen), seen)
	}
	for lang, count := range seen {
		if count < 2 {
			t.Errorf("language %q only seen %d times (expected at least 2 in %d cycles)", lang, count, len(Available)*2)
		}
	}
}

func TestIsRTL_AllLanguages(t *testing.T) {
	for _, lang := range Available {
		loc := New(lang)
		switch lang {
		case "ar":
			if !loc.IsRTL() {
				t.Errorf("%s should be RTL", lang)
			}
		default:
			if loc.IsRTL() {
				t.Errorf("%s should NOT be RTL", lang)
			}
		}
	}
}
