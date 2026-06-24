package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	editormod "ztutor/internal/editor"
	"ztutor/internal/sandbox"
)

var (
	asmOpcodeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true)
	asmRegStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("119"))
	asmOperStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	asmHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Background(lipgloss.Color("234"))
)

// x86-64 Intel-syntax register names.
var asmRegRegex = regexp.MustCompile(
	`\b(r(?:ax|bx|cx|dx|si|di|sp|bp|8|9|1[0-5]|ip)|` +
		`e(?:ax|bx|cx|dx|si|di|sp|bp|ip)|` +
		`(?:ax|bx|cx|dx|si|di|sp|bp)|` +
		`[abcd][lh]|sil|dil|spl|bpl|` +
		`(?:xmm|ymm|zmm)\d+|` +
		`[cdefgs]s)\b`)

// AssemblyWidget holds the side-by-side assembly panel state.
type AssemblyWidget struct {
	lines        []string
	offset       int
	stale        bool
	splitPct     int // editor's share of total width 20–80 (default 50)
	open         bool
	focused      bool
	flags        string // flags used when the assembly was generated
	lang         sandbox.Language
	width        int
	height       int
	currentFlags string // live flags value for header display
	annotated    bool   // show interleaved source+asm
}

func newAssemblyWidget(lang sandbox.Language) *AssemblyWidget {
	return &AssemblyWidget{lang: lang}
}

func (w *AssemblyWidget) ID() WidgetID    { return WidgetAssembly }
func (w *AssemblyWidget) Available() bool { return w.lang != nil && w.lang.HasAssembly() }
func (w *AssemblyWidget) IsOpen() bool    { return w.open }
func (w *AssemblyWidget) IsStale() bool   { return w.stale }
func (w *AssemblyWidget) Focused() bool   { return w.focused }
func (w *AssemblyWidget) Focus()          { w.focused = true }
func (w *AssemblyWidget) Blur()           { w.focused = false }
func (w *AssemblyWidget) Init() tea.Cmd   { return nil }

func (w *AssemblyWidget) SetLang(lang sandbox.Language) {
	w.lang = lang
	if lang == nil || !lang.HasAssembly() {
		w.open = false
		w.focused = false
	}
}

func (w *AssemblyWidget) Open(lines []string, flags string) {
	expanded := make([]string, len(lines))
	for i, l := range lines {
		expanded[i] = expandTabs(l)
	}
	w.lines = expanded
	w.offset = 0
	w.stale = false
	w.open = true
	w.flags = flags
}

// expandTabs replaces tab characters with spaces using tabstop=8, which
// matches the convention GCC/GNU assembler uses in .s output. This must be
// done at ingestion so lipgloss.Width (which counts \t as 0) is accurate
// throughout the entire rendering pipeline.
func expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			spaces := 8 - (col % 8)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			b.WriteByte(s[i])
			col++
		}
	}
	return b.String()
}

func (w *AssemblyWidget) Close() {
	w.open = false
	w.focused = false
}

func (w *AssemblyWidget) MarkStale() {
	if w.open {
		w.stale = true
	}
}

func (w *AssemblyWidget) ScrollDown(height int) {
	max := len(w.lines) - height
	if max < 0 {
		max = 0
	}
	if w.offset < max {
		w.offset++
	}
}

func (w *AssemblyWidget) ScrollUp() {
	if w.offset > 0 {
		w.offset--
	}
}

func (w *AssemblyWidget) ScrollTop() { w.offset = 0 }

func (w *AssemblyWidget) ScrollBottom(height int) {
	max := len(w.lines) - height
	if max < 0 {
		max = 0
	}
	w.offset = max
}

func (w *AssemblyWidget) ClampOffset(height int) {
	max := len(w.lines) - height
	if max < 0 {
		max = 0
	}
	if w.offset > max {
		w.offset = max
	}
}

