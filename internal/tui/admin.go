package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"ztutor/internal/db"
	"ztutor/internal/i18n"
	"ztutor/internal/license"
	"ztutor/internal/logutil"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Messages ──────────────────────────────────────────────────────────────────

type adminSetupDoneMsg struct{ username string }
type adminStudentAddedMsg struct{}
type adminStudentToggleMsg struct {
	username string
	enabled  bool
}
type adminPasswordResetDoneMsg struct{}
type adminCompileResultMsg struct{ output, errStr string }
type adminLessonSavedMsg struct{ id string }

// ── Navigate messages ─────────────────────────────────────────────────────────

type NavigateToAdminDashboard struct{}
type NavigateToAdminStudents struct{}
type NavigateToAdminAddStudent struct{}
type NavigateToAdminPasswordReset struct{ Username string }
type NavigateToAdminStudentDetail struct{ Username string }
type NavigateToAdminLessonCreate struct{}
type NavigateToAdminLessonImport struct{}
type NavigateToAdminAchievements struct{}
type NavigateToAdminLessonEdit struct{}
type NavigateToAdminLessonPicker struct{ Dir string }
type NavigateToAdminCourses struct{}
type NavigateToAdminCourseDetail struct {
	CourseID    string
	CourseTitle string
}
type NavigateToAdminExport struct{}
type NavigateToAdminLessonTargetPick struct{ Mode string } // "create", "import", "edit"
type adminLessonTargetPickedMsg struct{ Dir, Mode string }
type adminLessonEditSelectedMsg struct{ Dir string }
type navigateToStudentView struct{}

// ── AdminApp ──────────────────────────────────────────────────────────────────

type AdminApp struct {
	username         string
	db               *db.DB
	lic              *license.State
	lessonsDir       string
	coursesDir       string
	achievementsFile string
	freeMode         bool
	uiLang           string
	loc              *i18n.Locale

	current tea.Model
	sized
	LaunchStudent bool
}

func (a *AdminApp) WantsRelaunch() bool  { return a.LaunchStudent }
func (a *AdminApp) RelaunchUser() string { return a.LaunchStudentUsername() }

func NewAdminApp(username string, database *db.DB, lic *license.State, lessonsDir, coursesDir, achievementsFile string, width, height int) *AdminApp {
	freeMode := lic == nil || !lic.Licensed
	uiLang, _ := database.GetUserSetting(username, "lang")
	if uiLang == "" {
		uiLang = "en"
	}
	app := &AdminApp{
		username:         username,
		db:               database,
		lic:              lic,
		lessonsDir:       lessonsDir,
		coursesDir:       coursesDir,
		achievementsFile: achievementsFile,
		freeMode:         freeMode,
		uiLang:           uiLang,
		loc:              i18n.New(uiLang),
		sized:            sized{Width: width, Height: height},
	}

	hasUsers, err := database.HasUsers()
	if err != nil {
		logutil.Warn("admin: cannot check users: %v", err)
		hasUsers = true // conservative default: show dashboard
	}
	if !hasUsers {
		app.current = newAdminSetup(app.loc, width, height, freeMode)
	} else {
		app.current = newAdminDashboard(database, lic, app.loc, width, height)
	}

	return app
}

func (a *AdminApp) Init() tea.Cmd {
	if a.current != nil {
		return a.current.Init()
	}
	return nil
}

