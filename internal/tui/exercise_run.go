package tui

import (
	"strings"
	"time"

	"ztutor/internal/i18n"
	"ztutor/internal/sandbox"

	"github.com/charmbracelet/lipgloss"
)

func isCrashText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "segmentation fault") ||
		strings.Contains(lower, "sigsegv") ||
		strings.Contains(lower, "program crashed") ||
		strings.Contains(lower, "deadlysignal") ||
		strings.Contains(lower, "exited with code 139")
}

// codeEasterEggLine checks source for known patterns and returns a special companion
// line to display before running, or "" if none match.
func codeEasterEggLine(lang sandbox.Language, code string, loc *i18n.Locale) string {
	switch {
	case lang != nil && lang.Name() == "c" && strings.Contains(code, "#include <beer.h>"):
		return loc.T("exercise.egg.beer")
	case strings.Contains(code, "void main("):
		return loc.T("exercise.egg.void_main")
	case strings.Contains(code, "goto "):
		return loc.T("exercise.egg.goto")
	}
	return ""
}

func (es *ExerciseScreen) mascotMood() MascotMood {
	// Transitional states always override — they are temporary and obvious.
	if es.compiling || es.running {
		return MoodThinking
	}
	// Message handlers pin a specific mood via Speak(); use it when set.
	if pin := es.mascot.PinnedMood(); pin != "" {
		return pin
	}
	// Fallback: derive from exercise state.
	switch {
	case es.passed:
		return MoodHappy
	case es.hint.IsVisible():
		return MoodCurious
	case es.compositor.InAsmMode() || es.compositor.InOutputMode() || es.compositor.FocusID() == WidgetFileList:
		return MoodFocused
	default:
		return MoodIdle
	}
}

func (es *ExerciseScreen) startRun(line string) {
	es.compiling = true
	es.passed = false
	es.earnedStars = 0
	es.hint.Hide()
	es.output.SetContent("")
	es.tests.Clear()
	es.memory.Close()
	es.hexViewer.Clear()
	es.diag.SetCompiling(true)
	es.mascot.ClearPin()
	es.mascot.SetLine(line)
	es.runStarted = time.Now()
	es.timer.Start()
}

func calculateStars(attempts, hintsUsed int, hasWarnings bool) int {
	fewAttempts := attempts <= 2
	var stars int
	switch {
	case fewAttempts && !hasWarnings:
		stars = 3
	case fewAttempts || !hasWarnings:
		stars = 2
	default:
		stars = 1
	}
	// Each hint used reduces the star ceiling.
	switch {
	case hintsUsed >= 2 && stars > 1:
		stars = 1
	case hintsUsed == 1 && stars > 2:
		stars = 2
	}
	return stars
}

func (es *ExerciseScreen) successMascotLine(stars, attempts int, hasWarnings bool) string {
	switch {
	case stars == 3:
		return es.loc.T("exercise.mochi.success_perfect")
	case hasWarnings:
		return es.loc.T("exercise.mochi.success_warnings")
	case attempts > 2:
		return es.loc.T("exercise.mochi.success_attempts")
	default:
		return es.loc.T("exercise.mochi.success")
	}
}

func (es *ExerciseScreen) wrongOutputMascotLine(got, want string) string {
	if strings.TrimSpace(got) == "" {
		return es.loc.T("exercise.mochi.wrong_empty")
	}
	if strings.Contains(got, "\n") != strings.Contains(want, "\n") {
		return es.loc.T("exercise.mochi.wrong_newlines")
	}
	return es.loc.T("exercise.mochi.wrong_output")
}

func (es *ExerciseScreen) testFailureMascotLine(r sandbox.TestResult) string {
	if isCrashText(r.Error) || r.ExitCode == 139 {
		return es.loc.T("exercise.mochi.test_crash")
	}
	if r.Error != "" {
		return es.loc.T("exercise.mochi.test_error")
	}
	return es.wrongOutputMascotLine(r.Got, r.Want)
}

