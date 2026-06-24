package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ztutor/internal/editor"
	"ztutor/internal/sandbox"

	"gopkg.in/yaml.v3"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Lesson Creation Wizard ────────────────────────────────────────────────────

type lessonWizardStep int

const (
	stepMeta     lessonWizardStep = iota // ID, title, difficulty, tags
	stepContent                          // lesson.md body
	stepExercise                         // exercise.c starter
	stepExpected                         // expected output (compile or manual)
	stepTutorial                         // Mochi dialogue beats
	stepHints                            // hints
	stepSave                             // review and save
)

var wizardStepNames = []string{
	"Lesson Metadata",
	"Lesson Content",
	"Exercise Code",
	"Expected Output",
	"Tutorial Beats",
	"Hints",
	"Save",
}

var difficultyOptions = []string{"beginner", "intermediate", "advanced"}

type adminLessonCreateModel struct {
	lessonsDir string

	step lessonWizardStep

	// step 0: metadata
	idInput        textinput.Model
	titleInput     textinput.Model
	tagsInput      textinput.Model
	companiesInput textinput.Model
	difficulty     string
	isInterview    bool
	language       string
	premium        bool
	metaFocus      int // 0=id 1=title 2=type 3=difficulty 4=tags/companies 5=language 6=premium

	// step 1: lesson body
	contentArea textarea.Model

	// step 2: exercise starter
	exerciseEditor *editor.CodeEditor

	// step 3: expected output
	expectedMode   int // 0=solution editor, 1=manual
	solutionEditor *editor.CodeEditor
	manualExpected textarea.Model
	compiling      bool
	capturedOutput string
	captureErr     string

	// step 4: tutorial beats (one per line)
	tutorialArea textarea.Model

	// step 5: hints (--- separated blocks)
	hintsArea textarea.Model

	// file import overlay
	importMode  bool
	importInput textinput.Model
	importErr   string

	// edit mode (for editing existing lessons)
	editMode      bool
	editDir       string
	sectionPicker bool // edit mode: show section-jump menu instead of wizard step
	sectionCursor int  // selected row in the section-jump menu

	// general
	msg string
	sized
}

func newAdminLessonCreate(lessonsDir string, w, h int) *adminLessonCreateModel {
	idInput := textinput.New()
	idInput.Placeholder = "10-arrays"
	idInput.CharLimit = 64
	idInput.Width = 40

	titleInput := textinput.New()
	titleInput.Placeholder = "Arrays in C"
	titleInput.CharLimit = 128
	titleInput.Width = 40

	tagsInput := textinput.New()
	tagsInput.Placeholder = "arrays, pointers"
	tagsInput.CharLimit = 200
	tagsInput.Width = 40

	companiesInput := textinput.New()
	companiesInput.Placeholder = "Google, Amazon, Meta"
	companiesInput.CharLimit = 200
	companiesInput.Width = 40

	editorH, contentH := wizardEditorHeights(h)
	formW := wizardFormWidth(w)

	contentArea := textarea.New()
	contentArea.SetWidth(formW)
	contentArea.SetHeight(contentH)
	contentArea.ShowLineNumbers = false
	contentArea.Placeholder = "Write your lesson content in Markdown..."

	tutorialArea := textarea.New()
	tutorialArea.SetWidth(formW)
	tutorialArea.SetHeight(contentH)
	tutorialArea.ShowLineNumbers = false
	tutorialArea.Placeholder = "One dialogue beat per line. These appear before the exercise."

	hintsArea := textarea.New()
	hintsArea.SetWidth(formW)
	hintsArea.SetHeight(contentH)
	hintsArea.ShowLineNumbers = false
	hintsArea.Placeholder = "One hint per block. Separate blocks with a line containing only ---"

	manualExpected := textarea.New()
	manualExpected.SetWidth(formW)
	manualExpected.SetHeight(contentH / 2)
	manualExpected.ShowLineNumbers = false
	manualExpected.Placeholder = "Type the expected program output here..."

	importInput := textinput.New()
	importInput.Placeholder = "/absolute/path/to/file.c"
	importInput.CharLimit = 512
	importInput.Width = formW

	m := &adminLessonCreateModel{
		lessonsDir:     lessonsDir,
		difficulty:     "beginner",
		language:       "c",
		idInput:        idInput,
		titleInput:     titleInput,
		tagsInput:      tagsInput,
		companiesInput: companiesInput,
		contentArea:    contentArea,
		exerciseEditor: editor.New("", formW, editorH, "c"),
		solutionEditor: editor.New("", formW, editorH/2, "c"),
		manualExpected: manualExpected,
		tutorialArea:   tutorialArea,
		hintsArea:      hintsArea,
		importInput:    importInput,
		sized:          sized{Width: w, Height: h},
	}
	m.idInput.Focus()
	return m
}

func wizardFormWidth(w int) int {
	fw := w - 8
	if fw > 100 {
		fw = 100
	}
	if fw < 30 {
		fw = 30
	}
	return fw
}

func wizardEditorHeights(h int) (editorH, contentH int) {
	contentH = h - 10
	if contentH < 4 {
		contentH = 4
	}
	editorH = contentH / 2
	if editorH < 4 {
		editorH = 4
	}
	return
}

func newAdminLessonEdit(lessonsDir, existingDir string, w, h int) (*adminLessonCreateModel, error) {
	m := newAdminLessonCreate(lessonsDir, w, h)
	m.editMode = true
	m.editDir = existingDir
	if err := m.loadExisting(); err != nil {
		return nil, err
	}
	m.sectionPicker = true
	m.sectionCursor = 0
	return m, nil
}

