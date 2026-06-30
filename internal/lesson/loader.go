package lesson

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"ztutor/internal/logutil"
)

// TestCase holds one graded run: inputs and the expected output.
// Test 1 comes from expected.txt / stdin.txt / args.txt.
// Test N (N ≥ 2) comes from expected.N.txt / stdin.N.txt / args.N.txt.
type TestCase struct {
	Stdin             string
	Args              string // space-separated runtime arguments, not yet parsed
	Expected          string
	ExpectedStdout    string
	ExpectedStderr    string
	HasExpectedStdout bool
	HasExpectedStderr bool
}

// ExerciseFile is one file in a multi-file exercise (from the files/ subdirectory).
type ExerciseFile struct {
	Name     string // filename, e.g. "main.c"
	Language string // inferred from extension, overridable via frontmatter
	Editable bool   // false = shown read-only in the editor
	Content  string // initial content loaded from disk
}

type Lesson struct {
	ID         string
	Title      string
	Content    string
	Exercise   string
	Difficulty string     // from YAML frontmatter: beginner / intermediate / advanced
	Tags       []string   // from YAML frontmatter
	Companies  []string   // from YAML frontmatter: companies that asked this (interview mode)
	References []string   // from YAML frontmatter: book refs, URLs, man pages
	Tutorial   []string   // from YAML frontmatter: Mochi dialogue beats shown before the exercise
	Hints      []string   // from hints.txt, separated by \n---\n
	Trivia     []string   // from trivia.txt, separated by \n---\n
	Answer     string     // from answer.md: full explanation for interview questions
	Tests      []TestCase // ordered graded test cases; Tests[0] is the primary test
	IsPremium  bool       // true if this lesson requires a license
	IsOfficial bool       // true if loaded from the official/ directory (hash-verified)

	// Multi-file exercise support. When non-empty, Files takes precedence over Exercise.
	Files       []ExerciseFile // loaded from files/ subdirectory
	BuildCmd    string         // e.g. "make" — replaces language compiler; must produce ./prog
	BuildOutput string         // output binary name produced by BuildCmd (default "prog")

	// EnabledWidgets lists widget names from frontmatter "widgets:" key.
	// When empty, all widgets are enabled (backwards-compatible default).
	// Example: ["flags", "stdin", "asm"]
	EnabledWidgets []string

	// Language-related fields. Set by the parent course unless overridden in frontmatter.
	Language           string // e.g. "c", "python"
	SourceExtension    string // e.g. ".c", ".py"
	SyntaxHighlighting string // Chroma lexer name
}

// IsReadOnly returns true for lessons that have content but no exercise to submit.
// These are reading/intro lessons that are auto-completed on first visit.
func (l *Lesson) IsReadOnly() bool {
	return l.Exercise == "" && len(l.Files) == 0
}

// MaxStars returns the maximum stars achievable for this lesson.
// Read-only lessons auto-complete with 1 star; exercise lessons can earn up to 3.
func (l *Lesson) MaxStars() int {
	if l.IsReadOnly() {
		return 1
	}
	return 3
}

// langExtension maps language names to their canonical source extension.
// Used when frontmatter declares a lesson-level language override.
var langExtension = map[string]string{
	"c":          ".c",
	"cpp":        ".cpp",
	"python":     ".py",
	"go":         ".go",
	"rust":       ".rs",
	"ruby":       ".rb",
	"javascript": ".js",
	"typescript": ".ts",
	"asm":        ".s",
	"bash":       ".sh",
}

func LoadAll(lessonsDir string) ([]Lesson, error) {
	return LoadAllLang(lessonsDir, "en")
}

func LoadAllLang(lessonsDir, lang string) ([]Lesson, error) {
	entries, err := os.ReadDir(lessonsDir)
	if err != nil {
		return nil, err
	}

	var lessons []Lesson
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		lesson, err := LoadLang(filepath.Join(lessonsDir, entry.Name()), lang)
		if err != nil {
			logutil.Warn("lesson: skipping %s: %v", entry.Name(), err)
			continue
		}
		lessons = append(lessons, *lesson)
	}

	sort.Slice(lessons, func(i, j int) bool {
		return lessons[i].ID < lessons[j].ID
	})

	return lessons, nil
}

// partialTC accumulates fields for one numbered test case during loading.
type partialTC struct {
	Stdin     string
	Args      string
	Expect    string
	ExpectOut string
	ExpectErr string
	hasExp    bool
	hasOut    bool
	hasErr    bool
}

