package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/krishnassh/gophertube/internal/services"
	"github.com/krishnassh/gophertube/internal/types"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type searchState int

const (
	searchStateQuery searchState = iota
	searchStateLoading
	searchStateResults
	searchStateError
)

type searchResultMsg struct {
	videos []types.Video
	err    error
}

type searchModel struct {
	state       searchState
	input       textinput.Model
	filter      textinput.Model
	filterOn    bool
	spin        spinner.Model
	query       string
	limit       int
	searchLimit int
	videos      []types.Video
	cursor      int
	width       int
	height      int
	errMsg      string
	selected    int
	back        bool
	exit        bool
}

func newSearchModel(searchLimit int) searchModel {
	in := textinput.New()
	in.Placeholder = "Search YouTube…"
	in.Prompt = "> "
	in.PromptStyle = S.Accent
	in.TextStyle = S.Text
	in.PlaceholderStyle = S.Muted
	in.Cursor.Style = S.Accent
	in.Focus()

	f := textinput.New()
	f.Placeholder = "filter…"
	f.Prompt = "/ "
	f.PromptStyle = S.Accent
	f.TextStyle = S.Text
	f.PlaceholderStyle = S.Muted
	f.Cursor.Style = S.Accent
	f.CharLimit = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = S.Accent

	return searchModel{
		state:       searchStateQuery,
		input:       in,
		filter:      f,
		spin:        sp,
		searchLimit: searchLimit,
		selected:    -1,
	}
}

func newSearchModelWithState(searchLimit int, query string, videos []types.Video, cursor int) searchModel {
	m := newSearchModel(searchLimit)
	m.state = searchStateResults
	m.query = query
	m.videos = videos
	if cursor >= 0 && cursor < len(videos) {
		m.cursor = cursor
	}
	return m
}

func (m searchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.exit = true
			return m, tea.Quit
		case "esc":
			if m.state == searchStateResults {
				if m.filterOn {
					m.filterOn = false
					m.filter.SetValue("")
					m.filter.Blur()
					return m, nil
				}
				m.state = searchStateQuery
				m.input.SetValue(m.query)
				m.input.Focus()
				return m, nil
			}
			m.back = true
			return m, tea.Quit
		}
	}

	switch m.state {
	case searchStateQuery:
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
			m.query = strings.TrimSpace(m.input.Value())
			if m.query == "" {
				return m, nil
			}
			m.limit = m.searchLimit
			m.state = searchStateLoading
			return m, tea.Batch(m.spin.Tick, m.startSearchCmd())
		}
		return m, cmd

	case searchStateLoading:
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			return m, nil
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		case searchResultMsg:
			if msg.err != nil || len(msg.videos) == 0 {
				m.errMsg = "No results found."
				if msg.err != nil {
					m.errMsg = msg.err.Error()
				}
				m.state = searchStateError
				return m, nil
			}
			m.videos = msg.videos
			m.cursor = 0
			m.state = searchStateResults
			return m, nil
		}
		return m, nil

	case searchStateResults:
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			return m, nil
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if !m.filterOn && m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if !m.filterOn && m.cursor < len(m.videos)-1 {
					m.cursor++
				}
			case "/":
				m.filterOn = true
				m.filter.Focus()
				return m, nil
			case "tab":
				m.limit += m.searchLimit
				m.state = searchStateLoading
				return m, tea.Batch(m.spin.Tick, m.startSearchCmd())
			case "enter":
				m.selected = m.cursor
				return m, tea.Quit
			}
		}
		if m.filterOn {
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			return m, cmd
		}
		return m, nil

	case searchStateError:
		if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
			m.state = searchStateQuery
			m.input.SetValue("")
			m.input.Focus()
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

func (m searchModel) View() string {
	switch m.state {
	case searchStateQuery:
		body := "  " + m.input.View()
		hints := [][2]string{{"⏎", "Search"}, {"Esc", "Back"}, {"⌃C", "Quit"}}
		return screen(m.width, m.height, "Search", body, hints)

	case searchStateLoading:
		label := "Searching"
		if m.query != "" {
			label = fmt.Sprintf("Searching for %q", m.query)
		}
		body := "  " + m.spin.View() + " " + S.Muted.Render(label+"…")
		return screen(m.width, m.height, "Search", body, [][2]string{{"⌃C", "Quit"}})

	case searchStateResults:
		return m.renderResultsScreen()

	case searchStateError:
		body := "  " + S.Danger.Render("Error: "+m.errMsg) + "\n\n  " + S.Muted.Render("Press Enter to search again")
		return screen(m.width, m.height, "Search", body, [][2]string{{"⏎", "Retry"}, {"Esc", "Back"}, {"⌃C", "Quit"}})
	}
	return ""
}

