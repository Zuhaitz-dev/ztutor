package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/editor"
	"ztutor/internal/sandbox"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

// ── Views ─────────────────────────────────────────────────────────────────────

func (m *adminLessonCreateModel) viewSectionPicker() string {
	id := strings.TrimSpace(m.idInput.Value())
	title := strings.TrimSpace(m.titleInput.Value())

	var b strings.Builder
	b.WriteString(titleStyle("Edit Lesson") + "  " + dim(id+" — "+title) + "\n\n")

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))

	for i, name := range wizardStepNames {
		if i == m.sectionCursor {
			b.WriteString(selStyle.Render(fmt.Sprintf("> %s", name)) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", name))
		}
	}

	b.WriteString("\n" + helpBar("j/k choose", "enter edit section", "q back to list"))
	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}

func (m *adminLessonCreateModel) View() string {
	if m.sectionPicker {
		return m.viewSectionPicker()
	}

	var b strings.Builder

	stepName := wizardStepNames[int(m.step)]
	if m.editMode {
		if m.step < stepSave {
			b.WriteString(titleStyle("Edit Lesson") +
				dim(fmt.Sprintf("  Step %d/%d: %s", int(m.step)+1, int(stepSave), stepName)) + "\n\n")
		} else {
			b.WriteString(titleStyle("Edit Lesson") + dim("  Save") + "\n\n")
		}
	} else {
		if m.step < stepSave {
			b.WriteString(titleStyle("Create Lesson") +
				dim(fmt.Sprintf("  Step %d/%d: %s", int(m.step)+1, int(stepSave), stepName)) + "\n\n")
		} else {
			b.WriteString(titleStyle("Create Lesson") + dim("  Save") + "\n\n")
		}
	}

	switch m.step {
	case stepMeta:
		b.WriteString(m.viewMeta())
	case stepContent:
		b.WriteString(m.viewTextarea(&m.contentArea, "Write your lesson content in Markdown."))
	case stepExercise:
		b.WriteString(m.viewEditor(m.exerciseEditor, "Write the starter code students will see. Leave empty for a blank editor."))
	case stepExpected:
		b.WriteString(m.viewExpected())
	case stepTutorial:
		b.WriteString(m.viewTextarea(&m.tutorialArea, "One Mochi dialogue beat per line. These appear before the exercise starts."))
	case stepHints:
		b.WriteString(m.viewTextarea(&m.hintsArea, "One hint per block. Separate blocks with a line containing only ---"))
	case stepSave:
		b.WriteString(m.viewSave())
	}

	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}

func (m *adminLessonCreateModel) viewMeta() string {
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	labelW := 14

	field := func(label string, idx int, value string) string {
		lbl := fmt.Sprintf("%-*s", labelW, label+":")
		if m.metaFocus == idx {
			return focusStyle.Render(lbl) + " " + value
		}
		return dimStyle.Render(lbl) + " " + value
	}

	// type toggle display
	var typeVal string
	if m.metaFocus == 2 {
		if m.isInterview {
			typeVal = dim("Lesson") + "  " + focusStyle.Render("[ Interview ]")
		} else {
			typeVal = focusStyle.Render("[ Lesson ]") + "  " + dim("Interview")
		}
	} else {
		if m.isInterview {
			typeVal = dim("Interview")
		} else {
			typeVal = dim("Lesson")
		}
	}

	// difficulty display
	diffVal := "< " + m.difficulty + " >"
	if m.metaFocus != 3 {
		diffVal = dim(m.difficulty)
	}

	// language display
	langDisplay := m.language
	if l := sandbox.GetLanguage(m.language); l != nil {
		langDisplay = l.DisplayName()
	}
	langVal := "< " + langDisplay + " >"
	if m.metaFocus == 5 {
		langVal = focusStyle.Render("< " + langDisplay + " >")
	}

	var b strings.Builder
	b.WriteString(field("ID", 0, m.idInput.View()) + "\n")
	b.WriteString(field("Title", 1, m.titleInput.View()) + "\n")
	b.WriteString(field("Type", 2, typeVal) + "\n")
	b.WriteString(field("Difficulty", 3, diffVal) + "\n")
	b.WriteString(field("Language", 5, langVal) + "\n")
	if m.isInterview {
		b.WriteString(field("Companies", 4, m.companiesInput.View()) + "\n")
	} else {
		b.WriteString(field("Tags", 4, m.tagsInput.View()) + "\n")
	}

	if m.msg != "" {
		b.WriteString("\n" + exErrorStyle.Render(m.msg) + "\n")
	}

	if m.editMode {
		b.WriteString("\n" + helpBar("tab next field", "space/left/right toggle", "ctrl+n next", "ctrl+p menu", "ctrl+q cancel"))
	} else {
		b.WriteString("\n" + helpBar("tab next field", "space/left/right toggle", "ctrl+n next step", "ctrl+q cancel"))
	}
	return b.String()
}

func (m *adminLessonCreateModel) viewImportOverlay() string {
	var b strings.Builder
	b.WriteString(dim("Import from file (absolute path):") + "\n")
	b.WriteString(m.importInput.View() + "\n")
	if m.importErr != "" {
		b.WriteString(exErrorStyle.Render(m.importErr) + "\n")
	}
	b.WriteString(helpBar("enter load", "esc cancel"))
	return b.String()
}

func (m *adminLessonCreateModel) viewTextarea(ta *textarea.Model, hint string) string {
	var b strings.Builder
	b.WriteString(dim(hint) + "\n\n")
	b.WriteString(ta.View() + "\n\n")
	if m.importMode {
		b.WriteString(m.viewImportOverlay())
	} else {
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		b.WriteString(helpBar("ctrl+n next", backHint, "ctrl+j import", "ctrl+q cancel"))
	}
	return b.String()
}