// LoadLang loads a lesson from dir, preferring locale-specific files
// (e.g. lesson.es.md) before falling back to the default lesson.md.
func LoadLang(dir, lang string) (*Lesson, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	lesson := &Lesson{
		ID: filepath.Base(dir),
	}

	// testMap collects test-case data keyed by test number (1-based).
	testMap := make(map[int]*partialTC)

	var fm frontmatter // parsed from lesson.md — needed for files/ scanning
	var hasFilesDir bool

	for _, f := range dirEntries {
		name := f.Name()
		if f.IsDir() {
			if name == "files" {
				hasFilesDir = true
			}
			continue
		}
		path := filepath.Join(dir, name)

		switch {
		case name == "lesson.md" || (lang != "" && lang != "en" && name == "lesson."+lang+".md"):
			// Prefer the locale-specific file when both exist; the directory scan
			// may visit lesson.md before lesson.es.md — we overwrite either way so
			// the last match (sorted by ReadDir, alphabetical) wins.  To ensure the
			// locale file always wins, skip the base file when a locale file exists.
			if name == "lesson.md" && lang != "" && lang != "en" {
				if _, statErr := os.Stat(filepath.Join(dir, "lesson."+lang+".md")); statErr == nil {
					continue // locale-specific file exists; skip base file
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			raw := string(data)
			parsedFm, body := parseFrontmatter(raw)
			fm = parsedFm
			lesson.Difficulty = fm.Difficulty
			lesson.Tags = fm.Tags
			lesson.Companies = fm.Companies
			lesson.References = fm.References
			lesson.Tutorial = fm.Tutorial
			lesson.IsPremium = fm.Premium
			lesson.EnabledWidgets = fm.Widgets
			lesson.Content = body
			lesson.Title = extractTitle(lesson.Content)

		case name == "exercise.c", strings.HasPrefix(name, "exercise."):
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			lesson.Exercise = string(data)

		case name == "hints.txt" || (lang != "" && lang != "en" && name == "hints."+lang+".txt"):
			if name == "hints.txt" && lang != "" && lang != "en" {
				if _, statErr := os.Stat(filepath.Join(dir, "hints."+lang+".txt")); statErr == nil {
					continue
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			lesson.Hints = splitBlocks(string(data))

		case name == "trivia.txt" || (lang != "" && lang != "en" && name == "trivia."+lang+".txt"):
			if name == "trivia.txt" && lang != "" && lang != "en" {
				if _, statErr := os.Stat(filepath.Join(dir, "trivia."+lang+".txt")); statErr == nil {
					continue
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			lesson.Trivia = splitBlocks(string(data))

		case name == "answer.md" || (lang != "" && lang != "en" && name == "answer."+lang+".md"):
			if name == "answer.md" && lang != "" && lang != "en" {
				if _, statErr := os.Stat(filepath.Join(dir, "answer."+lang+".md")); statErr == nil {
					continue
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			lesson.Answer = strings.TrimSpace(string(data))

		case name == "expected.txt":
			data, _ := os.ReadFile(path)
			tc := getOrCreate(testMap, 1)
			tc.Expect = string(data)
			tc.hasExp = true

		case name == "expected.stdout.txt":
			data, _ := os.ReadFile(path)
			tc := getOrCreate(testMap, 1)
			tc.ExpectOut = string(data)
			tc.hasOut = true

		case name == "expected.stderr.txt":
			data, _ := os.ReadFile(path)
			tc := getOrCreate(testMap, 1)
			tc.ExpectErr = string(data)
			tc.hasErr = true

		case name == "stdin.txt":
			data, _ := os.ReadFile(path)
			getOrCreate(testMap, 1).Stdin = string(data)

		case name == "args.txt":
			data, _ := os.ReadFile(path)
			getOrCreate(testMap, 1).Args = strings.TrimSpace(string(data))

		default:
			// Numbered test files: expected.2.txt, stdin.3.txt, args.2.txt …
			n, field, ok := parseTestFilename(name)
			if !ok {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			tc := getOrCreate(testMap, n)
			switch field {
			case "expected":
				tc.Expect = string(data)
				tc.hasExp = true
			case "expected.stdout":
				tc.ExpectOut = string(data)
				tc.hasOut = true
			case "expected.stderr":
				tc.ExpectErr = string(data)
				tc.hasErr = true
			case "stdin":
				tc.Stdin = string(data)
			case "args":
				tc.Args = strings.TrimSpace(string(data))
			}
		}
	}

	// A lesson without content is broken (e.g. lesson.md is a directory).
	if lesson.Content == "" && lesson.Exercise == "" && len(lesson.Files) == 0 {
		return nil, fmt.Errorf("no lesson.md content found")
	}

	// Apply per-lesson language override from frontmatter (takes precedence over course-level).
	if fm.Language != "" {
		lesson.Language = fm.Language
		lesson.SyntaxHighlighting = fm.Language
		if ext, ok := langExtension[fm.Language]; ok {
			lesson.SourceExtension = ext
		}
	}
	if fm.Build != "" {
		lesson.BuildCmd = fm.Build
	}
	if fm.BuildOutput != "" {
		lesson.BuildOutput = fm.BuildOutput
	}

	// Load multi-file exercise from files/ subdirectory.
	if hasFilesDir {
		lesson.Files = loadExerciseFiles(filepath.Join(dir, "files"), fm.Files)
	}

	// Build Tests slice in order. Only include entries that have expected output.
	var nums []int
	for n, tc := range testMap {
		if tc.hasExp || tc.hasOut || tc.hasErr {
			nums = append(nums, n)
		}
	}
	sort.Ints(nums)
	for _, n := range nums {
		tc := testMap[n]
		lesson.Tests = append(lesson.Tests, TestCase{
			Stdin:             tc.Stdin,
			Args:              tc.Args,
			Expected:          tc.Expect,
			ExpectedStdout:    tc.ExpectOut,
			ExpectedStderr:    tc.ExpectErr,
			HasExpectedStdout: tc.hasOut,
			HasExpectedStderr: tc.hasErr,
		})
	}

	if lesson.Title == "" {
		lesson.Title = lesson.ID
	}

	return lesson, nil
}

// loadExerciseFiles reads all files from filesDir and returns them as ExerciseFile slice.
// fileMeta provides per-file overrides declared in the lesson frontmatter (editable, language).
func loadExerciseFiles(filesDir string, fileMeta []fileFrontmatter) []ExerciseFile {
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		return nil
	}

	metaByName := make(map[string]fileFrontmatter, len(fileMeta))
	for _, m := range fileMeta {
		metaByName[m.Name] = m
	}

	var result []ExerciseFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join(filesDir, name))
		if err != nil {
			logutil.Warn("lesson: read exercise file %s: %v", name, err)
			continue
		}

		ef := ExerciseFile{
			Name:     name,
			Editable: true,
			Language: langFromExtension(name),
			Content:  string(data),
		}
		if m, ok := metaByName[name]; ok {
			if m.Language != "" {
				ef.Language = m.Language
			}
			if m.Editable != nil {
				ef.Editable = *m.Editable
			}
		}
		result = append(result, ef)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// langFromExtension infers the language name from a filename's extension.
// Returns "" when the extension is unknown.
func langFromExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.ToLower(filepath.Base(filename))
	switch ext {
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "c"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".s", ".asm":
		return "asm"
	case ".rb":
		return "ruby"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".sh":
		return "bash"
	}
	if base == "makefile" || base == "gnumakefile" {
		return "makefile"
	}
	return ""
}

func getOrCreate(m map[int]*partialTC, n int) *partialTC {
	if m[n] == nil {
		m[n] = &partialTC{}
	}
	return m[n]
}

// parseTestFilename matches "expected.2.txt", "expected.stdout.2.txt",
// "expected.stderr.2.txt", "stdin.3.txt", "args.2.txt" etc.
// Returns (testNum ≥ 2, fieldName, true) or (0, "", false).
func parseTestFilename(name string) (num int, field string, ok bool) {
	for _, f := range []string{"expected", "expected.stdout", "expected.stderr", "stdin", "args"} {
		prefix := f + "."
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".txt") {
			if len(name) <= len(prefix)+4 {
				return 0, "", false
			}
			middle := name[len(prefix) : len(name)-4]
			n, err := strconv.Atoi(middle)
			if err == nil && n >= 2 {
				return n, f, true
			}
		}
	}
	return 0, "", false
}

type frontmatter struct {
	Difficulty  string            `yaml:"difficulty"`
	Tags        []string          `yaml:"tags"`
	Companies   []string          `yaml:"companies"`
	References  []string          `yaml:"references"`
	Tutorial    []string          `yaml:"tutorial"`
	Premium     bool              `yaml:"premium"`
	Language    string            `yaml:"language,omitempty"`
	Build       string            `yaml:"build,omitempty"`
	BuildOutput string            `yaml:"build_output,omitempty"`
	Files       []fileFrontmatter `yaml:"files,omitempty"`
	Widgets     []string          `yaml:"widgets,omitempty"`
}

// fileFrontmatter declares per-file metadata within a lesson's YAML frontmatter.
type fileFrontmatter struct {
	Name     string `yaml:"name"`
	Language string `yaml:"language,omitempty"`
	Editable *bool  `yaml:"editable,omitempty"` // nil = default true
}

func parseFrontmatter(raw string) (fm frontmatter, body string) {
	if !strings.HasPrefix(raw, "---") {
		return fm, raw
	}
	lines := strings.SplitN(raw, "\n", 2)
	if len(lines) < 2 {
		return fm, raw
	}

	endIdx := strings.Index(lines[1], "\n---\n")
	if endIdx == -1 {
		return fm, raw
	}

	doc := lines[1][:endIdx]
	body = lines[1][endIdx+5:]
	body = strings.TrimPrefix(body, "\n")

	if err := yaml.Unmarshal([]byte(doc), &fm); err != nil {
		return frontmatter{}, raw
	}
	return fm, body
}

// splitBlocks splits text on "\n---\n" and returns non-empty trimmed blocks.
func splitBlocks(raw string) []string {
	var out []string
	for _, part := range strings.Split(strings.TrimSpace(raw), "\n---\n") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func extractTitle(content string) string {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) > 0 {
		title := strings.TrimPrefix(lines[0], "# ")
		title = strings.TrimSpace(title)
		return title
	}
	return ""
}
