package tui

import (
	"strings"
	"testing"

	"ztutor/internal/i18n"
)

func TestStripANSI_RemovesEscapeSequences(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\x1b[1mhello\x1b[0m", "hello"},
		{"\x1b[31mERROR\x1b[0m: store to null", "ERROR: store to null"},
		{"no escapes", "no escapes"},
		{"", ""},
		{"\x1b[1m\x1b[31m#0 0xdeadbeef in main\x1b[0m\x1b[0m", "#0 0xdeadbeef in main"},
	}
	for _, c := range cases {
		got := stripANSI(c.in)
		if got != c.want {
			t.Errorf("stripANSI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHelpBar_SplitsOnFirstSpace(t *testing.T) {
	out := helpBar("^S compile", "^Q back")
	if strings.TrimSpace(out) == "" {
		t.Fatal("helpBar returned empty string")
	}
	if !strings.Contains(out, "^S") {
		t.Error("expected key ^S to appear in helpBar output")
	}
	if !strings.Contains(out, "^Q") {
		t.Error("expected key ^Q to appear in helpBar output")
	}
}

// An item with no space at index > 0 renders without a key pill — the whole
// string becomes dim description text. This is the Ctrl+B:files bug.
func TestHelpBar_ItemWithNoSpaceHasNoKeyPill(t *testing.T) {
	withColon := "Ctrl+B:files"
	withSpace := "^B files"
	if idx := strings.Index(withColon, " "); idx > 0 {
		t.Errorf("%q has a space at %d — would render a key pill (test premise is wrong)", withColon, idx)
	}
	if idx := strings.Index(withSpace, " "); idx <= 0 {
		t.Errorf("%q has no space — would NOT render a key pill", withSpace)
	}
}

// helpBarKeys is the authoritative list of locale keys whose values are fed to
// helpBar(). Every value must have a space at index > 0 so helpBar can extract
// the key portion and render it as a pill. Add new keys here when you add them
// to a helpBar call anywhere in the codebase.
var helpBarKeys = []string{
	"exercise.help.send",
	"exercise.help.kill",
	"exercise.help.back",
	"exercise.help.asm_resize",
	"exercise.help.asm_annotate",
	"exercise.help.asm_hide",
	"help.esc_back",
	"exercise.help.filelist.move",
	"exercise.help.filelist.select",
	"exercise.help.next",
	"exercise.help.run",
	"exercise.help.interactive",
	"exercise.help.asan",
	"exercise.help.gdb",
	"exercise.help.asm",
	"exercise.help.inputs",
	"exercise.help.output",
	"help.mochi",
	"exercise.help.trivia",
	"exercise.help.filelist",
	"exercise.help.output_resize",
	"lesson.help.scroll",
	"lesson.help.top_end",
}

// TestHelpBar_AllLocaleKeysHaveSpace checks every locale (en, es, zh, ar) for
// every key that feeds into helpBar and asserts the value contains a space at
// index > 0. A missing space means the key pill is never rendered — only dim
// description text appears, as happened with "Ctrl+B:files".
func TestHelpBar_AllLocaleKeysHaveSpace(t *testing.T) {
	for _, lang := range i18n.Available {
		loc := i18n.New(lang)
		for _, key := range helpBarKeys {
			val := loc.T(key)
			if val == key {
				t.Errorf("lang=%s key=%s: key not found in any locale", lang, key)
				continue
			}
			idx := strings.Index(val, " ")
			if idx <= 0 {
				t.Errorf("lang=%s key=%s value=%q: no space at index > 0 — helpBar will not render a key pill", lang, key, val)
			}
		}
	}
}
