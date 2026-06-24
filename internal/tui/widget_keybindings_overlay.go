package tui

import (
	"strings"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type KeybindingsOverlay struct {
	visible bool
	offset  int
	loc     *i18n.Locale
}

func newKeybindingsOverlay(loc *i18n.Locale) *KeybindingsOverlay {
	return &KeybindingsOverlay{loc: loc}
}

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

// prettyKey formats a keybinding constant (e.g. "ctrl+s", "f4", "a") into a
// display string (e.g. "Ctrl+S", "F4", "a"). Bare lowercase letters are kept
// as-is because case matters in terminals; letters following a modifier are
// uppercased. This is the single source of truth for how key names appear in
// the overlay — change a constant in keybindings.go and the display follows.
func prettyKey(k string) string {
	parts := strings.Split(k, "+")
	for i, p := range parts {
		switch p {
		case "ctrl":
			parts[i] = "Ctrl"
		case "alt":
			parts[i] = "Alt"
		case "shift":
			parts[i] = "Shift"
		case "enter":
			parts[i] = "Enter"
		case "tab":
			parts[i] = "Tab"
		case "esc":
			parts[i] = "Esc"
		case "space":
			parts[i] = "Space"
		case "up":
			parts[i] = "Up"
		case "down":
			parts[i] = "Down"
		case "left":
			parts[i] = "Left"
		case "right":
			parts[i] = "Right"
		default:
			if i > 0 && len(p) == 1 {
				// Single letter after a modifier: uppercase (ctrl+s → S)
				parts[i] = strings.ToUpper(p)
			} else if len(p) >= 2 && p[0] == 'f' {
				// Function key: f1 → F1, f10 → F10
				rest := p[1:]
				ok := len(rest) > 0
				for _, c := range rest {
					if c < '0' || c > '9' {
						ok = false
						break
					}
				}
				if ok {
					parts[i] = "F" + rest
				}
			}
			// Bare lowercase letters ("a", "m", "?", ".") are left as-is.
		}
	}
	return strings.Join(parts, "+")
}

func (w *KeybindingsOverlay) allLines() []string {
	T := w.loc.T
	headerStyle := te.NewStyle().Bold(true).Foreground(te.Color("212"))
	descStyle := te.NewStyle().Foreground(te.Color("252"))
	sectionStyle := te.NewStyle().Foreground(te.Color("214")).Bold(true)
	dimStyle := te.NewStyle().Foreground(te.Color("240"))

	pk := prettyKey // shorthand

	type kb struct{ key, desc string }
	bindings := []struct {
		section string
		keys    []kb
	}{
		{T("keybindings.section.run"), []kb{
			{pk(KeyRun), T("keybindings.run")},
			{pk(KeyInteract), T("keybindings.interactive")},
			{pk(KeyAsan), T("keybindings.asan")},
			{pk(KeyGdb), T("keybindings.gdb")},
		}},
		{T("keybindings.section.nav"), []kb{
			{pk(KeyInputs), T("keybindings.cycle_inputs")},
			{pk(KeyStdin), T("keybindings.focus_stdin")},
			{pk(KeyOutput), T("keybindings.toggle_output")},
			{pk(KeyAsm), T("keybindings.toggle_asm")},
			{pk(KeyAsmAnnotate), T("keybindings.asm_annotate")},
			{pk(KeyFileList), T("keybindings.jump_filelist")},
		}},
		{T("keybindings.section.editor"), []kb{
			{pk(KeyBackAlt), T("keybindings.normal_mode")},
			{"i", T("keybindings.insert_mode")},
			{"v", T("keybindings.visual_mode")},
			{"u / Ctrl+R", T("keybindings.undo_redo")},
			{"dd / yy / p", T("keybindings.del_yank_paste")},
			{"gg / G", T("keybindings.top_bottom")},
			{"w / b", T("keybindings.next_prev_word")},
		}},
		{T("keybindings.section.view"), []kb{
			{"j / k", T("keybindings.scroll")},
			{"g / G", T("keybindings.top_end")},
			{"[/]", T("keybindings.resize_split")},
			{pk(KeyFullEditor), T("keybindings.full_editor")},
			{pk(KeyFullAssembly), T("keybindings.full_assembly")},
			{pk(KeyFullOutput), T("keybindings.full_output")},
		}},
		{T("keybindings.section.misc"), []kb{
			{pk(KeyHintEx), T("keybindings.hint")},
			{pk(KeyMochi), T("keybindings.mochi")},
			{pk(KeyTrivia), T("keybindings.trivia")},
			{pk(KeyRef), T("keybindings.references")},
			{pk(KeyTimer), T("keybindings.timer")},
			{pk(KeyHexView), T("keybindings.hex_view")},
			{pk(KeyStructView), T("keybindings.struct_view")},
			{pk(KeyHelp), T("keybindings.open_help")},
			{pk(KeyBackEditor), T("keybindings.back")},
			{pk(KeyLanguage), T("keybindings.lang")},
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
