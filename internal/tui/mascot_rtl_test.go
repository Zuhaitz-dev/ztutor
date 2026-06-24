package tui

import (
	"strings"
	"testing"
)

func TestMirrorLine_ReversesAndMirrors(t *testing.T) {
	tests := []struct{ in, want string }{
		// mirrorLine both mirrors directional chars AND reverses the string.
		// For symmetric ASCII art, reversal+mirror preserves the original.
		// For asymmetric strings, both transformations are visible.
		{"abc", "cba"},
		{"(hello)", "(olleh)"}, // parens mirrored + reversed
	}
	for _, tt := range tests {
		got := mirrorLine(tt.in)
		if got != tt.want {
			t.Errorf("mirrorLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMirrorLine_Idempotent(t *testing.T) {
	for _, s := range []string{"( o.o )", ` /\_/\ `, "hello", " > ^ < "} {
		twice := mirrorLine(mirrorLine(s))
		if twice != s {
			t.Errorf("mirrorLine(mirrorLine(%q)) = %q, want %q", s, twice, s)
		}
	}
}

func TestMirrorSprites_Init(t *testing.T) {
	for mood, frames := range mascotSprites {
		mirrored, ok := mirrorSprites[mood]
		if !ok {
			t.Errorf("mirrorSprites missing mood %q", mood)
			continue
		}
		if len(mirrored) != len(frames) {
			t.Errorf("mood %q: frames count %d, mirror count %d", mood, len(frames), len(mirrored))
			continue
		}
		for i := range frames {
			if len(mirrored[i]) != len(frames[i]) {
				t.Errorf("mood %q frame %d: lines %d, mirror lines %d", mood, i, len(frames[i]), len(mirrored[i]))
			}
		}
	}
}

func TestMirrorSprites_MouthIsMirrored(t *testing.T) {
	for mood, frames := range mirrorSprites {
		for i, frame := range frames {
			orig := mascotSprites[mood][i]
			for j := range frame {
				got := frame[j]
				want := mirrorLine(orig[j])
				if got != want {
					t.Errorf("mirrorSprites[%q][%d][%d] = %q, want %q",
						mood, i, j, got, want)
				}
			}
		}
	}
}

func TestRenderMascotPanel_RTL_HasLtrMarker(t *testing.T) {
	result := renderMascotPanel(80, "Mochi", "مرحبا", MoodIdle, 0, true)

	if result == "" {
		t.Fatal("RTL panel should not be empty")
	}
	if !strings.Contains(result, ltrMarker) {
		t.Error("RTL should contain LTR marker (U+200E) around the cat sprite")
	}
}

func TestRenderMascotPanel_RTL_MirrorsSprite(t *testing.T) {
	ltr := renderMascotPanel(80, "Mochi", "test", MoodIdle, 0, false)
	rtl := renderMascotPanel(80, "Mochi", "test", MoodIdle, 0, true)
	if ltr == rtl {
		t.Error("LTR and RTL should produce different sprite renders (mirrored face)")
	}
}