func (m *adminLessonCreateModel) loadExisting() error {
	lessonPath := filepath.Join(m.editDir, "lesson.md")
	data, err := os.ReadFile(lessonPath)
	if err != nil {
		return fmt.Errorf("read lesson.md: %w", err)
	}
	content := string(data)

	// Parse YAML frontmatter between --- delimiters.
	var fm struct {
		Difficulty string   `yaml:"difficulty"`
		Tags       []string `yaml:"tags"`
		Companies  []string `yaml:"companies"`
		Tutorial   []string `yaml:"tutorial"`
		Premium    bool     `yaml:"premium"`
	}
	body := content
	if strings.HasPrefix(content, "---\n") {
		idx := strings.Index(content[4:], "\n---\n")
		if idx >= 0 {
			fmBlock := content[4 : idx+4]
			body = content[idx+9:]
			_ = yaml.Unmarshal([]byte(fmBlock), &fm)
		}
	}

	// Extract title from first markdown heading.
	title := ""
	afterFm := strings.TrimLeft(body, "\n")
	for _, line := range strings.Split(afterFm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			title = strings.TrimPrefix(line, "# ")
			break
		}
	}

	// Set metadata fields.
	id := filepath.Base(m.editDir)
	m.idInput.SetValue(id)
	m.titleInput.SetValue(title)
	m.difficulty = fm.Difficulty
	if m.difficulty == "" {
		m.difficulty = "beginner"
	}
	m.premium = fm.Premium
	if len(fm.Companies) > 0 {
		m.isInterview = true
		m.companiesInput.SetValue(strings.Join(fm.Companies, ", "))
	} else {
		m.isInterview = false
		m.tagsInput.SetValue(strings.Join(fm.Tags, ", "))
	}

	// Content body (after frontmatter and title).
	bodyText := afterFm
	if title != "" {
		idx := strings.Index(bodyText, title)
		if idx >= 0 {
			bodyText = bodyText[idx+len(title):]
		}
	}
	bodyText = strings.TrimSpace(bodyText)
	m.contentArea.SetValue(bodyText)

	// Tutorial beats.
	m.tutorialArea.SetValue(strings.Join(fm.Tutorial, "\n"))

	// Exercise code (any exercise.* file).
	var exercisePath string
	if entries, err := os.ReadDir(m.editDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), "exercise.") {
				exercisePath = filepath.Join(m.editDir, entry.Name())
				break
			}
		}
	}
	if exercisePath != "" {
		if exData, err := os.ReadFile(exercisePath); err == nil {
			m.exerciseEditor.SetContent(strings.TrimSpace(string(exData)))
			ext := filepath.Ext(exercisePath)
			for _, lang := range sandbox.AllLanguages() {
				if lang.SourceExtension() == ext {
					m.language = lang.Name()
					m.exerciseEditor.SetLanguage(lang.Name())
					m.solutionEditor.SetLanguage(lang.Name())
					break
				}
			}
		}
	}

	// Expected output.
	expectedPath := filepath.Join(m.editDir, "expected.txt")
	if expData, err := os.ReadFile(expectedPath); err == nil {
		m.expectedMode = 1
		m.manualExpected.SetValue(strings.TrimRight(string(expData), "\n"))
	}

	// Hints.
	hintsPath := filepath.Join(m.editDir, "hints.txt")
	if hData, err := os.ReadFile(hintsPath); err == nil {
		m.hintsArea.SetValue(strings.TrimRight(string(hData), "\n"))
	}

	return nil
}

func (m *adminLessonCreateModel) Init() tea.Cmd { return nil }

// focusCurrentStep blurs everything and focuses the component for the current step.
func (m *adminLessonCreateModel) focusCurrentStep() {
	m.idInput.Blur()
	m.titleInput.Blur()
	m.tagsInput.Blur()
	m.companiesInput.Blur()
	m.contentArea.Blur()
	m.exerciseEditor.Blur()
	m.solutionEditor.Blur()
	m.manualExpected.Blur()
	m.tutorialArea.Blur()
	m.hintsArea.Blur()

	switch m.step {
	case stepMeta:
		switch m.metaFocus {
		case 0:
			m.idInput.Focus()
		case 1:
			m.titleInput.Focus()
		case 4:
			if m.isInterview {
				m.companiesInput.Focus()
			} else {
				m.tagsInput.Focus()
			}
		}
	case stepContent:
		m.contentArea.Focus()
	case stepExercise:
		m.exerciseEditor.Focus()
	case stepExpected:
		if m.expectedMode == 0 {
			m.solutionEditor.Focus()
		} else {
			m.manualExpected.Focus()
		}
	case stepTutorial:
		m.tutorialArea.Focus()
	case stepHints:
		m.hintsArea.Focus()
	}
}

