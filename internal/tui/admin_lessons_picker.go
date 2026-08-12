package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Lesson Picker (for editing existing lessons) ──────────────────────────────

type lessonPickerItem struct {
	ID    string
	Title string
	Dir   string
}

type adminLessonPickerModel struct {
	lessonsDir string
	items      []lessonPickerItem
	cursor     int
	sized
	msg        string
	delConfirm bool
}

func newAdminLessonPicker(lessonsDir string, w, h int) *adminLessonPickerModel {
	m := &adminLessonPickerModel{
		lessonsDir: lessonsDir,
		sized:      sized{Width: w, Height: h},
	}
	m.loadItems()
	return m
}

func (m *adminLessonPickerModel) loadItems() {
	entries, err := os.ReadDir(m.lessonsDir)
	if err != nil {
		m.msg = "cannot read lessons directory: " + err.Error()
		return
	}

	var items []lessonPickerItem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(m.lessonsDir, entry.Name())
		mdPath := filepath.Join(dir, "lesson.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		content := string(data)
		title := extractLessonTitle(content)
		items = append(items, lessonPickerItem{
			ID:    entry.Name(),
			Title: title,
			Dir:   dir,
		})
	}
	m.items = items
	if len(items) == 0 {
		m.msg = "no existing lessons found"
	}
}

func extractLessonTitle(content string) string {
	body := content
	if strings.HasPrefix(content, "---\n") {
		idx := strings.Index(content[4:], "\n---\n")
		if idx >= 0 {
			body = content[idx+9:]
		}
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return "(no title)"
}

func (m *adminLessonPickerModel) Init() tea.Cmd { return nil }

func (m *adminLessonPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		if m.delConfirm {
			switch msg.String() {
			case "y", "Y":
				if len(m.items) > 0 {
					_ = os.RemoveAll(m.items[m.cursor].Dir)
					m.delConfirm = false
					m.loadItems()
					if m.cursor >= len(m.items) {
						m.cursor = max(0, len(m.items)-1)
					}
				}
			default:
				m.delConfirm = false
			}
			return m, nil
		}
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.items) > 0 {
				dir := m.items[m.cursor].Dir
				return m, backCmd(adminLessonEditSelectedMsg{Dir: dir})
			}
		case "d":
			if len(m.items) > 0 {
				m.delConfirm = true
			}
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminDashboard{})
		}
	}
	return m, nil
}

func (m *adminLessonPickerModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle("Edit Lesson") + dim("  Select a lesson to edit") + "\n\n")

	if m.msg != "" {
		b.WriteString(dim(m.msg) + "\n\n")
	}

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))

	visH := m.Height - 8
	if visH < 1 {
		visH = 1
	}
	start := m.cursor - visH/2
	if start < 0 {
		start = 0
	}
	end := start + visH
	if end > len(m.items) {
		end = len(m.items)
		start = end - visH
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		item := m.items[i]
		line := fmt.Sprintf("  %-24s %s", dimStyle.Render(item.ID), item.Title)
		if i == m.cursor {
			line = selStyle.Render(fmt.Sprintf("> %-24s %s", item.ID, item.Title))
		}
		b.WriteString(line + "\n")
	}

	if len(m.items) > 0 {
		b.WriteString("\n" + dim(fmt.Sprintf("%d/%d", m.cursor+1, len(m.items))))
	}

	if m.delConfirm && len(m.items) > 0 {
		b.WriteString("\n\n" + exErrorStyle.Render(
			fmt.Sprintf("Delete %q? This cannot be undone.", m.items[m.cursor].ID),
		))
		b.WriteString("\n" + dim("y confirm  any other key cancel"))
	} else {
		b.WriteString("\n\n" + helpBar("j/k choose", "enter edit", "d delete", "b/q back"))
	}

	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}
