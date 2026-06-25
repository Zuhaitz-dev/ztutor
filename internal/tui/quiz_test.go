package tui

import (
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
)

func testQuiz() lesson.Quiz {
	return lesson.Quiz{
		ID:    "quiz-1",
		Title: "Quiz",
		Questions: []lesson.QuizQuestion{
			{
				ID:     "q1",
				Prompt: "Pick A",
				Options: []lesson.QuizOption{
					{ID: "a", Text: "A", Correct: true},
					{ID: "b", Text: "B"},
				},
				Explanation: "A is correct.",
			},
			{
				ID:     "q2",
				Prompt: "Pick B",
				Options: []lesson.QuizOption{
					{ID: "a", Text: "A"},
					{ID: "b", Text: "B", Correct: true},
				},
				Explanation: "B is correct.",
			},
		},
	}
}

func TestQuizScreenScoresAndCompletes(t *testing.T) {
	q := testQuiz()
	screen := NewQuizScreen(q, 0, 80, 24, i18n.New("en"))

	model, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	screen = model.(*QuizScreen)
	if cmd != nil {
		t.Fatal("first answer should not complete the quiz")
	}
	if !screen.submitted["q1"] {
		t.Fatal("first question was not submitted")
	}

	model, cmd = screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	screen = model.(*QuizScreen)
	if cmd != nil || screen.index != 1 {
		t.Fatalf("second enter should advance to q2, index=%d cmd=%v", screen.index, cmd)
	}

	model, _ = screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	screen = model.(*QuizScreen)
	if screen.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", screen.cursor)
	}

	model, cmd = screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	screen = model.(*QuizScreen)
	if cmd != nil {
		t.Fatal("second answer should show feedback before completing")
	}

	_, cmd = screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("final enter should emit completion command")
	}
	msg := cmd()
	done, ok := msg.(lessonCompletedMsg)
	if !ok {
		t.Fatalf("completion command emitted %T", msg)
	}
	if done.lessonID != q.ID || done.stars != 3 {
		t.Fatalf("completion = %+v, want quiz id and 3 stars", done)
	}
}

func TestQuizScreenPartialScoreStars(t *testing.T) {
	screen := NewQuizScreen(testQuiz(), 0, 80, 24, i18n.New("en"))
	screen.selected["q1"] = 0
	screen.selected["q2"] = 0
	if got := screen.stars(); got != 2 {
		t.Fatalf("stars = %d, want 2 for half correct", got)
	}
}