func (m *adminLessonCreateModel) cycleDifficulty(dir int) {
	cur := 0
	for i, d := range difficultyOptions {
		if d == m.difficulty {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(difficultyOptions)) % len(difficultyOptions)
	m.difficulty = difficultyOptions[cur]
}

func (m *adminLessonCreateModel) cycleLanguage(dir int) {
	all := sandbox.AllLanguages()
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	cur := 0
	for i, k := range keys {
		if k == m.language {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(keys)) % len(keys)
	m.language = keys[cur]

	lang := sandbox.GetLanguage(m.language)
	if lang != nil {
		m.exerciseEditor.SetLanguage(lang.Name())
		m.solutionEditor.SetLanguage(lang.Name())
	}
}

func (m *adminLessonCreateModel) setMetaFocus(f int) {
	switch m.metaFocus {
	case 0:
		m.idInput.Blur()
	case 1:
		m.titleInput.Blur()
	case 4:
		m.tagsInput.Blur()
		m.companiesInput.Blur()
	}
	m.metaFocus = (f + 7) % 7
	switch m.metaFocus {
	case 0:
		m.idInput.Focus()
	case 1:
		m.titleInput.Focus()
	case 4:
		if m.isInterview {
			m.companiesInput.Focus()
		} else {
			m.tagsInput.Focus()
		}
	}
}

func (m *adminLessonCreateModel) targetDir() string {
	return m.lessonsDir
}

func (m *adminLessonCreateModel) advanceStep() tea.Cmd {
	if m.step == stepMeta {
		id := strings.TrimSpace(m.idInput.Value())
		if id == "" {
			m.msg = "ID is required"
			return nil
		}
		if strings.ContainsAny(id, " \t/\\") {
			m.msg = "ID must be a slug (no spaces or slashes)"
			return nil
		}
		if tgt := m.targetDir(); tgt != "" {
			if _, err := os.Stat(filepath.Join(tgt, id)); err == nil {
				m.msg = "a lesson with that ID already exists"
				return nil
			}
		}
		if strings.TrimSpace(m.titleInput.Value()) == "" {
			m.msg = "Title is required"
			return nil
		}
	}
	m.msg = ""
	if m.step < stepSave {
		m.step++
		m.focusCurrentStep()
	}
	return nil
}

func (m *adminLessonCreateModel) retreatStep() {
	if m.step > 0 {
		m.step--
		m.focusCurrentStep()
	}
}

func (m *adminLessonCreateModel) resize(w, h int) {
	m.Width = w
	m.Height = h
	editorH, contentH := wizardEditorHeights(h)
	formW := wizardFormWidth(w)
	m.contentArea.SetWidth(formW)
	m.contentArea.SetHeight(contentH)
	m.tutorialArea.SetWidth(formW)
	m.tutorialArea.SetHeight(contentH)
	m.hintsArea.SetWidth(formW)
	m.hintsArea.SetHeight(contentH)
	m.manualExpected.SetWidth(formW)
	m.manualExpected.SetHeight(contentH / 2)
	m.exerciseEditor.SetSize(formW, editorH)
	m.solutionEditor.SetSize(formW, editorH)
}

func (m *adminLessonCreateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
		m.resize(msg.Width, msg.Height)
		return m, nil

	case adminCompileResultMsg:
		m.compiling = false
		if msg.errStr != "" {
			m.captureErr = msg.errStr
			m.capturedOutput = ""
		} else {
			m.capturedOutput = msg.output
			m.captureErr = ""
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		// Import overlay intercepts all keys when active.
		if m.importMode {
			return m.updateImport(msg)
		}

		// Section-jump menu (edit mode only) intercepts all keys when active.
		if m.sectionPicker {
			return m.updateSectionPicker(msg)
		}

		if key == "ctrl+q" {
			return m, backCmd(NavigateToAdminDashboard{})
		}
		// In edit mode, esc or ctrl+p returns to the section-jump menu.
		if m.editMode && (key == "esc" || key == "ctrl+p") {
			m.sectionPicker = true
			return m, nil
		}
		if key == "ctrl+p" {
			m.retreatStep()
			return m, nil
		}
		if key == "ctrl+n" {
			return m, m.advanceStep()
		}
		if key == "ctrl+j" && m.step != stepMeta && m.step != stepSave {
			m.importMode = true
			m.importErr = ""
			m.importInput.SetValue("")
			m.importInput.Focus()
			return m, textinput.Blink
		}

		switch m.step {
		case stepMeta:
			return m.updateMeta(msg)
		case stepContent:
			return m.updateTextarea(msg, &m.contentArea)
		case stepExercise:
			return m.updateEditor(msg, m.exerciseEditor)
		case stepExpected:
			return m.updateExpected(msg)
		case stepTutorial:
			return m.updateTextarea(msg, &m.tutorialArea)
		case stepHints:
			return m.updateTextarea(msg, &m.hintsArea)
		case stepSave:
			return m.updateSave(msg)
		}
	}
	return m, nil
}

func (m *adminLessonCreateModel) updateMeta(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.setMetaFocus(m.metaFocus + 1)
		return m, nil
	case "shift+tab", "up":
		m.setMetaFocus(m.metaFocus - 1)
		return m, nil
	case "enter":
		if m.metaFocus == 6 {
			return m, m.advanceStep()
		}
		m.setMetaFocus(m.metaFocus + 1)
		return m, nil
	case "left", " ":
		switch m.metaFocus {
		case 2:
			m.isInterview = !m.isInterview
			m.tagsInput.Blur()
			m.companiesInput.Blur()
			return m, nil
		case 3:
			m.cycleDifficulty(-1)
			return m, nil
		case 5:
			m.cycleLanguage(-1)
			return m, nil
		case 6:
			m.premium = !m.premium
			return m, nil
		}
	case "right":
		switch m.metaFocus {
		case 2:
			m.isInterview = !m.isInterview
			m.tagsInput.Blur()
			m.companiesInput.Blur()
			return m, nil
		case 3:
			m.cycleDifficulty(1)
			return m, nil
		case 5:
			m.cycleLanguage(1)
			return m, nil
		case 6:
			m.premium = !m.premium
			return m, nil
		}
	}
	var cmd tea.Cmd
	switch m.metaFocus {
	case 0:
		m.idInput, cmd = m.idInput.Update(msg)
	case 1:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case 4:
		if m.isInterview {
			m.companiesInput, cmd = m.companiesInput.Update(msg)
		} else {
			m.tagsInput, cmd = m.tagsInput.Update(msg)
		}
	}
	return m, cmd
}

func (m *adminLessonCreateModel) updateTextarea(msg tea.KeyMsg, ta *textarea.Model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	*ta, cmd = ta.Update(msg)
	return m, cmd
}

func (m *adminLessonCreateModel) updateEditor(msg tea.KeyMsg, ed *editor.CodeEditor) (tea.Model, tea.Cmd) {
	newEd, cmd := ed.Update(msg)
	*ed = *newEd
	return m, cmd
}

func (m *adminLessonCreateModel) updateExpected(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.expectedMode == 0 {
			m.solutionEditor.Blur()
			m.expectedMode = 1
			m.manualExpected.Focus()
		} else {
			m.manualExpected.Blur()
			m.expectedMode = 0
			m.solutionEditor.Focus()
		}
		return m, nil
	case "ctrl+r":
		if m.expectedMode == 0 && !m.compiling {
			m.compiling = true
			m.capturedOutput = ""
			m.captureErr = ""
			code := m.solutionEditor.Value()
			return m, adminCompileCmd(code, m.language)
		}
	}
	if m.expectedMode == 0 {
		return m.updateEditor(msg, m.solutionEditor)
	}
	return m.updateTextarea(msg, &m.manualExpected)
}

func (m *adminLessonCreateModel) updateSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if err := m.saveLesson(); err != nil {
			m.msg = exErrorStyle.Render("save failed: " + err.Error())
			return m, nil
		}
		id := strings.TrimSpace(m.idInput.Value())
		return m, backCmd(adminLessonSavedMsg{id: id})
	case "b", "ctrl+p":
		if m.editMode {
			m.sectionPicker = true
			return m, nil
		}
		m.retreatStep()
		return m, nil
	}
	return m, nil
}

