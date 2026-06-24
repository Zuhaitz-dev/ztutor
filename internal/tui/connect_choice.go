package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ztutor/internal/i18n"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

// ── Tick ─────────────────────────────────────────────────────────────────────

type connectCodeTick struct{}

func connectCodeTickCmd() tea.Cmd {
	return tea.Tick(65*time.Millisecond, func(time.Time) tea.Msg { return connectCodeTick{} })
}

// ── Syntax highlight cache ────────────────────────────────────────────────────

var (
	connectHighlights    [][]string // [snippetIdx][lineIdx] → ANSI-colored line
	connectHighlightOnce sync.Once
	connectChromaStyle   = styles.Get("monokai")
)

func ensureConnectHighlights() {
	connectHighlightOnce.Do(func() {
		if connectChromaStyle == nil {
			connectChromaStyle = styles.Fallback
		}
		connectHighlights = make([][]string, len(connectSnippets))
		for i, s := range connectSnippets {
			code := strings.Join(s.lines, "\n")
			connectHighlights[i] = chromaLines(s.langID, code, len(s.lines))
		}
	})
}

// chromaLines tokenises code with chroma (monokai) and returns exactly wantN
// ANSI-coloured lines. Trailing empty lines are added or trimmed as needed.
func chromaLines(langID, code string, wantN int) []string {
	lexer := lexers.Get(langID)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iter, err := lexer.Tokenise(nil, code)
	if err != nil {
		raw := strings.Split(code, "\n")
		for len(raw) < wantN {
			raw = append(raw, "")
		}
		return raw[:wantN]
	}

	var (
		cur    strings.Builder
		result []string
	)

	for tok := iter(); tok != chroma.EOF; tok = iter() {
		entry := connectChromaStyle.Get(tok.Type)
		s := lipgloss.NewStyle()
		if entry.Colour.IsSet() {
			s = s.Foreground(lipgloss.Color(entry.Colour.String()))
		}
		if entry.Bold == chroma.Yes {
			s = s.Bold(true)
		}
		parts := strings.Split(tok.Value, "\n")
		for i, p := range parts {
			if i > 0 {
				result = append(result, cur.String())
				cur.Reset()
			}
			if p != "" {
				cur.WriteString(s.Render(p))
			}
		}
	}
	result = append(result, cur.String())

	for len(result) < wantN {
		result = append(result, "")
	}
	return result[:wantN]
}

// ── Code snippets ─────────────────────────────────────────────────────────────

const (
	connectCharsPerFrame = 6  // visible chars revealed per tick (65 ms)
	connectPauseFrames   = 30 // hold the finished snippet (~2 s)
	connectClearFrames   = 10 // blank wipe before next snippet (~0.65 s)
)

type codeSnippet struct {
	lang   string // display name shown in the badge
	langID string // chroma lexer identifier
	lines  []string
}

