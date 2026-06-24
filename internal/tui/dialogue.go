package tui

import (
	"strings"
	"time"

	"ztutor/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Messages ─────────────────────────────────────────────────────────────────

type dialogueTypeTick struct{}
type dialogueAdvanceTick struct{ BeatIdx int }

func dialogueTypeTickCmd() tea.Cmd {
	return tea.Tick(22*time.Millisecond, func(_ time.Time) tea.Msg { return dialogueTypeTick{} })
}

func dialogueAutoAdvanceCmd(d time.Duration, beatIdx int) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg { return dialogueAdvanceTick{BeatIdx: beatIdx} })
}

// ── Types ────────────────────────────────────────────────────────────────────

// DialogueBeat is one step in a dialogue sequence.
type DialogueBeat struct {
	Text      string
	Mood      MascotMood
	Speed     int // chars per tick; 0 = default (3)
	AutoDelay time.Duration
}

// DialoguePhase tracks where we are in a beat.
type DialoguePhase int

const (
	DialogueTyping  DialoguePhase = iota
	DialogueWaiting               // text complete, waiting for auto-advance
	DialogueHolding               // text complete, waiting for any key
	DialogueChoice                // text complete, waiting for user to pick a choice
)

// ── Engine ───────────────────────────────────────────────────────────────────

// dialogueEngine handles typewriter animation and mascot sprite rendering.
type dialogueEngine struct {
	Beats   []DialogueBeat
	BeatIdx int
	CharIdx int
	Phase   DialoguePhase
	Frame   int // mascot animation frame

	// Rendering config.
	BoxWidth int // width of the text box (0 = auto-calculate)
	MaxLines int // max text lines in the box

	// In RTL mode, how to align the mascot sprite within the width.
	// intro uses Center, pre_exercise uses Right (matching rtlWrap behavior).
	rtlMascotAlign lipgloss.Position
}

func (d *dialogueEngine) Advance() bool {
	if d.BeatIdx+1 < len(d.Beats) {
		d.BeatIdx++
		d.CharIdx = 0
		d.Phase = DialogueTyping
		return true
	}
	return false
}

func (d *dialogueEngine) TypeTick() tea.Cmd {
	return dialogueTypeTickCmd()
}

func (d *dialogueEngine) AutoAdvanceCmd() tea.Cmd {
	beat := d.Beats[d.BeatIdx]
	return dialogueAutoAdvanceCmd(beat.AutoDelay, d.BeatIdx)
}

// FinishBeat completes the current beat's typewriter animation and transitions
// to the appropriate waiting/holding phase.
func (d *dialogueEngine) FinishBeat() tea.Cmd {
	beat := d.Beats[d.BeatIdx]
	d.CharIdx = len([]rune(beat.Text))
	if beat.AutoDelay > 0 {
		d.Phase = DialogueWaiting
		return d.AutoAdvanceCmd()
	}
	d.Phase = DialogueHolding
	return nil
}

// Helper: resolve RTL from locale (nil-safe).
func (d *dialogueEngine) rtl(loc *i18n.Locale) bool {
	if loc == nil {
		return false
	}
	return loc.IsRTL()
}

// textBoxWidth returns the effective text-box width.
func (d *dialogueEngine) textBoxWidth(width int) int {
	if d.BoxWidth > 0 {
		return d.BoxWidth
	}
	boxW := 60
	if width-8 < boxW {
		boxW = width - 8
	}
	if boxW < 20 {
		boxW = 20
	}
	return boxW
}

// RenderMascot renders the mascot sprite for the current beat's mood and frame.
func (d *dialogueEngine) RenderMascot(loc *i18n.Locale, width int) string {
	beat := d.Beats[d.BeatIdx]
	sprite := mascotSprite(beat.Mood, d.Frame)
	spriteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Bold(true)

	alignW := d.textBoxWidth(width) + 2

	var b strings.Builder
	for _, line := range sprite {
		rendered := spriteStyle.Render(line)
		if d.rtl(loc) {
			rendered = lipgloss.NewStyle().Width(alignW).Align(d.rtlMascotAlign).Render(rendered)
		}
		b.WriteString(rendered + "\n")
	}
	return b.String()
}

// RenderTextBox renders the word-wrapped dialogue text in a RoundedBorder box.
func (d *dialogueEngine) RenderTextBox(loc *i18n.Locale, width int) string {
	beat := d.Beats[d.BeatIdx]
	runes := []rune(beat.Text)
	idx := d.CharIdx
	if idx > len(runes) {
		idx = len(runes)
	}
	visible := string(runes[:idx])

	cursor := ""
	if d.Phase == DialogueTyping {
		if d.Frame%2 == 0 {
			cursor = "_"
		} else {
			cursor = " "
		}
	}

	boxW := d.textBoxWidth(width)

	paragraphs := strings.Split(visible+cursor, "\n")
	var wrapped []string
	for _, p := range paragraphs {
		lines := wrapPlainText(p, boxW-2)
		if len(lines) == 0 {
			wrapped = append(wrapped, "")
		} else {
			wrapped = append(wrapped, lines...)
		}
	}
	boxH := d.MaxLines
	if boxH <= 0 {
		boxH = 4
	}
	for len(wrapped) < boxH {
		wrapped = append(wrapped, "")
	}
	wrapped = wrapped[:boxH]

	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1).
		Width(boxW)
	if d.rtl(loc) {
		boxStyle = boxStyle.Align(lipgloss.Right)
	}
	return boxStyle.Render(textStyle.Render(strings.Join(wrapped, "\n")))
}

// boxW returns the effective text-box width, exposed so callers can align
// ancillary UI (e.g. help bars) to the same width.
func (d *dialogueEngine) boxW(width int) int {
	return d.textBoxWidth(width)
}
