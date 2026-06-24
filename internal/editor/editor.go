package editor

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	monokaiStyleOnce sync.Once
	monokaiStyle     *chroma.Style

	editorLineNumStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	editorCursorLineNumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	editorErrorLineNumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	editorWarningLineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func ensureMonokai() {
	monokaiStyleOnce.Do(func() {
		monokaiStyle = styles.Get("monokai")
		if monokaiStyle == nil {
			monokaiStyle = styles.Fallback
		}
	})
}

type editorMode int

const (
	modeInsert editorMode = iota
	modeNormal
)

type visualKind int

const (
	visualNone visualKind = iota
	visualLine
)

const maxUndoSteps = 200

type editorSnapshot struct {
	lines [][]rune
	row   int
	col   int
}

// CodeEditor is a syntax-highlighted, optionally vim-modal code editor.
type CodeEditor struct {
	lines       [][]rune
	row         int
	col         int
	offset      int
	width       int
	height      int
	focused     bool
	Diagnostics map[int]string // 1-indexed line → "error"/"warning"/"note"

	vimMode   bool
	mode      editorMode
	pendingOp string // multi-key sequences: "d", "g", "y", "r"

	visual    visualKind
	anchorRow int

	register [][]rune // unnamed yank register (per-editor, not shared)

	undoStack     []editorSnapshot
	redoStack     []editorSnapshot
	preInsertSnap *editorSnapshot // snapshot taken on entering insert mode

	lexer           chroma.Lexer
	tokenStyleCache map[chroma.TokenType]lipgloss.Style
}

func New(content string, width, height int, langName string) *CodeEditor {
	ensureMonokai()
	lexer := lexers.Get(langName)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	return &CodeEditor{
		lines:           splitRuneLines(content),
		width:           width,
		height:          height,
		lexer:           lexer,
		tokenStyleCache: make(map[chroma.TokenType]lipgloss.Style),
	}
}

func splitRuneLines(content string) [][]rune {
	strs := strings.Split(content, "\n")
	lines := make([][]rune, len(strs))
	for i, s := range strs {
		lines[i] = []rune(s)
	}
	return lines
}

func (e *CodeEditor) SetContent(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	e.lines = splitRuneLines(s)
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.row, e.col, e.offset = 0, 0, 0
	e.mode = modeNormal
	e.pendingOp = ""
	e.visual = visualNone
	e.undoStack = nil
	e.redoStack = nil
	e.preInsertSnap = nil
}

func (e *CodeEditor) Focus()        { e.focused = true }
func (e *CodeEditor) Blur()         { e.focused = false }
func (e *CodeEditor) Focused() bool { return e.focused }
func (e *CodeEditor) Init() tea.Cmd { return nil }
func (e *CodeEditor) Row() int      { return e.row }

func (e *CodeEditor) SetVimMode(on bool) {
	e.vimMode = on
	if on {
		e.mode = modeNormal
	} else {
		e.mode = modeInsert
	}
}

// Mode returns "" for default keybindings, or "NORMAL"/"INSERT"/"VISUAL LINE" for vim.
func (e *CodeEditor) Mode() string {
	if !e.vimMode {
		return ""
	}
	if e.visual == visualLine {
		return "VISUAL LINE"
	}
	if e.mode == modeNormal {
		return "NORMAL"
	}
	return "INSERT"
}

func (e *CodeEditor) Value() string {
	strs := make([]string, len(e.lines))
	for i, r := range e.lines {
		strs[i] = string(r)
	}
	return strings.Join(strs, "\n")
}

func (e *CodeEditor) SetSize(w, h int) {
	e.width = w
	e.height = h
	e.clampOffset()
}

func (e *CodeEditor) SetLanguage(langName string) {
	lexer := lexers.Get(langName)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	e.lexer = chroma.Coalesce(lexer)
	e.tokenStyleCache = make(map[chroma.TokenType]lipgloss.Style)
}

// ── Clamp helpers ────────────────────────────────────────────────────────────

func (e *CodeEditor) clampCol() {
	max := len(e.lines[e.row])
	if e.vimMode && e.mode == modeNormal && max > 0 {
		max-- // cursor rests on last char, not past it
	}
	if e.col > max {
		e.col = max
	}
	if e.col < 0 {
		e.col = 0
	}
}

