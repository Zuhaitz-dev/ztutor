package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Bulk Lesson Import Screen ─────────────────────────────────────────────────

type importResultStatus int

const (
	importStatusImported importResultStatus = iota
	importStatusSkipped
	importStatusError
)

type lessonImportResult struct {
	id     string
	status importResultStatus
	detail string
}

type adminImportResultMsg struct {
	results []lessonImportResult
}

type adminLessonImportModel struct {
	lessonsDir string
	pathInput  textinput.Model

	results   []lessonImportResult
	importing bool
	done      bool
	scrollOff int

	msg string
	sized
}

func newAdminLessonImport(lessonsDir string, w, h int) *adminLessonImportModel {
	ti := textinput.New()
	ti.Placeholder = "/absolute/path/to/lesson-pack"
	ti.CharLimit = 512
	ti.Width = w - 10
	ti.Focus()

	return &adminLessonImportModel{
		lessonsDir: lessonsDir,
		pathInput:  ti,
		sized:      sized{Width: w, Height: h},
	}
}

func (m *adminLessonImportModel) Init() tea.Cmd { return textinput.Blink }

func (m *adminLessonImportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
		m.pathInput.Width = m.Width - 10

	case adminImportResultMsg:
		m.importing = false
		m.done = true
		m.results = msg.results

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+q", "q", "esc":
			if !m.importing {
				return m, backCmd(NavigateToAdminDashboard{})
			}
		}

		if m.done {
			switch msg.String() {
			case "b", "enter":
				return m, backCmd(NavigateToAdminDashboard{})
			case "j", "down":
				if m.scrollOff < len(m.results)-1 {
					m.scrollOff++
				}
			case "k", "up":
				if m.scrollOff > 0 {
					m.scrollOff--
				}
			}
			return m, nil
		}

		if m.importing {
			return m, nil
		}

		switch msg.String() {
		case "enter":
			src := strings.TrimSpace(m.pathInput.Value())
			if src == "" {
				m.msg = "path is required"
				return m, nil
			}
			m.importing = true
			m.msg = ""
			dst := m.lessonsDir
			return m, func() tea.Msg {
				return adminImportResultMsg{results: runLessonImport(src, dst)}
			}
		}

		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *adminLessonImportModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle("Import Lessons") + "\n\n")

	if m.importing {
		b.WriteString(dim("Scanning and copying lessons...") + "\n")
		return lipgloss.NewStyle().Padding(1, 3).Render(b.String())
	}

	if m.done {
		imported, skipped, errored := 0, 0, 0
		for _, r := range m.results {
			switch r.status {
			case importStatusImported:
				imported++
			case importStatusSkipped:
				skipped++
			case importStatusError:
				errored++
			}
		}

		okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
		skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))

		b.WriteString(fmt.Sprintf("%s  %s  %s\n\n",
			okStyle.Render(fmt.Sprintf("[+] %d imported", imported)),
			skipStyle.Render(fmt.Sprintf("[-] %d skipped", skipped)),
			errStyle.Render(fmt.Sprintf("[!] %d errors", errored)),
		))

		listH := m.Height - 10
		if listH < 3 {
			listH = 3
		}

		visible := m.results
		if m.scrollOff > 0 && m.scrollOff < len(visible) {
			visible = visible[m.scrollOff:]
		}
		shown := 0
		for _, r := range visible {
			if shown >= listH {
				break
			}
			var prefix, detail string
			switch r.status {
			case importStatusImported:
				prefix = okStyle.Render("[+]")
			case importStatusSkipped:
				prefix = skipStyle.Render("[-]")
				detail = dim(" — " + r.detail)
			case importStatusError:
				prefix = errStyle.Render("[!]")
				detail = errStyle.Render(" — " + r.detail)
			}
			b.WriteString(fmt.Sprintf("  %s %s%s\n", prefix, r.id, detail))
			shown++
		}

		if len(m.results) > listH {
			b.WriteString(dim(fmt.Sprintf("\n  %d/%d  (j/k to scroll)", m.scrollOff+shown, len(m.results))) + "\n")
		}

		b.WriteString("\n" + helpBar("enter/b back to dashboard"))
		return lipgloss.NewStyle().Padding(1, 3).Render(b.String())
	}

	b.WriteString(dim("Enter the absolute path to a directory containing lesson folders.") + "\n")
	b.WriteString(dim("Each subdirectory with a lesson.md file will be imported.") + "\n\n")
	b.WriteString(m.pathInput.View() + "\n")

	if m.msg != "" {
		b.WriteString("\n" + exErrorStyle.Render(m.msg) + "\n")
	}

	b.WriteString("\n" + helpBar("enter import", "esc/q back", "ctrl+c quit"))
	return lipgloss.NewStyle().Padding(1, 3).Render(b.String())
}

// runLessonImport copies lesson subdirectories from src into dst.
// A subdirectory is considered a lesson if it contains lesson.md.
func runLessonImport(src, dst string) []lessonImportResult {
	entries, err := os.ReadDir(src)
	if err != nil {
		return []lessonImportResult{{
			id: src, status: importStatusError, detail: err.Error(),
		}}
	}

	var results []lessonImportResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := os.Stat(filepath.Join(src, name, "lesson.md")); err != nil {
			continue // not a lesson dir
		}
		dstPath := filepath.Join(dst, name)
		if _, err := os.Stat(dstPath); err == nil {
			results = append(results, lessonImportResult{
				id: name, status: importStatusSkipped, detail: "already exists",
			})
			continue
		}
		if err := copyDir(filepath.Join(src, name), dstPath); err != nil {
			results = append(results, lessonImportResult{
				id: name, status: importStatusError, detail: err.Error(),
			})
		} else {
			results = append(results, lessonImportResult{
				id: name, status: importStatusImported,
			})
		}
	}

	if len(results) == 0 {
		results = append(results, lessonImportResult{
			id:     src,
			status: importStatusError,
			detail: "no lesson directories found (each must contain lesson.md)",
		})
	}

	return results
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
