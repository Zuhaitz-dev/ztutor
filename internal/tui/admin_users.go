package tui

import (
	"fmt"
	"sort"
	"strings"

	"ztutor/internal/db"
	"ztutor/internal/lesson"
	"ztutor/internal/license"
	"ztutor/internal/logutil"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Student List ──────────────────────────────────────────────────────────────

type adminStudentListModel struct {
	db *db.DB
	sized
	users  []db.User
	offset int
}

func newAdminStudentList(database *db.DB, w, h int) *adminStudentListModel {
	users, err := database.ListUsers()
	if err != nil {
		logutil.Warn("admin: ListUsers: %v", err)
	}
	return &adminStudentListModel{db: database, sized: sized{Width: w, Height: h}, users: users}
}

func (m *adminStudentListModel) Init() tea.Cmd { return nil }

func (m *adminStudentListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.offset < len(m.users)-1 {
				m.offset++
			}
		case "k", "up":
			if m.offset > 0 {
				m.offset--
			}
		case "e":
			if len(m.users) > 0 {
				u := m.users[m.offset]
				newState := !u.Enabled
				m.db.SetUserEnabled(u.Username, newState)
				return m, backCmd(adminStudentToggleMsg{username: u.Username, enabled: newState})
			}
		case "p":
			if len(m.users) > 0 {
				u := m.users[m.offset]
				return m, backCmd(NavigateToAdminPasswordReset{Username: u.Username})
			}
		case "enter":
			if len(m.users) > 0 {
				u := m.users[m.offset]
				return m, backCmd(NavigateToAdminStudentDetail{Username: u.Username})
			}
		case "a":
			return m, backCmd(NavigateToAdminAddStudent{})
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminDashboard{})
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminStudentListModel) View() string {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(1, 2).
		Width(62)

	total := len(m.users)
	scrollInfo := ""
	if total > 0 {
		scrollInfo = " " + dim(fmt.Sprintf("%d/%d", m.offset+1, total))
	}
	title := titleStyle("Students") + scrollInfo

	enabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	adminStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber))

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, u := range m.users {
		status := enabledStyle.Render("[+]") + " "
		if !u.Enabled {
			status = disabledStyle.Render("[-]") + " "
		}
		role := dim("student")
		if u.Role == db.RoleAdmin {
			role = adminStyle.Render("admin")
		}
		line := fmt.Sprintf("%s%-20s %s", status, u.Username, role)
		if i == m.offset {
			line = cursorStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}

	if total == 0 {
		b.WriteString(dim("No students yet. Press 'a' to add one."))
	}

	b.WriteString(dim("\nenter detail  p reset pw  e toggle  a add  b back  q back"))

	return center(m.Width, m.Height, border.Render(b.String()))
}

// ── Add Student ───────────────────────────────────────────────────────────────

type adminAddStudentModel struct {
	input textinput.Model
	db    *db.DB
	lic   *license.State
	sized
	msg         string
	showPending bool // password shown, waiting for keypress to navigate
}

func newAdminAddStudent(database *db.DB, lic *license.State, w, h int) *adminAddStudentModel {
	ti := textinput.New()
	ti.Placeholder = "student username"
	ti.CharLimit = 32
	ti.Width = 30
	ti.Focus()
	return &adminAddStudentModel{input: ti, db: database, lic: lic, sized: sized{Width: w, Height: h}}
}

func (m *adminAddStudentModel) Init() tea.Cmd { return nil }

