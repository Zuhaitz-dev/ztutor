package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type stackFrame struct {
	num      int
	address  string
	funcName string
	file     string
	line     int
}

type heapStats struct {
	inUseBytes  int
	inUseBlocks int
	totalAllocs int
	totalFrees  int
	totalBytes  int
	defLeaked   int
	indLeaked   int
}

type MemoryWidget struct {
	summary    []string
	frames     []stackFrame
	heap       *heapStats
	open       bool
	showFrames bool
	showHeap   bool
	loc        *i18n.Locale
}

var frameRegex = regexp.MustCompile(`#(\d+)\s+(0x[0-9a-fA-F]+)\s+in\s+(\S+)\s*(.+?)\s*$`)

// ansiEscape matches ANSI/VT100 color and style escape sequences of the form
// ESC [ ... m (and other single-letter terminators). ASAN emits these even
// when stdout/stderr is a pipe, which corrupts regex matching and hex display.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// stripANSI removes ANSI escape sequences from s. Call this before feeding
// sanitizer output to the memory widget parser or the hex viewer.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func newMemoryWidget(loc *i18n.Locale) *MemoryWidget {
	return &MemoryWidget{loc: loc}
}

func (w *MemoryWidget) Available() bool { return true }

func (w *MemoryWidget) IsOpen() bool   { return w.open }
func (w *MemoryWidget) SetOpen(v bool) { w.open = v }
func (w *MemoryWidget) Toggle()        { w.open = !w.open }
func (w *MemoryWidget) Close()         { w.open = false }

func (w *MemoryWidget) SetAsanOutput(raw string) {
	w.summary = nil
	w.frames = nil
	w.heap = nil
	w.showFrames = false
	w.showHeap = false
	if raw == "" {
		return
	}

	// ASAN emits ANSI color codes even to pipes; strip them before any regex
	// or keyword matching so frame parsing and summary extraction work cleanly.
	raw = stripANSI(raw)

	w.summary = extractMemorySummary(raw)
	if len(w.summary) == 0 {
		w.summary = []string{w.loc.T("exercise.memory.no_issues")}
	}

	w.frames = extractStackTrace(raw)
	if len(w.frames) > 0 {
		w.showFrames = true
	}

	w.heap = extractHeapStats(raw)
	if w.heap != nil {
		w.showHeap = true
	}
}

func (w *MemoryWidget) View() string {
	if !w.open {
		return ""
	}
	hdrStyle := te.NewStyle().Bold(true).Foreground(te.Color("117"))
	subStyle := te.NewStyle().Foreground(te.Color("214")).Bold(true)
	bodyStyle := te.NewStyle().Foreground(te.Color("252"))
	addrStyle := te.NewStyle().Foreground(te.Color("240"))
	fnStyle := te.NewStyle().Foreground(te.Color("39")).Bold(true)
	fileStyle := te.NewStyle().Foreground(te.Color("246"))
	leakStyle := te.NewStyle().Foreground(te.Color("196")).Bold(true)
	okStyle := te.NewStyle().Foreground(te.Color("42"))

	var b strings.Builder
	b.WriteString(hdrStyle.Render(" " + w.loc.T("exercise.memory.header") + " "))
	b.WriteString("\n")

	if w.showFrames && len(w.frames) > 0 {
		b.WriteString(subStyle.Render(w.loc.T("exercise.memory.stack_trace")))
		b.WriteString("\n")
		for _, f := range w.frames {
			fn := fnStyle.Render(f.funcName)
			loc := ""
			if f.file != "" {
				loc = " " + fileStyle.Render(fmt.Sprintf("%s:%d", f.file, f.line))
			}
			addr := " " + addrStyle.Render(f.address)
			b.WriteString(fmt.Sprintf("  #%d %s%s%s\n", f.num, fn, loc, addr))
		}
		b.WriteString("\n")
	}

	if w.showHeap && w.heap != nil {
		h := w.heap
		b.WriteString(subStyle.Render(w.loc.T("exercise.memory.heap_summary")))
		b.WriteString("\n")
		alloc := okStyle.Render(fmt.Sprintf("%d", h.totalBytes))
		leakCount := h.defLeaked + h.indLeaked
		leakColor := okStyle
		if leakCount > 0 {
			leakColor = leakStyle
		}
		b.WriteString(fmt.Sprintf("  %s: %s  %s: %s\n",
			w.loc.T("exercise.memory.bytes"), alloc,
			w.loc.T("exercise.memory.leaks"), leakColor.Render(fmt.Sprintf("%d", leakCount))))
		b.WriteString(fmt.Sprintf("  %s: %d  %s: %d\n",
			w.loc.T("exercise.memory.allocs"), h.totalAllocs,
			w.loc.T("exercise.memory.frees"), h.totalFrees))
		b.WriteString("\n")
	}

	if len(w.summary) > 0 {
		for _, line := range w.summary {
			styled := bodyStyle.Render(line)
			if strings.Contains(line, "definitely lost") || strings.Contains(line, "indirectly lost") {
				styled = leakStyle.Render(line)
			} else if strings.Contains(line, "HEAP SUMMARY") || strings.Contains(line, "LEAK SUMMARY") {
				styled = hdrStyle.Render(line)
			} else if strings.Contains(line, "0x") {
				idx := strings.Index(line, "0x")
				if idx > 0 {
					styled = bodyStyle.Render(line[:idx]) + addrStyle.Render(line[idx:])
				}
			} else if strings.Contains(line, "no memory issues") || strings.Contains(line, "no leaks") {
				styled = okStyle.Render(line)
			}
			b.WriteString("  " + styled + "\n")
		}
	}
	return b.String()
}