func (e *CodeEditor) clampOffset() {
	if e.row < e.offset {
		e.offset = e.row
	}
	if e.row >= e.offset+e.height {
		e.offset = e.row - e.height + 1
	}
	if e.offset < 0 {
		e.offset = 0
	}
}

// ── Undo / redo ──────────────────────────────────────────────────────────────

func (e *CodeEditor) snapshot() editorSnapshot {
	lines := make([][]rune, len(e.lines))
	for i, l := range e.lines {
		lines[i] = append([]rune(nil), l...)
	}
	return editorSnapshot{lines: lines, row: e.row, col: e.col}
}

func (e *CodeEditor) pushUndo() {
	s := e.snapshot()
	e.undoStack = append(e.undoStack, s)
	if len(e.undoStack) > maxUndoSteps {
		e.undoStack = e.undoStack[1:]
	}
	e.redoStack = nil
}

func (e *CodeEditor) applySnapshot(s editorSnapshot) {
	e.lines = s.lines
	e.row = s.row
	e.col = s.col
	e.clampOffset()
}

func (e *CodeEditor) undo() {
	if len(e.undoStack) == 0 {
		return
	}
	e.redoStack = append(e.redoStack, e.snapshot())
	s := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]
	e.applySnapshot(s)
}

func (e *CodeEditor) redo() {
	if len(e.redoStack) == 0 {
		return
	}
	e.undoStack = append(e.undoStack, e.snapshot())
	s := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]
	e.applySnapshot(s)
}

// commitInsertUndo pushes the pre-insert snapshot to the undo stack if the
// content has actually changed since entering insert mode.
func (e *CodeEditor) commitInsertUndo() {
	if e.preInsertSnap == nil {
		return
	}
	snap := *e.preInsertSnap
	e.preInsertSnap = nil
	// Compare content — don't pollute undo stack on no-op insert sessions.
	snapVal := make([]string, len(snap.lines))
	for i, l := range snap.lines {
		snapVal[i] = string(l)
	}
	if strings.Join(snapVal, "\n") == e.Value() {
		return
	}
	e.undoStack = append(e.undoStack, snap)
	if len(e.undoStack) > maxUndoSteps {
		e.undoStack = e.undoStack[1:]
	}
	e.redoStack = nil
}

// ── Visual helpers ────────────────────────────────────────────────────────────

func (e *CodeEditor) visualRange() (start, end int) {
	start, end = e.anchorRow, e.row
	if start > end {
		start, end = end, start
	}
	return
}

func (e *CodeEditor) yankVisual() {
	start, end := e.visualRange()
	e.register = make([][]rune, end-start+1)
	for i, row := range e.lines[start : end+1] {
		e.register[i] = append([]rune(nil), row...)
	}
	e.row = start
	e.col = 0
}

func (e *CodeEditor) deleteVisual() {
	e.yankVisual() // fills register, sets row/col
	start, end := e.anchorRow, e.row
	if start > end {
		start, end = end, start
	}
	e.lines = append(e.lines[:start], e.lines[end+1:]...)
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.row = start
	if e.row >= len(e.lines) {
		e.row = len(e.lines) - 1
	}
	e.col = 0
	e.clampOffset()
}

func (e *CodeEditor) indentVisual(add bool) {
	start, end := e.visualRange()
	for i := start; i <= end; i++ {
		if add {
			e.lines[i] = append([]rune("    "), e.lines[i]...)
		} else {
			for j := 0; j < 4 && len(e.lines[i]) > 0 && e.lines[i][0] == ' '; j++ {
				e.lines[i] = e.lines[i][1:]
			}
		}
	}
}

// ── Paste ─────────────────────────────────────────────────────────────────────

func (e *CodeEditor) pasteAfter() {
	if len(e.register) == 0 {
		return
	}
	newLines := make([][]rune, len(e.lines)+len(e.register))
	copy(newLines, e.lines[:e.row+1])
	for i, l := range e.register {
		newLines[e.row+1+i] = append([]rune(nil), l...)
	}
	copy(newLines[e.row+1+len(e.register):], e.lines[e.row+1:])
	e.lines = newLines
	e.row++
	e.col = 0
	e.clampOffset()
}

func (e *CodeEditor) pasteBefore() {
	if len(e.register) == 0 {
		return
	}
	newLines := make([][]rune, len(e.lines)+len(e.register))
	copy(newLines, e.lines[:e.row])
	for i, l := range e.register {
		newLines[e.row+i] = append([]rune(nil), l...)
	}
	copy(newLines[e.row+len(e.register):], e.lines[e.row:])
	e.lines = newLines
	e.col = 0
	e.clampOffset()
}

