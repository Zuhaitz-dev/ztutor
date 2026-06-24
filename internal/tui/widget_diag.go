package tui

import (
	"strings"
	"time"

	"ztutor/internal/i18n"
	"ztutor/internal/sandbox"

	"github.com/charmbracelet/lipgloss"
)

// DiagnosticsWidget stores syntax-check results and renders the single-line
// diagnostic summary shown between the output panel and the mascot.
// It is not in the Tab cycle — it is managed directly by ExerciseScreen.
type DiagnosticsWidget struct {
	diagnostics  []sandbox.Diagnostic
	lastDuration time.Duration
	compiling    bool
	curLineFn    func() int // returns 1-indexed cursor row in the editor
	width        int
	loc          *i18n.Locale
}

func newDiagnosticsWidget(curLineFn func() int, width int, loc *i18n.Locale) *DiagnosticsWidget {
	return &DiagnosticsWidget{curLineFn: curLineFn, width: width, loc: loc}
}

// SetLocale replaces the locale used for rendering.
func (w *DiagnosticsWidget) SetLocale(loc *i18n.Locale) { w.loc = loc }

// SetDiagnostics replaces the current diagnostic list.
func (w *DiagnosticsWidget) SetDiagnostics(diags []sandbox.Diagnostic) { w.diagnostics = diags }

// SetDuration stores the last compile/run duration shown when no diagnostics are present.
func (w *DiagnosticsWidget) SetDuration(d time.Duration) { w.lastDuration = d }

// SetCompiling marks whether a compile is in progress (suppresses duration display).
func (w *DiagnosticsWidget) SetCompiling(c bool) { w.compiling = c }

// SetWidth updates the terminal width used for message truncation.
func (w *DiagnosticsWidget) SetWidth(n int) { w.width = n }

// Get returns the raw diagnostic list, used by ExerciseScreen to push line
// markers into the editor gutter.
func (w *DiagnosticsWidget) Get() []sandbox.Diagnostic { return w.diagnostics }

// View renders a single line: the diag for the cursor's line if one exists,
// otherwise a count summary, or the last run duration.
func (w *DiagnosticsWidget) View() string {
	T := w.loc.T
	if len(w.diagnostics) == 0 {
		if w.lastDuration > 0 && !w.compiling {
			return dim(T("exercise.diag.ran_in", w.lastDuration.Seconds()))
		}
		return ""
	}
	curLine := w.curLineFn()
	for _, d := range w.diagnostics {
		if d.Line == curLine && (d.Kind == "error" || d.Kind == "warning") {
			msg := d.Message
			maxW := w.width - 10
			if maxW < 10 {
				maxW = 10
			}
			if len(msg) > maxW {
				msg = msg[:maxW-3] + "..."
			}
			if d.Kind == "error" {
				return exErrorStyle.Render("✗ " + msg)
			}
			return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render("⚠ " + msg)
		}
	}
	// No diagnostic on cursor line — show totals.
	errors, warns := 0, 0
	for _, d := range w.diagnostics {
		switch d.Kind {
		case "error":
			errors++
		case "warning":
			warns++
		}
	}
	var parts []string
	if errors > 0 {
		parts = append(parts, exErrorStyle.Render(T("exercise.diag.errors", errors)))
	}
	if warns > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render(T("exercise.diag.warnings", warns)))
	}
	return strings.Join(parts, "  ")
}
