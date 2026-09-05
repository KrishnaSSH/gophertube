package cli

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type mainMenuModel struct {
	choices []string
	cursor  int
	choice  string
	aborted bool
	width   int
	height  int
}

func newMainMenuModel() mainMenuModel {
	return mainMenuModel{
		choices: []string{"Search YouTube", "Search Downloads", "Settings", "Quit"},
		cursor:  0,
	}
}

func (m mainMenuModel) Init() tea.Cmd {
	return nil
}

func (m mainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		case "esc":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.choice = m.choices[m.cursor]
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m mainMenuModel) View() string {
	var b strings.Builder
	for i, c := range m.choices {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  " + listItem(c, c == m.choices[m.cursor]))
	}
	hints := [][2]string{{"↑↓", "Navigate"}, {"⏎", "Select"}, {"Esc/⌃C", "Quit"}}
	return screen(m.width, m.height, "", b.String(), hints)
}

func runMainMenuTea() (string, bool, error) {
	p := tea.NewProgram(newMainMenuModel(), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", false, err
	}
	model := m.(mainMenuModel)
	if model.aborted || model.choice == "Quit" {
		return "", true, nil
	}
	return model.choice, false, nil
}

// Generic single-column selection menu, used for action/quality/downloads/theme pickers.

type menuListModel struct {
	title    string
	choices  []string
	cursor   int
	selected string
	back     bool
	exit     bool
	width    int
	height   int
}

func newMenuListModel(title string, choices []string) menuListModel {
	return menuListModel{
		title:   title,
		choices: choices,
		cursor:  0,
	}
}

func (m menuListModel) Init() tea.Cmd {
	return nil
}

func (m menuListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.exit = true
			return m, tea.Quit
		case "esc":
			m.back = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.choices) > 0 {
				m.selected = m.choices[m.cursor]
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m menuListModel) View() string {
	var b strings.Builder
	for i, c := range m.choices {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  " + listItem(c, i == m.cursor))
	}
	hints := [][2]string{{"↑↓", "Navigate"}, {"⏎", "Select"}, {"Esc", "Back"}, {"⌃C", "Quit"}}
	return screen(m.width, m.height, m.title, b.String(), hints)
}

func runMenuTea(title string, choices []string) (selected string, back bool, exit bool, err error) {
	p := tea.NewProgram(newMenuListModel(title, choices), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", false, false, err
	}
	model := m.(menuListModel)
	return model.selected, model.back, model.exit, nil
}