// ── Line operations ───────────────────────────────────────────────────────────

func (e *CodeEditor) deleteLine() {
	e.register = [][]rune{append([]rune(nil), e.lines[e.row]...)}
	if len(e.lines) == 1 {
		e.lines[0] = []rune{}
		e.col = 0
		return
	}
	e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
	if e.row >= len(e.lines) {
		e.row = len(e.lines) - 1
	}
	e.clampCol()
	e.clampOffset()
}

func (e *CodeEditor) deleteWord() {
	next := e.wordForward()
	line := e.lines[e.row]
	e.lines[e.row] = append(line[:e.col:e.col], line[next:]...)
}

func (e *CodeEditor) openLineBelow() {
	newLines := make([][]rune, len(e.lines)+1)
	copy(newLines, e.lines[:e.row+1])
	newLines[e.row+1] = []rune{}
	copy(newLines[e.row+2:], e.lines[e.row+1:])
	e.lines = newLines
	e.row++
	e.col = 0
	e.clampOffset()
}

func (e *CodeEditor) openLineAbove() {
	newLines := make([][]rune, len(e.lines)+1)
	copy(newLines, e.lines[:e.row])
	newLines[e.row] = []rune{}
	copy(newLines[e.row+1:], e.lines[e.row:])
	e.lines = newLines
	e.col = 0
	e.clampOffset()
}

// ── Word motion ───────────────────────────────────────────────────────────────