var connectSnippets = []codeSnippet{
	{
		lang:   "C",
		langID: "c",
		lines: []string{
			`#include <stdio.h>`,
			`#include <stdlib.h>`,
			``,
			`typedef struct Node {`,
			`    int          value;`,
			`    struct Node *next;`,
			`} Node;`,
			``,
			`static Node *push(Node *head, int v) {`,
			`    Node *n = malloc(sizeof *n);`,
			`    if (!n) return head;`,
			`    *n = (Node){ v, head };`,
			`    return n;`,
			`}`,
			``,
			`int main(void) {`,
			`    Node *list = NULL;`,
			`    for (int i = 0; i < 6; i++)`,
			`        list = push(list, i * i);`,
			`    for (Node *n = list; n; n = n->next)`,
			`        printf("%d ", n->value);`,
			`    puts("");`,
			`}`,
		},
	},
	{
		lang:   "Python",
		langID: "python",
		lines: []string{
			`from dataclasses import dataclass`,
			`from typing import Generator`,
			``,
			`@dataclass`,
			`class Tree:`,
			`    val:   int`,
			`    left:  "Tree | None" = None`,
			`    right: "Tree | None" = None`,
			``,
			`def inorder(t: "Tree | None") -> Generator[int, None, None]:`,
			`    if t is None:`,
			`        return`,
			`    yield from inorder(t.left)`,
			`    yield t.val`,
			`    yield from inorder(t.right)`,
			``,
			`root = Tree(4,`,
			`    Tree(2, Tree(1), Tree(3)),`,
			`    Tree(6, Tree(5), Tree(7)))`,
			``,
			`print(list(inorder(root)))`,
			`# → [1, 2, 3, 4, 5, 6, 7]`,
		},
	},
	{
		lang:   "Go",
		langID: "go",
		lines: []string{
			`package main`,
			``,
			`import (`,
			`    "fmt"`,
			`    "sync"`,
			`)`,
			``,
			`func fanIn(cs ...<-chan int) <-chan int {`,
			`    var wg sync.WaitGroup`,
			`    out := make(chan int)`,
			`    relay := func(c <-chan int) {`,
			`        defer wg.Done()`,
			`        for v := range c { out <- v }`,
			`    }`,
			`    wg.Add(len(cs))`,
			`    for _, c := range cs { go relay(c) }`,
			`    go func() { wg.Wait(); close(out) }()`,
			`    return out`,
			`}`,
			``,
			`func main() {`,
			`    a, b := make(chan int, 3), make(chan int, 3)`,
			`    for _, v := range []int{1, 3, 5} { a <- v }`,
			`    for _, v := range []int{2, 4, 6} { b <- v }`,
			`    close(a); close(b)`,
			`    for v := range fanIn(a, b) { fmt.Println(v) }`,
			`}`,
		},
	},
	{
		lang:   "Rust",
		langID: "rust",
		lines: []string{
			`use std::collections::HashMap;`,
			``,
			`fn word_count(text: &str) -> HashMap<&str, usize> {`,
			`    text.split_whitespace().fold(`,
			`        HashMap::new(),`,
			`        |mut map, word| {`,
			`            *map.entry(word).or_insert(0) += 1;`,
			`            map`,
			`        },`,
			`    )`,
			`}`,
			``,
			`fn main() {`,
			`    let text = "the quick brown fox \`,
			`                jumps over the lazy dog";`,
			`    let counts = word_count(text);`,
			`    let mut pairs: Vec<_> = counts.iter().collect();`,
			`    pairs.sort_by_key(|(w, _)| *w);`,
			`    for (word, n) in pairs {`,
			`        println!("{word:>10}  {n}");`,
			`    }`,
			`}`,
		},
	},
	{
		lang:   "JavaScript",
		langID: "javascript",
		lines: []string{
			`class EventEmitter {`,
			`    #ls = new Map();`,
			``,
			`    on(ev, fn) {`,
			`        const prev = this.#ls.get(ev) ?? [];`,
			`        this.#ls.set(ev, [...prev, fn]);`,
			`        return this;`,
			`    }`,
			``,
			`    once(ev, fn) {`,
			`        const wrap = (...a) => {`,
			`            fn(...a);`,
			`            this.off(ev, wrap);`,
			`        };`,
			`        return this.on(ev, wrap);`,
			`    }`,
			``,
			`    off(ev, fn) {`,
			`        this.#ls.set(ev,`,
			`            (this.#ls.get(ev) ?? []).filter(f => f !== fn));`,
			`        return this;`,
			`    }`,
			``,
			`    emit(ev, ...args) {`,
			`        this.#ls.get(ev)?.forEach(fn => fn(...args));`,
			`    }`,
			`}`,
			``,
			`const bus = new EventEmitter();`,
			`bus.on("msg", v => console.log("got:", v));`,
			`bus.emit("msg", 42);`,
		},
	},
}

// ── Screen ────────────────────────────────────────────────────────────────────

type connectChoiceScreen struct {
	loc *i18n.Locale
	sized
	cursor    int
	codeFrame int
	langIdx   int
	execAddr  string // non-empty when user has configured a remote execution server
}

func NewConnectChoiceScreen(loc *i18n.Locale, w, h int, execAddr string) *connectChoiceScreen {
	return &connectChoiceScreen{loc: loc, sized: sized{Width: w, Height: h}, execAddr: execAddr}
}

