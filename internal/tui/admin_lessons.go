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
	metaFocus      int // 0=id 1=title 2=type 3=difficulty 4=tags/companies 5=language

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
	m.metaFocus = (f + 6) % 6
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
		if m.metaFocus == 5 {
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
	}
	fmData, err := yaml.Marshal(lessonFM{
		Difficulty: m.difficulty,
		Tags:       tags,
		Companies:  companies,
		Tutorial:   tutorial,
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
	if err := os.WriteFile(filepath.Join(dir, "lesson.md"), []byte(lessonMD), 0600); err != nil {
		return err
	}

	if code := strings.TrimSpace(m.exerciseEditor.Value()); code != "" {
		ext := ".c"
		if lang := sandbox.GetLanguage(m.language); lang != nil {
			ext = lang.SourceExtension()
		}
		if err := os.WriteFile(filepath.Join(dir, "exercise"+ext), []byte(code+"\n"), 0600); err != nil {
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
		if err := os.WriteFile(filepath.Join(dir, "expected.txt"), []byte(expected), 0600); err != nil {
			return err
		}
	}

	if hints := strings.TrimSpace(m.hintsArea.Value()); hints != "" {
		if err := os.WriteFile(filepath.Join(dir, "hints.txt"), []byte(hints+"\n"), 0600); err != nil {
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