func isWordChar(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func (e *CodeEditor) wordForward() int {
	line := e.lines[e.row]
	col := e.col
	if col >= len(line) {
		return col
	}
	if isWordChar(line[col]) {
		for col < len(line) && isWordChar(line[col]) {
			col++
		}
	} else {
		for col < len(line) && !isWordChar(line[col]) && line[col] != ' ' {
			col++
		}
	}
	for col < len(line) && line[col] == ' ' {
		col++
	}
	return col
}

func (e *CodeEditor) wordBackward() int {
	line := e.lines[e.row]
	col := e.col
	if col == 0 {
		return 0
	}
	col--
	for col > 0 && line[col] == ' ' {
		col--
	}
	if isWordChar(line[col]) {
		for col > 0 && isWordChar(line[col-1]) {
			col--
		}
	} else {
		for col > 0 && !isWordChar(line[col-1]) && line[col-1] != ' ' {
			col--
		}
	}
	return col
}

func (e *CodeEditor) wordEnd() int {
	line := e.lines[e.row]
	col := e.col
	if col >= len(line)-1 {
		return col
	}
	col++
	for col < len(line) && line[col] == ' ' {
		col++
	}
	if col < len(line) && isWordChar(line[col]) {
		for col < len(line)-1 && isWordChar(line[col+1]) {
			col++
		}
	} else {
		for col < len(line)-1 && !isWordChar(line[col+1]) && line[col+1] != ' ' {
			col++
		}
	}
	return col
}

// ── Update ────────────────────────────────────────────────────────────────────

func (e *CodeEditor) Update(msg tea.Msg) (*CodeEditor, tea.Cmd) {
	if !e.focused {
		return e, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return e, nil
	}

	if e.vimMode {
		// Visual mode intercepts all keys.
		if e.visual == visualLine {
			return e.updateVisual(km)
		}

		if e.mode == modeNormal {
			return e.updateNormal(km)
		}

		// Insert mode: ESC exits back to normal.
		if km.String() == "esc" {
			e.commitInsertUndo()
			e.mode = modeNormal
			e.pendingOp = ""
			if e.col > 0 {
				e.col--
			}
			return e, nil
		}
	}

	return e.updateInsert(km)
}

// updateVisual handles keys while in visual line mode.
func (e *CodeEditor) updateVisual(km tea.KeyMsg) (*CodeEditor, tea.Cmd) {
	key := km.String()

	// Pending gg in visual mode.
	if e.pendingOp == "g" {
		e.pendingOp = ""
		if key == "g" {
			e.row = 0
			e.clampOffset()
		}
		return e, nil
	}

	switch key {
	case "esc":
		e.visual = visualNone
	case "y":
		e.yankVisual()
		e.visual = visualNone
	case "d", "x":
		e.pushUndo()
		e.deleteVisual()
		e.visual = visualNone
	case "c":
		e.pushUndo()
		e.deleteVisual()
		e.visual = visualNone
		e.mode = modeInsert
		snap := e.snapshot()
		e.preInsertSnap = &snap
	case ">":
		e.pushUndo()
		e.indentVisual(true)
	case "<":
		e.pushUndo()
		e.indentVisual(false)
	case "j", "down":
		if e.row < len(e.lines)-1 {
			e.row++
			e.clampOffset()
		}
	case "k", "up":
		if e.row > 0 {
			e.row--
			e.clampOffset()
		}
	case "G":
		e.row = len(e.lines) - 1
		e.clampOffset()
	case "g":
		e.pendingOp = "g"
	}
	return e, nil
}

// updateNormal handles keys in vim normal mode.
func (e *CodeEditor) updateNormal(km tea.KeyMsg) (*CodeEditor, tea.Cmd) {
	key := km.String()

	// ── Resolve pending operator ─────────────────────────────────────────────
	if e.pendingOp == "d" {
		e.pendingOp = ""
		switch key {
		case "d":
			e.pushUndo()
			e.deleteLine()
		case "w":
			e.pushUndo()
			e.deleteWord()
		case "$":
			e.pushUndo()
			e.lines[e.row] = append([]rune(nil), e.lines[e.row][:e.col]...)
		case "0":
			e.pushUndo()
			e.lines[e.row] = append([]rune(nil), e.lines[e.row][e.col:]...)
			e.col = 0
		}
		e.clampCol()
		return e, nil
	}
	if e.pendingOp == "y" {
		e.pendingOp = ""
		if key == "y" {
			e.register = [][]rune{append([]rune(nil), e.lines[e.row]...)}
		}
		return e, nil
	}
	if e.pendingOp == "g" {
		e.pendingOp = ""
		if key == "g" {
			e.row, e.col = 0, 0
			e.clampOffset()
		}
		return e, nil
	}
	if e.pendingOp == "r" {
		e.pendingOp = ""
		if len(km.Runes) == 1 && km.Runes[0] >= 32 && e.col < len(e.lines[e.row]) {
			e.pushUndo()
			e.lines[e.row][e.col] = km.Runes[0]
		}
		return e, nil
	}

	// ── Motion ────────────────────────────────────────────────────────────────
	switch key {
	case "h", "left":
		if e.col > 0 {
			e.col--
		}
	case "l", "right":
		if e.col < len(e.lines[e.row])-1 {
			e.col++
		}
	case "j", "down":
		if e.row < len(e.lines)-1 {
			e.row++
			e.clampCol()
			e.clampOffset()
		}
	case "k", "up":
		if e.row > 0 {
			e.row--
			e.clampCol()
			e.clampOffset()
		}
	case "0":
		e.col = 0
	case "^":
		for i, r := range e.lines[e.row] {
			if r != ' ' && r != '\t' {
				e.col = i
				break
			}
		}
	case "$":
		e.col = len(e.lines[e.row])
		if e.col > 0 {
			e.col--
		}
	case "w":
		e.col = e.wordForward()
	case "b":
		e.col = e.wordBackward()
	case "e":
		e.col = e.wordEnd()
	case "G":
		e.row = len(e.lines) - 1
		e.clampCol()
		e.clampOffset()
	case "g":
		e.pendingOp = "g"

	// ── Enter insert mode ─────────────────────────────────────────────────────
	case "i":
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.mode = modeInsert
	case "a":
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.mode = modeInsert
		if e.col < len(e.lines[e.row]) {
			e.col++
		}
	case "A":
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.mode = modeInsert
		e.col = len(e.lines[e.row])
	case "I":
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.mode = modeInsert
		e.col = 0
	case "o":
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.openLineBelow()
		e.mode = modeInsert
	case "O":
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.openLineAbove()
		e.mode = modeInsert

	// ── Visual ────────────────────────────────────────────────────────────────
	case "V":
		e.visual = visualLine
		e.anchorRow = e.row

	// ── Undo / redo ───────────────────────────────────────────────────────────
	case "u":
		e.undo()
	case "ctrl+r":
		e.redo()

	// ── Yank / paste ──────────────────────────────────────────────────────────
	case "y":
		e.pendingOp = "y"
	case "p":
		if len(e.register) > 0 {
			e.pushUndo()
			e.pasteAfter()
		}
	case "P":
		if len(e.register) > 0 {
			e.pushUndo()
			e.pasteBefore()
		}

	// ── Editing ───────────────────────────────────────────────────────────────
	case "x":
		if e.col < len(e.lines[e.row]) {
			e.pushUndo()
			line := e.lines[e.row]
			e.register = [][]rune{{line[e.col]}}
			e.lines[e.row] = append(line[:e.col:e.col], line[e.col+1:]...)
			e.clampCol()
		}
	case "X":
		if e.col > 0 {
			e.pushUndo()
			line := e.lines[e.row]
			e.lines[e.row] = append(line[:e.col-1:e.col-1], line[e.col:]...)
			e.col--
		}
	case "d":
		e.pendingOp = "d"
	case "D":
		e.pushUndo()
		e.lines[e.row] = append([]rune(nil), e.lines[e.row][:e.col]...)
		e.clampCol()
	case "s":
		e.pushUndo()
		if e.col < len(e.lines[e.row]) {
			line := e.lines[e.row]
			e.lines[e.row] = append(line[:e.col:e.col], line[e.col+1:]...)
		}
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.mode = modeInsert
	case "S":
		e.pushUndo()
		e.lines[e.row] = []rune{}
		e.col = 0
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.mode = modeInsert
	case "C":
		e.pushUndo()
		e.lines[e.row] = append([]rune(nil), e.lines[e.row][:e.col]...)
		snap := e.snapshot()
		e.preInsertSnap = &snap
		e.mode = modeInsert
	case "r":
		e.pendingOp = "r"
	case ">":
		e.pushUndo()
		e.lines[e.row] = append([]rune("    "), e.lines[e.row]...)
	case "<":
		e.pushUndo()
		for j := 0; j < 4 && len(e.lines[e.row]) > 0 && e.lines[e.row][0] == ' '; j++ {
			e.lines[e.row] = e.lines[e.row][1:]
		}
		e.clampCol()
	}

	return e, nil
}

// updateInsert handles keys in default / vim-insert mode.
func (e *CodeEditor) updateInsert(km tea.KeyMsg) (*CodeEditor, tea.Cmd) {
	switch km.String() {
	case "up":
		if e.row > 0 {
			e.row--
			e.clampCol()
			e.clampOffset()
		}
	case "down":
		if e.row < len(e.lines)-1 {
			e.row++
			e.clampCol()
			e.clampOffset()
		}
	case "left":
		if e.col > 0 {
			e.col--
		} else if e.row > 0 {
			e.row--
			e.col = len(e.lines[e.row])
			e.clampOffset()
		}
	case "right":
		if e.col < len(e.lines[e.row]) {
			e.col++
		} else if e.row < len(e.lines)-1 {
			e.row++
			e.col = 0
			e.clampOffset()
		}
	case "home", "ctrl+a":
		e.col = 0
	case "end", "ctrl+e":
		e.col = len(e.lines[e.row])
	case "enter":
		before := append([]rune(nil), e.lines[e.row][:e.col]...)
		after := append([]rune(nil), e.lines[e.row][e.col:]...)
		newLines := make([][]rune, len(e.lines)+1)
		copy(newLines, e.lines[:e.row+1])
		newLines[e.row] = before
		newLines[e.row+1] = after
		copy(newLines[e.row+2:], e.lines[e.row+1:])
		e.lines = newLines
		e.row++
		e.col = 0
		e.clampOffset()
	case "backspace":
		if e.col > 0 {
			line := e.lines[e.row]
			e.lines[e.row] = append(line[:e.col-1:e.col-1], line[e.col:]...)
			e.col--
		} else if e.row > 0 {
			prevLen := len(e.lines[e.row-1])
			e.lines[e.row-1] = append(e.lines[e.row-1], e.lines[e.row]...)
			e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
			e.row--
			e.col = prevLen
			e.clampOffset()
		}
	case "delete":
		if e.col < len(e.lines[e.row]) {
			line := e.lines[e.row]
			e.lines[e.row] = append(line[:e.col:e.col], line[e.col+1:]...)
		} else if e.row < len(e.lines)-1 {
			e.lines[e.row] = append(e.lines[e.row], e.lines[e.row+1]...)
			e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
		}
	case "tab":
		spaces := []rune("    ")
		line := e.lines[e.row]
		newLine := make([]rune, len(line)+4)
		copy(newLine, line[:e.col])
		copy(newLine[e.col:], spaces)
		copy(newLine[e.col+4:], line[e.col:])
		e.lines[e.row] = newLine
		e.col += 4
	default:
		runes := km.Runes
		if len(runes) == 1 && runes[0] >= 32 {
			ch := runes[0]
			line := e.lines[e.row]
			newLine := make([]rune, len(line)+1)
			copy(newLine, line[:e.col])
			newLine[e.col] = ch
			copy(newLine[e.col+1:], line[e.col:])
			e.lines[e.row] = newLine
			e.col++
		}
	}
	return e, nil
}

// ── Syntax highlighting ───────────────────────────────────────────────────────

func (e *CodeEditor) getTokenStyle(t chroma.TokenType) lipgloss.Style {
	if s, ok := e.tokenStyleCache[t]; ok {
		return s
	}
	entry := monokaiStyle.Get(t)
	s := lipgloss.NewStyle()
	if entry.Colour.IsSet() {
		s = s.Foreground(lipgloss.Color(entry.Colour.String()))
	} else {
		s = s.Foreground(lipgloss.Color("252"))
	}
	e.tokenStyleCache[t] = s
	return s
}

func (e *CodeEditor) highlightedLines() []string {
	code := e.Value()
	iter, err := e.lexer.Tokenise(nil, code)
	if err != nil {
		return strings.Split(code, "\n")
	}

	var cur bytes.Buffer
	var result []string

	for {
		tok := iter()
		if tok == chroma.EOF {
			break
		}
		style := e.getTokenStyle(tok.Type)
		parts := strings.Split(tok.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				result = append(result, cur.String())
				cur.Reset()
			}
			if part != "" {
				cur.WriteString(style.Render(part))
			}
		}
	}
	result = append(result, cur.String())
	return result
}

