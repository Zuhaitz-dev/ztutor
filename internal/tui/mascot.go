package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	mascotNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorHex))

	mascotBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(0, 1)

	mascotFaceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAmber)).
			Bold(true)

	mascotTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBody))
)

const mascotPanelHeight = 5

// ltrMarker is the Unicode left-to-right mark (U+200E). Placed at the start of
// ASCII art in RTL contexts, it prevents the terminal's bidi algorithm from
// reordering or mirroring the cat sprite characters.
const ltrMarker = "\u200E"

// mirrorSprite lines are pre-computed RTL-facing versions of every sprite
// frame. The cat faces toward the RTL text instead of away from it.
var mirrorSprites = map[MascotMood][][]string{}

func init() {
	for mood, frames := range mascotSprites {
		mirrored := make([][]string, len(frames))
		for i, frame := range frames {
			mirrored[i] = make([]string, len(frame))
			for j, line := range frame {
				mirrored[i][j] = mirrorLine(line)
			}
		}
		mirrorSprites[mood] = mirrored
	}
}

// mirrorLine reflects ASCII art horizontally: reverses character order and
// swaps directional pairs (/↔\, (↔), >↔<, etc.).
func mirrorLine(s string) string {
	runes := []rune(s)
	n := len(runes)
	out := make([]rune, n)
	for i, r := range runes {
		var m rune
		switch r {
		case '/':
			m = '\\'
		case '\\':
			m = '/'
		case '(':
			m = ')'
		case ')':
			m = '('
		case '>':
			m = '<'
		case '<':
			m = '>'
		case '{':
			m = '}'
		case '}':
			m = '{'
		case '[':
			m = ']'
		case ']':
			m = '['
		default:
			m = r
		}
		out[n-1-i] = m
	}
	return string(out)
}

func renderMascotPanel(width int, speaker, text string, mood MascotMood, frame int, rtl bool) string {
	if width < 30 {
		return ""
	}
	if speaker == "" {
		speaker = "Mochi"
	}
	if text == "" {
		text = "Pick a lesson and I will keep an eye on the sharp edges."
	}

	innerW := width - 4
	if innerW < 20 {
		innerW = 20
	}

	var catLines []string
	if rtl {
		catLines = mirrorSprite(mood, frame)
	} else {
		catLines = mascotSprite(mood, frame)
	}
	name := mascotNameStyle.Render(speaker + ":")
	catW := mascotSpriteWidth
	gapW := 2
	textW := innerW - catW - gapW
	if textW < 12 {
		textW = innerW
		catLines = nil
		catW = 0
		gapW = 0
	}

	lines := wrapPlainText(text, textW)
	if len(lines) == 0 {
		lines = []string{""}
	}
	if len(lines) > 2 {
		lines = lines[:2]
		if rtl {
			// RTL: truncation marker goes at the logical start so it appears
			// on the right (reading-start side) in a BiDi-aware terminal.
			lines[1] = "..." + truncRunes(lines[1], max(0, textW-3))
		} else {
			lines[1] = truncRunes(lines[1], max(0, textW-3)) + "..."
		}
	}
	for len(lines) < 2 {
		lines = append(lines, "")
	}

	dialogue := []string{
		name,
		mascotTextStyle.Render(lines[0]),
		mascotTextStyle.Render(lines[1]),
	}
	bodyLines := make([]string, 3)
	alignR := lipgloss.NewStyle().Width(textW).Align(lipgloss.Right)
	for i := range bodyLines {
		if rtl {
			// RTL: cat on left (with LTR marker to pin ASCII art direction),
			// text right-aligned in the right column — mirrors the LTR layout.
			left := ""
			if len(catLines) > 0 {
				left = mascotFaceStyle.Render(ltrMarker+catLines[i]) + strings.Repeat(" ", gapW)
			}
			bodyLines[i] = padVis(left+alignR.Render(dialogue[i]), innerW)
		} else {
			left := ""
			if len(catLines) > 0 {
				left = mascotFaceStyle.Render(catLines[i]) + strings.Repeat(" ", gapW)
			}
			bodyLines[i] = padVis(left+dialogue[i], innerW)
		}
	}
	body := strings.Join(bodyLines, "\n")

	return mascotBoxStyle.Width(width - 2).Render(body)
}

func mirrorSprite(mood MascotMood, frame int) []string {
	frames := mirrorSprites[mood]
	if len(frames) == 0 {
		frames = mirrorSprites[MoodIdle]
	}
	return frames[frame%len(frames)]
}

const mascotSpriteWidth = 7

func mascotSprite(mood MascotMood, frame int) []string {
	frames := mascotSprites[mood]
	if len(frames) == 0 {
		frames = mascotSprites[MoodIdle]
	}
	return frames[frame%len(frames)]
}

var mascotSprites = map[MascotMood][][]string{
	MoodIdle: {
		{` /\_/\ `, `( o.o )`, ` > ^ < `},
		{` /\_/\ `, `( -.- )`, ` > ^ < `},
		{` /\_/\ `, `( o.o )`, ` > ^ < `},
		{` /\_/\ `, `( o.o )`, `  >^<  `},
	},
	MoodCurious: {
		{` /\_/\ `, `( o.? )`, ` > ^ < `},
		{` /\_/\ `, `( ?.? )`, ` > ^ < `},
	},
	MoodHappy: {
		{` /\_/\ `, `( ^.^ )`, ` > ^ < `},
		{` /\_/\ `, `( ^o^ )`, ` \ ^ / `},
	},
	MoodWorried: {
		{` /\_/\ `, `( o_o )`, ` > ^ < `},
		{` /\_/\ `, `( ;_; )`, ` > ^ < `},
	},
	MoodCrashed: {
		{` /\_/\ `, `( x.x )`, ` > _ < `},
		{` /\_/\ `, `( x_x )`, ` > _ < `},
	},
	MoodThinking: {
		{` /\_/\ `, `( o.o )`, ` > ? < `},
		{` /\_/\ `, `( -.- )`, ` > ? < `},
		{` /\_/\ `, `( o.o )`, ` > ? < `},
	},
	MoodFocused: {
		{` /\_/\ `, `( 0.0 )`, ` > # < `},
		{` /\_/\ `, `( 0_0 )`, ` > # < `},
	},
}

func wrapPlainText(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	var current string
	for _, word := range words {
		if current == "" {
			current = truncRunes(word, width)
			continue
		}
		if lipgloss.Width(current)+1+lipgloss.Width(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = truncRunes(word, width)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