func (m *adminAddStudentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// Any key while password is on screen navigates to student list.
		if m.showPending {
			return m, backCmd(adminStudentAddedMsg{})
		}
		switch msg.String() {
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.msg = exErrorStyle.Render("username cannot be empty")
				return m, nil
			}
			if m.lic != nil && m.lic.MaxStudents > 0 {
				count, err := m.db.CountUsers()
				if err != nil {
					logutil.Warn("admin: CountUsers: %v", err)
					m.msg = exErrorStyle.Render("cannot verify seat limit — try again")
					return m, nil
				}
				if count >= m.lic.MaxStudents {
					m.msg = exErrorStyle.Render(fmt.Sprintf("license limit: max %d students", m.lic.MaxStudents))
					return m, nil
				}
			}
			pw := db.GenerateStudentPassword()
			if err := m.db.CreateUser(name, pw, db.RoleStudent); err != nil {
				m.msg = exErrorStyle.Render(err.Error())
				return m, nil
			}
			m.msg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(
				fmt.Sprintf("created: %s\npassword: %s", name, pw),
			)
			m.showPending = true
			m.input.Blur()
			return m, nil
		case "esc", "q":
			return m, backCmd(NavigateToAdminStudents{})
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *adminAddStudentModel) View() string {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(2, 4).
		Width(54)

	title := titleStyle("Add Student")
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Render("Username:")
	help := dim("Press Enter to create. A random password will be generated.")

	content := title + "\n\n" + prompt + "\n" + m.input.View() + "\n\n" + help
	if m.msg != "" {
		content += "\n\n" + m.msg
	}
	if m.showPending {
		content += "\n\n" + dim("press any key to continue")
	} else {
		content += "\n\n" + dim("esc/q back  enter create")
	}

	return center(m.Width, m.Height, border.Render(content))
}

// ── Password Reset ────────────────────────────────────────────────────────────

type adminPasswordResetModel struct {
	username string
	input    textinput.Model
	db       *db.DB
	sized
	msg  string
	done bool
}

func newAdminPasswordReset(username string, database *db.DB, w, h int) *adminPasswordResetModel {
	ti := textinput.New()
	ti.Placeholder = "new password (blank = generate)"
	ti.CharLimit = 64
	ti.Width = 34
	ti.Focus()
	return &adminPasswordResetModel{
		username: username,
		input:    ti,
		db:       database,
		sized:    sized{Width: w, Height: h},
	}
}

func (m *adminPasswordResetModel) Init() tea.Cmd { return nil }