func (w *AssemblyWidget) ShrinkEditor() {
	if w.splitPct == 0 {
		w.splitPct = 50
	}
	if w.splitPct > 25 {
		w.splitPct -= 5
	}
}

func (w *AssemblyWidget) GrowEditor() {
	if w.splitPct == 0 {
		w.splitPct = 50
	}
	if w.splitPct < 75 {
		w.splitPct += 5
	}
}

// LeftWidth returns the editor panel's pixel width in the side-by-side view.
func (w *AssemblyWidget) LeftWidth(totalW int) int {
	pct := w.splitPct
	if pct < 20 || pct > 80 {
		pct = 50
	}
	return (totalW - 1) * pct / 100
}

// RightWidth returns the assembly panel's pixel width.
func (w *AssemblyWidget) RightWidth(totalW int) int {
	return totalW - w.LeftWidth(totalW) - 1
}

// SetSize stores the panel dimensions for standalone rendering via View.
func (w *AssemblyWidget) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// SetCurrentFlags stores the live flags value so View and RenderSideBySide can
// show it in the header without the caller passing it on every render call.
func (w *AssemblyWidget) SetCurrentFlags(f string) { w.currentFlags = f }

func (w *AssemblyWidget) Annotated() bool     { return w.annotated }
func (w *AssemblyWidget) SetAnnotated(v bool) { w.annotated = v }
func (w *AssemblyWidget) ToggleAnnotated()    { w.annotated = !w.annotated }

// Update satisfies the Widget interface (key events handled externally).
func (w *AssemblyWidget) Update(msg tea.Msg) (Widget, tea.Cmd) { return w, nil }

// View renders the assembly panel using stored dimensions.
// Returns empty string when dimensions are unset or panel is not open.
func (w *AssemblyWidget) View() string {
	if w.height == 0 || w.width == 0 {
		return ""
	}
	return strings.Join(w.RenderLines(w.height, w.width, w.currentFlags), "\n")
}

// RenderLines returns the assembly panel as a slice of `height` rendered lines.
// currentFlags is the live flags input value; it overrides the stored generation
// flags in the header so the user can see what would change on a refresh.
func (w *AssemblyWidget) RenderLines(height, width int, currentFlags string) []string {
	if height <= 0 {
		return []string{}
	}
	if width < 10 {
		width = 10
	}
	result := make([]string, height)

	const headerLines = 1
	if height <= headerLines {
		result[0] = asmHeaderStyle.Render(padVis(" asm", width))
		return result
	}
	contentH := height - headerLines

	var hdr strings.Builder
	hdr.WriteString(" asm")
	displayFlags := strings.TrimSpace(currentFlags)
	if displayFlags == "" {
		displayFlags = strings.TrimSpace(w.flags)
	}
	if displayFlags != "" {
		hdr.WriteString("  " + displayFlags)
	}
	if len(w.lines) > 0 {
		hdr.WriteString(fmt.Sprintf("  %d/%d", w.offset+1, len(w.lines)))
	}
	if w.annotated {
		hdr.WriteString("  [annotated]")
	}
	if w.stale {
		hdr.WriteString("  [stale — ^A refresh]")
	}
	// clampToWidth truncates a styled ANSI string to at most width visual chars
	// and pads it to exactly width. Applied after highlighting so ANSI codes
	// from syntax coloring are preserved up to the cut point.
	clampToWidth := func(s string) string {
		vw := lipgloss.Width(s)
		if vw > width {
			s = lipgloss.NewStyle().MaxWidth(width).Render(s)
			vw = lipgloss.Width(s)
		}
		if vw < width {
			s += strings.Repeat(" ", width-vw)
		}
		return s
	}

	result[0] = asmHeaderStyle.Render(clampToWidth(hdr.String()))

	if len(w.lines) == 0 {
		result[1] = dim(" ^A to generate assembly")
		return result
	}

	for i := 0; i < contentH; i++ {
		idx := w.offset + i
		if idx >= len(w.lines) {
			break
		}
		line := w.lines[idx]

		// Syntax-highlight the raw line first, then clamp the styled result.
		// Truncating before highlighting would inject ANSI codes into what
		// highlightAsmInstruction treats as plain text, corrupting the output.
		trimmed := strings.TrimSpace(line)
		var rendered string
		switch {
		case w.annotated && strings.HasPrefix(trimmed, "# "):
			rendered = lipgloss.NewStyle().Foreground(lipgloss.Color("246")).Italic(true).Render(line)
		case strings.HasSuffix(trimmed, ":"):
			rendered = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAmber)).Render(line)
		case strings.HasPrefix(trimmed, "."):
			rendered = dim(line)
		default:
			rendered = highlightAsmInstruction(line)
		}
		result[headerLines+i] = clampToWidth(rendered)
	}
	return result
}

