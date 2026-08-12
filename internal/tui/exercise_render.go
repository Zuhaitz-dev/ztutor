package tui

import (
	"strings"

	editormod "ztutor/internal/editor"

	"github.com/charmbracelet/lipgloss"
)

func (es *ExerciseScreen) View() string {
	if es.kbOverlay.IsVisible() {
		content := es.kbOverlay.View(es.loc, es.Width, es.Height)
		c := NewCanvas(es.Width, es.Height)
		c.DrawAt(0, content, lipgloss.Height(content))
		return c.String()
	}

	if es.compositor.InFullscreen() {
		return es.renderFullscreen()
	}

	T := es.loc.T
	l := NewTerminalLayout(es.Width, es.Height)

	l.AddFixed("header", nil, func(w int) string {
		langName := ""
		if es.lang != nil {
			langName = es.lang.Name()
		}
		title := exHeaderStyle.Render(es.lesson.Title)
		if langName != "" {
			title += "  " + dim("["+langName+"]")
		}
		return title + "\n" + dim(T("exercise.subtitle")) + "\n"
	})

	// Both sides of the split adjust together so each Ctrl+Up/Down step moves
	// the divider by one unit out of 10, giving ~3-5% change per keypress.
	// ASAN active overrides to a fixed 3:7 split so memory+hex have room.
	editorWeight := 10 - es.outputSplit
	outputWeight := es.outputSplit
	if es.memory.IsOpen() && es.hexViewer.IsVisible() {
		editorWeight, outputWeight = 3, 7
	}

	l.AddFlex("editor", editorWeight, nil, func(w, h int) string {
		return es.renderEditorArea(w, h)
	})

	l.AddFixed("inputs", nil, func(w int) string {
		if es.running {
			return es.renderRunningInput()
		}
		return es.renderInputsSection(w)
	})

	l.AddFlex("output", outputWeight, nil, func(w, h int) string {
		es.compositor.SetOutputH(h)
		return es.renderOutputPanel(w, h)
	})

	l.AddFixed("extras", func() bool {
		return es.timer.IsVisible() || es.progress.IsVisible()
	}, func(w int) string {
		var parts []string
		if es.timer.IsVisible() {
			parts = append(parts, es.timer.View())
		}
		if es.progress.IsVisible() {
			parts = append(parts, es.progress.View())
		}
		return strings.Join(parts, "\n")
	})

	l.AddFixed("streak", func() bool {
		return es.streak.IsVisible()
	}, func(w int) string {
		return es.streak.View()
	})

	l.AddFixed("console", func() bool {
		return es.console.IsVisible()
	}, func(w int) string {
		return es.console.View()
	})

	l.AddFixed("structinsp", func() bool {
		return es.structInsp.IsVisible()
	}, func(w int) string {
		return es.structInsp.View()
	})

	l.AddFixed("diag", nil, func(w int) string {
		return es.diag.View()
	})

	l.AddFixed("mascot", func() bool {
		return !es.mascot.IsHidden()
	}, func(w int) string {
		es.mascot.SetMood(es.mascotMood())
		return es.mascot.View()
	})

	l.AddFixed("helpbar", nil, func(w int) string {
		return es.compositor.HelpBar(es.passed, es.running, es.fileList != nil && es.fileList.Available(), es.loc)
	})

	return l.Render()
}

func (es *ExerciseScreen) renderEditorArea(w, h int) string {
	if h < 1 {
		h = 1
	}

	// Reserve one row for the VIM mode status line when applicable.
	modeLine := es.vimStatusLine()
	modeH := 0
	if modeLine != "" && h > 1 && !es.compositor.InFullscreen() {
		modeH = 1
	}
	contentH := h - modeH

	es.compositor.editorH = contentH
	if es.compositor.InAsmMode() {
		es.assembly.SetCurrentFlags(es.flags.Value())
		return es.assembly.RenderSideBySide(es.editor, w, contentH, true)
	}

	edW := w - editormod.LineNumWidth - 2
	if es.fileList != nil && es.fileList.Available() {
		edW -= fileListWidth + 1
	}
	if edW < 10 {
		edW = 10
	}
	es.editor.SetSize(edW, contentH)

	var result string

	if es.fileList != nil && es.fileList.Available() {
		es.fileList.SetSize(fileListWidth, contentH)
		listLines := strings.Split(es.fileList.View(), "\n")
		editorLines := strings.Split(es.editor.View(), "\n")

		divSt := fileListDividerStyle
		if es.compositor.FocusID() == WidgetFileList {
			divSt = fileListDividerFocStyle
		}
		div := divSt.Render("│")

		isRTL := es.loc.IsRTL()
		var out strings.Builder
		for i := 0; i < contentH; i++ {
			var left, right string
			if i < len(listLines) {
				left = listLines[i]
			}
			if i < len(editorLines) {
				right = editorLines[i]
			}
			if isRTL {
				left, right = right, left
			}
			pad := fileListWidth - lipgloss.Width(left)
			if pad < 0 {
				pad = 0
			}
			out.WriteString(left)
			out.WriteString(strings.Repeat(" ", pad))
			out.WriteString(div)
			out.WriteString(right)
			if i < contentH-1 {
				out.WriteString("\n")
			}
		}
		result = out.String()
	} else {
		result = es.editor.View()
	}

	if modeLine != "" {
		result += "\n" + modeLine
	}
	return result
}

