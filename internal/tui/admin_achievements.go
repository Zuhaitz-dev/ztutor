package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Admin Achievements Screen ──────────────────────────────────────────────────

type achievementsSubMode int

const (
	achList achievementsSubMode = iota
	achCreate
)

type adminAchievementsModel struct {
	achievementsFile string
	mode             achievementsSubMode
	list             []Achievement
	cursor           int
	offset           int

	// create form
	idInput   textinput.Model
	nameInput textinput.Model
	iconInput textinput.Model
	descInput textinput.Model
	isSecret  bool
	formFocus int // 0=id 1=name 2=icon 3=desc 4=secret
	formMsg   string

	msg string
	sized
}

func newAdminAchievements(achievementsFile string, w, h int) *adminAchievementsModel {
	idIn := textinput.New()
	idIn.Placeholder = "my-achievement"
	idIn.CharLimit = 64
	idIn.Width = 36
	idIn.Focus()

	nameIn := textinput.New()
	nameIn.Placeholder = "Achievement Name"
	nameIn.CharLimit = 64
	nameIn.Width = 36

	iconIn := textinput.New()
	iconIn.Placeholder = "[X]"
	iconIn.CharLimit = 8
	iconIn.Width = 10

	descIn := textinput.New()
	descIn.Placeholder = "What this achievement means"
	descIn.CharLimit = 128
	descIn.Width = 36

	return &adminAchievementsModel{
		achievementsFile: achievementsFile,
		mode:             achList,
		list:             AllAchievements(),
		idInput:          idIn,
		nameInput:        nameIn,
		iconInput:        iconIn,
		descInput:        descIn,
		sized:            sized{Width: w, Height: h},
	}
}

func (m *adminAchievementsModel) Init() tea.Cmd { return nil }

func (m *adminAchievementsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleResize(msg)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.mode == achCreate {
			return m.updateCreate(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *adminAchievementsModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.list)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "n":
		m.mode = achCreate
		m.idInput.SetValue("")
		m.nameInput.SetValue("")
		m.iconInput.SetValue("")
		m.descInput.SetValue("")
		m.isSecret = false
		m.formFocus = 0
		m.formMsg = ""
		m.idInput.Focus()
		m.nameInput.Blur()
		m.iconInput.Blur()
		m.descInput.Blur()
		return m, textinput.Blink
	case "d":
		if m.cursor < len(m.list) {
			a := m.list[m.cursor]
			if m.isCustom(a.ID) {
				if err := deleteAndSaveAchievement(a.ID, m.achievementsFile); err != nil {
					m.msg = exErrorStyle.Render("delete failed: " + err.Error())
				} else {
					m.list = AllAchievements()
					if m.cursor >= len(m.list) && m.cursor > 0 {
						m.cursor--
					}
					m.msg = ""
				}
			} else {
				m.msg = exErrorStyle.Render("cannot delete built-in achievements")
			}
		}
	case "b", "esc", "q":
		return m, backCmd(NavigateToAdminDashboard{})
	}
	return m, nil
}

func (m *adminAchievementsModel) isCustom(id string) bool {
	for _, a := range customAchievements {
		if a.ID == id {
			return true
		}
	}
	return false
}

func (m *adminAchievementsModel) setFormFocus(f int) {
	inputs := []*textinput.Model{&m.idInput, &m.nameInput, &m.iconInput, &m.descInput}
	for _, in := range inputs {
		in.Blur()
	}
	m.formFocus = (f + 5) % 5
	if m.formFocus < 4 {
		inputs[m.formFocus].Focus()
	}
}

func (m *adminAchievementsModel) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = achList
		return m, nil
	case "tab", "down":
		m.setFormFocus(m.formFocus + 1)
		return m, nil
	case "shift+tab", "up":
		m.setFormFocus(m.formFocus - 1)
		return m, nil
	case "enter":
		if m.formFocus == 3 {
			return m, m.saveNewAchievement()
		}
		if m.formFocus == 4 {
			return m, m.saveNewAchievement()
		}
		m.setFormFocus(m.formFocus + 1)
		return m, nil
	case " ":
		if m.formFocus == 4 {
			m.isSecret = !m.isSecret
			return m, nil
		}
	case "ctrl+s":
		return m, m.saveNewAchievement()
	}
	var cmd tea.Cmd
	switch m.formFocus {
	case 0:
		m.idInput, cmd = m.idInput.Update(msg)
	case 1:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 2:
		m.iconInput, cmd = m.iconInput.Update(msg)
	case 3:
		m.descInput, cmd = m.descInput.Update(msg)
	}
	return m, cmd
}