func (m *adminLessonCreateModel) updateSectionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc", "ctrl+q":
		dir := m.lessonsDir
		return m, backCmd(NavigateToAdminLessonPicker{Dir: dir})
	case "j", "down":
		if m.sectionCursor < int(stepSave) {
			m.sectionCursor++
		}
	case "k", "up":
		if m.sectionCursor > 0 {
			m.sectionCursor--
		}
	case "enter", " ":
		m.step = lessonWizardStep(m.sectionCursor)
		m.sectionPicker = false
		m.focusCurrentStep()
	}
	return m, nil
}

func (m *adminLessonCreateModel) updateImport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.importMode = false
		m.importErr = ""
		m.importInput.Blur()
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.importInput.Value())
		if err := m.loadFileIntoStep(path); err != nil {
			m.importErr = err.Error()
			return m, nil
		}
		m.importMode = false
		m.importErr = ""
		m.importInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.importInput, cmd = m.importInput.Update(msg)
	return m, cmd
}

func (m *adminLessonCreateModel) loadFileIntoStep(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	switch m.step {
	case stepContent:
		m.contentArea.SetValue(content)
	case stepExercise:
		m.exerciseEditor.SetContent(content)
	case stepExpected:
		if m.expectedMode == 0 {
			m.solutionEditor.SetContent(content)
		} else {
			m.manualExpected.SetValue(content)
		}
	case stepTutorial:
		m.tutorialArea.SetValue(content)
	case stepHints:
		m.hintsArea.SetValue(content)
	}
	return nil
}

// adminCompileCmd runs the sandbox in a goroutine and returns the result.
func adminCompileCmd(code, language string) tea.Cmd {
	lang := sandbox.GetLanguage(language)
	if lang == nil {
		lang = sandbox.GetLanguage("c")
	}
	return func() tea.Msg {
		result, err := sandbox.Run(lang, map[string]string{lang.SourceFileName(): code}, "", "", nil, nil)
		if err != nil {
			return adminCompileResultMsg{errStr: err.Error()}
		}
		if result.Error != "" {
			return adminCompileResultMsg{errStr: result.Error}
		}
		return adminCompileResultMsg{output: result.Output}
	}
}

// saveLesson writes all lesson files to disk.
func (m *adminLessonCreateModel) saveLesson() error {
	id := strings.TrimSpace(m.idInput.Value())
	var dir string
	if m.editMode {
		dir = m.editDir
		if filepath.Base(dir) != id {
			newDir := filepath.Join(filepath.Dir(dir), id)
			if err := os.Rename(dir, newDir); err != nil {
				return fmt.Errorf("rename lesson dir: %w", err)
			}
			m.editDir = newDir
			dir = newDir
		}
	} else {
		dir = filepath.Join(m.targetDir(), id)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	var tags, companies []string
	if m.isInterview {
		companies = parseLessonTags(m.companiesInput.Value())
	} else {
		tags = parseLessonTags(m.tagsInput.Value())
	}
	tutorial := parseTutorialBeats(m.tutorialArea.Value())

	type lessonFM struct {
		Difficulty string   `yaml:"difficulty"`
		Tags       []string `yaml:"tags,omitempty"`
		Companies  []string `yaml:"companies,omitempty"`
		Tutorial   []string `yaml:"tutorial,omitempty"`
		Premium    bool     `yaml:"premium,omitempty"`
	}
	fmData, err := yaml.Marshal(lessonFM{
		Difficulty: m.difficulty,
		Tags:       tags,
		Companies:  companies,
		Tutorial:   tutorial,
		Premium:    m.premium,
	})
	if err != nil {
		return err
	}

	title := strings.TrimSpace(m.titleInput.Value())
	body := strings.TrimSpace(m.contentArea.Value())
	lessonMD := "---\n" + string(fmData) + "---\n# " + title + "\n"
	if body != "" {
		lessonMD += "\n" + body + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "lesson.md"), []byte(lessonMD), 0644); err != nil {
		return err
	}

	if code := strings.TrimSpace(m.exerciseEditor.Value()); code != "" {
		ext := ".c"
		if lang := sandbox.GetLanguage(m.language); lang != nil {
			ext = lang.SourceExtension()
		}
		if err := os.WriteFile(filepath.Join(dir, "exercise"+ext), []byte(code+"\n"), 0644); err != nil {
			return err
		}
	}

	expected := ""
	if m.expectedMode == 0 {
		expected = m.capturedOutput
	} else {
		expected = m.manualExpected.Value()
	}
	if strings.TrimSpace(expected) != "" {
		if err := os.WriteFile(filepath.Join(dir, "expected.txt"), []byte(expected), 0644); err != nil {
			return err
		}
	}

	if hints := strings.TrimSpace(m.hintsArea.Value()); hints != "" {
		if err := os.WriteFile(filepath.Join(dir, "hints.txt"), []byte(hints+"\n"), 0644); err != nil {
			return err
		}
	}

	return nil
}

func parseLessonTags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseTutorialBeats(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ── Views ─────────────────────────────────────────────────────────────────────

func (m *adminLessonCreateModel) viewSectionPicker() string {
	id := strings.TrimSpace(m.idInput.Value())
	title := strings.TrimSpace(m.titleInput.Value())

	var b strings.Builder
	b.WriteString(titleStyle("Edit Lesson") + "  " + dim(id+" — "+title) + "\n\n")

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))

	for i, name := range wizardStepNames {
		if i == m.sectionCursor {
			b.WriteString(selStyle.Render(fmt.Sprintf("> %s", name)) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", name))
		}
	}

	b.WriteString("\n" + helpBar("j/k choose", "enter edit section", "q back to list"))
	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}