func extractStackTrace(raw string) []stackFrame {
	var frames []stackFrame
	seenZero := false
	for _, line := range strings.Split(raw, "\n") {
		matches := frameRegex.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) < 4 {
			continue
		}
		num, _ := strconv.Atoi(matches[1])
		// ASAN emits multiple stack traces for some errors (access site + allocation
		// site). Stop at the second #0 so we only show the first (primary) trace.
		if num == 0 {
			if seenZero {
				break
			}
			seenZero = true
		}
		loc := matches[4]
		f := stackFrame{
			address:  matches[2],
			funcName: matches[3],
			num:      num,
		}
		// loc is typically "(path+offset) (BuildId: ...)" or "(file.c:line)".
		// Extract only the content of the first pair of parens.
		if idx := strings.Index(loc, "("); idx >= 0 {
			inner := loc[idx+1:]
			if end := strings.Index(inner, ")"); end >= 0 {
				loc = inner[:end]
			} else {
				loc = inner
			}
		}
		if parts := strings.SplitN(loc, ":", 2); len(parts) == 2 {
			f.file = parts[0]
			if n, err := strconv.Atoi(parts[1]); err == nil {
				f.line = n
			}
		}
		frames = append(frames, f)
	}
	return frames
}

func extractHeapStats(raw string) *heapStats {
	var h heapStats
	found := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "HEAP SUMMARY") {
			found = true
			continue
		}
		if !found {
			continue
		}
		clean := stripAsanPrefix(line)
		if strings.Contains(clean, "in use at exit") {
			fmt.Sscanf(clean, "in use at exit: %d bytes in %d blocks", &h.inUseBytes, &h.inUseBlocks)
		}
		if strings.Contains(clean, "total heap usage") {
			fmt.Sscanf(clean, "total heap usage: %d allocs, %d frees, %d bytes allocated",
				&h.totalAllocs, &h.totalFrees, &h.totalBytes)
		}
		if strings.Contains(clean, "definitely lost") {
			fmt.Sscanf(clean, "%d bytes in", &h.defLeaked)
		}
		if strings.Contains(clean, "indirectly lost") {
			fmt.Sscanf(clean, "%d bytes in", &h.indLeaked)
		}
		if strings.Contains(line, "no leaks are possible") || strings.Contains(line, "All heap blocks were freed") {
			break
		}
	}
	if !found {
		return nil
	}
	return &h
}

func stripAsanPrefix(s string) string {
	if idx := strings.Index(s, "=="); idx >= 0 {
		end := strings.Index(s[idx+2:], "==")
		if end >= 0 {
			return strings.TrimSpace(s[idx+end+4:])
		}
	}
	return s
}

func extractMemorySummary(asanOutput string) []string {
	var summary []string
	lines := strings.Split(asanOutput, "\n")
	inSummary := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "heap summary") || strings.Contains(lower, "leak summary") || strings.Contains(lower, "error summary") {
			inSummary = true
			summary = append(summary, line)
			continue
		}
		if inSummary {
			if strings.Contains(lower, "runtime error") || strings.Contains(lower, "addresssanitizer") {
				break
			}
			summary = append(summary, line)
			if strings.Contains(lower, "suppressed") || strings.Contains(lower, "summary") {
				break
			}
		}
	}
	return summary
}