// vimStatusLine returns a styled VIM mode indicator when applicable:
//   - keymap is "vim"
//   - editor is the focused widget
//   - editor reports a non-empty mode
//
// Returns empty string otherwise.
func (es *ExerciseScreen) vimStatusLine() string {
	if es.keymap != "vim" || es.compositor.FocusID() != WidgetEditor {
		return ""
	}
	mode := es.editor.Mode()
	if mode == "" {
		return ""
	}
	return lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(ColorAccent)).
		Render("-- " + mode + " --")
}

func (es *ExerciseScreen) renderInputsSection(w int) string {
	half := w / 2
	es.flags.SetSize(half, 0)
	es.args.SetSize(w-half, 0)
	flagsView := es.flags.View()
	if pad := half - lipgloss.Width(flagsView); pad > 0 {
		flagsView += strings.Repeat(" ", pad)
	}
	es.stdin.SetSize(w, 0)
	return flagsView + es.args.View() + "\n" + es.stdin.View()
}

func (es *ExerciseScreen) renderRunningInput() string {
	T := es.loc.T
	runStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAmber))
	return runStyle.Render(T("exercise.running_label")) + "  " + dim(T("exercise.running_hint")) + "\n" +
		flagsLabelFocusedStyle.Render(T("exercise.label.input")) +
		es.runInput.View() + flagsLabelStyle.Render("]")
}

func (es *ExerciseScreen) renderOutputPanel(w, h int) string {
	borderW := w - 4
	outBorderColor := lipgloss.Color("240")
	if es.compositor.InOutputMode() {
		outBorderColor = lipgloss.Color(ColorAccent)
	}
	bordered := func(content string) string {
		return exOutputStyle.BorderForeground(outBorderColor).Width(borderW).Height(h).Render(content)
	}

	if es.reference.IsVisible() {
		return bordered(es.reference.View())
	}

	T := es.loc.T
	var outputView string
	switch {
	case es.hint.IsVisible():
		outputView = es.hint.View()
	case es.compiling:
		outputView = dim(T("exercise.status.compiling"))
	case es.running && es.output.Content() == "":
		outputView = dim(T("exercise.status.waiting"))
	default:
		outputView = es.output.ViewScrolled(h)
	}
	if outputView == "" && es.previousStars > 0 {
		filled := strings.Repeat("★", es.previousStars)
		empty := strings.Repeat("☆", 3-es.previousStars)
		starDisp := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render(filled + empty)
		if es.previousStars == 3 {
			outputView = starDisp + "  " + dim(T("exercise.status.perfect_prev"))
		} else {
			outputView = dim(T("exercise.status.prev_best_prefix")) + starDisp + dim(T("exercise.status.prev_best_suffix"))
		}
	}
	if outputView == "" {
		outputView = dim(T("exercise.status.empty_output"))
	}

	return bordered(outputView)
}

func (es *ExerciseScreen) renderFullscreen() string {
	l := NewTerminalLayout(es.Width, es.Height)

	l.AddFlex("widget", 1, nil, func(w, h int) string {
		switch es.compositor.FullscreenID() {
		case WidgetEditor:
			return es.renderEditorArea(w, h)
		case WidgetAssembly:
			return es.renderAssemblyFullscreen(w, h)
		case WidgetOutput:
			es.compositor.SetOutputH(h)
			return es.renderOutputPanel(w, h)
		}
		return ""
	})

	if es.compositor.FullscreenID() == WidgetEditor {
		l.AddFixed("vim", func() bool {
			return es.vimStatusLine() != ""
		}, func(w int) string {
			return es.vimStatusLine()
		})
	}

	l.AddFixed("helpbar", nil, func(w int) string {
		return helpBar(es.loc.T("exercise.help.fullscreen_exit"), es.loc.T("exercise.help.fullscreen_back"))
	})

	return l.Render()
}

func (es *ExerciseScreen) renderAssemblyFullscreen(w, h int) string {
	if h < 1 {
		h = 1
	}
	panelW := w - 6
	if panelW < 20 {
		panelW = 20
	}
	contentH := h - 2
	if contentH < 1 {
		contentH = 1
	}
	lines := es.assembly.RenderLines(contentH, panelW, es.flags.Value())

	content := strings.Join(lines, "\n")
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(0, 1).
		Width(w - 4).
		Height(h)

	return borderStyle.Render(content)
}