func (s *connectChoiceScreen) SetLocale(loc *i18n.Locale) { s.loc = loc }

func (s *connectChoiceScreen) Init() tea.Cmd {
	ensureConnectHighlights()
	return connectCodeTickCmd()
}

func (s *connectChoiceScreen) snippetTotalChars(idx int) int {
	total := 0
	for _, line := range connectSnippets[idx].lines {
		total += len(line) + 1
	}
	return total
}

func (s *connectChoiceScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.HandleResize(msg)

	case connectCodeTick:
		s.codeFrame++
		totalChars := s.snippetTotalChars(s.langIdx)
		typingFrames := (totalChars + connectCharsPerFrame - 1) / connectCharsPerFrame
		if s.codeFrame >= typingFrames+connectPauseFrames+connectClearFrames {
			s.langIdx = (s.langIdx + 1) % len(connectSnippets)
			s.codeFrame = 0
		}
		return s, connectCodeTickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case KeyLanguage:
			s.loc = s.loc.Next()
		case "j", "down":
			s.cursor = (s.cursor + 1) % 3
		case "k", "up":
			s.cursor = (s.cursor + 2) % 3
		case "enter":
			switch s.cursor {
			case 0:
				return s, backCmd(NavigateToMenu{})
			case 1:
				return s, backCmd(NavigateToLicenseEntry{})
			case 2:
				return s, backCmd(NavigateToRemoteConfig{})
			}
		case "q", "esc", "ctrl+c":
			return s, tea.Quit
		}
	}
	return s, nil
}

// renderCodePanel produces exactly `height` lines of typewriter-animated,
// syntax-highlighted code using the pre-computed chroma output.
func (s *connectChoiceScreen) renderCodePanel(width, height int) string {
	snippet := connectSnippets[s.langIdx]
	highlighted := connectHighlights[s.langIdx]

	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder))
	// Cursor glyph: dim block so it doesn't fight the syntax colours.
	cursorStr := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)).Render("▋")

	totalChars := s.snippetTotalChars(s.langIdx)
	typingFrames := (totalChars + connectCharsPerFrame - 1) / connectCharsPerFrame
	charsShown := s.codeFrame * connectCharsPerFrame

	// Clear phase: brief blank wipe before the next snippet starts typing.
	inClearPhase := s.codeFrame >= typingFrames+connectPauseFrames

	// 3 chars for line number ("nn "); 1 extra for the cursor glyph.
	maxLineW := width - 4

	var codeLines []string

	if !inClearPhase {
		done := charsShown >= totalChars
		rem := charsShown

		for lineIdx, line := range snippet.lines {
			if !done && rem < 0 {
				break
			}
			numStr := lineNumStyle.Render(fmt.Sprintf("%2d ", lineIdx+1))
			lineLen := len(line)
			hlLine := ""
			if lineIdx < len(highlighted) {
				hlLine = highlighted[lineIdx]
			}

			var rendered string
			switch {
			case done || rem > lineLen:
				// Full line — use pre-highlighted version, truncated to panel width.
				rendered = numStr + truncate.String(hlLine, uint(maxLineW))
				if !done {
					rem -= lineLen + 1
				}
			case rem == lineLen:
				// Last character of this line typed; cursor sits at end.
				rendered = numStr + truncate.String(hlLine, uint(maxLineW-1)) + cursorStr
				rem = -1
			case rem > 0:
				// Mid-line: reveal rem visible chars of the highlighted line.
				rendered = numStr + truncate.String(hlLine, uint(rem)) + cursorStr
				rem = -1
			default:
				// rem == 0: newline of previous line just consumed; cursor at BOL.
				rendered = numStr + cursorStr
				rem = -1
			}
			codeLines = append(codeLines, rendered)
		}
	}

	// Code area: height - 2  (1 blank line at top, 1 badge line at bottom).
	codeH := height - 2
	for len(codeLines) < codeH {
		codeLines = append(codeLines, "")
	}
	if len(codeLines) > codeH {
		codeLines = codeLines[len(codeLines)-codeH:]
	}

	var b strings.Builder
	b.WriteString("\n")
	for _, l := range codeLines {
		b.WriteString(l)
		b.WriteString("\n")
	}

	// Language badge: brightens once typing finishes, dims during clear phase.
	badge := "── " + snippet.lang + " ──"
	done := charsShown >= totalChars
	switch {
	case inClearPhase:
		b.WriteString(dimStyle.Render(badge))
	case done:
		// Use the language's keyword colour from monokai as accent.
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color("166")).Bold(true)
		b.WriteString(accent.Render(badge))
	default:
		b.WriteString(dimStyle.Render(badge))
	}

	return b.String()
}