func (m *adminLessonCreateModel) View() string {
	if m.sectionPicker {
		return m.viewSectionPicker()
	}

	var b strings.Builder

	stepName := wizardStepNames[int(m.step)]
	if m.editMode {
		if m.step < stepSave {
			b.WriteString(titleStyle("Edit Lesson") +
				dim(fmt.Sprintf("  Step %d/%d: %s", int(m.step)+1, int(stepSave), stepName)) + "\n\n")
		} else {
			b.WriteString(titleStyle("Edit Lesson") + dim("  Save") + "\n\n")
		}
	} else {
		if m.step < stepSave {
			b.WriteString(titleStyle("Create Lesson") +
				dim(fmt.Sprintf("  Step %d/%d: %s", int(m.step)+1, int(stepSave), stepName)) + "\n\n")
		} else {
			b.WriteString(titleStyle("Create Lesson") + dim("  Save") + "\n\n")
		}
	}

	switch m.step {
	case stepMeta:
		b.WriteString(m.viewMeta())
	case stepContent:
		b.WriteString(m.viewTextarea(&m.contentArea, "Write your lesson content in Markdown."))
	case stepExercise:
		b.WriteString(m.viewEditor(m.exerciseEditor, "Write the starter code students will see. Leave empty for a blank editor."))
	case stepExpected:
		b.WriteString(m.viewExpected())
	case stepTutorial:
		b.WriteString(m.viewTextarea(&m.tutorialArea, "One Mochi dialogue beat per line. These appear before the exercise starts."))
	case stepHints:
		b.WriteString(m.viewTextarea(&m.hintsArea, "One hint per block. Separate blocks with a line containing only ---"))
	case stepSave:
		b.WriteString(m.viewSave())
	}

	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}

func (m *adminLessonCreateModel) viewMeta() string {
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	labelW := 14

	field := func(label string, idx int, value string) string {
		lbl := fmt.Sprintf("%-*s", labelW, label+":")
		if m.metaFocus == idx {
			return focusStyle.Render(lbl) + " " + value
		}
		return dimStyle.Render(lbl) + " " + value
	}

	// type toggle display
	var typeVal string
	if m.metaFocus == 2 {
		if m.isInterview {
			typeVal = dim("Lesson") + "  " + focusStyle.Render("[ Interview ]")
		} else {
			typeVal = focusStyle.Render("[ Lesson ]") + "  " + dim("Interview")
		}
	} else {
		if m.isInterview {
			typeVal = dim("Interview")
		} else {
			typeVal = dim("Lesson")
		}
	}

	// difficulty display
	diffVal := "< " + m.difficulty + " >"
	if m.metaFocus != 3 {
		diffVal = dim(m.difficulty)
	}

	// language display
	langDisplay := m.language
	if l := sandbox.GetLanguage(m.language); l != nil {
		langDisplay = l.DisplayName()
	}
	langVal := "< " + langDisplay + " >"
	if m.metaFocus == 5 {
		langVal = focusStyle.Render("< " + langDisplay + " >")
	}

	var b strings.Builder
	b.WriteString(field("ID", 0, m.idInput.View()) + "\n")
	b.WriteString(field("Title", 1, m.titleInput.View()) + "\n")
	b.WriteString(field("Type", 2, typeVal) + "\n")
	b.WriteString(field("Difficulty", 3, diffVal) + "\n")
	b.WriteString(field("Language", 5, langVal) + "\n")
	premiumVal := dim("no")
	if m.premium {
		premiumVal = focusStyle.Render("yes")
	}
	if m.metaFocus == 6 {
		premiumVal = "< " + premiumVal + " >"
	} else {
		premiumVal = dim("< ") + premiumVal + dim(" >")
	}
	b.WriteString(field("Premium", 6, premiumVal) + "\n")
	if m.isInterview {
		b.WriteString(field("Companies", 4, m.companiesInput.View()) + "\n")
	} else {
		b.WriteString(field("Tags", 4, m.tagsInput.View()) + "\n")
	}

	if m.msg != "" {
		b.WriteString("\n" + exErrorStyle.Render(m.msg) + "\n")
	}

	if m.editMode {
		b.WriteString("\n" + helpBar("tab next field", "space/left/right toggle", "ctrl+n next", "ctrl+p menu", "ctrl+q cancel"))
	} else {
		b.WriteString("\n" + helpBar("tab next field", "space/left/right toggle", "ctrl+n next step", "ctrl+q cancel"))
	}
	return b.String()
}

func (m *adminLessonCreateModel) viewImportOverlay() string {
	var b strings.Builder
	b.WriteString(dim("Import from file (absolute path):") + "\n")
	b.WriteString(m.importInput.View() + "\n")
	if m.importErr != "" {
		b.WriteString(exErrorStyle.Render(m.importErr) + "\n")
	}
	b.WriteString(helpBar("enter load", "esc cancel"))
	return b.String()
}

func (m *adminLessonCreateModel) viewTextarea(ta *textarea.Model, hint string) string {
	var b strings.Builder
	b.WriteString(dim(hint) + "\n\n")
	b.WriteString(ta.View() + "\n\n")
	if m.importMode {
		b.WriteString(m.viewImportOverlay())
	} else {
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		b.WriteString(helpBar("ctrl+n next", backHint, "ctrl+j import", "ctrl+q cancel"))
	}
	return b.String()
}

func (m *adminLessonCreateModel) viewEditor(ed *editor.CodeEditor, hint string) string {
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(dim(hint) + "\n\n")
	b.WriteString(borderStyle.Render(ed.View()) + "\n\n")
	if m.importMode {
		b.WriteString(m.viewImportOverlay())
	} else {
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		b.WriteString(helpBar("ctrl+n next", backHint, "ctrl+j import", "ctrl+q cancel"))
	}
	return b.String()
}

func (m *adminLessonCreateModel) viewExpected() string {
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(0, 1)

	modeA := "Solution"
	modeB := "Manual"
	if m.expectedMode == 0 {
		modeA = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render("[Solution]")
		modeB = dim("Manual")
	} else {
		modeA = dim("Solution")
		modeB = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render("[Manual]")
	}

	var b strings.Builder
	b.WriteString(dim("Mode: ") + modeA + " " + modeB + "  " + dim("(tab to switch)") + "\n\n")

	if m.expectedMode == 0 {
		b.WriteString(dim("Write the correct solution, then press ctrl+r to compile and capture output.") + "\n\n")
		b.WriteString(borderStyle.Render(m.solutionEditor.View()) + "\n\n")
		if m.compiling {
			b.WriteString(dim("compiling...") + "\n")
		} else if m.capturedOutput != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(
				fmt.Sprintf("captured: %q", m.capturedOutput)) + "\n")
		} else if m.captureErr != "" {
			b.WriteString(exErrorStyle.Render("error: "+m.captureErr) + "\n")
		}
		b.WriteString("\n")
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		if m.importMode {
			b.WriteString(m.viewImportOverlay())
		} else {
			b.WriteString(helpBar("ctrl+r compile", "ctrl+j import", "ctrl+n next", backHint, "tab switch"))
		}
	} else {
		b.WriteString(dim("Type the exact expected output of the exercise.") + "\n\n")
		b.WriteString(m.manualExpected.View() + "\n\n")
		backHint := "ctrl+p back"
		if m.editMode {
			backHint = "ctrl+p menu"
		}
		if m.importMode {
			b.WriteString(m.viewImportOverlay())
		} else {
			b.WriteString(helpBar("ctrl+n next", backHint, "ctrl+j import", "tab switch"))
		}
	}

	return b.String()
}

