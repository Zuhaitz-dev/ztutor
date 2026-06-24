package tui

import (
	"fmt"
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

// creditTier defines the visual structure of one patron tier.
// The entries (names) are loaded from config/credits.yaml at runtime.
type creditTier struct {
	LabelKey string // locale key → translated tier label
	YAMLKey  string // key in credits.yaml
	Color    lipgloss.Color
	PerRow   int
}

var maecenasTiers = []creditTier{
	{LabelKey: "credits.tier.archon", YAMLKey: "archon", Color: "220", PerRow: 1},
	{LabelKey: "credits.tier.patron", YAMLKey: "patron", Color: "183", PerRow: 2},
	{LabelKey: "credits.tier.supporter", YAMLKey: "supporter", Color: "109", PerRow: 3},
}

// creditsFileData mirrors the structure of config/credits.yaml.
type creditsFileData struct {
	Archon       []creditEntry `yaml:"archon"`
	Patron       []creditEntry `yaml:"patron"`
	Supporter    []creditEntry `yaml:"supporter"`
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

// CreditsScreen displays the Maecenas patron tiers and contributor list.
// Names are loaded from config/credits.yaml; the screen is empty if the file
// does not exist yet.
type CreditsScreen struct {
	sized
	loc          *i18n.Locale
	scroll       int
	tierEntries  [][]creditEntry // parallel to maecenasTiers
	contributors []creditEntry
}

func NewCreditsScreen(width, height int, loc *i18n.Locale) *CreditsScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	cfg := loadCreditsFile()
	return &CreditsScreen{
		sized: sized{Width: width, Height: height},
		loc:   loc,
		tierEntries: [][]creditEntry{
			cfg.Archon,
			cfg.Patron,
			cfg.Supporter,
		},
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

// creditsTierRule renders: ─── LABEL ──────────────────────────
func creditsTierRule(label string, color lipgloss.Color, width int) string {
	pad := "  " + label + "  "
	styledPad := lipgloss.NewStyle().Foreground(color).Bold(true).Render(pad)
	used := len([]rune(pad))
	left := width / 4
	right := max(0, width-used-left)
	return dim(strings.Repeat("─", left)) + styledPad + dim(strings.Repeat("─", right))
}

func (cs *CreditsScreen) buildLines() []string {
	T := cs.loc.T
	var lines []string
	w := min(cs.Width-2, creditsWidth)

	// ── Title ──────────────────────────────────────────────────────────────────
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	lines = append(lines, titleStyle.Render(spacedLabel(T("credits.title"))))
	lines = append(lines, "")

	// ── Maecenas section ───────────────────────────────────────────────────────
	maeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true)
	lines = append(lines, maeStyle.Render(spacedLabel(T("credits.maecenas_header"))))
	lines = append(lines, dim(strings.Repeat("─", w)))
	lines = append(lines, "")

	anyMaecenas := false
	for i, tier := range maecenasTiers {
		entries := cs.tierEntries[i]
		if len(entries) == 0 {
			continue
		}
		anyMaecenas = true

		lines = append(lines, creditsTierRule(T(tier.LabelKey), tier.Color, w))
		lines = append(lines, "")

		if tier.PerRow == 1 {
			star := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("★")
			nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true)
			for _, e := range entries {
				lines = append(lines, "  "+star+"  "+nameStyle.Render(e.Name)+"  "+star)
			}
		} else {
			colW := w / tier.PerRow
			nameStyle := lipgloss.NewStyle().Foreground(tier.Color)
			for start := 0; start < len(entries); start += tier.PerRow {
				var row strings.Builder
				row.WriteString("  ")
				for j := 0; j < tier.PerRow && start+j < len(entries); j++ {
					if j > 0 {
						row.WriteString("  ")
					}
					row.WriteString(nameStyle.Render(fmt.Sprintf("%-*s", colW-2, entries[start+j].Name)))
				}
				lines = append(lines, row.String())
			}
		}
		lines = append(lines, "")
	}

	if !anyMaecenas {
		lines = append(lines, "  "+dim(T("credits.maecenas_empty")))
		lines = append(lines, "")
	}

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
	T := cs.loc.T
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
	b.WriteString(helpBar(T("help.q_back")))
	result := b.String()
	return rtlWrap(cs.loc.IsRTL(), result, cs.Width)
}
