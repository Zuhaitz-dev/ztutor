package lesson

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"ztutor/internal/logutil"
)

type Section struct {
	ID             string
	Title          string
	Type           string // "exercises", "interviews", "quizzes", "exams", "challenges"
	Lessons        []Lesson
	Quizzes        []Quiz
	Challenges     []Challenge
	AvailableTools []string
	Schedule       string
}

// CourseLayout controls how the TUI presents the lesson list for a course.
// Declared in course.yaml under the layout key.
type CourseLayout string

const (
	CourseLayoutList CourseLayout = "list" // default: flat scrollable list
	CourseLayoutPath CourseLayout = "path" // Duolingo-style sequential node path
)

type Course struct {
	ID          string
	Title       string
	Description string
	Order       int
	Language    string
	Sections    []Section

	TotalLessons    int
	TotalChallenges int
	TotalQuizzes    int

	SourceExtension      string
	SyntaxHighlighting   string
	AvailableTools       []string
	DefaultWidgets       []string
	CourseIntro          []string
	ProgrammingLanguages []string

	// Layout controls the TUI presentation mode for this course's lesson list.
	// Defaults to CourseLayoutList when not specified in course.yaml.
	Layout CourseLayout

	// UILanguages lists the UI locale codes in which this course's content
	// is available (e.g. ["en", "es", "ar"]).  Declared in course.yaml under
	// the ui_languages key; empty means no language indicators are shown.
	UILanguages []string
}

type sectionManifest struct {
	ID        string `yaml:"id"`
	Title     string `yaml:"title"`
	Type      string `yaml:"type"`
	Dir       string `yaml:"dir"`
	Schedule  string `yaml:"schedule,omitempty"`
	Toolchain struct {
		AvailableTools []string `yaml:"available_tools,omitempty"`
	} `yaml:"toolchain,omitempty"`
}

type courseManifest struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Language    string `yaml:"language"`
	Order       int    `yaml:"order"`
	Layout      string `yaml:"layout,omitempty"`

	UILanguages     []string            `yaml:"ui_languages,omitempty"`
	DefaultWidgets  []string            `yaml:"default_widgets,omitempty"`
	CourseIntro     []string            `yaml:"course_intro,omitempty"`
	CourseIntroI18N map[string][]string `yaml:"course_intro_i18n,omitempty"`
	Sections        []sectionManifest   `yaml:"sections"`

	Toolchain struct {
		SourceExtension    string   `yaml:"source_extension,omitempty"`
		SyntaxHighlighting string   `yaml:"syntax_highlighting,omitempty"`
		AvailableTools     []string `yaml:"available_tools,omitempty"`
	} `yaml:"toolchain"`
}

func LoadCoursesLang(dir, lang string) ([]Course, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var courses []Course
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		courseDir := filepath.Join(dir, entry.Name())
		c, err := LoadCourseLang(courseDir, lang)
		if err != nil || len(c.Sections) == 0 {
			continue
		}
		courses = append(courses, c)
	}

	sort.Slice(courses, func(i, j int) bool {
		if courses[i].Order != courses[j].Order {
			return courses[i].Order < courses[j].Order
		}
		return courses[i].ID < courses[j].ID
	})

	return courses, nil
}