// injectCursor inserts a reverse-video block cursor at rune position col,
// correctly skipping ANSI CSI sequences.
func injectCursor(ansi string, col int) string {
	var out strings.Builder
	pos := 0
	i := 0
	injected := false

	for i < len(ansi) {
		if ansi[i] == '\033' && i+1 < len(ansi) && ansi[i+1] == '[' {
			j := i + 2
			for j < len(ansi) && (ansi[j] < 0x40 || ansi[j] > 0x7E) {
				j++
			}
			if j < len(ansi) {
				j++
			}
			out.WriteString(ansi[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(ansi[i:])
		if !injected && pos == col {
			out.WriteString("\033[7m")
			out.WriteRune(r)
			out.WriteString("\033[27m")
			injected = true
		} else {
			out.WriteRune(r)
		}
		pos++
		i += size
	}

	if !injected {
		out.WriteString("\033[7m \033[27m")
	}
	return out.String()
}

const LineNumWidth = 4

var visualSelectStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("24")).
	Foreground(lipgloss.Color("252"))

func (e *CodeEditor) View() string {
	hlLines := e.highlightedLines()
	for len(hlLines) < len(e.lines) {
		hlLines = append(hlLines, "")
	}

	end := e.offset + e.height
	if end > len(e.lines) {
		end = len(e.lines)
	}

	// Compute visual selection range once.
	selStart, selEnd := -1, -1
	if e.visual == visualLine {
		selStart, selEnd = e.visualRange()
	}

	var b strings.Builder
	for i := e.offset; i < end; i++ {
		// Line number: cursor line > diagnostic line > plain.
		num := fmt.Sprintf("%3d ", i+1)
		switch {
		case i == e.row:
			b.WriteString(editorCursorLineNumStyle.Render(num))
		case e.Diagnostics[i+1] == "error":
			b.WriteString(editorErrorLineNumStyle.Render(num))
		case e.Diagnostics[i+1] == "warning":
			b.WriteString(editorWarningLineNumStyle.Render(num))
		default:
			b.WriteString(editorLineNumStyle.Render(num))
		}

		// Content: visual selection overrides syntax highlighting.
		if selStart >= 0 && i >= selStart && i <= selEnd {
			b.WriteString(visualSelectStyle.Width(e.width).Render(string(e.lines[i])))
		} else if i == e.row && e.focused {
			lineHL := ""
			if i < len(hlLines) {
				lineHL = hlLines[i]
			}
			b.WriteString(injectCursor(lineHL, e.col))
		} else {
			if i < len(hlLines) {
				b.WriteString(hlLines[i])
			}
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
