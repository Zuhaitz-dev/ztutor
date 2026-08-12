package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Lesson Target Picker ──────────────────────────────────────────────────────

type lessonTarget struct {
	Label string
	Dir   string
}

// buildLessonTargets enumerates course sections in coursesDir and appends
// the legacy lessonsDir if it exists.
func buildLessonTargets(coursesDir, lessonsDir string) []lessonTarget {
	var targets []lessonTarget
	if coursesDir != "" {
		courseEntries, _ := os.ReadDir(coursesDir)
		for _, ce := range courseEntries {
			if !ce.IsDir() {
				continue
			}
			courseDir := filepath.Join(coursesDir, ce.Name())
			subs, _ := os.ReadDir(courseDir)
			for _, se := range subs {
				if !se.IsDir() {
					continue
				}
				secDir := filepath.Join(courseDir, se.Name())
				// Only include dirs that contain lesson subdirectories.
				inner, _ := os.ReadDir(secDir)
				hasLessons := false
				for _, le := range inner {
					if le.IsDir() {
						hasLessons = true
						break
					}
				}
				if !hasLessons {
					continue
				}
				label := ce.Name() + " / " + se.Name()
				targets = append(targets, lessonTarget{Label: label, Dir: secDir})
			}
		}
	}
	if lessonsDir != "" {
		if _, err := os.Stat(lessonsDir); err == nil {
			targets = append(targets, lessonTarget{Label: "lessons (legacy)", Dir: lessonsDir})
		}
	}
	return targets
}

type adminLessonTargetPickerModel struct {
	targets []lessonTarget
	mode    string // "create", "import", "edit"
	cursor  int
	sized
}

func newAdminLessonTargetPicker(targets []lessonTarget, mode string, w, h int) *adminLessonTargetPickerModel {
	return &adminLessonTargetPickerModel{targets: targets, mode: mode, sized: sized{Width: w, Height: h}}
}

func (m *adminLessonTargetPickerModel) Init() tea.Cmd { return nil }

func (m *adminLessonTargetPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.targets)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.targets) == 0 {
				return m, nil
			}
			t := m.targets[m.cursor]
			mode := m.mode
			return m, backCmd(adminLessonTargetPickedMsg{Dir: t.Dir, Mode: mode})
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminDashboard{})
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminLessonTargetPickerModel) View() string {
	modeLabel := map[string]string{
		"create":   "Create Lesson",
		"import":   "Import Lesson",
		"edit":     "Edit Lesson",
		"scaffold": "Scaffold Lesson Files",
	}[m.mode]

	var b strings.Builder
	b.WriteString(titleStyle(modeLabel) + dim("  Select target location") + "\n\n")

	if len(m.targets) == 0 {
		b.WriteString(dim("No course sections found. Add a course first.") + "\n")
	}

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))

	for i, t := range m.targets {
		dir := dimStyle.Render(t.Dir)
		if i == m.cursor {
			b.WriteString(selStyle.Render(fmt.Sprintf("> %-34s %s", t.Label, t.Dir)) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %-34s %s", t.Label, dir) + "\n")
		}
	}

	b.WriteString("\n" + helpBar("j/k choose", "enter select", "b back"))
	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}
