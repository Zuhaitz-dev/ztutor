package tui

import (
	"strings"

	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const fileListWidth = 22

var (
	fileListHeaderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Background(lipgloss.Color("234"))
	fileListItemStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	fileListItemActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	fileListItemCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	fileListDividerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder))
	fileListDividerFocStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))
)

// FileListWidget renders a narrow sidebar listing exercise files and allows
// the user to switch between them. It sends fileSelectedMsg when the active
// file changes (Enter key). ExerciseScreen manages the cursor via MoveUp/MoveDown.
type FileListWidget struct {
	files     []lesson.ExerciseFile
	cursor    int
	activeIdx int
	focused   bool
	width     int
	height    int
}

func newFileListWidget(files []lesson.ExerciseFile) *FileListWidget {
	return &FileListWidget{files: files, width: fileListWidth}
}

func (w *FileListWidget) ID() WidgetID    { return WidgetFileList }
func (w *FileListWidget) Available() bool { return len(w.files) > 1 }
func (w *FileListWidget) Focused() bool   { return w.focused }
func (w *FileListWidget) Focus()          { w.focused = true }
func (w *FileListWidget) Blur()           { w.focused = false }
func (w *FileListWidget) Init() tea.Cmd   { return nil }
func (w *FileListWidget) SetSize(width, height int) {
	w.width, w.height = width, height
}

// SetActive updates the highlighted active file without triggering a message.
func (w *FileListWidget) SetActive(idx int) {
	w.activeIdx = idx
	w.cursor = idx
}

// Cursor returns the cursor position.
func (w *FileListWidget) Cursor() int { return w.cursor }

// MoveUp moves the cursor one row up.
func (w *FileListWidget) MoveUp() {
	if w.cursor > 0 {
		w.cursor--
	}
}

// MoveDown moves the cursor one row down.
func (w *FileListWidget) MoveDown() {
	if w.cursor < len(w.files)-1 {
		w.cursor++
	}
}

// Update satisfies the Widget interface; ExerciseScreen drives the cursor
// via direct methods (MoveUp/MoveDown) and emits fileSelectedMsg itself.
func (w *FileListWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	return w, nil
}

func (w *FileListWidget) View() string {
	if w.width < 4 || len(w.files) == 0 {
		return ""
	}

	innerW := w.width - 1 // -1 for the right divider column rendered by ExerciseScreen
	var b strings.Builder

	// Header row
	hdr := padVis(" files", innerW)
	b.WriteString(fileListHeaderStyle.Render(hdr))

	for i, f := range w.files {
		if i >= w.height-1 {
			break
		}
		b.WriteString("\n")

		name := f.Name
		maxNameW := innerW - 2
		if maxNameW < 1 {
			maxNameW = 1
		}
		if len(name) > maxNameW {
			if maxNameW > 3 {
				name = name[:maxNameW-3] + "..."
			} else {
				name = name[:maxNameW]
			}
		}

		prefix := "  "
		if i == w.cursor && w.focused {
			prefix = "> "
		}

		line := padVis(prefix+name, innerW)
		switch {
		case i == w.activeIdx && i == w.cursor && w.focused:
			b.WriteString(fileListItemActiveStyle.Render(line))
		case i == w.activeIdx:
			b.WriteString(fileListItemActiveStyle.Render(line))
		case i == w.cursor && w.focused:
			b.WriteString(fileListItemCursorStyle.Render(line))
		default:
			b.WriteString(fileListItemStyle.Render(line))
		}
	}

	return b.String()
}