func (m *adminLessonCreateModel) viewSave() string {
	typeStr := "lesson"
	var tagsLabel, tagsVal string
	if m.isInterview {
		typeStr = "interview"
		tagsLabel = "Companies:"
		tagsVal = strings.Join(parseLessonTags(m.companiesInput.Value()), ", ")
	} else {
		tagsLabel = "Tags:"
		tagsVal = strings.Join(parseLessonTags(m.tagsInput.Value()), ", ")
	}
	if tagsVal == "" {
		tagsVal = dim("(none)")
	}
	// keep variable name for rest of function
	tags := tagsVal

	tutorial := parseTutorialBeats(m.tutorialArea.Value())
	hints := strings.TrimSpace(m.hintsArea.Value())
	hintCount := 0
	for _, block := range strings.Split(hints, "\n---\n") {
		if strings.TrimSpace(block) != "" {
			hintCount++
		}
	}

	expected := ""
	if m.expectedMode == 0 {
		expected = m.capturedOutput
	} else {
		expected = m.manualExpected.Value()
	}
	expectedSummary := dim("(none)")
	if strings.TrimSpace(expected) != "" {
		preview := strings.TrimSpace(expected)
		if len(preview) > 30 {
			preview = preview[:28] + ".."
		}
		expectedSummary = fmt.Sprintf("%q", preview)
	}

	exerciseLines := 0
	for _, l := range strings.Split(m.exerciseEditor.Value(), "\n") {
		if strings.TrimSpace(l) != "" {
			exerciseLines++
		}
	}
	contentLines := len(strings.Split(strings.TrimSpace(m.contentArea.Value()), "\n"))
	if strings.TrimSpace(m.contentArea.Value()) == "" {
		contentLines = 0
	}

	label := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded)).Render
	val := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Render

	var b strings.Builder
	b.WriteString(dim("Review and save the lesson:") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", label("ID:         "), val(strings.TrimSpace(m.idInput.Value()))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Title:      "), val(strings.TrimSpace(m.titleInput.Value()))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Type:       "), val(typeStr)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Difficulty: "), val(m.difficulty)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label(fmt.Sprintf("%-12s", tagsLabel)), val(tags)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Content:    "), val(fmt.Sprintf("%d lines", contentLines))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Exercise:   "), val(fmt.Sprintf("%d lines", exerciseLines))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Expected:   "), val(expectedSummary)))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Tutorial:   "), val(fmt.Sprintf("%d beats", len(tutorial)))))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Hints:      "), val(fmt.Sprintf("%d blocks", hintCount))))

	noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber))
	if exerciseLines == 0 {
		b.WriteString("\n" + noteStyle.Render("note: no exercise code — students will see an empty editor"))
	}
	if strings.TrimSpace(expected) == "" {
		b.WriteString("\n" + noteStyle.Render("note: no expected output — program runs but output is not checked"))
	}

	if m.msg != "" {
		b.WriteString("\n" + m.msg + "\n")
	}

	backHint := "b/ctrl+p back"
	if m.editMode {
		backHint = "ctrl+p menu"
	}
	b.WriteString("\n" + helpBar("enter save", backHint, "ctrl+q cancel"))
	return b.String()
}

// ── Lesson Picker (for editing existing lessons) ──────────────────────────────

type lessonPickerItem struct {
	ID    string
	Title string
	Dir   string
}

type adminLessonPickerModel struct {
	lessonsDir string
	items      []lessonPickerItem
	cursor     int
	sized
	msg        string
	delConfirm bool
}

func newAdminLessonPicker(lessonsDir string, w, h int) *adminLessonPickerModel {
	m := &adminLessonPickerModel{
		lessonsDir: lessonsDir,
		sized:      sized{Width: w, Height: h},
	}
	m.loadItems()
	return m
}

func (m *adminLessonPickerModel) loadItems() {
	entries, err := os.ReadDir(m.lessonsDir)
	if err != nil {
		m.msg = "cannot read lessons directory: " + err.Error()
		return
	}

	var items []lessonPickerItem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(m.lessonsDir, entry.Name())
		mdPath := filepath.Join(dir, "lesson.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		content := string(data)
		title := extractLessonTitle(content)
		items = append(items, lessonPickerItem{
			ID:    entry.Name(),
			Title: title,
			Dir:   dir,
		})
	}
	m.items = items
	if len(items) == 0 {
		m.msg = "no existing lessons found"
	}
}

func extractLessonTitle(content string) string {
	body := content
	if strings.HasPrefix(content, "---\n") {
		idx := strings.Index(content[4:], "\n---\n")
		if idx >= 0 {
			body = content[idx+9:]
		}
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return "(no title)"
}

func (m *adminLessonPickerModel) Init() tea.Cmd { return nil }

func (m *adminLessonPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		if m.delConfirm {
			switch msg.String() {
			case "y", "Y":
				if len(m.items) > 0 {
					_ = os.RemoveAll(m.items[m.cursor].Dir)
					m.delConfirm = false
					m.loadItems()
					if m.cursor >= len(m.items) {
						m.cursor = max(0, len(m.items)-1)
					}
				}
			default:
				m.delConfirm = false
			}
			return m, nil
		}
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.items) > 0 {
				dir := m.items[m.cursor].Dir
				return m, backCmd(adminLessonEditSelectedMsg{Dir: dir})
			}
		case "d":
			if len(m.items) > 0 {
				m.delConfirm = true
			}
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminDashboard{})
		}
	}
	return m, nil
}

