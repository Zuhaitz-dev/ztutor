//go:build linux

package tui

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ztutor/internal/logutil"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sys/unix"
)

const (
	evKey = 0x01
	evAbs = 0x03

	btnSouth  = 0x130
	btnEast   = 0x131
	btnNorth  = 0x133
	btnWest   = 0x134
	btnTL     = 0x136
	btnTR     = 0x137
	btnSelect = 0x13a
	btnStart  = 0x13b
	btnDUp    = 0x220
	btnDDown  = 0x221
	btnDLeft  = 0x222
	btnDRight = 0x223

	absX     = 0x00
	absY     = 0x01
	absHat0X = 0x10
	absHat0Y = 0x11
)

type NativeGamepad struct {
	events <-chan string
}

type inputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

type gamepadAxisState struct {
	center int32
	dir    int
	ready  bool
}

func NewNativeGamepad() *NativeGamepad {
	devices := nativeGamepadDevices()
	if len(devices) == 0 {
		return nil
	}
	ch := make(chan string, 32)
	opened := 0
	for _, path := range devices {
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			logutil.Debug("gamepad: cannot open %s: %v", path, err)
			continue
		}
		opened++
		logutil.Info("gamepad: reading %s", path)
		go readGamepadEvents(f, ch)
	}
	if opened == 0 {
		return nil
	}
	return &NativeGamepad{events: ch}
}

func (g *NativeGamepad) Next() tea.Cmd {
	if g == nil || g.events == nil {
		return nil
	}
	return func() tea.Msg {
		key, ok := <-g.events
		if !ok {
			return nil
		}
		return gamepadInputMsg{Key: key}
	}
}

func nativeGamepadDevices() []string {
	if os.Getenv("ZTUTOR_GAMEPAD") == "0" {
		return nil
	}
	if path := strings.TrimSpace(os.Getenv("ZTUTOR_GAMEPAD_DEVICE")); path != "" {
		return []string{path}
	}
	var out []string
	for _, pattern := range []string{
		"/dev/input/by-id/*event-joystick",
		"/dev/input/by-path/*event-joystick",
	} {
		matches, _ := filepath.Glob(pattern)
		out = append(out, matches...)
	}
	return uniquePaths(out)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	var out []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		key := path
		if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != "" {
			key = resolved
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, path)
	}
	return out
}

func readGamepadEvents(r io.Reader, out chan<- string) {
	var stickX, stickY gamepadAxisState
	last := make(map[string]time.Time)
	for {
		var ev inputEvent
		if err := binary.Read(r, binary.LittleEndian, &ev); err != nil {
			if !errors.Is(err, io.EOF) {
				logutil.Debug("gamepad: read stopped: %v", err)
			}
			return
		}
		if key, ok := mapNativeGamepadEvent(ev.Type, ev.Code, ev.Value, &stickX, &stickY); ok && allowGamepadKey(last, key, time.Now()) {
			select {
			case out <- key:
			default:
			}
		}
	}
}

func allowGamepadKey(last map[string]time.Time, key string, now time.Time) bool {
	delay := 160 * time.Millisecond
	switch key {
	case KeySelect, KeyBackAlt, KeyRun, KeyHintEx, KeyHelp:
		delay = 450 * time.Millisecond
	case KeySection, KeySectionPrev:
		delay = 260 * time.Millisecond
	}
	if t, ok := last[key]; ok && now.Sub(t) < delay {
		return false
	}
	last[key] = now
	return true
}

func mapNativeGamepadEvent(eventType, code uint16, value int32, stickX, stickY *gamepadAxisState) (string, bool) {
	switch eventType {
	case evKey:
		if value != 1 {
			return "", false
		}
		switch code {
		case btnSouth:
			return KeySelect, true
		case btnEast:
			return KeyBackAlt, true
		case btnWest:
			return KeyRun, true
		case btnNorth:
			return KeyHintEx, true
		case btnDUp:
			return KeyUp, true
		case btnDDown:
			return KeyDown, true
		case btnDLeft:
			return KeyLeft, true
		case btnDRight:
			return KeyRight, true
		case btnTL, btnSelect:
			return KeySectionPrev, true
		case btnTR:
			return KeySection, true
		case btnStart:
			return KeyHelp, true
		}
	case evAbs:
		switch code {
		case absHat0X:
			switch value {
			case -1:
				return KeyLeft, true
			case 1:
				return KeyRight, true
			}
		case absHat0Y:
			switch value {
			case -1:
				return KeyUp, true
			case 1:
				return KeyDown, true
			}
		case absX:
			return axisKey(value, stickX, KeyLeft, KeyRight)
		case absY:
			return axisKey(value, stickY, KeyUp, KeyDown)
		}
	}
	return "", false
}

func axisKey(value int32, state *gamepadAxisState, negativeKey, positiveKey string) (string, bool) {
	if !state.ready {
		state.center = value
		state.ready = true
		return "", false
	}
	deadzone := int32(60)
	if state.center > 1000 {
		deadzone = 16000
	}
	next := 0
	key := ""
	switch {
	case value <= state.center-deadzone:
		next = -1
		key = negativeKey
	case value >= state.center+deadzone:
		next = 1
		key = positiveKey
	}
	if next == state.dir {
		return "", false
	}
	state.dir = next
	if next == 0 {
		return "", false
	}
	return key, true
}
