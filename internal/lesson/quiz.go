package lesson

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Quiz struct {
	ID          string
	Title       string
	Description string
	Difficulty  string
	Tags        []string
	Questions   []QuizQuestion
}

type QuizQuestion struct {
	ID          string
	Kind        string
	Prompt      string
	Options     []QuizOption
	Explanation string
}

type QuizOption struct {
	ID      string
	Text    string
	Correct bool
}

type localizedString struct {
	Value string
	I18N  map[string]string
}

func (s localizedString) Text(lang string) string {
	if lang != "" {
		if v := strings.TrimSpace(s.I18N[lang]); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(s.I18N["en"]); v != "" {
		return v
	}
	return s.Value
}

type quizManifest struct {
	ID              string            `yaml:"id"`
	Title           string            `yaml:"title"`
	TitleI18N       map[string]string `yaml:"title_i18n,omitempty"`
	Description     string            `yaml:"description,omitempty"`
	DescriptionI18N map[string]string `yaml:"description_i18n,omitempty"`
	Difficulty      string            `yaml:"difficulty,omitempty"`
	Tags            []string          `yaml:"tags,omitempty"`
	Questions       []struct {
		ID              string            `yaml:"id"`
		Kind            string            `yaml:"kind,omitempty"`
		Prompt          string            `yaml:"prompt"`
		PromptI18N      map[string]string `yaml:"prompt_i18n,omitempty"`
		Explanation     string            `yaml:"explanation,omitempty"`
		ExplanationI18N map[string]string `yaml:"explanation_i18n,omitempty"`
		Options         []struct {
			ID       string            `yaml:"id"`
			Text     string            `yaml:"text"`
			TextI18N map[string]string `yaml:"text_i18n,omitempty"`
			Correct  bool              `yaml:"correct"`
		} `yaml:"options"`
	} `yaml:"questions"`
}

func LoadQuizzes(dir, lang string) ([]Quiz, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var quizzes []Quiz
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		q, err := LoadQuiz(filepath.Join(dir, entry.Name()), lang)
		if err != nil {
			continue
		}
		quizzes = append(quizzes, *q)
	}
	sort.Slice(quizzes, func(i, j int) bool { return quizzes[i].ID < quizzes[j].ID })
	return quizzes, nil
}

func LoadQuiz(dir, lang string) (*Quiz, error) {
	path := filepath.Join(dir, "quiz.yaml")
	if lang != "" && lang != "en" {
		localized := filepath.Join(dir, "quiz."+lang+".yaml")
		if _, err := os.Stat(localized); err == nil {
			path = localized
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m quizManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	q := &Quiz{
		ID:          strings.TrimSpace(m.ID),
		Title:       localizedString{Value: m.Title, I18N: m.TitleI18N}.Text(lang),
		Description: localizedString{Value: m.Description, I18N: m.DescriptionI18N}.Text(lang),
		Difficulty:  m.Difficulty,
		Tags:        m.Tags,
	}
	if q.ID == "" {
		q.ID = filepath.Base(dir)
	}
	for _, qm := range m.Questions {
		qq := QuizQuestion{
			ID:          strings.TrimSpace(qm.ID),
			Kind:        strings.TrimSpace(qm.Kind),
			Prompt:      localizedString{Value: qm.Prompt, I18N: qm.PromptI18N}.Text(lang),
			Explanation: localizedString{Value: qm.Explanation, I18N: qm.ExplanationI18N}.Text(lang),
		}
		if qq.ID == "" {
			return nil, fmt.Errorf("quiz %s has a question without id", q.ID)
		}
		if qq.Kind == "" {
			qq.Kind = "single_choice"
		}
		for _, om := range qm.Options {
			qq.Options = append(qq.Options, QuizOption{
				ID:      strings.TrimSpace(om.ID),
				Text:    localizedString{Value: om.Text, I18N: om.TextI18N}.Text(lang),
				Correct: om.Correct,
			})
		}
		if len(qq.Options) == 0 {
			return nil, fmt.Errorf("quiz %s question %s has no options", q.ID, qq.ID)
		}
		q.Questions = append(q.Questions, qq)
	}
	if q.Title == "" {
		q.Title = q.ID
	}
	if len(q.Questions) == 0 {
		return nil, fmt.Errorf("quiz %s has no questions", q.ID)
	}
	return q, nil
}