func (m *adminLessonCreateModel) viewEditor(ed *editor.CodeEditor, hint string) string {
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(dim(hint) + "\n\n")
	b.WriteString(borderStyle.Render(ed.View()) + "\n\n")
	if m.importMode {
		b.WriteString(m.viewImportOverlay())
	} else {
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		b.WriteString(helpBar("ctrl+n next", backHint, "ctrl+j import", "ctrl+q cancel"))
	}
	return b.String()
}

func (m *adminLessonCreateModel) viewExpected() string {
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1)

	var modeA, modeB string
	if m.expectedMode == 0 {
		modeA = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render("[Solution]")
		modeB = dim("Manual")
	} else {
		modeA = dim("Solution")
		modeB = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render("[Manual]")
	}

	var b strings.Builder
	b.WriteString(dim("Mode: ") + modeA + " " + modeB + "  " + dim("(tab to switch)") + "\n\n")

	if m.expectedMode == 0 {
		b.WriteString(dim("Write the correct solution, then press ctrl+r to compile and capture output.") + "\n\n")
		b.WriteString(borderStyle.Render(m.solutionEditor.View()) + "\n\n")
		if m.compiling {
			b.WriteString(dim("compiling...") + "\n")
		} else if m.capturedOutput != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(
				fmt.Sprintf("captured: %q", m.capturedOutput)) + "\n")
		} else if m.captureErr != "" {
			b.WriteString(exErrorStyle.Render("error: "+m.captureErr) + "\n")
		}
		b.WriteString("\n")
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		if m.importMode {
			b.WriteString(m.viewImportOverlay())
		} else {
			b.WriteString(helpBar("ctrl+r compile", "ctrl+j import", "ctrl+n next", backHint, "tab switch"))
		}
	} else {
		b.WriteString(dim("Type the exact expected output of the exercise.") + "\n\n")
		b.WriteString(m.manualExpected.View() + "\n\n")
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		if m.importMode {
			b.WriteString(m.viewImportOverlay())
		} else {
			b.WriteString(helpBar("ctrl+n next", backHint, "ctrl+j import", "tab switch"))
		}
	}

	return b.String()
}

func (m *adminLessonCreateModel) viewSave() string {
	typeStr := "lesson"
	var tagsLabel, tagsVal string
	if m.isInterview {
		typeStr = "interview"
		tagsLabel = "Companies:"
		tagsVal = strings.Join(parseLessonTags(m.companiesInput.Value()), ", ")
	} else {
		tagsLabel = "Tags:"
		tagsVal = strings.Join(parseLessonTags(m.tagsInput.Value()), ", ")
	}
	if tagsVal == "" {
		tagsVal = dim("(none)")
	}
	// keep variable name for rest of function
	tags := tagsVal

	tutorial := parseTutorialBeats(m.tutorialArea.Value())
	hints := strings.TrimSpace(m.hintsArea.Value())
	hintCount := 0
	for _, block := range strings.Split(hints, "\n---\n") {
		if strings.TrimSpace(block) != "" {
			hintCount++
		}
	}

	expected := ""
	if m.expectedMode == 0 {
		expected = m.capturedOutput
	} else {
		expected = m.manualExpected.Value()
	}
	expectedSummary := dim("(none)")
	if strings.TrimSpace(expected) != "" {
		preview := strings.TrimSpace(expected)
		if len(preview) > 30 {
			preview = preview[:28] + ".."
		}
		expectedSummary = fmt.Sprintf("%q", preview)
	}

	exerciseLines := 0
	for _, l := range strings.Split(m.exerciseEditor.Value(), "\n") {
		if strings.TrimSpace(l) != "" {
			exerciseLines++
		}
	}
	contentLines := len(strings.Split(strings.TrimSpace(m.contentArea.Value()), "\n"))
	if strings.TrimSpace(m.contentArea.Value()) == "" {
		contentLines = 0
	}

	label := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded)).Render
	val := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Render

	var b strings.Builder
	b.WriteString(dim("Review and save the lesson:") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", label("ID:         "), val(strings.TrimSpace(m.idInput.Value()))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Title:      "), val(strings.TrimSpace(m.titleInput.Value()))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Type:       "), val(typeStr)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Difficulty: "), val(m.difficulty)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label(fmt.Sprintf("%-12s", tagsLabel)), val(tags)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Content:    "), val(fmt.Sprintf("%d lines", contentLines))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Exercise:   "), val(fmt.Sprintf("%d lines", exerciseLines))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Expected:   "), val(expectedSummary)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Tutorial:   "), val(fmt.Sprintf("%d beats", len(tutorial)))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Hints:      "), val(fmt.Sprintf("%d blocks", hintCount))))

	noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber))
	if exerciseLines == 0 {
		b.WriteString("\n" + noteStyle.Render("note: no exercise code — students will see an empty editor"))
	}
	if strings.TrimSpace(expected) == "" {
		b.WriteString("\n" + noteStyle.Render("note: no expected output — program runs but output is not checked"))
	}

	if m.msg != "" {
		b.WriteString("\n" + m.msg + "\n")
	}

	backHint := "b/ctrl+p back"
	if m.editMode {
		backHint = "ctrl+p menu"
	}
	b.WriteString("\n" + helpBar("enter save", backHint, "ctrl+q cancel"))
	return b.String()
}
