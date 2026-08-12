package tui

import (
	"fmt"
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── C Matrix Rain ─────────────────────────────────────────────────────────────

var cRainChars = []rune("0123456789abcdef*&{}();=+-<>!|^~#\\_?%@")

// rainGradient: index 0 = head (bright white), index 1+ = fading green trail.
var rainGradient = []lipgloss.Color{
	"231", // head — bright white
	"46",  // trail 1
	"40",
	"34",
	"28",
	"22",
	"22",
	"22",
	"238", // very dim tail
	"238",
	"238",
	"238",
	"238",
	"238",
	"238",
	"238",
}

// rainAnsiPfx pre-computes ANSI xterm-256 sequences for each gradient level.
var rainAnsiPfx []string

// forceLTRText wraps a fragment in LTR marks so BiDi-aware terminals keep its
// internal ordering stable when embedded inside RTL text.
func forceLTRText(s string) string {
	return ltrMarker + s + ltrMarker
}

func init() {
	rainAnsiPfx = make([]string, len(rainGradient))
	for i, color := range rainGradient {
		rainAnsiPfx[i] = fmt.Sprintf("\033[38;5;%sm", string(color))
	}
}

type rainTickMsg struct{}

// rainTickCmd runs the rain at ~12 fps, independent of the mascot clock.
func rainTickCmd() tea.Cmd {
	return tea.Tick(83*time.Millisecond, func(time.Time) tea.Msg {
		return rainTickMsg{}
	})
}

type rainCol struct {
	head  float64
	speed float64
	trail int
	chars []rune
}

func newRainCol(h int) rainCol {
	size := h + 30
	chars := make([]rune, size)
	for i := range chars {
		chars[i] = cRainChars[rand.Intn(len(cRainChars))]
	}
	return rainCol{
		head:  -float64(rand.Intn(h + 1)),
		speed: 0.5 + rand.Float64()*1.0,
		trail: 5 + rand.Intn(11),
		chars: chars,
	}
}

func (c *rainCol) tick(h int) {
	c.head += c.speed
	hi := int(c.head)
	if rand.Intn(3) == 0 && hi >= 0 && hi < len(c.chars) {
		c.chars[hi] = cRainChars[rand.Intn(len(cRainChars))]
	}
	if c.head-float64(c.trail) > float64(h) {
		size := h + 30
		chars := make([]rune, size)
		for i := range chars {
			chars[i] = cRainChars[rand.Intn(len(cRainChars))]
		}
		c.head = -float64(rand.Intn(h/2 + 1))
		c.speed = 0.5 + rand.Float64()*1.0
		c.trail = 5 + rand.Intn(11)
		c.chars = chars
	}
}

func (c *rainCol) renderAt(row int) string {
	headRow := int(c.head)
	dist := headRow - row
	if dist < 0 || dist > c.trail {
		return " "
	}
	var ch rune
	if row >= 0 && row < len(c.chars) {
		ch = c.chars[row]
	} else {
		ch = ' '
	}
	idx := dist
	if idx >= len(rainAnsiPfx) {
		idx = len(rainAnsiPfx) - 1
	}
	return rainAnsiPfx[idx] + forceLTRText(string(ch)) + ansiReset
}
