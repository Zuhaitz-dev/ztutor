package logutil

import "testing"

func TestFmsg(t *testing.T) {
	if got := fmsg("plain message"); got != "plain message" {
		t.Errorf("fmsg plain = %q", got)
	}
	if got := fmsg("%s-%d", "x", 3); got != "x-3" {
		t.Errorf("fmsg formatted = %q, want x-3", got)
	}
}

func TestSanitizeLog(t *testing.T) {
	cases := map[string]string{
		"plain":         "plain",
		"line\nbreak":   "line\\nbreak",
		"tab\there":     "tab\\there",
		"cr\r":          "cr\\r",
		"nul\x00byte":   "nul\\0byte",
		"mix\n\t\r\x00": "mix\\n\\t\\r\\0",
	}
	for in, want := range cases {
		if got := SanitizeLog(in); got != want {
			t.Errorf("SanitizeLog(%q) = %q, want %q", in, got, want)
		}
	}
}
