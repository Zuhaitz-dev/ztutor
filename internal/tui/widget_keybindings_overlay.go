package tui

import (
	"strings"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type KeybindingsOverlay struct {
	visible    bool
	offset     int
	loc        *i18n.Locale
	hasGamepad bool
}

func newKeybindingsOverlay(loc *i18n.Locale) *KeybindingsOverlay {
	return &KeybindingsOverlay{loc: loc}
}

func (w *KeybindingsOverlay) SetHasGamepad(v bool) { w.hasGamepad = v }

func (w *KeybindingsOverlay) Available() bool { return true }

func (w *KeybindingsOverlay) Toggle() {
	w.visible = !w.visible
	w.offset = 0
}
func (w *KeybindingsOverlay) Hide()           { w.visible = false }
func (w *KeybindingsOverlay) IsVisible() bool { return w.visible }
func (w *KeybindingsOverlay) Focused() bool   { return w.visible }

func (w *KeybindingsOverlay) ScrollDown(maxHeight int) {
	if maxHeight < 1 {
		maxHeight = 1
	}
	visible := maxHeight - 1
	if visible < 1 {
		visible = 1
	}
	max := len(w.allLines()) - visible
	if max < 0 {
		max = 0
	}
	if w.offset < max {
		w.offset++
	}
}

func (w *KeybindingsOverlay) ScrollUp() {
	if w.offset > 0 {
		w.offset--
	}
}

func (w *KeybindingsOverlay) ScrollTop() { w.offset = 0 }

func (w *KeybindingsOverlay) ScrollBottom(maxHeight int) {
	if maxHeight < 1 {
		maxHeight = 1
	}
	visible := maxHeight - 1
	if visible < 1 {
		visible = 1
	}
	max := len(w.allLines()) - visible
	if max < 0 {
		max = 0
	}
	w.offset = max
}

func (w *KeybindingsOverlay) allLines() []string {
	T := w.loc.T
	headerStyle := te.NewStyle().Bold(true).Foreground(te.Color("212"))
	descStyle := te.NewStyle().Foreground(te.Color("252"))
	sectionStyle := te.NewStyle().Foreground(te.Color("214")).Bold(true)
	dimStyle := te.NewStyle().Foreground(te.Color("240"))

	type kb struct{ key, desc string }
	type kbSection struct {
		section string
		keys    []kb
	}
	actionKB := func(id KeyActionID, labelKey string) kb {
		action, ok := LookupKeyAction(id)
		if !ok {
			return kb{string(id), T(labelKey)}
		}
		return kb{keyDisplay(action.Keys), T(labelKey)}
	}
	keysKB := func(keys []string, labelKey string) kb {
		return kb{keyDisplay(keys), T(labelKey)}
	}

	var controllerSection kbSection
	if w.hasGamepad {
		controllerSection = kbSection{T("keybindings.section.controller_native"), []kb{
			{"A / Cross", T("keybindings.controller_select")},
			{"B / Circle", T("keybindings.controller_back")},
			{"D-pad", T("keybindings.controller_move")},
			{"X / Square", T("keybindings.controller_run")},
			{"Y / Triangle", T("keybindings.controller_hint")},
			{"LB / L1", T("keybindings.controller_prev")},
			{"RB / R1", T("keybindings.controller_next")},
			{"Start", T("keybindings.controller_open_help")},
		}}
	} else {
		controllerSection = kbSection{T("keybindings.section.controller"), []kb{
			keysKB([]string{"f13"}, "keybindings.controller_select"),
			keysKB([]string{"f14"}, "keybindings.controller_back"),
			keysKB([]string{"f15", "f16", "f17", "f18"}, "keybindings.controller_move"),
			keysKB([]string{"f19"}, "keybindings.controller_run"),
			keysKB([]string{"f20"}, "keybindings.controller_hint"),
		}}
	}

	bindings := []kbSection{
		{T("keybindings.section.run"), []kb{
			actionKB(ActionRun, "keybindings.run"),
			actionKB(ActionInteractive, "keybindings.interactive"),
			actionKB(ActionASAN, "keybindings.asan"),
			actionKB(ActionGDB, "keybindings.gdb"),
		}},
		{T("keybindings.section.nav"), []kb{
			actionKB(ActionInputs, "keybindings.cycle_inputs"),
			keysKB([]string{KeyStdin}, "keybindings.focus_stdin"),
			actionKB(ActionOutput, "keybindings.toggle_output"),
			actionKB(ActionAssembly, "keybindings.toggle_asm"),
			actionKB(ActionAsmAnnotate, "keybindings.asm_annotate"),
			actionKB(ActionFileList, "keybindings.jump_filelist"),
		}},
		{T("keybindings.section.editor"), []kb{
			keysKB([]string{KeyBackAlt}, "keybindings.normal_mode"),
			{"i", T("keybindings.insert_mode")},
			{"v", T("keybindings.visual_mode")},
			{"u / Ctrl+R", T("keybindings.undo_redo")},
			{"dd / yy / p", T("keybindings.del_yank_paste")},
			{"gg / G", T("keybindings.top_bottom")},
			{"w / b", T("keybindings.next_prev_word")},
		}},
		{T("keybindings.section.view"), []kb{
			actionKB(ActionScroll, "keybindings.scroll"),
			actionKB(ActionTopEnd, "keybindings.top_end"),
			{"[/]", T("keybindings.resize_split")},
			actionKB(ActionFullEditor, "keybindings.full_editor"),
			actionKB(ActionFullAssembly, "keybindings.full_assembly"),
			actionKB(ActionFullOutput, "keybindings.full_output"),
		}},
		controllerSection,
		{T("keybindings.section.misc"), []kb{
			actionKB(ActionHint, "keybindings.hint"),
			actionKB(ActionMochi, "keybindings.mochi"),
			actionKB(ActionTrivia, "keybindings.trivia"),
			actionKB(ActionReferences, "keybindings.references"),
			actionKB(ActionTimer, "keybindings.timer"),
			actionKB(ActionHexView, "keybindings.hex_view"),
			actionKB(ActionStructView, "keybindings.struct_view"),
			actionKB(ActionHelp, "keybindings.open_help"),
			actionKB(ActionExerciseBack, "keybindings.back"),
			actionKB(ActionLanguage, "keybindings.lang"),
		}},
	}

	var lines []string
	rtl := w.loc.IsRTL()
	lines = append(lines, headerStyle.Render(" "+T("keybindings.title")+" "))
	lines = append(lines, dimStyle.Render(strings.Repeat("─", 40)))

	for _, section := range bindings {
		lines = append(lines, "")
		if rtl {
			lines = append(lines, sectionStyle.Render(section.section+dirArrow(rtl)))
		} else {
			lines = append(lines, sectionStyle.Render(dirArrow(rtl)+section.section))
		}
		for _, k := range section.keys {
			key := renderKey(k.key)
			desc := descStyle.Render(k.desc)
			// renderKey adds 1-char padding on each side, so measure after render.
			pad := 19 - te.Width(key)
			if pad < 2 {
				pad = 2
			}
			if rtl {
				lines = append(lines, "  "+desc+strings.Repeat(" ", pad)+key)
			} else {
				lines = append(lines, "  "+key+strings.Repeat(" ", pad)+desc)
			}
		}
	}
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render(T("keybindings.close_hint")))
	return lines
}

func (w *KeybindingsOverlay) View(loc *i18n.Locale, width, height int) string {
	if !w.visible {
		return ""
	}
	w.loc = loc
	lines := w.allLines()

	start := w.offset
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	visible := strings.Join(lines[start:end], "\n")

	borderStyle := te.NewStyle().
		BorderStyle(te.RoundedBorder()).
		BorderForeground(te.Color("212")).
		Padding(1, 2).
		Width(width - 4)

	if loc.IsRTL() {
		borderStyle = borderStyle.Align(te.Right)
	}

	return borderStyle.Render(visible)
}
