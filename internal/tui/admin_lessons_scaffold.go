package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ztutor/internal/sandbox"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Scaffold Lesson ───────────────────────────────────────────────────────────

// adminLessonScaffoldModel writes template files to disk so the operator can
// author lesson content in their preferred editor, then use option 7 (Edit) to
// compile the expected output and review it.
type adminLessonScaffoldModel struct {
	targetDir  string
	idInput    textinput.Model
	titleInput textinput.Model
	language   string
	focus      int // 0=id 1=title 2=language
	done       bool
	result     string // absolute path to the scaffolded dir
	errStr     string
	sized
}

func newAdminLessonScaffold(targetDir string, w, h int) *adminLessonScaffoldModel {
	idInput := textinput.New()
	idInput.Placeholder = "10-arrays"
	idInput.CharLimit = 64
	idInput.Width = 40
	idInput.Focus()

	titleInput := textinput.New()
	titleInput.Placeholder = "Arrays in C"
	titleInput.CharLimit = 128
	titleInput.Width = 40

	return &adminLessonScaffoldModel{
		targetDir:  targetDir,
		idInput:    idInput,
		titleInput: titleInput,
		language:   "c",
		sized:      sized{Width: w, Height: h},
	}
}

func (m *adminLessonScaffoldModel) Init() tea.Cmd { return nil }

func (m *adminLessonScaffoldModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		if m.done {
			return m, backCmd(NavigateToAdminDashboard{})
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+q", "esc":
			return m, backCmd(NavigateToAdminDashboard{})
		case "tab", "down":
			if m.focus < 2 {
				m.scaffoldSetFocus(m.focus + 1)
			}
			return m, nil
		case "shift+tab", "up":
			if m.focus > 0 {
				m.scaffoldSetFocus(m.focus - 1)
			}
			return m, nil
		case "enter":
			if m.focus < 2 {
				m.scaffoldSetFocus(m.focus + 1)
			} else {
				m.runScaffold()
			}
			return m, nil
		case "left", " ":
			if m.focus == 2 {
				m.scaffoldCycleLang(-1)
			}
			return m, nil
		case "right":
			if m.focus == 2 {
				m.scaffoldCycleLang(1)
			}
			return m, nil
		default:
			var cmd tea.Cmd
			switch m.focus {
			case 0:
				m.idInput, cmd = m.idInput.Update(msg)
			case 1:
				m.titleInput, cmd = m.titleInput.Update(msg)
			}
			return m, cmd
		}
	}
	return m, nil
}

func (m *adminLessonScaffoldModel) scaffoldSetFocus(f int) {
	m.idInput.Blur()
	m.titleInput.Blur()
	m.focus = f
	switch f {
	case 0:
		m.idInput.Focus()
	case 1:
		m.titleInput.Focus()
	}
}

func (m *adminLessonScaffoldModel) scaffoldCycleLang(dir int) {
	all := sandbox.AllLanguages()
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cur := 0
	for i, k := range keys {
		if k == m.language {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(keys)) % len(keys)
	m.language = keys[cur]
}

func (m *adminLessonScaffoldModel) runScaffold() {
	id := strings.TrimSpace(m.idInput.Value())
	if id == "" {
		m.errStr = "ID is required"
		return
	}
	if strings.ContainsAny(id, " \t/\\") {
		m.errStr = "ID must be a slug (no spaces or slashes)"
		return
	}
	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		m.errStr = "Title is required"
		return
	}

	dir := filepath.Join(m.targetDir, id)
	if _, err := os.Stat(dir); err == nil {
		m.errStr = "a lesson with that ID already exists in this location"
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		m.errStr = "could not create directory: " + err.Error()
		return
	}

	lang := sandbox.GetLanguage(m.language)
	ext := ".c"
	if lang != nil {
		ext = lang.SourceExtension()
	}

	lessonMD := fmt.Sprintf(
		"---\ndifficulty: beginner\ntags:\n  - change-me\ntutorial:\n  - Welcome to the %s lesson. Edit this line — it is Mochi's opening line.\n---\n# %s\n\nWrite your lesson content here in Markdown.\n\nThis text appears before the exercise. Explain the concept clearly.\n",
		title, title,
	)
	if err := os.WriteFile(filepath.Join(dir, "lesson.md"), []byte(lessonMD), 0600); err != nil {
		m.errStr = "write lesson.md: " + err.Error()
		return
	}

	if err := os.WriteFile(filepath.Join(dir, "exercise"+ext), []byte(scaffoldExerciseCode(m.language)), 0600); err != nil {
		m.errStr = "write exercise file: " + err.Error()
		return
	}

	hints := "Replace this with your first hint.\n---\nReplace this with your second hint.\n"
	if err := os.WriteFile(filepath.Join(dir, "hints.txt"), []byte(hints), 0600); err != nil {
		m.errStr = "write hints.txt: " + err.Error()
		return
	}

	abs, _ := filepath.Abs(dir)
	m.result = abs
	m.done = true
}

func scaffoldExerciseCode(language string) string {
	switch language {
	case "c":
		return `#include <stdio.h>

/* TODO: Write the starter code students will see.
   Leave key parts unimplemented to guide their work. */

int main(void) {
    /* your code here */
    return 0;
}
`
	case "python", "py":
		return `# TODO: Write the starter code students will see.
# Leave key parts unimplemented to guide their work.

def main():
    pass  # your code here

if __name__ == "__main__":
    main()
`
	default:
		return "// TODO: Write the starter code students will see.\n"
	}
}

func (m *adminLessonScaffoldModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle("Scaffold Lesson Files") + "\n\n")

	if m.done {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("Files written to:") + "\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true).Render(m.result) + "\n\n")
		b.WriteString(dim("Edit lesson.md, exercise.*, and hints.txt in your preferred editor.") + "\n")
		b.WriteString(dim("To set expected output: use option 7 (Edit Lesson), go to Expected Output,") + "\n")
		b.WriteString(dim("paste or write the solution, and press ctrl+r to compile and capture.") + "\n\n")
		b.WriteString(dim("press any key to return to dashboard"))
		return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
			lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
	}

	b.WriteString(dim("Creates a lesson directory with template files you edit in your preferred editor.") + "\n\n")

	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	dimLbl := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	labelW := 10

	field := func(label string, idx int, value string) string {
		lbl := fmt.Sprintf("%-*s", labelW, label+":")
		if m.focus == idx {
			return focusStyle.Render(lbl) + " " + value
		}
		return dimLbl.Render(lbl) + " " + value
	}

	langDisplay := m.language
	if l := sandbox.GetLanguage(m.language); l != nil {
		langDisplay = l.DisplayName()
	}
	langVal := dim("< ") + langDisplay + dim(" >")
	if m.focus == 2 {
		langVal = focusStyle.Render("< " + langDisplay + " >")
	}

	lang := sandbox.GetLanguage(m.language)
	ext := ".c"
	if lang != nil {
		ext = lang.SourceExtension()
	}

	b.WriteString(field("ID", 0, m.idInput.View()) + "\n")
	b.WriteString(field("Title", 1, m.titleInput.View()) + "\n")
	b.WriteString(field("Language", 2, langVal) + "\n\n")
	b.WriteString(dim(fmt.Sprintf("Creates: lesson.md  exercise%s  hints.txt", ext)) + "\n")

	if m.errStr != "" {
		b.WriteString("\n" + exErrorStyle.Render(m.errStr) + "\n")
	}

	b.WriteString("\n" + helpBar("tab/enter next field", "left/right language", "enter on language to create", "ctrl+q cancel"))

	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}
