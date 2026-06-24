package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/db"
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type courseListItem struct {
	ID       string
	Title    string
	Language string
	Enrolled int
}

type adminCourseListModel struct {
	db     *db.DB
	items  []courseListItem
	cursor int
	lang   string
	sized
}

func newAdminCourseList(database *db.DB, coursesDir, lang string, w, h int) *adminCourseListModel {
	m := &adminCourseListModel{
		db:    database,
		lang:  lang,
		sized: sized{Width: w, Height: h},
	}
	if coursesDir != "" {
		courses, _ := lesson.LoadCoursesLang(coursesDir, lang)
		for _, c := range courses {
			count, _ := database.CountEnrollments(c.ID)
			m.items = append(m.items, courseListItem{
				ID:       c.ID,
				Title:    c.Title,
				Language: c.Language,
				Enrolled: count,
			})
		}
	}
	return m
}

func (m *adminCourseListModel) Init() tea.Cmd { return nil }

func (m *adminCourseListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
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
				c := m.items[m.cursor]
				return m, backCmd(NavigateToAdminCourseDetail{CourseID: c.ID, CourseTitle: c.Title})
			}
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminDashboard{})
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminCourseListModel) View() string {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(1, 2).
		Width(72)

	total := len(m.items)
	scrollInfo := ""
	if total > 0 {
		scrollInfo = " " + dim(fmt.Sprintf("%d/%d", m.cursor+1, total))
	}
	title := titleStyle("Courses") + scrollInfo

	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	langStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHex))

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, item := range m.items {
		lang := langStyle.Render(fmt.Sprintf("[%s]", item.Language))
		enrolled := dimStyle.Render(fmt.Sprintf("%d enrolled", item.Enrolled))
		title := lipgloss.NewStyle().Bold(true).Render(item.Title)
		line := title + "  " + lang + "  " + enrolled
		if i == m.cursor {
			line = cursorStyle.Render("> ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}

	if total == 0 {
		b.WriteString(dim("No courses found."))
	}

	b.WriteString("\n" + helpBar("enter enrollment", "b back"))

	return center(m.Width, m.Height, border.Render(b.String()))
}

// ── Course Detail / Enrollment Management ─────────────────────────────────────

type courseDetailUser struct {
	Username string
	Enrolled bool
}

type adminCourseDetailModel struct {
	db          *db.DB
	courseID    string
	courseTitle string
	items       []courseDetailUser
	cursor      int
	offset      int
	msg         string
	sized
}

func newAdminCourseDetail(database *db.DB, courseID, courseTitle string, w, h int) *adminCourseDetailModel {
	m := &adminCourseDetailModel{
		db:          database,
		courseID:    courseID,
		courseTitle: courseTitle,
		sized:       sized{Width: w, Height: h},
	}
	m.reload()
	return m
}

func (m *adminCourseDetailModel) reload() {
	users, err := m.db.ListUsers()
	if err != nil {
		m.msg = "error loading users: " + err.Error()
		return
	}
	enrolled, err := m.db.ListEnrolledUsers(m.courseID)
	if err != nil {
		m.msg = "error loading enrollments: " + err.Error()
		return
	}
	set := make(map[string]bool, len(enrolled))
	for _, u := range enrolled {
		set[u] = true
	}
	m.items = nil
	for _, u := range users {
		m.items = append(m.items, courseDetailUser{Username: u.Username, Enrolled: set[u.Username]})
	}
}

func (m *adminCourseDetailModel) Init() tea.Cmd { return nil }

func (m *adminCourseDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		ph := m.panelHeight()
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				if m.cursor >= m.offset+ph {
					m.offset++
				}
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "e", "enter":
			if m.cursor < len(m.items) {
				item := &m.items[m.cursor]
				if item.Enrolled {
					if err := m.db.DeleteEnrollment(item.Username, m.courseID); err != nil {
						m.msg = "error: " + err.Error()
					} else {
						item.Enrolled = false
						m.msg = item.Username + " unenrolled"
					}
				} else {
					if err := m.db.Enroll(item.Username, m.courseID); err != nil {
						m.msg = "error: " + err.Error()
					} else {
						item.Enrolled = true
						m.msg = item.Username + " enrolled"
					}
				}
			}
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminCourses{})
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminCourseDetailModel) panelHeight() int {
	h := m.Height - 12
	if h < 3 {
		h = 3
	}
	return h
}

func (m *adminCourseDetailModel) View() string {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(1, 2).
		Width(60)

	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)

	header := titleStyle(m.courseTitle) + dim("  enrollment")
	scrollInfo := ""
	if len(m.items) > 0 {
		scrollInfo = " " + dim(fmt.Sprintf("%d/%d", m.cursor+1, len(m.items)))
	}

	var b strings.Builder
	b.WriteString(header + scrollInfo + "\n\n")

	ph := m.panelHeight()
	end := m.offset + ph
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.offset; i < end; i++ {
		item := m.items[i]
		badge := dimStyle.Render("[  ]")
		if item.Enrolled {
			badge = okStyle.Render("[OK]")
		}
		line := badge + " " + item.Username
		if i == m.cursor {
			line = cursorStyle.Render("> ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}

	if len(m.items) == 0 {
		b.WriteString(dim("No users found.") + "\n")
	}

	if m.msg != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(m.msg) + "\n")
	}

	b.WriteString("\n" + helpBar("j/k navigate", "e toggle enrollment", "b back"))
	return center(m.Width, m.Height, border.Render(b.String()))
}
