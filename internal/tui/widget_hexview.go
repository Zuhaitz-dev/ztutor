package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type HexViewerWidget struct {
	data    []byte
	visible bool
	offset  int
	loc     *i18n.Locale
}

func newHexViewerWidget(loc *i18n.Locale) *HexViewerWidget {
	return &HexViewerWidget{loc: loc}
}

func (w *HexViewerWidget) Available() bool { return true }
func (w *HexViewerWidget) IsVisible() bool { return w.visible }

func (w *HexViewerWidget) SetHexDump(text string) {
	w.data = []byte(text)
	w.offset = 0
	w.visible = len(text) > 0
}

func (w *HexViewerWidget) Clear() {
	w.data = nil
	w.visible = false
	w.offset = 0
}

func (w *HexViewerWidget) Toggle() { w.visible = !w.visible }

func (w *HexViewerWidget) View(maxHeight int) string {
	if !w.visible || len(w.data) == 0 {
		return ""
	}
	hdrStyle := te.NewStyle().Bold(true).Foreground(te.Color("117"))
	addrStyle := te.NewStyle().Foreground(te.Color("240"))
	hexStyle := te.NewStyle().Foreground(te.Color("81"))
	asciiStyle := te.NewStyle().Foreground(te.Color("252"))
	zeroStyle := te.NewStyle().Foreground(te.Color("238"))

	bytesPerLine := 16
	totalLines := (len(w.data) + bytesPerLine - 1) / bytesPerLine
	if maxHeight < 1 {
		maxHeight = 1
	}

	endLine := w.offset + maxHeight - 1
	if endLine >= totalLines {
		endLine = totalLines - 1
	}
	if endLine < 0 {
		endLine = 0
	}

	var b strings.Builder
	header := fmt.Sprintf(" %s  (%d bytes, %d-%d/%d)",
		w.loc.T("exercise.hexview.header"), len(w.data),
		w.offset*bytesPerLine, (endLine+1)*bytesPerLine, len(w.data))
	b.WriteString(hdrStyle.Render(header))
	b.WriteString("\n")

	addrPad := "  "
	hexPad := " "
	asciiPad := " "

	for line := w.offset; line <= endLine; line++ {
		start := line * bytesPerLine
		end := start + bytesPerLine
		if end > len(w.data) {
			end = len(w.data)
		}
		if start >= len(w.data) {
			break
		}

		addr := fmt.Sprintf("%08x", start)
		b.WriteString(addrStyle.Render(addrPad + addr))
		b.WriteString(hexPad)

		var hexParts []string
		var asciiParts []string
		for i := start; i < end; i++ {
			b := w.data[i]
			hexParts = append(hexParts, hexStyle.Render(fmt.Sprintf("%02x", b)))
			if b >= 32 && b < 127 {
				asciiParts = append(asciiParts, asciiStyle.Render(string(rune(b))))
			} else {
				asciiParts = append(asciiParts, zeroStyle.Render("."))
			}
		}
		for i := end; i < start+bytesPerLine; i++ {
			hexParts = append(hexParts, "  ")
			asciiParts = append(asciiParts, " ")
		}
		b.WriteString(strings.Join(hexParts, " "))
		b.WriteString(asciiPad)
		b.WriteString(strings.Join(asciiParts, ""))
		b.WriteString("\n")
	}
	return b.String()
}
