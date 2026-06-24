// Package tui implements the terminal user interface for ztutor using
// the Bubble Tea framework. It contains screens (exercise, lesson, menu, etc.),
// the compositor for widget focus management and key routing, a declarative
// TerminalLayout engine, and all UI widgets.
package tui

import (
	"strings"

	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
)

// WidgetID identifies a panel within the exercise screen.
type WidgetID int

const (
	WidgetFileList WidgetID = iota
	WidgetEditor
	WidgetFlags
	WidgetArgs
	WidgetStdin
	WidgetOutput
	WidgetAssembly
	WidgetDiagnostics
	WidgetMascot
	WidgetHints
	WidgetTests
	WidgetTrivia
	WidgetProgress
	WidgetMemory
	WidgetReference
	WidgetTimer
	WidgetKeybindingsOverlay
	WidgetStreak
	WidgetConsole
)

// EnabledWidgets is an opt-in set of widget IDs.
// When empty (nil or len==0), all widgets are considered enabled — this preserves
// backwards compatibility for lessons that don't declare a widgets list.
// Lessons opt in via frontmatter:  widgets: [flags, stdin, asm]
type EnabledWidgets map[WidgetID]bool

// Has reports whether the given widget should be active. An empty set means all enabled.
func (ew EnabledWidgets) Has(id WidgetID) bool {
	if len(ew) == 0 {
		return true
	}
	return ew[id]
}

// ParseWidgets converts the widget name strings from lesson frontmatter into an
// EnabledWidgets set. Unrecognised names are silently ignored.
func ParseWidgets(names []string) EnabledWidgets {
	if len(names) == 0 {
		return nil
	}
	ew := make(EnabledWidgets)
	for _, name := range names {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "filelist", "file_list":
			ew[WidgetFileList] = true
		case "editor":
			ew[WidgetEditor] = true
		case "flags":
			ew[WidgetFlags] = true
		case "args":
			ew[WidgetArgs] = true
		case "stdin":
			ew[WidgetStdin] = true
		case "output":
			ew[WidgetOutput] = true
		case "assembly", "asm":
			ew[WidgetAssembly] = true
		case "diag", "diagnostics":
			ew[WidgetDiagnostics] = true
		case "mascot":
			ew[WidgetMascot] = true
		case "hints":
			ew[WidgetHints] = true
		case "tests":
			ew[WidgetTests] = true
		case "trivia":
			ew[WidgetTrivia] = true
		case "progress":
			ew[WidgetProgress] = true
		case "memory":
			ew[WidgetMemory] = true
		case "reference", "references":
			ew[WidgetReference] = true
		case "timer":
			ew[WidgetTimer] = true
		case "keybindings", "keybindings_overlay", "help_overlay":
			ew[WidgetKeybindingsOverlay] = true
		case "streak":
			ew[WidgetStreak] = true
		case "console":
			ew[WidgetConsole] = true
		}
	}
	return ew
}

// Widget is the full interface for tab-cycle widgets that participate in focus
// management (Ctrl+F cycling) and key routing. These widgets receive Init,
// Update, Focus, Blur, and SetSize calls from the compositor.
//
// DisplayWidget is a lighter implicit contract for readout or overlay widgets
// that only need View() and Available(). They are managed directly by
// ExerciseScreen and are never in the tab cycle. Several widgets
// (OutputWidget, HintWidget, MascotWidget, DiagnosticsWidget, TestsWidget,
// TriviaWidget, ProgressWidget, TimerWidget, MemoryWidget, ReferenceWidget,
// KeybindingsOverlay, StreakWidget, ConsoleWidget,
// HexViewerWidget, StructInspectorWidget) follow this pattern.
//
// Tab-cycle widgets (EditorWidget, FlagsWidget, ArgsWidget, StdinWidget,
// FileListWidget, AssemblyWidget) implement the full Widget interface.
type Widget interface {
	Init() tea.Cmd
	Update(tea.Msg) (Widget, tea.Cmd)
	View() string

	ID() WidgetID
	SetSize(width, height int)

	Focus()
	Blur()
	Focused() bool

	// Available reports whether this widget applies to the current active file
	// and language. Unavailable widgets are hidden and skipped in focus cycling.
	Available() bool
}

// ActiveFile tracks one exercise file's mutable runtime state.
type ActiveFile struct {
	File    lesson.ExerciseFile
	Content string           // current content — kept in sync with editor on file switch
	Lang    sandbox.Language // resolved from File.Language; may be nil for unknown types
}

// fileSelectedMsg is sent when the user picks a different file in the file list.
type fileSelectedMsg struct{ idx int }
