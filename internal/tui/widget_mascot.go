package tui

// MascotMood is the set of named emotional states the mascot sprite can express.
// Each constant maps directly to a key in mascotSprites.
type MascotMood string

const (
	MoodIdle     MascotMood = "idle"
	MoodHappy    MascotMood = "happy"
	MoodWorried  MascotMood = "worried"
	MoodCrashed  MascotMood = "crashed"
	MoodThinking MascotMood = "thinking"
	MoodCurious  MascotMood = "curious"
	MoodFocused  MascotMood = "focused"
)

// MascotWidget wraps mascot display state and delegates rendering to renderMascotPanel.
// It is not in the Tab cycle — it is driven directly by ExerciseScreen.
type MascotWidget struct {
	name   string
	line   string
	pinned MascotMood // set by Speak(); overrides dynamic derivation until ClearPin()
	mood   MascotMood // current rendered mood, set each frame by the screen
	frame  int
	hidden bool
	width  int
	rtl    bool
}

func newMascotWidget(name, welcomeLine string, width int, rtl bool) *MascotWidget {
	return &MascotWidget{name: name, line: welcomeLine, mood: MoodIdle, width: width, rtl: rtl}
}

// SetLine replaces the companion speech line without changing the pinned mood.
func (w *MascotWidget) SetLine(s string) { w.line = s }

// Speak sets both the speech text and pins the mood for this message.
// The pin persists until ClearPin() is called, so the sprite reflects the
// emotional context of the message even as exercise state changes underneath.
func (w *MascotWidget) Speak(text string, mood MascotMood) {
	w.line = text
	w.pinned = mood
}

// ClearPin removes the pinned mood so dynamic derivation takes over again.
// Call this at the start of a new run so "thinking" shows during compilation.
func (w *MascotWidget) ClearPin() { w.pinned = "" }

// PinnedMood returns the currently pinned mood, or "" if none is set.
func (w *MascotWidget) PinnedMood() MascotMood { return w.pinned }

// SetMood sets the current rendered mood (called each frame by the screen's View).
func (w *MascotWidget) SetMood(m MascotMood) { w.mood = m }

// SetFrame advances the animation frame used by the sprite renderer.
func (w *MascotWidget) SetFrame(n int) { w.frame = n }

// SetWidth updates the terminal width used for panel layout.
func (w *MascotWidget) SetWidth(n int) { w.width = n }

// ToggleHidden flips the mascot's visibility.
func (w *MascotWidget) ToggleHidden() { w.hidden = !w.hidden }

// IsHidden reports whether the mascot panel is currently suppressed.
func (w *MascotWidget) IsHidden() bool { return w.hidden }

func (w *MascotWidget) SetRTL(v bool) { w.rtl = v }

// Line returns the current companion speech line, for reading by callers.
func (w *MascotWidget) Line() string { return w.line }

// View renders the mascot panel using the current state.
func (w *MascotWidget) View() string {
	return renderMascotPanel(w.width, w.name, w.line, w.mood, w.frame, w.rtl)
}