func (m *adminLessonPickerModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle("Edit Lesson") + dim("  Select a lesson to edit") + "\n\n")

	if m.msg != "" {
		b.WriteString(dim(m.msg) + "\n\n")
	}

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))

	visH := m.Height - 8
	if visH < 1 {
		visH = 1
	}
	start := m.cursor - visH/2
	if start < 0 {
		start = 0
	}
	end := start + visH
	if end > len(m.items) {
		end = len(m.items)
		start = end - visH
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		item := m.items[i]
		line := fmt.Sprintf("  %-24s %s", dimStyle.Render(item.ID), item.Title)
		if i == m.cursor {
			line = selStyle.Render(fmt.Sprintf("> %-24s %s", item.ID, item.Title))
		}
		b.WriteString(line + "\n")
	}

	if len(m.items) > 0 {
		b.WriteString("\n" + dim(fmt.Sprintf("%d/%d", m.cursor+1, len(m.items))))
	}

	if m.delConfirm && len(m.items) > 0 {
		b.WriteString("\n\n" + exErrorStyle.Render(
			fmt.Sprintf("Delete %q? This cannot be undone.", m.items[m.cursor].ID),
		))
		b.WriteString("\n" + dim("y confirm  any other key cancel"))
	} else {
		b.WriteString("\n\n" + helpBar("j/k choose", "enter edit", "d delete", "b/q back"))
	}

	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}

// ── Lesson Target Picker ──────────────────────────────────────────────────────

type lessonTarget struct {
	Label string
	Dir   string
}

// buildLessonTargets enumerates course sections in coursesDir and appends
// the legacy lessonsDir if it exists.
func buildLessonTargets(coursesDir, lessonsDir string) []lessonTarget {
	var targets []lessonTarget
	if coursesDir != "" {
		courseEntries, _ := os.ReadDir(coursesDir)
		for _, ce := range courseEntries {
			if !ce.IsDir() {
				continue
			}
			courseDir := filepath.Join(coursesDir, ce.Name())
			subs, _ := os.ReadDir(courseDir)
			for _, se := range subs {
				if !se.IsDir() {
					continue
				}
				secDir := filepath.Join(courseDir, se.Name())
				// Only include dirs that contain lesson subdirectories.
				inner, _ := os.ReadDir(secDir)
				hasLessons := false
				for _, le := range inner {
					if le.IsDir() {
						hasLessons = true
						break
					}
				}
				if !hasLessons {
					continue
				}
				label := ce.Name() + " / " + se.Name()
				targets = append(targets, lessonTarget{Label: label, Dir: secDir})
			}
		}
	}
	if lessonsDir != "" {
		if _, err := os.Stat(lessonsDir); err == nil {
			targets = append(targets, lessonTarget{Label: "lessons (legacy)", Dir: lessonsDir})
		}
	}
	return targets
}

type adminLessonTargetPickerModel struct {
	targets []lessonTarget
	mode    string // "create", "import", "edit"
	cursor  int
	sized
}

func newAdminLessonTargetPicker(targets []lessonTarget, mode string, w, h int) *adminLessonTargetPickerModel {
	return &adminLessonTargetPickerModel{targets: targets, mode: mode, sized: sized{Width: w, Height: h}}
}

func (m *adminLessonTargetPickerModel) Init() tea.Cmd { return nil }

func (m *adminLessonTargetPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.targets)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.targets) == 0 {
				return m, nil
			}
			t := m.targets[m.cursor]
			mode := m.mode
			return m, backCmd(adminLessonTargetPickedMsg{Dir: t.Dir, Mode: mode})
		case "b", "esc", "q":
			return m, backCmd(NavigateToAdminDashboard{})
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *adminLessonTargetPickerModel) View() string {
	modeLabel := map[string]string{
		"create":   "Create Lesson",
		"import":   "Import Lesson",
		"edit":     "Edit Lesson",
		"scaffold": "Scaffold Lesson Files",
	}[m.mode]

	var b strings.Builder
	b.WriteString(titleStyle(modeLabel) + dim("  Select target location") + "\n\n")

	if len(m.targets) == 0 {
		b.WriteString(dim("No course sections found. Add a course first.") + "\n")
	}

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))

	for i, t := range m.targets {
		dir := dimStyle.Render(t.Dir)
		if i == m.cursor {
			b.WriteString(selStyle.Render(fmt.Sprintf("> %-34s %s", t.Label, t.Dir)) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %-34s %s", t.Label, dir) + "\n")
		}
	}

	b.WriteString("\n" + helpBar("j/k choose", "enter select", "b back"))
	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}

// ── Scaffold Lesson ───────────────────────────────────────────────────────────

// adminLessonScaffoldModel writes template files to disk so the operator can
// author lesson content in their preferred editor, then use option 7 (Edit) to
// compile the expected output and review it.
type adminLessonScaffoldModel struct {
	targetDir  string
	idInput    textinput.Model
	titleInput textinput.Model
	language   string
	focus      int // 0=id 1=title 2=language
	done       bool
	result     string // absolute path to the scaffolded dir
	errStr     string
	sized
}

func newAdminLessonScaffold(targetDir string, w, h int) *adminLessonScaffoldModel {
	idInput := textinput.New()
	idInput.Placeholder = "10-arrays"
	idInput.CharLimit = 64
	idInput.Width = 40
	idInput.Focus()

	titleInput := textinput.New()
	titleInput.Placeholder = "Arrays in C"
	titleInput.CharLimit = 128
	titleInput.Width = 40

	return &adminLessonScaffoldModel{
		targetDir:  targetDir,
		idInput:    idInput,
		titleInput: titleInput,
		language:   "c",
		sized:      sized{Width: w, Height: h},
	}
}

func (m *adminLessonScaffoldModel) Init() tea.Cmd { return nil }

