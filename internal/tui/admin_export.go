package tui

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ztutor/internal/db"
	"ztutor/internal/license"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const exportsDir = "exports"
const importsDir = "imports"

type adminExportModel struct {
	db  *db.DB
	lic *license.State
	sized
	cursor int
	result string // success path / summary text or error text
	done   bool   // action ran; show result and wait for keypress
	isErr  bool
}

type exportOption struct {
	key   string
	label string
	desc  string
}

var exportOptions = []exportOption{
	{
		key:   "p",
		label: "Progress Report",
		desc:  "One row per (student, lesson): username, lesson_id, stars, completed_at",
	},
	{
		key:   "r",
		label: "Student Roster",
		desc:  "One row per student: username, enabled, total_stars, lessons_done, streak, last_active, joined",
	},
	{
		key:   "i",
		label: "Import Students from CSV",
		desc:  "Read imports/students.csv (username[,password]); write exports/passwords-DATE.csv",
	},
}

func newAdminExport(database *db.DB, lic *license.State, w, h int) *adminExportModel {
	return &adminExportModel{db: database, lic: lic, sized: sized{Width: w, Height: h}}
}

func (m *adminExportModel) Init() tea.Cmd { return nil }

func (m *adminExportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)

	case tea.KeyMsg:
		if m.done {
			return m, backCmd(NavigateToAdminDashboard{})
		}
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(exportOptions)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter", " ":
			m.runAction(m.cursor)
		case "p":
			m.cursor = 0
			m.runAction(0)
		case "r":
			m.cursor = 1
			m.runAction(1)
		case "i":
			m.cursor = 2
			m.runAction(2)
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminDashboard{})
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminExportModel) runAction(idx int) {
	if idx == 2 {
		m.runImport()
		return
	}
	m.runExport(idx)
}

func (m *adminExportModel) runExport(idx int) {
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		m.result = "could not create exports/ directory: " + err.Error()
		m.isErr = true
		m.done = true
		return
	}

	stamp := time.Now().Format("2006-01-02")
	var (
		data []byte
		err  error
		name string
	)

	switch idx {
	case 0:
		name = fmt.Sprintf("progress-%s.csv", stamp)
		data, err = m.db.ExportProgressCSV()
	case 1:
		name = fmt.Sprintf("roster-%s.csv", stamp)
		data, err = m.db.ExportRosterCSV()
	}

	if err != nil {
		m.result = "export failed: " + err.Error()
		m.isErr = true
		m.done = true
		return
	}

	outPath := filepath.Join(exportsDir, name)
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		m.result = "write failed: " + err.Error()
		m.isErr = true
		m.done = true
		return
	}

	abs, _ := filepath.Abs(outPath)
	m.result = abs
	m.isErr = false
	m.done = true
}

func (m *adminExportModel) runImport() {
	// Check imports/students.csv exists.
	csvPath := filepath.Join(importsDir, "students.csv")
	data, err := os.ReadFile(csvPath)
	if err != nil {
		m.result = fmt.Sprintf("could not read %s: %v\n\nCreate the file with one row per student:\n  username[,password]", csvPath, err)
		m.isErr = true
		m.done = true
		return
	}

	maxStudents := 0
	if m.lic != nil && m.lic.MaxStudents > 0 {
		maxStudents = m.lic.MaxStudents
	}

	result, err := m.db.ImportStudentsCSV(data, maxStudents)
	if err != nil {
		m.result = "import failed: " + err.Error()
		m.isErr = true
		m.done = true
		return
	}

	// Write password file (0600 — sensitive).
	var summary strings.Builder
	if len(result.Created) > 0 {
		if err := os.MkdirAll(exportsDir, 0o755); err != nil {
			m.result = "could not create exports/ directory: " + err.Error()
			m.isErr = true
			m.done = true
			return
		}
		stamp := time.Now().Format("2006-01-02")
		pwPath := filepath.Join(exportsDir, fmt.Sprintf("passwords-%s.csv", stamp))

		var pwBuf bytes.Buffer
		w := csv.NewWriter(&pwBuf)
		_ = w.Write([]string{"username", "password"})
		for _, cred := range result.Created {
			_ = w.Write([]string{cred.Username, cred.Password})
		}
		w.Flush()

		if err := os.WriteFile(pwPath, pwBuf.Bytes(), 0o600); err != nil {
			m.result = "could not write password file: " + err.Error()
			m.isErr = true
			m.done = true
			return
		}
		abs, _ := filepath.Abs(pwPath)
		summary.WriteString(fmt.Sprintf("Created %d student(s).\n", len(result.Created)))
		summary.WriteString(fmt.Sprintf("Passwords written to:\n%s\n", abs))
		summary.WriteString("Use scp or sftp to retrieve and distribute credentials.")
	} else {
		summary.WriteString("No new students created.")
	}
	if len(result.Skipped) > 0 {
		summary.WriteString(fmt.Sprintf("\n\nSkipped %d existing account(s): %s",
			len(result.Skipped), strings.Join(result.Skipped, ", ")))
	}
	for _, e := range result.Errors {
		summary.WriteString("\nerror: " + e)
	}

	m.result = summary.String()
	m.isErr = false
	m.done = true
}

func (m *adminExportModel) View() string {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(2, 4).
		Width(70)

	var b strings.Builder
	b.WriteString(titleStyle("Export / Import") + "\n\n")

	if m.done {
		if m.isErr {
			b.WriteString(exErrorStyle.Render("error: "+m.result) + "\n\n")
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(m.result) + "\n\n")
		}
		b.WriteString(dim("press any key to return"))
		return center(m.Width, m.Height, border.Render(b.String()))
	}

	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))

	for i, opt := range exportOptions {
		prefix := "  "
		label := labelStyle.Render(opt.label)
		if i == m.cursor {
			prefix = cursorStyle.Render("> ")
		}
		b.WriteString(prefix + label + "\n")
		b.WriteString("    " + descStyle.Render(opt.desc) + "\n\n")
	}

	b.WriteString(dim("Exports go to exports/  |  Place imports/students.csv before importing."))
	b.WriteString("\n\n")
	b.WriteString(dim("j/k choose  enter run  q back"))

	return center(m.Width, m.Height, border.Render(b.String()))
}
