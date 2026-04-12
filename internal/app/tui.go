package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type menuModel struct {
	choices []string
	cursor  int
	choice  string
	aborted bool
}

func newMenuModel() menuModel {
	return menuModel{
		choices: []string{"Search YouTube", "Search Downloads", "Settings", "Quit"},
		cursor:  0,
	}
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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

func (m menuModel) View() string {
	header := "\n" + uiIndent() + textEmphasis.Render("GopherTube") + " " + textMuted.Render(version) + "\n\n"
	body := ""
	for i, choice := range m.choices {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		body += fmt.Sprintf("%s%s%s\n", uiIndent(), textEmphasis.Render(cursor), textPrimary.Render(choice))
	}
	return header + body
}

func stringsJoin(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

func runMainMenuTea() (string, bool, error) {
	p := tea.NewProgram(newMenuModel(), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", false, err
	}
	model := m.(menuModel)
	if model.aborted {
		return "", true, nil
	}
	if model.choice == "Quit" {
		return "", true, nil
	}
	return model.choice, false, nil
}