func (s *connectChoiceScreen) View() string {
	T := s.loc.T
	rtl := s.loc.IsRTL()

	titleSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	selectedSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true)
	normalSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	descSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))

	var execTitle, execDesc string
	if s.execAddr != "" {
		execTitle = fmt.Sprintf(T("connect.exec_configured"), s.execAddr)
		execDesc = T("connect.exec_configured_desc")
	} else {
		execTitle = T("connect.exec")
		execDesc = T("connect.exec_desc")
	}
	options := []struct{ title, desc string }{
		{T("connect.offline"), T("connect.offline_desc")},
		{T("connect.license"), T("connect.license_desc")},
		{execTitle, execDesc},
	}

	var b strings.Builder
	b.WriteString("\n\n")
	if rtl {
		b.WriteString(titleSt.Render(T("connect.title")))
	} else {
		b.WriteString(titleSt.Render("  " + T("connect.title")))
	}
	b.WriteString("\n\n\n")
	for i, opt := range options {
		selected := i == s.cursor
		style := normalSt
		if selected {
			style = selectedSt
		}
		if rtl {
			b.WriteString(style.Render(opt.title))
			if selected {
				b.WriteString(dirArrow(true)) // " ◂"
			} else {
				b.WriteString("  ") // same width as dirArrow to keep items stable
			}
			b.WriteString("\n")
			b.WriteString(descSt.Render(opt.desc))
		} else {
			if selected {
				b.WriteString("▸ ")
			} else {
				b.WriteString("  ")
			}
			b.WriteString(style.Render(opt.title))
			b.WriteString("\n  ")
			b.WriteString(descSt.Render(opt.desc))
		}
		b.WriteString("\n\n")
	}
	b.WriteString("\n\n")
	b.WriteString(descSt.Render(T("connect.help")))
	leftContent := b.String()

	const (
		minWidthForAnim = 90
		optionsPanelW   = 46
	)

	// Narrow path: single centered column.
	if s.Width < minWidthForAnim || s.Width-optionsPanelW-3 < 28 {
		content := leftContent
		if rtl {
			content = rtlAlignBlock(leftContent, s.Width)
		}
		return center(s.Width, s.Height, content)
	}

	codeW := s.Width - optionsPanelW - 3

	// Options panel: vertically centered, right-aligned when RTL.
	optCanvas := NewCanvas(optionsPanelW, s.Height)
	if rtl {
		optCanvas.DrawCenter(rtlAlignBlock(leftContent, optionsPanelW))
	} else {
		optCanvas.DrawCenter(leftContent)
	}

	// Code panel: pre-split once to avoid re-splitting per row.
	// Each line is padded to exactly codeW so the separator and options panel
	// land at fixed columns regardless of how much code has been typed so far.
	codePanelLines := strings.Split(s.renderCodePanel(codeW, s.Height), "\n")
	for i, line := range codePanelLines {
		if vw := lipgloss.Width(line); vw < codeW {
			codePanelLines[i] = line + strings.Repeat(" ", codeW-vw)
		}
	}

	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("235"))
	sepChar := sepStyle.Render("│")

	// Composite: RTL puts code left, options right; LTR is the reverse.
	c := NewCanvas(s.Width, s.Height)
	for row := 0; row < s.Height; row++ {
		cl := ""
		if row < len(codePanelLines) {
			cl = codePanelLines[row]
		}
		ol := optCanvas.rows[row]
		if rtl {
			c.rows[row] = c.normalize(cl + " " + sepChar + " " + ol)
		} else {
			c.rows[row] = c.normalize(ol + " " + sepChar + " " + cl)
		}
	}
	return c.String()
}
