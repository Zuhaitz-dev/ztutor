package tui

import (
	"strings"

	"ztutor/internal/i18n"
	"ztutor/internal/sandbox"
)

// TestsWidget stores multi-test run results and formats them for the output panel.
// It is not in the Tab cycle — it is managed directly by ExerciseScreen.
type TestsWidget struct {
	results []sandbox.TestResult
	loc     *i18n.Locale
}

func newTestsWidget(loc *i18n.Locale) *TestsWidget {
	return &TestsWidget{loc: loc}
}

// SetLocale replaces the locale used for rendering.
func (w *TestsWidget) SetLocale(loc *i18n.Locale) { w.loc = loc }

// SetResults replaces the stored test results.
func (w *TestsWidget) SetResults(results []sandbox.TestResult) { w.results = results }

// Clear removes any stored test results.
func (w *TestsWidget) Clear() { w.results = nil }

// HasResults reports whether there are any results to display.
func (w *TestsWidget) HasResults() bool { return len(w.results) > 0 }

// View formats the failed test diffs as a multi-line string suitable for the output panel.
// It only renders failed tests; callers handle the pass-summary separately.
func (w *TestsWidget) View() string {
	T := w.loc.T
	var b strings.Builder
	for _, r := range w.results {
		if r.Passed {
			continue
		}
		b.WriteString("\n\n" + T("exercise.result.test_label", r.Num))
		switch {
		case r.Error != "":
			b.WriteString("\n" + exErrorStyle.Render(T("exercise.result.runtime_error", r.Error)))
		case r.ExitCode != 0:
			b.WriteString("\n" + exErrorStyle.Render(T("exercise.result.runtime_exit", r.ExitCode)))
		default:
			b.WriteString("\n" + dim(T("exercise.result.diff_hint")) + "\n")
			b.WriteString(diffOutput(r.Got, r.Want))
		}
	}
	return strings.TrimPrefix(b.String(), "\n\n")
}
