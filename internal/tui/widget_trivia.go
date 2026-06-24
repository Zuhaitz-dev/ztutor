package tui

// TriviaWidget cycles through a list of trivia strings.
// Callers display the current item by injecting it into the mascot companion line.
// It is not in the Tab cycle — it is driven directly by ExerciseScreen key handlers.
type TriviaWidget struct {
	items   []string
	current int
}

func newTriviaWidget(items []string) *TriviaWidget {
	return &TriviaWidget{items: items}
}

// Available reports whether there are any trivia items to show.
func (w *TriviaWidget) Available() bool { return len(w.items) > 0 }

// Next advances to the next trivia item, wrapping around at the end.
func (w *TriviaWidget) Next() {
	if len(w.items) == 0 {
		return
	}
	w.current = (w.current + 1) % len(w.items)
}

// Current returns the current trivia string, or "" if none are available.
func (w *TriviaWidget) Current() string {
	if len(w.items) == 0 {
		return ""
	}
	return w.items[w.current]
}
