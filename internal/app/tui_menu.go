package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menuListModel struct {
	title    string
	help     string
	choices  []string
	cursor   int
	selected string
	back     bool
	exit     bool
}

func newMenuListModel(title, help string, choices []string) menuListModel {
	return menuListModel{
		title:   title,
		help:    help,
		choices: choices,
		cursor:  0,
	}
}

func (m menuListModel) Init() tea.Cmd {
	return nil
}

func (m menuListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	lines := []string{}
	if m.title != "" {
		lines = append(lines, uiIndent()+textEmphasis.Render(m.title))
		lines = append(lines, "")
	}
	for i, c := range m.choices {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		lines = append(lines, uiIndent()+textEmphasis.Render(cursor)+textPrimary.Render(c))
	}
	if m.help != "" {
		lines = append(lines, "")
		lines = append(lines, uiIndent()+textMuted.Render(m.help))
	}
	return strings.Join(lines, "\n")
}

func runMenuTea(title, help string, choices []string) (selected string, back bool, exit bool, err error) {
	p := tea.NewProgram(newMenuListModel(title, help, choices), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", false, false, err
	}
	model := m.(menuListModel)
	return model.selected, model.back, model.exit, nil
}