func (m *adminLessonScaffoldModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)
	case tea.KeyMsg:
		if m.done {
			return m, backCmd(NavigateToAdminDashboard{})
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+q", "esc":
			return m, backCmd(NavigateToAdminDashboard{})
		case "tab", "down":
			if m.focus < 2 {
				m.scaffoldSetFocus(m.focus + 1)
			}
			return m, nil
		case "shift+tab", "up":
			if m.focus > 0 {
				m.scaffoldSetFocus(m.focus - 1)
			}
			return m, nil
		case "enter":
			if m.focus < 2 {
				m.scaffoldSetFocus(m.focus + 1)
			} else {
				m.runScaffold()
			}
			return m, nil
		case "left", " ":
			if m.focus == 2 {
				m.scaffoldCycleLang(-1)
			}
			return m, nil
		case "right":
			if m.focus == 2 {
				m.scaffoldCycleLang(1)
			}
			return m, nil
		default:
			var cmd tea.Cmd
			switch m.focus {
			case 0:
				m.idInput, cmd = m.idInput.Update(msg)
			case 1:
				m.titleInput, cmd = m.titleInput.Update(msg)
			}
			return m, cmd
		}
	}
	return m, nil
}

func (m *adminLessonScaffoldModel) scaffoldSetFocus(f int) {
	m.idInput.Blur()
	m.titleInput.Blur()
	m.focus = f
	switch f {
	case 0:
		m.idInput.Focus()
	case 1:
		m.titleInput.Focus()
	}
}

func (m *adminLessonScaffoldModel) scaffoldCycleLang(dir int) {
	all := sandbox.AllLanguages()
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cur := 0
	for i, k := range keys {
		if k == m.language {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(keys)) % len(keys)
	m.language = keys[cur]
}

func (m *adminLessonScaffoldModel) runScaffold() {
	id := strings.TrimSpace(m.idInput.Value())
	if id == "" {
		m.errStr = "ID is required"
		return
	}
	if strings.ContainsAny(id, " \t/\\") {
		m.errStr = "ID must be a slug (no spaces or slashes)"
		return
	}
	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		m.errStr = "Title is required"
		return
	}

	dir := filepath.Join(m.targetDir, id)
	if _, err := os.Stat(dir); err == nil {
		m.errStr = "a lesson with that ID already exists in this location"
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		m.errStr = "could not create directory: " + err.Error()
		return
	}

	lang := sandbox.GetLanguage(m.language)
	ext := ".c"
	if lang != nil {
		ext = lang.SourceExtension()
	}

	lessonMD := fmt.Sprintf(
		"---\ndifficulty: beginner\ntags:\n  - change-me\ntutorial:\n  - Welcome to the %s lesson. Edit this line — it is Mochi's opening line.\npremium: false\n---\n# %s\n\nWrite your lesson content here in Markdown.\n\nThis text appears before the exercise. Explain the concept clearly.\n",
		title, title,
	)
	if err := os.WriteFile(filepath.Join(dir, "lesson.md"), []byte(lessonMD), 0644); err != nil {
		m.errStr = "write lesson.md: " + err.Error()
		return
	}

	if err := os.WriteFile(filepath.Join(dir, "exercise"+ext), []byte(scaffoldExerciseCode(m.language)), 0644); err != nil {
		m.errStr = "write exercise file: " + err.Error()
		return
	}

	hints := "Replace this with your first hint.\n---\nReplace this with your second hint.\n"
	if err := os.WriteFile(filepath.Join(dir, "hints.txt"), []byte(hints), 0644); err != nil {
		m.errStr = "write hints.txt: " + err.Error()
		return
	}

	abs, _ := filepath.Abs(dir)
	m.result = abs
	m.done = true
}

func scaffoldExerciseCode(language string) string {
	switch language {
	case "c":
		return `#include <stdio.h>

/* TODO: Write the starter code students will see.
   Leave key parts unimplemented to guide their work. */

int main(void) {
    /* your code here */
    return 0;
}
`
	case "python", "py":
		return `# TODO: Write the starter code students will see.
# Leave key parts unimplemented to guide their work.

def main():
    pass  # your code here

if __name__ == "__main__":
    main()
`
	default:
		return "// TODO: Write the starter code students will see.\n"
	}
}

func (m *adminLessonScaffoldModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle("Scaffold Lesson Files") + "\n\n")

	if m.done {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("Files written to:") + "\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true).Render(m.result) + "\n\n")
		b.WriteString(dim("Edit lesson.md, exercise.*, and hints.txt in your preferred editor.") + "\n")
		b.WriteString(dim("To set expected output: use option 7 (Edit Lesson), go to Expected Output,") + "\n")
		b.WriteString(dim("paste or write the solution, and press ctrl+r to compile and capture.") + "\n\n")
		b.WriteString(dim("press any key to return to dashboard"))
		return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
			lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
	}

	b.WriteString(dim("Creates a lesson directory with template files you edit in your preferred editor.") + "\n\n")

	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	dimLbl := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	labelW := 10

	field := func(label string, idx int, value string) string {
		lbl := fmt.Sprintf("%-*s", labelW, label+":")
		if m.focus == idx {
			return focusStyle.Render(lbl) + " " + value
		}
		return dimLbl.Render(lbl) + " " + value
	}

	langDisplay := m.language
	if l := sandbox.GetLanguage(m.language); l != nil {
		langDisplay = l.DisplayName()
	}
	langVal := dim("< ") + langDisplay + dim(" >")
	if m.focus == 2 {
		langVal = focusStyle.Render("< " + langDisplay + " >")
	}

	lang := sandbox.GetLanguage(m.language)
	ext := ".c"
	if lang != nil {
		ext = lang.SourceExtension()
	}

	b.WriteString(field("ID", 0, m.idInput.View()) + "\n")
	b.WriteString(field("Title", 1, m.titleInput.View()) + "\n")
	b.WriteString(field("Language", 2, langVal) + "\n\n")
	b.WriteString(dim(fmt.Sprintf("Creates: lesson.md  exercise%s  hints.txt", ext)) + "\n")

	if m.errStr != "" {
		b.WriteString("\n" + exErrorStyle.Render(m.errStr) + "\n")
	}

	b.WriteString("\n" + helpBar("tab/enter next field", "left/right language", "enter on language to create", "ctrl+q cancel"))

	return lipgloss.Place(m.Width, m.Height, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(1, 3).Render(b.String()))
}