// RenderSideBySide renders the editor and assembly panel side by side.
// It sizes the editor via SetSize/View calls, then merges the two panels
// into a single string separated by a vertical divider. The caller is
// responsible for calling SetCurrentFlags before this if flags may have changed.
func (w *AssemblyWidget) RenderSideBySide(editor *EditorWidget, totalW, h int, asmFocused bool) string {
	leftW := w.LeftWidth(totalW)
	rightW := w.RightWidth(totalW)

	edW := leftW - editormod.LineNumWidth - 2
	if edW < 10 {
		edW = 10
		leftW = edW + editormod.LineNumWidth + 2
		rightW = totalW - leftW - 1
	}
	editor.SetSize(edW, h)
	editorLines := strings.Split(editor.View(), "\n")
	asmLines := w.RenderLines(h, rightW, w.currentFlags)

	divColor := lipgloss.Color(ColorBorder)
	if asmFocused {
		divColor = lipgloss.Color(ColorAccent)
	}
	div := lipgloss.NewStyle().Foreground(divColor).Render("│")

	var b strings.Builder
	for i := 0; i < h; i++ {
		var left, right string
		if i < len(editorLines) {
			left = editorLines[i]
		}
		if i < len(asmLines) {
			right = asmLines[i]
		}

		// Clamp both columns to their exact allocated widths so the divider
		// always sits at column leftW and the composite row is always exactly
		// totalW visual chars wide.
		clamp := func(s string, w int) string {
			vw := lipgloss.Width(s)
			if vw > w {
				s = lipgloss.NewStyle().MaxWidth(w).Render(s)
				vw = lipgloss.Width(s)
			}
			if vw < w {
				s += strings.Repeat(" ", w-vw)
			}
			return s
		}
		left = clamp(left, leftW)
		right = clamp(right, rightW)

		b.WriteString(left)
		b.WriteString(div)
		b.WriteString(right)
		if i < h-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// colorizeAsmOperands colorizes register names in an operand string.
func colorizeAsmOperands(s string) string {
	idxs := asmRegRegex.FindAllStringIndex(s, -1)
	if len(idxs) == 0 {
		return asmOperStyle.Render(s)
	}
	var b strings.Builder
	prev := 0
	for _, m := range idxs {
		if m[0] > prev {
			b.WriteString(asmOperStyle.Render(s[prev:m[0]]))
		}
		b.WriteString(asmRegStyle.Render(s[m[0]:m[1]]))
		prev = m[1]
	}
	if prev < len(s) {
		b.WriteString(asmOperStyle.Render(s[prev:]))
	}
	return b.String()
}

// highlightAsmInstruction applies syntax coloring to a single assembly instruction line.
func highlightAsmInstruction(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return line
	}
	lead := line[:len(line)-len(trimmed)]
	opcodeEnd := strings.IndexAny(trimmed, " \t")
	if opcodeEnd < 0 {
		return lead + asmOpcodeStyle.Render(trimmed)
	}
	opcode := trimmed[:opcodeEnd]
	rest := trimmed[opcodeEnd:]
	return lead + asmOpcodeStyle.Render(opcode) + colorizeAsmOperands(rest)
}
