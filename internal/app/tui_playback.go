package app

import (
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type playbackModel struct {
	title     string
	channel   string
	duration  string
	published string
	prefix    string
	args      []string
	running   bool
	finished  bool
	err       error
	exit      bool
	back      bool
	proc      *os.Process
	tmpPath   string
}

type playbackFinishedMsg struct{ err error }
type startPlaybackMsg struct{}

func newPlaybackModel(title, channel, duration, published, prefix string, args []string) playbackModel {
	return playbackModel{
		title:     title,
		channel:   channel,
		duration:  duration,
		published: published,
		prefix:    prefix,
		args:      args,
	}
}

func (m playbackModel) Init() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return startPlaybackMsg{}
	})
}

func (m playbackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.proc != nil {
				m.proc.Signal(os.Interrupt)
			}
			m.exit = true
			return m, tea.Quit
		case "esc", "q":
			if m.proc != nil {
				m.proc.Signal(os.Interrupt)
				// We don't quit immediately, wait for playbackFinishedMsg
				// but set back = true so we know where to go.
				m.back = true
				return m, nil
			}
			m.back = true
			return m, tea.Quit
		}

	case startPlaybackMsg:
		if m.running {
			return m, nil
		}
		m.running = true

		tmpFile, err := os.CreateTemp("", "gophertube-mpv-*.conf")
		var mpvArgs []string
		if err == nil {
			m.tmpPath = tmpFile.Name()
			tmpFile.WriteString("ESC quit\nq quit\nENTER quit\n")
			tmpFile.Close()
			mpvArgs = append([]string{"--input-conf=" + m.tmpPath}, m.args...)
		} else {
			mpvArgs = m.args
		}

		c := exec.Command("mpv", mpvArgs...)
		// Redirect output to prevent terminal garbling
		c.Stdout = nil
		c.Stderr = nil
		
		if err := c.Start(); err != nil {
			m.running = false
			m.finished = true
			m.err = err
			return m, nil
		}
		m.proc = c.Process

		return m, func() tea.Msg {
			err := c.Wait()
			if m.tmpPath != "" {
				os.Remove(m.tmpPath)
			}
			return playbackFinishedMsg{err}
		}

	case playbackFinishedMsg:
		m.running = false
		m.finished = true
		m.err = msg.err
		m.proc = nil
		
		// If we were already planning to go back or exit, do it now.
		if m.exit {
			return m, tea.Quit
		}
		if m.back {
			return m, tea.Quit
		}
		
		// Otherwise, stay on screen to show "finished" status
		return m, nil
	}

	return m, nil
}

func (m playbackModel) View() string {
	status := textMuted.Render("Preparing playback...")
	if m.running {
		status = textStrong.Render("Playing...")
	} else if m.finished {
		status = textStrong.Render("Playback finished.")
		if m.err != nil && m.err.Error() != "exit status 4" && m.err.Error() != "signal: interrupt" {
			status = textError.Render("Playback Error: " + m.err.Error())
		}
	}

	lines := []string{
		uiIndent() + textEmphasis.Render(m.prefix) + textStrong.Render(m.title),
		uiIndent() + textStrong.Render("Channel: ") + textEmphasis.Render(m.channel),
		uiIndent() + textWarn.Render("Duration: ") + textAccent.Render(m.duration),
		uiIndent() + textMuted.Render("Published: ") + textPrimary.Render(m.published),
		uiIndent() + textEmphasis.Render(dividerLine),
		uiIndent() + status,
		"",
		uiIndent() + textMuted.Render("Esc to back • Ctrl+C to exit"),
	}

	const playbackPadBottom = 2
	for i := 0; i < playbackPadBottom; i++ {
		lines = append(lines, "")
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.NewStyle().Padding(uiPadTop, 0, uiPadBottom, 0).Render(content)
}

func runPlaybackTea(title, channel, duration, published, prefix string, args []string) (exit bool, back bool, err error) {
	// We don't use AltScreen so the metadata stays visible if we exit/suspend, 
	// although with background process it doesn't matter as much.
	// But let's use it for a clean full-screen experience.
	p := tea.NewProgram(newPlaybackModel(title, channel, duration, published, prefix, args), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return false, false, err
	}
	model := m.(playbackModel)
	return model.exit, model.back, model.err
}