func (a *AdminApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ^L: save the language to DB synchronously before doing anything else.
	// Using an async cmd (changeLangMsg) had a race: if the user pressed ^L
	// then immediately pressed v (student view), the admin TUI could quit before
	// the cmd result was processed, so SetUserSetting never ran and the student
	// TUI started in the wrong language.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == KeyLanguage {
		next := a.loc.Next()
		if err := a.db.SetUserSetting(a.username, "lang", next.Lang()); err != nil {
			logutil.Warn("failed to save lang for %s: %v", a.username, err)
		}
		a.uiLang = next.Lang()
		a.loc = next
		switch a.current.(type) {
		case *adminDashboardModel:
			a.current = newAdminDashboard(a.db, a.lic, a.loc, a.Width, a.Height)
			return a, a.current.Init()
		case *adminSetupModel:
			a.current = newAdminSetup(a.loc, a.Width, a.Height, a.freeMode)
			return a, a.current.Init()
		}
		// Other admin screens don't carry locale; language takes effect on
		// next navigation back to the dashboard.
		return a, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width <= 0 || msg.Height <= 0 {
			return a, nil
		}
		a.HandleResize(msg)
		m, cmd := a.current.Update(msg)
		a.current = m
		return a, cmd

	case adminSetupDoneMsg:
		if a.freeMode {
			if err := a.db.CreateUser(msg.username, "", db.RoleStudent); err != nil {
				return a, nil
			}
			a.username = msg.username
			a.LaunchStudent = true
			return a, tea.Quit
		}
		if err := a.db.CreateUser(msg.username, "", db.RoleAdmin); err != nil {
			return a, nil
		}
		a.username = msg.username
		a.current = newAdminDashboard(a.db, a.lic, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case adminStudentAddedMsg:
		a.current = newAdminStudentList(a.db, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case adminStudentToggleMsg:
		a.db.SetUserEnabled(msg.username, msg.enabled)
		a.current = newAdminStudentList(a.db, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case adminPasswordResetDoneMsg:
		a.current = newAdminStudentList(a.db, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case adminLessonSavedMsg:
		a.current = newAdminDashboard(a.db, a.lic, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case changeLangMsg:
		// Secondary path: a sub-screen emitted changeLangMsg directly.
		// The primary ^L path above handles DB save synchronously; this case
		// handles any legacy callers and keeps loc in sync.
		if a.uiLang != msg.lang {
			if err := a.db.SetUserSetting(a.username, "lang", msg.lang); err != nil {
				logutil.Warn("failed to save lang for %s: %v", a.username, err)
			}
			a.uiLang = msg.lang
			a.loc = i18n.New(msg.lang)
		}
		switch a.current.(type) {
		case *adminDashboardModel:
			a.current = newAdminDashboard(a.db, a.lic, a.loc, a.Width, a.Height)
			return a, a.current.Init()
		case *adminSetupModel:
			a.current = newAdminSetup(a.loc, a.Width, a.Height, a.freeMode)
			return a, a.current.Init()
		}
		return a, nil

	case NavigateToAdminDashboard:
		a.current = newAdminDashboard(a.db, a.lic, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminStudents:
		a.current = newAdminStudentList(a.db, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminAddStudent:
		a.current = newAdminAddStudent(a.db, a.lic, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminPasswordReset:
		a.current = newAdminPasswordReset(msg.Username, a.db, a.loc, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminStudentDetail:
		a.current = newAdminStudentDetail(msg.Username, a.db, a.coursesDir, a.uiLang, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminLessonCreate:
		a.current = newAdminLessonCreate(a.lessonsDir, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminLessonImport:
		a.current = newAdminLessonImport(a.lessonsDir, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminAchievements:
		a.current = newAdminAchievements(a.achievementsFile, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminExport:
		a.current = newAdminExport(a.db, a.lic, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminCourses:
		a.current = newAdminCourseList(a.db, a.coursesDir, a.uiLang, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminCourseDetail:
		a.current = newAdminCourseDetail(a.db, msg.CourseID, msg.CourseTitle, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminLessonTargetPick:
		targets := buildLessonTargets(a.coursesDir, a.lessonsDir)
		a.current = newAdminLessonTargetPicker(targets, msg.Mode, a.Width, a.Height)
		return a, a.current.Init()

	case adminLessonTargetPickedMsg:
		switch msg.Mode {
		case "create":
			a.current = newAdminLessonCreate(msg.Dir, a.Width, a.Height)
		case "import":
			a.current = newAdminLessonImport(msg.Dir, a.Width, a.Height)
		case "edit":
			a.current = newAdminLessonPicker(msg.Dir, a.Width, a.Height)
		case "scaffold":
			a.current = newAdminLessonScaffold(msg.Dir, a.Width, a.Height)
		}
		return a, a.current.Init()

	case NavigateToAdminLessonEdit:
		a.current = newAdminLessonPicker(a.lessonsDir, a.Width, a.Height)
		return a, a.current.Init()

	case NavigateToAdminLessonPicker:
		a.current = newAdminLessonPicker(msg.Dir, a.Width, a.Height)
		return a, a.current.Init()

	case adminLessonEditSelectedMsg:
		// Use the lesson's parent dir so new lessons land in the same section.
		sectionDir := filepath.Dir(msg.Dir)
		m, err := newAdminLessonEdit(sectionDir, msg.Dir, a.Width, a.Height)
		if err != nil {
			a.current = newAdminDashboardWithErr(a.db, a.lic, a.loc, err.Error(), a.Width, a.Height)
			return a, a.current.Init()
		}
		a.current = m
		return a, a.current.Init()

	case navigateToStudentView:
		a.LaunchStudent = true
		return a, tea.Quit
	}

	if a.current != nil {
		m, cmd := a.current.Update(msg)
		a.current = m
		return a, cmd
	}

	return a, nil
}

func (a *AdminApp) View() string {
	if a.current != nil {
		return a.current.View()
	}
	return "loading..."
}

func (a *AdminApp) LaunchStudentUsername() string {
	return a.username
}

// ── Rain overlay helper ───────────────────────────────────────────────────────

// overlayBoxOnRain renders a full-width/height rain grid and splices the
// rendered box into the centre of it row by row, so rain shows behind the box.
func overlayBoxOnRain(rainCols []rainCol, box string, totalW, totalH int) string {
	boxLines := strings.Split(box, "\n")
	for len(boxLines) > 0 && boxLines[len(boxLines)-1] == "" {
		boxLines = boxLines[:len(boxLines)-1]
	}
	boxH := len(boxLines)
	boxW := lipgloss.Width(box)

	boxX := (totalW - boxW) / 2
	boxY := (totalH - boxH) / 2
	if boxX < 0 {
		boxX = 0
	}
	if boxY < 0 {
		boxY = 0
	}

	var rows []string
	for row := 0; row < totalH; row++ {
		var line strings.Builder
		if row >= boxY && row < boxY+boxH {
			boxRowIdx := row - boxY
			// Left rain
			for col := 0; col < boxX && col < len(rainCols); col++ {
				line.WriteString(rainCols[col].renderAt(row))
			}
			// Box line
			if boxRowIdx < len(boxLines) {
				line.WriteString(boxLines[boxRowIdx])
			}
			// Right rain
			for col := boxX + boxW; col < totalW && col < len(rainCols); col++ {
				line.WriteString(rainCols[col].renderAt(row))
			}
		} else {
			for col := 0; col < totalW && col < len(rainCols); col++ {
				line.WriteString(rainCols[col].renderAt(row))
			}
		}
		rows = append(rows, line.String())
	}
	return strings.Join(rows, "\n")
}

// overlayTwoPanelsOnRain renders left and right as independent boxes on the rain
// so rain shows through the gap between them and below the shorter panel.
func overlayTwoPanelsOnRain(rainCols []rainCol, left, right string, gapW, totalW, totalH int) string {
	trim := func(lines []string) []string {
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return lines
	}
	leftLines := trim(strings.Split(left, "\n"))
	rightLines := trim(strings.Split(right, "\n"))
	leftH := len(leftLines)
	rightH := len(rightLines)
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	combinedW := leftW + gapW + rightW
	startX := (totalW - combinedW) / 2
	if startX < 0 {
		startX = 0
	}
	rightX := startX + leftW + gapW

	maxH := leftH
	if rightH > maxH {
		maxH = rightH
	}
	startY := (totalH - maxH) / 2
	if startY < 0 {
		startY = 0
	}

	var rows []string
	for row := 0; row < totalH; row++ {
		var line strings.Builder
		li := row - startY
		ri := row - startY
		inLeft := li >= 0 && li < leftH
		inRight := ri >= 0 && ri < rightH

		col := 0
		// Rain to the left of left panel.
		for ; col < startX && col < len(rainCols); col++ {
			line.WriteString(rainCols[col].renderAt(row))
		}
		// Left panel or rain behind it.
		if inLeft {
			line.WriteString(leftLines[li])
			col = startX + leftW
		} else {
			for ; col < startX+leftW && col < len(rainCols); col++ {
				line.WriteString(rainCols[col].renderAt(row))
			}
			col = startX + leftW
		}
		// Gap between panels.
		for ; col < rightX && col < len(rainCols); col++ {
			line.WriteString(rainCols[col].renderAt(row))
		}
		col = rightX
		// Right panel or rain behind it.
		if inRight {
			line.WriteString(rightLines[ri])
			col = rightX + rightW
		} else {
			for ; col < rightX+rightW && col < len(rainCols); col++ {
				line.WriteString(rainCols[col].renderAt(row))
			}
			col = rightX + rightW
		}
		// Rain to the right of right panel.
		for ; col < totalW && col < len(rainCols); col++ {
			line.WriteString(rainCols[col].renderAt(row))
		}

		rows = append(rows, line.String())
	}
	return strings.Join(rows, "\n")
}

func ensureFullRain(cols *[]rainCol, rainH *int, w, h int) {
	if len(*cols) != w || *rainH != h {
		*cols = make([]rainCol, w)
		for i := range *cols {
			(*cols)[i] = newRainCol(h)
		}
		*rainH = h
	}
}

// ── Admin Setup Screen (first run) ────────────────────────────────────────────

type adminSetupModel struct {
	input textinput.Model
	loc   *i18n.Locale
	sized
	mascotFrame int
	rainCols    []rainCol
	rainH       int
	freeMode    bool
}

func newAdminSetup(loc *i18n.Locale, w, h int, freeMode bool) *adminSetupModel {
	if loc == nil {
		loc = i18n.New("en")
	}
	ti := textinput.New()
	ti.Placeholder = "learner"
	if !freeMode {
		ti.Placeholder = "admin"
	}
	ti.CharLimit = 32
	ti.Width = 30
	ti.Focus()
	return &adminSetupModel{input: ti, loc: loc, sized: sized{Width: w, Height: h}, freeMode: freeMode}
}

func (m *adminSetupModel) Init() tea.Cmd {
	return tea.Batch(mascotTickCmd(), rainTickCmd())
}

func (m *adminSetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case rainTickMsg:
		for i := range m.rainCols {
			m.rainCols[i].tick(m.rainH)
		}
		return m, rainTickCmd()
	case mascotTickMsg:
		m.mascotFrame++
		return m, mascotTickCmd()
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				name = "learner"
				if !m.freeMode {
					name = "admin"
				}
			}
			return m, backCmd(adminSetupDoneMsg{username: name})
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *adminSetupModel) View() string {
	T := m.loc.T
	borderColors := []lipgloss.Color{"212", "213", "211", "213"}
	bc := borderColors[m.mascotFrame%len(borderColors)]
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(bc).
		Padding(2, 4).
		Width(50)

	rtl := m.loc.IsRTL()
	if rtl {
		border = border.Align(lipgloss.Right)
	}

	setupPrefix := "admin.setup.business."
	if m.freeMode {
		setupPrefix = "admin.setup.free."
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent)).Render(T(setupPrefix + "title"))
	subtitle := dim(T(setupPrefix + "subtitle"))
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Render(T(setupPrefix + "prompt"))

	content := title + "\n\n" + subtitle + "\n\n" + prompt + "\n" + m.input.View() + "\n\n" +
		dim(T(setupPrefix+"hint1")) + "\n" +
		dim(T(setupPrefix+"hint2")) + "\n\n" +
		helpBar(T("help.language"))

	box := border.Render(content)

	if m.Width < 60 || m.Height < 10 {
		return center(m.Width, m.Height, box)
	}

	ensureFullRain(&m.rainCols, &m.rainH, m.Width, m.Height)
	out := overlayBoxOnRain(m.rainCols, box, m.Width, m.Height)
	if rtl {
		out = addLTRMark(out)
	}
	return out
}

// ── Admin Dashboard ───────────────────────────────────────────────────────────

// adminDashboardSnippets cycles on the right panel — mixed languages.
var adminDashboardSnippets = []string{
	`int main(void) {`,
	`  printf("hello\n");`,
	`malloc(sizeof(Node))`,
	`while (*p++) p++;`,
	`*(int*)0 = 0; /* :) */`,
	`#include <stdio.h>`,
	`free(ptr); ptr = NULL;`,
	`def hello():`,
	`  print("hello")`,
	`for i in range(n):`,
	`if __name__ == "__main__":`,
	`gcc -Wall -Wextra`,
	`python3 -m pytest`,
	`if err != nil { ... }`,
}

type adminDashboardModel struct {
	db  *db.DB
	lic *license.State
	loc *i18n.Locale
	sized
	mascotFrame int
	rainCols    []rainCol
	rainH       int
	flashErr    string
}

func newAdminDashboard(database *db.DB, lic *license.State, loc *i18n.Locale, w, h int) *adminDashboardModel {
	if loc == nil {
		loc = i18n.New("en")
	}
	return &adminDashboardModel{db: database, lic: lic, loc: loc, sized: sized{Width: w, Height: h}}
}

func newAdminDashboardWithErr(database *db.DB, lic *license.State, loc *i18n.Locale, errMsg string, w, h int) *adminDashboardModel {
	m := newAdminDashboard(database, lic, loc, w, h)
	m.flashErr = errMsg
	return m
}

func (m *adminDashboardModel) Init() tea.Cmd {
	return tea.Batch(mascotTickCmd(), rainTickCmd())
}

func (m *adminDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case rainTickMsg:
		for i := range m.rainCols {
			m.rainCols[i].tick(m.rainH)
		}
		return m, rainTickCmd()
	case mascotTickMsg:
		m.mascotFrame++
		return m, mascotTickCmd()
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "1", "s":
			return m, backCmd(NavigateToAdminStudents{})
		case "2", "a":
			return m, backCmd(NavigateToAdminAddStudent{})
		case "3", "v":
			return m, backCmd(navigateToStudentView{})
		case "4", "c":
			return m, backCmd(NavigateToAdminCourses{})
		case "5", "l":
			return m, backCmd(NavigateToAdminLessonTargetPick{Mode: "create"})
		case "6", "i":
			return m, backCmd(NavigateToAdminLessonTargetPick{Mode: "import"})
		case "7", "e":
			return m, backCmd(NavigateToAdminLessonTargetPick{Mode: "edit"})
		case "8", "g":
			return m, backCmd(NavigateToAdminAchievements{})
		case "9", "x":
			return m, backCmd(NavigateToAdminExport{})
		case "0", "f":
			return m, backCmd(NavigateToAdminLessonTargetPick{Mode: "scaffold"})
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminDashboardModel) View() string {
	// ── Left panel ─────────────────────────────────────────────────────────────
	studentCount, scErr := m.db.CountUsers()
	if scErr != nil {
		logutil.Warn("admin: CountUsers: %v", scErr)
	}
	leaderboard, lbErr := m.db.Leaderboard()
	if lbErr != nil {
		logutil.Warn("admin: Leaderboard: %v", lbErr)
	}
	totalLessons := 0
	for _, e := range leaderboard {
		totalLessons += e.LessonsDone
	}

	T := m.loc.T
	rtl := m.loc.IsRTL()
	textAlign := lipgloss.Left
	if rtl {
		textAlign = lipgloss.Right
	}
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)

	title := titleStyle(T("admin.title"))
	stats := dim(T("admin.students")) + "  " + numStyle.Render(fmt.Sprintf("%d", studentCount)) + "\n" +
		dim(T("admin.lessons_done")) + "  " + numStyle.Render(fmt.Sprintf("%d", totalLessons))

	licInfo := bold(T("admin.license"))
	if m.lic != nil && m.lic.Licensed {
		licInfo += " " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(forceLTRText(m.lic.Licensee))
		if m.lic.MaxStudents > 0 {
			atCap := studentCount >= m.lic.MaxStudents
			capStr := T("admin.seats", studentCount, m.lic.MaxStudents)
			if atCap {
				licInfo += "\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render(capStr)
			} else {
				licInfo += "\n  " + dim(capStr)
			}
		}
		if !m.lic.ExpiresAt.IsZero() {
			licInfo += "\n  " + T("admin.expires", forceLTRText(m.lic.ExpiresAt.Format("2006-01-02")))
		}
		var feats []string
		if len(m.lic.UnlockedCourses) > 0 {
			feats = append(feats, "premium")
		}
		if m.lic.HasMultiUser {
			feats = append(feats, "multi-user")
		}
		if len(feats) > 0 {
			licInfo += "\n  " + T("admin.features", forceLTRText(strings.Join(feats, ", ")))
		}
	} else {
		licInfo += " " + dim(T("admin.free_tier"))
	}

	hl := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHex))
	menuItem := func(key, label string) string {
		k := dim(key+".") + " " + hl.Render(label)
		if rtl {
			return hl.Render(label) + " " + dim("."+key)
		}
		return k
	}
	menu := bold(T("admin.menu.title")) + "\n" +
		"  " + menuItem("1", T("admin.menu.students")) + "\n" +
		"  " + menuItem("2", T("admin.menu.add_student")) + "\n" +
		"  " + menuItem("3", T("admin.menu.student_view")) + "\n" +
		"  " + menuItem("4", T("admin.menu.courses")) + "\n" +
		"  " + menuItem("5", T("admin.menu.create")) + "\n" +
		"  " + menuItem("6", T("admin.menu.import")) + "\n" +
		"  " + menuItem("7", T("admin.menu.edit")) + "\n" +
		"  " + menuItem("8", T("admin.menu.achievements")) + "\n" +
		"  " + menuItem("9", T("admin.menu.export")) + "\n" +
		"  " + menuItem("0", T("admin.menu.scaffold")) + "\n" +
		"  " + menuItem("q", T("admin.menu.quit"))

	errLine := ""
	if m.flashErr != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render("err: "+m.flashErr)
	}
	langHint := "\n\n" + helpBar(T("help.language"), T("admin.menu.quit"))
	leftContent := title + "\n\n" + stats + "\n\n" + licInfo + "\n\n" + menu + errLine + langHint

	leftPanel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(2, 4).
		Align(textAlign).
		Width(52).
		Render(leftContent)

	// ── Right panel ────────────────────────────────────────────────────────────
	catStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Bold(true)
	sprite := mascotSprite(MoodIdle, m.mascotFrame)
	var catLines strings.Builder
	for _, line := range sprite {
		catLines.WriteString(catStyle.Render(line) + "\n")
	}

	// Pulsing label under mascot.
	labelColors := []lipgloss.Color{"212", "213", "211", "213"}
	labelColor := labelColors[m.mascotFrame%len(labelColors)]
	mascotLabel := lipgloss.NewStyle().Foreground(labelColor).Bold(true).Render("  Mochi")

	// Cycling code snippet — changes every ~3s (mascotFrame/6).
	snippet := adminDashboardSnippets[(m.mascotFrame/6)%len(adminDashboardSnippets)]
	snippetStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded)).Italic(true)
	snippetLine := snippetStyle.Render(snippet)

	// Server status indicators.
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Bold(true)
	status := func(label, val string, ok bool) string {
		badge := okStyle.Render("[OK]")
		if !ok {
			badge = warnStyle.Render("[--]")
		}
		if rtl {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Render(val) + " " + dim(label) + " " + badge
		}
		return badge + " " + dim(label) + " " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Render(val)
	}

	licensed := m.lic != nil && m.lic.Licensed
	serverStatus := status(T("admin.status.license"), func() string {
		if licensed {
			return m.lic.Licensee
		}
		return T("admin.status.free")
	}(), licensed) + "\n" +
		status(T("admin.status.students"), T("admin.status.registered", studentCount), studentCount > 0) + "\n" +
		status(T("admin.status.lessons"), T("admin.status.completed", totalLessons), totalLessons > 0)

	// Top student from leaderboard.
	topLine := dim(T("admin.status.no_students"))
	if len(leaderboard) > 0 {
		top := leaderboard[0]
		topLine = dim(T("admin.status.top")+" ") + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render(top.Username) +
			dim(" "+T("admin.status.done", top.LessonsDone))
	}

	rightContent := catLines.String() + mascotLabel + "\n\n" +
		snippetLine + "\n\n" +
		serverStatus + "\n\n" +
		topLine

	rightPanel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Align(textAlign).
		Padding(2, 3).
		Width(34).
		Render(rightContent)

	if m.Width < 60 || m.Height < 10 {
		both := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, lipgloss.NewStyle().Width(2).Render(""), rightPanel)
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, both)
	}

	ensureFullRain(&m.rainCols, &m.rainH, m.Width, m.Height)
	if rtl {
		return addLTRMark(overlayTwoPanelsOnRain(m.rainCols, rightPanel, leftPanel, 2, m.Width, m.Height))
	}
	return overlayTwoPanelsOnRain(m.rainCols, leftPanel, rightPanel, 2, m.Width, m.Height)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// addLTRMark prefixes every line with U+200E so BiDi-aware terminals keep
// panel borders in LTR order even when Arabic text is present in panels.
func addLTRMark(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "\u200e" + l
	}
	return strings.Join(lines, "\n")
}

func center(w, h int, content string) string {
	if w <= 0 || h <= 0 {
		return content
	}
	c := NewCanvas(w, h)
	c.DrawCenter(content)
	return c.String()
}
