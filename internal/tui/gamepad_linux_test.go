//go:build linux

package tui

import (
	"testing"
	"time"
)

func TestMapNativeGamepadEvent_Buttons(t *testing.T) {
	var x, y gamepadAxisState
	cases := []struct {
		name string
		code uint16
		want string
	}{
		{name: "cross", code: btnSouth, want: KeySelect},
		{name: "circle", code: btnEast, want: KeyBackAlt},
		{name: "square", code: btnWest, want: KeyRun},
		{name: "triangle", code: btnNorth, want: KeyHintEx},
		{name: "options", code: btnStart, want: KeyHelp},
		{name: "dpad up", code: btnDUp, want: KeyUp},
		{name: "dpad down", code: btnDDown, want: KeyDown},
		{name: "dpad left", code: btnDLeft, want: KeyLeft},
		{name: "dpad right", code: btnDRight, want: KeyRight},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := mapNativeGamepadEvent(evKey, tc.code, 1, &x, &y)
			if !ok || got != tc.want {
				t.Fatalf("mapped to %q, %v, want %q", got, ok, tc.want)
			}
		})
	}
}

func TestMapNativeGamepadEvent_IgnoresButtonRelease(t *testing.T) {
	var x, y gamepadAxisState
	if got, ok := mapNativeGamepadEvent(evKey, btnSouth, 0, &x, &y); ok || got != "" {
		t.Fatalf("release mapped to %q, %v", got, ok)
	}
}

func TestMapNativeGamepadEvent_HatAxes(t *testing.T) {
	var x, y gamepadAxisState
	cases := []struct {
		code  uint16
		value int32
		want  string
	}{
		{code: absHat0X, value: -1, want: KeyLeft},
		{code: absHat0X, value: 1, want: KeyRight},
		{code: absHat0Y, value: -1, want: KeyUp},
		{code: absHat0Y, value: 1, want: KeyDown},
	}
	for _, tc := range cases {
		got, ok := mapNativeGamepadEvent(evAbs, tc.code, tc.value, &x, &y)
		if !ok || got != tc.want {
			t.Fatalf("axis %d/%d mapped to %q, %v, want %q", tc.code, tc.value, got, ok, tc.want)
		}
	}
}

func TestAxisKey_DebouncesStickHold(t *testing.T) {
	state := gamepadAxisState{center: 128, ready: true}
	got, ok := axisKey(0, &state, KeyLeft, KeyRight)
	if !ok || got != KeyLeft {
		t.Fatalf("first tilt mapped to %q, %v", got, ok)
	}
	got, ok = axisKey(0, &state, KeyLeft, KeyRight)
	if ok || got != "" {
		t.Fatalf("held tilt repeated as %q, %v", got, ok)
	}
	got, ok = axisKey(128, &state, KeyLeft, KeyRight)
	if ok || got != "" || state.dir != 0 {
		t.Fatalf("center mapped to %q, %v, state %d", got, ok, state.dir)
	}
}

func TestAxisKey_CalibratesFirstValueAsNeutral(t *testing.T) {
	var state gamepadAxisState
	if got, ok := axisKey(128, &state, KeyLeft, KeyRight); ok || got != "" {
		t.Fatalf("first neutral value mapped to %q, %v", got, ok)
	}
	if !state.ready || state.center != 128 {
		t.Fatalf("axis state = %+v, want ready center 128", state)
	}
	if got, ok := axisKey(255, &state, KeyLeft, KeyRight); !ok || got != KeyRight {
		t.Fatalf("right tilt mapped to %q, %v", got, ok)
	}
}

func TestAllowGamepadKey_ThrottlesSelect(t *testing.T) {
	last := map[string]time.Time{}
	now := time.Unix(100, 0)
	if !allowGamepadKey(last, KeySelect, now) {
		t.Fatal("first select should be allowed")
	}
	if allowGamepadKey(last, KeySelect, now.Add(200*time.Millisecond)) {
		t.Fatal("second select inside debounce window should be blocked")
	}
	if !allowGamepadKey(last, KeySelect, now.Add(500*time.Millisecond)) {
		t.Fatal("select after debounce window should be allowed")
	}
}