// diffOutput renders a line-by-line diff between got and want.
func diffOutput(got, want string) string {
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	wantLines := strings.Split(strings.TrimSpace(want), "\n")
	addSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	delSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	maxLen := len(gotLines)
	if len(wantLines) > maxLen {
		maxLen = len(wantLines)
	}
	var b strings.Builder
	for i := 0; i < maxLen; i++ {
		wLine := ""
		if i < len(wantLines) {
			wLine = wantLines[i]
		}
		gLine := ""
		if i < len(gotLines) {
			gLine = gotLines[i]
		}
		if wLine == gLine {
			b.WriteString(dim("  " + wLine))
		} else {
			if i < len(wantLines) {
				b.WriteString(addSt.Render("+ " + wLine))
				b.WriteString("\n")
			}
			if i < len(gotLines) {
				b.WriteString(delSt.Render("- " + gLine))
			}
		}
		if i < maxLen-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (es *ExerciseScreen) checkPassed(result *sandbox.Result, hasWarnings bool) {
	if len(es.lesson.Tests) == 0 {
		es.passed = true
		es.earnedStars = calculateStars(es.attempts, es.hint.HintsUsed(), hasWarnings)
		es.mascot.Speak(es.successMascotLine(es.earnedStars, es.attempts, hasWarnings), MoodHappy)
		es.output.SetContent(exSuccessStyle.Render(es.loc.T("exercise.result.compiled")) + "\n" + es.starMessage() + "\n\n" + result.Output)
		return
	}
	tc := es.lesson.Tests[0]
	compared := sandbox.CompareResult(1, result, sandbox.TestInput{
		Expected:          tc.Expected,
		ExpectedStdout:    tc.ExpectedStdout,
		ExpectedStderr:    tc.ExpectedStderr,
		HasExpectedStdout: tc.HasExpectedStdout,
		HasExpectedStderr: tc.HasExpectedStderr,
	})
	if compared.Passed {
		es.passed = true
		es.earnedStars = calculateStars(es.attempts, es.hint.HintsUsed(), hasWarnings)
		es.mascot.Speak(es.successMascotLine(es.earnedStars, es.attempts, hasWarnings), MoodHappy)
		es.output.SetContent(exSuccessStyle.Render(es.loc.T("exercise.result.correct")) + "\n" + es.starMessage() + "\n\n" + result.Output)
	} else {
		es.passed = false
		es.earnedStars = 0
		es.mascot.Speak(es.wrongOutputMascotLine(compared.Got, compared.Want), MoodWorried)
		diff := diffOutput(compared.Got, compared.Want)
		es.output.SetContent(exErrorStyle.Render(es.loc.T("exercise.result.mismatch")) + "\n" + dim(es.loc.T("exercise.result.diff_hint")) + "\n\n" + diff)
	}
}

// scoreboard renders a compact per-test pass/fail row: [✓][✓][✗]
func scoreboard(results []sandbox.TestResult) string {
	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	var b strings.Builder
	for _, r := range results {
		if r.Passed {
			b.WriteString(passStyle.Render("[✓]"))
		} else {
			b.WriteString(failStyle.Render("[✗]"))
		}
	}
	return b.String()
}

func (es *ExerciseScreen) checkAllTestsPassed(results []sandbox.TestResult) {
	passed, total := 0, len(results)
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	es.tests.SetResults(results)
	es.progress.SetResult(total, passed)
	board := scoreboard(results)
	if passed == total {
		es.passed = true
		es.earnedStars = calculateStars(es.attempts, es.hint.HintsUsed(), es.lastHasWarnings)
		es.mascot.Speak(es.successMascotLine(es.earnedStars, es.attempts, es.lastHasWarnings), MoodHappy)
		header := exSuccessStyle.Render(es.loc.T("exercise.result.all_passed", total))
		es.output.SetContent(header + "  " + board + "\n" + es.starMessage())
		es.tests.Clear() // pass case: summary is enough; no per-test diff needed
	} else {
		es.passed = false
		es.earnedStars = 0
		var firstFail *sandbox.TestResult
		for i := range results {
			if !results[i].Passed {
				firstFail = &results[i]
				break
			}
		}
		if firstFail != nil {
			es.mascot.Speak(es.testFailureMascotLine(*firstFail), MoodWorried)
		}
		// Header goes to output; detailed diffs go to tests widget (shown in panel).
		header := exErrorStyle.Render(es.loc.T("exercise.result.tests_failed", passed, total)) + "  " + board
		es.output.SetContent(header + "\n\n" + es.tests.View())
		es.tests.Clear() // content is now baked into output; widget is done
	}
}

func (es *ExerciseScreen) starMessage() string {
	stars := es.earnedStars
	filled := strings.Repeat("★", stars)
	empty := strings.Repeat("☆", 3-stars)
	starStr := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render(filled + empty)

	T := es.loc.T
	var reason string
	switch stars {
	case 3:
		reason = dim(T("exercise.stars.perfect", es.attempts))
	case 2:
		if es.attempts <= 2 {
			reason = dim(T("exercise.stars.warnings"))
		} else {
			reason = dim(T("exercise.stars.attempts", es.attempts))
		}
	default:
		reason = dim(T("exercise.stars.default"))
	}

	suffix := ""
	if es.previousStars > 0 && stars > es.previousStars {
		suffix = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(T("exercise.stars.new_best"))
	} else if es.previousStars > 0 && stars <= es.previousStars {
		suffix = "  " + dim(T("exercise.stars.prev_best", strings.Repeat("★", es.previousStars)+strings.Repeat("☆", 3-es.previousStars)))
	}

	hintNote := ""
	if es.hint.HintsUsed() > 0 {
		hintNote = "  " + dim(es.loc.T("exercise.mochi.hint_used", es.hint.HintsUsed()))
	}

	return starStr + "  " + reason + hintNote + suffix
}

func (es *ExerciseScreen) formatResult(r *sandbox.Result, err error) string {
	if err != nil {
		return exErrorStyle.Render(es.loc.T("exercise.result.error", err))
	}
	if r.Error != "" {
		if strings.Contains(strings.ToLower(r.Error), "compilation error") {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.compiler_wall_simple"), MoodWorried)
		} else if strings.Contains(r.Error, "timed out") {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.timeout"), MoodWorried)
		} else if isCrashText(r.Error) {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.crash"), MoodCrashed)
		} else {
			es.mascot.Speak(es.loc.T("exercise.mochi.runtime_error"), MoodWorried)
		}
		return exErrorStyle.Render(r.Error)
	}
	out := r.Output
	if r.ExitCode != 0 {
		es.passed = false
		es.earnedStars = 0
		if r.ExitCode == 139 || isCrashText(out) {
			es.mascot.Speak(es.loc.T("exercise.mochi.segfault"), MoodCrashed)
		} else {
			es.mascot.Speak(es.loc.T("exercise.mochi.exit_code", r.ExitCode), MoodWorried)
		}
		if out != "" {
			out += "\n\n"
		}
		out += exErrorStyle.Render(es.loc.T("exercise.result.exit_nonzero", r.ExitCode))
	}
	return out
}

func buildAchievementEvents(passed bool, attempts, stars int, lastHasWarnings bool, lang sandbox.Language, code string, extra ...string) []string {
	events := make([]string, 0, 10)
	events = append(events, extra...)
	if passed {
		events = append(events, "pass")
		if attempts == 1 {
			events = append(events, "pass_1attempt")
		}
		if attempts >= 5 {
			events = append(events, "pass_5attempts")
		}
		if stars == 3 {
			events = append(events, "pass_3star")
		}
		if !lastHasWarnings {
			events = append(events, "pass_nowarnings")
		}
	}
	if lang != nil && lang.Name() == "c" && strings.Contains(code, "#include <beer.h>") {
		events = append(events, "beer")
	}
	return events
}