func (m *adminPasswordResetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.done {
			return m, backCmd(adminPasswordResetDoneMsg{})
		}
		switch msg.String() {
		case "enter":
			pw := strings.TrimSpace(m.input.Value())
			if pw == "" {
				pw = db.GenerateStudentPassword()
			}
			if err := m.db.SetUserPassword(m.username, pw); err != nil {
				m.msg = exErrorStyle.Render("failed: " + err.Error())
				return m, nil
			}
			m.msg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(
				fmt.Sprintf("password updated\nnew password: %s", pw),
			)
			m.done = true
			m.input.Blur()
			return m, nil
		case "esc", "q":
			return m, backCmd(adminPasswordResetDoneMsg{})
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *adminPasswordResetModel) View() string {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(2, 4).
		Width(54)

	title := titleStyle("Reset Password") + "  " + dim(m.username)
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Render("New password:")
	help := dim("Leave blank to generate a random password.")

	content := title + "\n\n" + prompt + "\n" + m.input.View() + "\n\n" + help
	if m.msg != "" {
		content += "\n\n" + m.msg
	}
	if m.done {
		content += "\n\n" + dim("press any key to return")
	} else {
		content += "\n\n" + dim("esc/q back  enter set password")
	}

	return center(m.Width, m.Height, border.Render(content))
}

// ── Student Detail ─────────────────────────────────────────────────────────────

type adminStudentDetailModel struct {
	username string
	user     *db.User
	db       *db.DB
	progress map[string]int
	lessons  []lesson.Lesson
	offset   int
	sized
	coursesDir string
	lang       string

	// grant achievement overlay
	grantMode   bool
	grantCursor int
	grantMsg    string

	// enrollment section
	enrollMode   bool
	enrollOffset int
}

type detailItem struct {
	id    string
	title string
	stars int
}

func newAdminStudentDetail(username string, database *db.DB, coursesDir, lang string, w, h int) *adminStudentDetailModel {
	user, uErr := database.GetUser(username)
	if uErr != nil {
		logutil.Warn("admin: GetUser(%s): %v", username, uErr)
	}
	progress, pErr := database.Progress(username)
	if pErr != nil {
		logutil.Warn("admin: Progress(%s): %v", username, pErr)
	}
	if progress == nil {
		progress = map[string]int{}
	}
	var lessons []lesson.Lesson
	if coursesDir != "" {
		courses, cErr := lesson.LoadCoursesLang(coursesDir, lang)
		if cErr != nil {
			logutil.Warn("admin: LoadCoursesLang: %v", cErr)
		}
		for _, c := range courses {
			for _, sec := range c.Sections {
				lessons = append(lessons, sec.Lessons...)
			}
		}
	}
	return &adminStudentDetailModel{
		username:   username,
		user:       user,
		db:         database,
		progress:   progress,
		lessons:    lessons,
		coursesDir: coursesDir,
		lang:       lang,
		sized:      sized{Width: w, Height: h},
	}
}

func (m *adminStudentDetailModel) Init() tea.Cmd { return nil }

func (m *adminStudentDetailModel) buildItems() []detailItem {
	var items []detailItem
	seen := make(map[string]bool)
	for _, l := range m.lessons {
		items = append(items, detailItem{id: l.ID, title: l.Title, stars: m.progress[l.ID]})
		seen[l.ID] = true
	}
	var extra []string
	for id := range m.progress {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		items = append(items, detailItem{id: id, title: id, stars: m.progress[id]})
	}
	return items
}

func (m *adminStudentDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		if m.grantMode {
			return m.updateGrant(msg)
		}
		if m.enrollMode {
			return m.updateEnrollment(msg)
		}
		items := m.buildItems()
		switch msg.String() {
		case "j", "down":
			if m.offset < len(items)-1 {
				m.offset++
			}
		case "k", "up":
			if m.offset > 0 {
				m.offset--
			}
		case "g":
			m.grantMode = true
			m.grantCursor = 0
			m.grantMsg = ""
		case "e":
			m.enrollMode = true
			m.enrollOffset = 0
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminStudents{})
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminStudentDetailModel) updateGrant(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	all := AllAchievements()
	switch msg.String() {
	case "j", "down":
		if m.grantCursor < len(all)-1 {
			m.grantCursor++
		}
	case "k", "up":
		if m.grantCursor > 0 {
			m.grantCursor--
		}
	case "enter", " ":
		if m.grantCursor < len(all) {
			a := all[m.grantCursor]
			if err := m.db.GrantAchievement(m.username, a.ID); err != nil {
				m.grantMsg = exErrorStyle.Render("error: " + err.Error())
			} else {
				m.grantMsg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("[+] granted: " + a.Name)
				m.grantMode = false
			}
		}
	case "esc", "q":
		m.grantMode = false
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *adminStudentDetailModel) updateEnrollment(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	courses, _ := lesson.LoadCoursesLang(m.coursesDir, m.lang)
	switch msg.String() {
	case "j", "down":
		if m.enrollOffset < len(courses)-1 {
			m.enrollOffset++
		}
	case "k", "up":
		if m.enrollOffset > 0 {
			m.enrollOffset--
		}
	case "enter":
		if m.enrollOffset < len(courses) {
			c := courses[m.enrollOffset]
			if m.db.IsEnrolled(m.username, c.ID) {
				m.db.DeleteEnrollment(m.username, c.ID)
			} else {
				m.db.Enroll(m.username, c.ID)
			}
		}
	case "esc", "q":
		m.enrollMode = false
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *adminStudentDetailModel) View() string {
	items := m.buildItems()

	completed := 0
	totalStars := 0
	for _, s := range m.progress {
		if s > 0 {
			completed++
			totalStars += s
		}
	}

	var headerBadge string
	if m.user != nil {
		if m.user.Role == db.RoleAdmin {
			headerBadge = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render("admin")
		} else {
			headerBadge = "  " + dim("student")
		}
		if !m.user.Enabled {
			headerBadge += "  " + exErrorStyle.Render("[disabled]")
		}
	}

	statsLine := dim(fmt.Sprintf("%d/%d lessons  %d stars", completed, len(items), totalStars))

	const headerLines = 5
	const footerLines = 2
	visibleH := m.Height - headerLines - footerLines
	if visibleH < 1 {
		visibleH = 1
	}
	if m.offset+visibleH > len(items) && len(items) > visibleH {
		m.offset = len(items) - visibleH
	}

	var b strings.Builder
	b.WriteString(bold(m.username) + headerBadge + "\n")
	b.WriteString(statsLine + "\n")
	b.WriteString(strings.Repeat("─", 50) + "\n\n")

	end := m.offset + visibleH
	if end > len(items) {
		end = len(items)
	}

	for _, item := range items[m.offset:end] {
		stars := starsStyle(item.stars)
		title := item.title
		if len(title) > 30 {
			title = title[:28] + ".."
		}
		b.WriteString(fmt.Sprintf("  %s  %-32s %s\n", stars, title, dim(item.id)))
	}

	if len(items) == 0 {
		b.WriteString(dim("  No activity yet.\n"))
	}

	courses, _ := lesson.LoadCoursesLang(m.coursesDir, m.lang)
	if len(courses) > 0 {
		b.WriteString("\n" + bold("Enrollments") + "\n")
		b.WriteString(strings.Repeat("─", 50) + "\n")
		for _, c := range courses {
			badge := dim("[ ]")
			if m.db.IsEnrolled(m.username, c.ID) {
				badge = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("[✓]")
			}
			title := c.Title
			if len(title) > 30 {
				title = title[:28] + ".."
			}
			b.WriteString(fmt.Sprintf("  %s  %-32s %s\n", badge, title, dim(c.ID)))
		}
		b.WriteString("\n" + dim("e manage enrollments"))
	}

	if m.grantMsg != "" {
		b.WriteString("\n" + m.grantMsg + "\n")
	}

	b.WriteString("\n" + helpBar("j/k scroll", "g grant achievement", "e enrollments", "b/q back"))

	main := lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))

	if m.grantMode {
		return m.renderGrantOverlay()
	}
	if m.enrollMode {
		return m.renderEnrollmentOverlay()
	}
	return main
}

func (m *adminStudentDetailModel) renderGrantOverlay() string {
	all := AllAchievements()

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))

	var b strings.Builder
	b.WriteString(bold("Grant Achievement") + "  " + dim("for "+m.username) + "\n\n")

	visH := m.Height - 10
	if visH < 3 {
		visH = 3
	}
	start := m.grantCursor - visH/2
	if start < 0 {
		start = 0
	}
	end := start + visH
	if end > len(all) {
		end = len(all)
		start = end - visH
		if start < 0 {
			start = 0
		}
	}
	for i, a := range all[start:end] {
		idx := start + i
		line := fmt.Sprintf("  %s  %-22s  %s", a.Icon, a.Name, dimStyle.Render(a.Desc))
		if idx == m.grantCursor {
			line = selStyle.Render(fmt.Sprintf("> %s  %-22s  %s", a.Icon, a.Name, a.Desc))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + helpBar("j/k choose", "enter grant", "esc cancel"))

	overlayW := 66
	if m.Width-6 < overlayW {
		overlayW = m.Width - 6
	}
	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(1, 3).
		Width(overlayW).
		Render(b.String())

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, panel)
}

