package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Canvas is a fixed width×height raster of terminal rows. Every row is always
// exactly width visible characters wide. Drawing into the canvas normalizes
// each line, so no widget can accidentally produce stale-char artifacts from
// BubbleTea's diff renderer.
type Canvas struct {
	width  int
	height int
	rows   []string
}

// NewCanvas creates a canvas filled with blank lines (spaces).
func NewCanvas(w, h int) *Canvas {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	blank := strings.Repeat(" ", w)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = blank
	}
	return &Canvas{width: w, height: h, rows: rows}
}

// Draw writes content starting at row. Lines are normalized to exactly width
// chars. Excess content lines are clipped; rows not covered are filled as blank.
func (c *Canvas) Draw(row int, content string) {
	if row < 0 || row >= c.height {
		return
	}
	lines := strings.Split(content, "\n")
	blank := strings.Repeat(" ", c.width)
	for i, line := range lines {
		r := row + i
		if r >= c.height {
			break
		}
		c.rows[r] = c.normalize(line)
	}
	// Fill remaining rows as blank if content has fewer lines than we expected.
	for i := len(lines); row+i < c.height && i < c.height-row; i++ {
		c.rows[row+i] = blank
		break // we don't know the intent — caller should size correctly
	}
}

// DrawAt fills rows [row, row+h) with content. Lines are normalized.
// If content has fewer lines than h, remaining rows are blanked.
func (c *Canvas) DrawAt(row int, content string, h int) {
	blank := strings.Repeat(" ", c.width)
	lines := strings.Split(content, "\n")
	for i := 0; i < h; i++ {
		r := row + i
		if r >= c.height {
			break
		}
		if i < len(lines) {
			c.rows[r] = c.normalize(lines[i])
		} else {
			c.rows[r] = blank
		}
	}
}

// DrawCenter vertically centers content on the canvas. If content overflows,
// it is placed at the top with no centering.
func (c *Canvas) DrawCenter(content string) {
	lines := strings.Split(content, "\n")
	h := len(lines)
	if h == 0 {
		return
	}
	top := (c.height - h) / 2
	if top < 0 {
		top = 0
	}
	c.Draw(top, content)
}

// DrawBottom places content at the bottom of the canvas.
func (c *Canvas) DrawBottom(content string) {
	lines := strings.Split(content, "\n")
	h := len(lines)
	c.Draw(c.height-h, content)
}

// DrawBox wraps content in a styled border box and draws it at row.
func (c *Canvas) DrawBox(row int, content string, style lipgloss.Style) {
	styled := style.Render(content)
	c.Draw(row, styled)
}

// String returns the canvas content as a single string with rows joined by
// newlines. No trailing newline — BubbleTea appends its own cursor positioning.
func (c *Canvas) String() string {
	return strings.Join(c.rows, "\n")
}

// normalize returns a string exactly c.width visual characters wide.
// Narrow lines are space-padded; wide lines are ANSI-safely truncated.
func (c *Canvas) normalize(line string) string {
	vw := lipgloss.Width(line)
	switch {
	case vw == c.width:
		return line
	case vw < c.width:
		return line + strings.Repeat(" ", c.width-vw)
	default:
		truncated := lipgloss.NewStyle().MaxWidth(c.width).Render(line)
		if pad := c.width - lipgloss.Width(truncated); pad > 0 {
			return truncated + strings.Repeat(" ", pad)
		}
		return truncated
	}
}