func (m searchModel) renderResultsScreen() string {
	width, height := frameSize(m.width, m.height)

	// footer hint line height (1) + chrome overhead (header, 2 dividers, 2 blanks)
	bodyH := height - chromeOverhead - 1
	panelH := bodyH - 3 // caption line + top/bottom border
	if panelH < 6 {
		panelH = 6
	}

	leftOuter := min(64, width*3/5)
	if leftOuter < 34 {
		leftOuter = 34
	}
	rightOuter := width - leftOuter - 1
	if rightOuter < 28 {
		rightOuter = 28
		leftOuter = width - rightOuter - 1
	}
	leftW := max(20, leftOuter-4)
	rightW := max(20, rightOuter-4)

	listTitle := fmt.Sprintf("Results (%d)", len(m.videos))
	left := panel(listTitle, leftW, panelH, m.renderList(panelH))
	right := panel("Details", rightW, panelH, m.renderDetails(rightW))

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	hints := [][2]string{{"↑↓", "Navigate"}, {"⏎", "Select"}, {"Tab", "More"}, {"/", "Filter"}, {"Esc", "Back"}}
	return screen(m.width, m.height, "Search Results", body, hints)
}

func (m searchModel) renderList(panelH int) string {
	filtered := m.filteredIndices()
	cursor := m.cursor
	if cursor >= len(filtered) {
		cursor = len(filtered) - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	var lines []string
	if m.filterOn {
		lines = append(lines, S.Accent.Render(m.filter.View()), "")
	}

	listHeight := panelH - len(lines)
	if listHeight < 1 {
		listHeight = 1
	}

	start := cursor - listHeight/2
	if start < 0 {
		start = 0
	}
	end := start + listHeight
	if end > len(filtered) {
		end = len(filtered)
		start = end - listHeight
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		v := m.videos[filtered[i]]
		title := v.Title
		if title == "" {
			title = v.URL
		}
		title = lipgloss.NewStyle().MaxWidth(56).Render(title)
		lines = append(lines, listItem(title, i == cursor))
	}
	return strings.Join(lines, "\n")
}

func (m searchModel) renderDetails(w int) string {
	filtered := m.filteredIndices()
	if len(filtered) == 0 || m.cursor < 0 || m.cursor >= len(filtered) {
		return S.Muted.Render("No selection")
	}
	v := m.videos[filtered[m.cursor]]
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(currentTheme.Text).Width(w)

	row := func(label, value string) string {
		if value == "" {
			return ""
		}
		return "\n" + S.Muted.Render(label+" ") + S.Text.Render(value)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(v.Title))
	b.WriteString(row("Channel:  ", v.Author))
	b.WriteString(row("Duration: ", v.Duration))
	b.WriteString(row("Published:", v.Published))
	b.WriteString(row("Views:    ", v.Views))
	return b.String()
}

func (m searchModel) startSearchCmd() tea.Cmd {
	query := m.query
	limit := m.limit
	return func() tea.Msg {
		videos, err := services.SearchYouTube(query, limit, nil)
		return searchResultMsg{videos: videos, err: err}
	}
}

func (m searchModel) filteredIndices() []int {
	if !m.filterOn || strings.TrimSpace(m.filter.Value()) == "" {
		out := make([]int, len(m.videos))
		for i := range m.videos {
			out[i] = i
		}
		return out
	}
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	type hit struct {
		idx   int
		score int
	}
	hits := make([]hit, 0, len(m.videos))
	for i, v := range m.videos {
		title := strings.ToLower(v.Title)
		author := strings.ToLower(v.Author)
		if s, ok := fuzzyScore(title, q); ok {
			hits = append(hits, hit{idx: i, score: s})
			continue
		}
		if s, ok := fuzzyScore(author, q); ok {
			hits = append(hits, hit{idx: i, score: s - 5})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})
	out := make([]int, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.idx)
	}
	return out
}

func fuzzyScore(text, query string) (int, bool) {
	if query == "" {
		return 0, true
	}
	ti := 0
	score := 0
	lastMatch := -2
	for _, qc := range query {
		found := false
		for ti < len(text) {
			tc := text[ti]
			if rune(tc) == qc {
				found = true
				if ti == lastMatch+1 {
					score += 4
				} else {
					score += 2
				}
				if ti == 0 {
					score += 2
				}
				lastMatch = ti
				ti++
				break
			}
			ti++
		}
		if !found {
			return 0, false
		}
	}
	return score, true
}

func runSearchTea(searchLimit int) (query string, videos []types.Video, selected int, back bool, exit bool, err error) {
	p := tea.NewProgram(newSearchModel(searchLimit), tea.WithAltScreen(), tea.WithMouseAllMotion())
	m, err := p.Run()
	if err != nil {
		return "", nil, -1, false, false, err
	}
	model := m.(searchModel)
	return model.query, model.videos, model.selected, model.back, model.exit, nil
}

func runSearchTeaWithState(searchLimit int, query string, videos []types.Video, cursor int) (q string, vids []types.Video, selected int, back bool, exit bool, err error) {
	p := tea.NewProgram(newSearchModelWithState(searchLimit, query, videos, cursor), tea.WithAltScreen(), tea.WithMouseAllMotion())
	m, err := p.Run()
	if err != nil {
		return "", nil, -1, false, false, err
	}
	model := m.(searchModel)
	return model.query, model.videos, model.selected, model.back, model.exit, nil
}