func (m *adminStudentDetailModel) renderEnrollmentOverlay() string {
	courses, _ := lesson.LoadCoursesLang(m.coursesDir, m.lang)

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))

	var b strings.Builder
	b.WriteString(bold("Manage Enrollments") + "  " + dim("for "+m.username) + "\n\n")

	visH := m.Height - 10
	if visH < 3 {
		visH = 3
	}
	start := m.enrollOffset - visH/2
	if start < 0 {
		start = 0
	}
	end := start + visH
	if end > len(courses) {
		end = len(courses)
		start = end - visH
		if start < 0 {
			start = 0
		}
	}
	for i, c := range courses[start:end] {
		idx := start + i
		badge := "[ ]"
		if m.db.IsEnrolled(m.username, c.ID) {
			badge = "[✓]"
		}
		title := c.Title
		if len(title) > 32 {
			title = title[:30] + ".."
		}
		line := fmt.Sprintf("  %s  %-34s  %s", badge, title, dim(c.ID))
		if idx == m.enrollOffset {
			line = selStyle.Render(fmt.Sprintf("> %s  %-34s  %s", badge, title, c.ID))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + helpBar("j/k choose", "enter toggle", "esc back"))

	overlayW := 66
	if m.Width-6 < overlayW {
		overlayW = m.Width - 6
	}
	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(1, 3).
		Width(overlayW).
		Render(b.String())

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, panel)
}
