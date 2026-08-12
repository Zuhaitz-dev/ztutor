package tui

import (
	"os"
	"path/filepath"
	"strings"

	"ztutor/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

type NavigateToCredits struct{}

// creditEntry is one person in the credits.
// YAML tags are used when loading config/credits.yaml.
type creditEntry struct {
	Name   string `yaml:"name"`
	GitHub string `yaml:"github"`
}

// creditsFileData mirrors the structure of config/credits.yaml.
type creditsFileData struct {
	Contributors []creditEntry `yaml:"contributors"`
}

// loadCreditsFile reads config/credits.yaml and returns its contents.
// Returns empty data if the file does not exist or is malformed — no crash.
func loadCreditsFile() creditsFileData {
	data, err := os.ReadFile(filepath.Join("config", "credits.yaml"))
	if err != nil {
		return creditsFileData{}
	}
	var cfg creditsFileData
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return creditsFileData{}
	}
	return cfg
}

// CreditsScreen displays the contributor list.
// Names are loaded from config/credits.yaml; the screen is empty if the file
// does not exist yet.
type CreditsScreen struct {
	sized
	loc          *i18n.Locale
	scroll       int
	contributors []creditEntry
}

func NewCreditsScreen(width, height int, loc *i18n.Locale) *CreditsScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	cfg := loadCreditsFile()
	return &CreditsScreen{
		sized:        sized{Width: width, Height: height},
		loc:          loc,
		contributors: cfg.Contributors,
	}
}

func (cs *CreditsScreen) Init() tea.Cmd { return nil }

func (cs *CreditsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cs.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case KeyBack, KeyBackAlt, KeyBackEditor:
			return cs, backCmd(NavigateToMenu{})
		case KeyDown, KeyDownVim:
			cs.scroll++
		case KeyUp, KeyUpVim:
			if cs.scroll > 0 {
				cs.scroll--
			}
		case KeyScrollTop:
			cs.scroll = 0
		case KeyScrollBot:
			cs.scroll = 9999
		}
	}
	return cs, nil
}

const creditsWidth = 54

// spacedLabel inserts spaces between each character: "CREDITS" → "C R E D I T S".
func spacedLabel(s string) string {
	runes := []rune(strings.ToUpper(s))
	parts := make([]string, len(runes))
	for i, r := range runes {
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

func (cs *CreditsScreen) buildLines() []string {
	T := cs.loc.T
	var lines []string
	w := min(cs.Width-2, creditsWidth)

	// ── Title ──────────────────────────────────────────────────────────────────
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	lines = append(lines, titleStyle.Render(spacedLabel(T("credits.title"))))
	lines = append(lines, "")

	// ── Contributors section ───────────────────────────────────────────────────
	lines = append(lines, dim(strings.Repeat("─", w)))
	lines = append(lines, "")
	contribTitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true)
	lines = append(lines, contribTitleStyle.Render(spacedLabel(T("credits.contributors_header"))))
	lines = append(lines, "")

	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	githubStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

	for _, c := range cs.contributors {
		line := "  " + nameStyle.Render(c.Name)
		if c.GitHub != "" {
			line += "  " + githubStyle.Render("@"+c.GitHub)
		}
		lines = append(lines, line)
	}

	if len(cs.contributors) == 0 {
		lines = append(lines, "  "+dim(T("credits.contributors_empty")))
	}

	lines = append(lines, "")
	return lines
}

func (cs *CreditsScreen) View() string {
	lines := cs.buildLines()

	visible := cs.Height - 2
	if visible < 1 {
		visible = 1
	}
	maxScroll := max(0, len(lines)-visible)
	if cs.scroll > maxScroll {
		cs.scroll = maxScroll
	}
	end := min(cs.scroll+visible, len(lines))

	var b strings.Builder
	b.WriteString(strings.Join(lines[cs.scroll:end], "\n"))
	b.WriteString("\n")
	b.WriteString(actionHelpBar(cs.loc, HA(ActionBack)))
	result := b.String()
	return rtlWrap(cs.loc.IsRTL(), result, cs.Width)
}
