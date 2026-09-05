package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	width     int
	height    int
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
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

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

		if m.exit || m.back {
			return m, tea.Quit
		}
		return m, nil
	}

	return m, nil
}

func (m playbackModel) View() string {
	dot := S.Muted.Render("○")
	status := S.Muted.Render("Preparing playback…")
	if m.running {
		dot = S.Success.Render("●")
		status = S.Success.Render("Playing")
	} else if m.finished {
		dot = S.Muted.Render("○")
		status = S.Bold.Render("Playback finished")
		if m.err != nil && m.err.Error() != "exit status 4" && m.err.Error() != "signal: interrupt" {
			dot = S.Danger.Render("●")
			status = S.Danger.Render("Playback error: " + m.err.Error())
		}
	}

	row := func(label, value string) string {
		if value == "" {
			return ""
		}
		return fmt.Sprintf("  %s%s\n", S.Muted.Render(label+" "), S.Text.Render(value))
	}

	var b strings.Builder
	b.WriteString("  " + S.Accent.Render(m.prefix) + S.Bold.Render(m.title) + "\n\n")
	b.WriteString(row("Channel:  ", m.channel))
	b.WriteString(row("Duration: ", m.duration))
	b.WriteString(row("Published:", m.published))
	b.WriteString("\n")
	b.WriteString("  " + dot + " " + status)

	hints := [][2]string{{"Esc/q", "Stop & back"}, {"⌃C", "Stop & quit"}}
	return screen(m.width, m.height, "Now Playing", b.String(), hints)
}

func runPlaybackTea(title, channel, duration, published, prefix string, args []string) (exit bool, back bool, err error) {
	p := tea.NewProgram(newPlaybackModel(title, channel, duration, published, prefix, args), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return false, false, err
	}
	model := m.(playbackModel)
	return model.exit, model.back, model.err
}