func (m *adminAchievementsModel) saveNewAchievement() tea.Cmd {
	id := strings.TrimSpace(m.idInput.Value())
	if id == "" {
		m.formMsg = "ID is required"
		return nil
	}
	if strings.ContainsAny(id, " \t/\\") {
		m.formMsg = "ID must be a slug (no spaces or slashes)"
		return nil
	}
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		m.formMsg = "Name is required"
		return nil
	}
	icon := strings.TrimSpace(m.iconInput.Value())
	if icon == "" {
		icon = "[?]"
	}
	desc := strings.TrimSpace(m.descInput.Value())

	// Check for ID conflict
	for _, a := range AllAchievements() {
		if a.ID == id {
			m.formMsg = "an achievement with that ID already exists"
			return nil
		}
	}

	a := Achievement{ID: id, Name: name, Icon: icon, Desc: desc, Secret: m.isSecret}
	if err := appendAndSaveAchievement(a, m.achievementsFile); err != nil {
		m.formMsg = "save failed: " + err.Error()
		return nil
	}
	m.list = AllAchievements()
	m.mode = achList
	m.cursor = len(m.list) - 1
	m.msg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(fmt.Sprintf("[+] %s added", id))
	return nil
}

func (m *adminAchievementsModel) View() string {
	if m.mode == achCreate {
		return m.viewCreate()
	}
	return m.viewList()
}

func (m *adminAchievementsModel) viewList() string {
	var b strings.Builder

	builtinCount := len(allAchievements)
	customCount := len(customAchievements)
	b.WriteString(titleStyle("Achievements") + "  " +
		dim(fmt.Sprintf("%d built-in  %d custom", builtinCount, customCount)) + "\n\n")

	builtinBadge := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHex)).Render("[built-in]")
	customBadge := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render("[custom]  ")
	secretBadge := dim("[secret]  ")

	listH := m.Height - 8
	if listH < 3 {
		listH = 3
	}

	if m.offset > m.cursor {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+listH {
		m.offset = m.cursor - listH + 1
	}

	end := m.offset + listH
	if end > len(m.list) {
		end = len(m.list)
	}

	for i, a := range m.list[m.offset:end] {
		idx := m.offset + i
		var badge string
		if a.Secret {
			badge = secretBadge
		} else if m.isCustom(a.ID) {
			badge = customBadge
		} else {
			badge = builtinBadge
		}
		line := fmt.Sprintf("  %s  %-5s  %-22s  %s  %s",
			a.Icon, "", a.Name, badge, dim(a.Desc))
		if idx == m.cursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color(ColorBG)).Foreground(lipgloss.Color("255")).
				Render(fmt.Sprintf("> %s  %-5s  %-22s  %s  %s",
					a.Icon, "", a.Name, badge, a.Desc))
		}
		b.WriteString(line + "\n")
	}

	if len(m.list) > listH {
		b.WriteString(dim(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.list))) + "\n")
	}

	if m.msg != "" {
		b.WriteString("\n" + m.msg + "\n")
	}

	b.WriteString("\n" + helpBar("j/k navigate", "n new", "d delete (custom only)", "b/q back"))

	return lipgloss.NewStyle().Padding(1, 3).Render(b.String())
}

func (m *adminAchievementsModel) viewCreate() string {
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded))
	labelW := 10

	field := func(label string, idx int, value string) string {
		lbl := fmt.Sprintf("%-*s", labelW, label+":")
		if m.formFocus == idx {
			return focusStyle.Render(lbl) + " " + value
		}
		return dimStyle.Render(lbl) + " " + value
	}

	var secretVal string
	if m.formFocus == 4 {
		if m.isSecret {
			secretVal = dim("No") + "  " + focusStyle.Render("[ Yes ]")
		} else {
			secretVal = focusStyle.Render("[ No ]") + "  " + dim("Yes")
		}
	} else {
		if m.isSecret {
			secretVal = dim("Yes")
		} else {
			secretVal = dim("No")
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle("New Achievement") + "\n\n")
	b.WriteString(field("ID", 0, m.idInput.View()) + "\n")
	b.WriteString(field("Name", 1, m.nameInput.View()) + "\n")
	b.WriteString(field("Icon", 2, m.iconInput.View()) + "\n")
	b.WriteString(field("Desc", 3, m.descInput.View()) + "\n")
	b.WriteString(field("Secret", 4, secretVal) + "\n")

	if m.formMsg != "" {
		b.WriteString("\n" + exErrorStyle.Render(m.formMsg) + "\n")
	}

	b.WriteString("\n" + helpBar("tab next", "space toggle secret", "ctrl+s save", "esc cancel"))

	return lipgloss.NewStyle().Padding(1, 3).Render(b.String())
}