func LoadCourseLang(courseDir, lang string) (Course, error) {
	c := Course{
		ID:    filepath.Base(courseDir),
		Title: filepath.Base(courseDir),
	}

	m, manifestLoaded := loadManifest(courseDir, &c)

	if manifestLoaded {
		c.DefaultWidgets = m.DefaultWidgets
		c.CourseIntro = m.CourseIntro
		if localized := m.CourseIntroI18N[lang]; len(localized) > 0 {
			c.CourseIntro = localized
		}
		switch CourseLayout(m.Layout) {
		case CourseLayoutPath:
			c.Layout = CourseLayoutPath
		default:
			c.Layout = CourseLayoutList
		}
		for _, sm := range m.Sections {
			s := Section{
				ID:       sm.ID,
				Title:    sm.Title,
				Type:     sm.Type,
				Schedule: sm.Schedule,
			}
			if len(sm.Toolchain.AvailableTools) > 0 {
				s.AvailableTools = sm.Toolchain.AvailableTools
			}
			sectionDir := filepath.Join(courseDir, sm.Dir)
			loadSection(sectionDir, &s, lang)
			c.Sections = append(c.Sections, s)
		}
	} else {
		// Legacy: no manifest — load entire dir as one exercises section.
		lessons, err := LoadAllLang(courseDir, lang)
		if err == nil && len(lessons) > 0 {
			c.Sections = []Section{{
				ID:      "default",
				Title:   "Lessons",
				Type:    "exercises",
				Lessons: lessons,
			}}
		}
	}

	// Defaults.
	if c.Language == "" {
		c.Language = "c"
	}
	if c.SourceExtension == "" {
		c.SourceExtension = ".c"
	}
	if c.SyntaxHighlighting == "" {
		c.SyntaxHighlighting = c.Language
	}

	// Compute aggregates and apply language info.
	var courseLangs []string
	seenLangs := make(map[string]bool)
	addLang := func(lang string) {
		lang = strings.ToLower(strings.TrimSpace(lang))
		if lang == "" || seenLangs[lang] {
			return
		}
		seenLangs[lang] = true
		courseLangs = append(courseLangs, lang)
	}
	addLang(c.Language)
	for i := range c.Sections {
		s := &c.Sections[i]
		if s.Type == "" {
			s.Type = "exercises"
		}
		if len(s.AvailableTools) == 0 && len(c.AvailableTools) > 0 {
			s.AvailableTools = c.AvailableTools
		}
		applyLanguageToSection(s, c.Language, c.SourceExtension, c.SyntaxHighlighting)
		c.TotalLessons += len(s.Lessons)
		c.TotalQuizzes += len(s.Quizzes)
		c.TotalChallenges += len(s.Challenges)
		for i := range s.Lessons {
			l := &s.Lessons[i]
			if len(l.EnabledWidgets) == 0 && len(c.DefaultWidgets) > 0 {
				l.EnabledWidgets = c.DefaultWidgets
			}
			addLang(l.Language)
		}
	}
	c.ProgrammingLanguages = courseLangs

	return c, nil
}

func loadManifest(courseDir string, c *Course) (*courseManifest, bool) {
	for _, name := range []string{"course.yaml", "course.yml"} {
		data, err := os.ReadFile(filepath.Join(courseDir, name))
		if err != nil {
			continue
		}
		var m courseManifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.Title != "" {
			c.Title = m.Title
		}
		c.Description = m.Description
		c.Order = m.Order
		c.Language = m.Language
		c.UILanguages = m.UILanguages
		if m.Toolchain.SourceExtension != "" {
			c.SourceExtension = m.Toolchain.SourceExtension
		}
		if m.Toolchain.SyntaxHighlighting != "" {
			c.SyntaxHighlighting = m.Toolchain.SyntaxHighlighting
		}
		if len(m.Toolchain.AvailableTools) > 0 {
			c.AvailableTools = m.Toolchain.AvailableTools
		}
		return &m, true
	}
	return nil, false
}

func loadSection(dir string, s *Section, lang string) {
	if _, err := os.Stat(dir); err != nil {
		logutil.Warn("course: section dir missing: %s", dir)
		return
	}
	switch s.Type {
	case "challenges":
		challenges, err := LoadChallenges(dir)
		if err == nil {
			s.Challenges = challenges
		}
	case "quizzes":
		quizzes, err := LoadQuizzes(dir, lang)
		if err == nil {
			s.Quizzes = quizzes
		}
	default:
		lessons, err := LoadAllLang(dir, lang)
		if err == nil {
			s.Lessons = lessons
		}
	}
}

func applyLanguageToSection(s *Section, lang, ext, syntax string) {
	for i := range s.Lessons {
		l := &s.Lessons[i]
		// Don't overwrite language for lessons that declared their own
		// in frontmatter (indicated by Language already being set).
		if l.Language != "" {
			continue
		}
		l.Language = lang
		l.SourceExtension = ext
		l.SyntaxHighlighting = syntax
	}
}

func LoadAsSingleCourseLang(dir, lang string) ([]Course, error) {
	c, err := LoadCourseLang(dir, lang)
	if err != nil {
		return nil, err
	}
	return []Course{c}, nil
}
