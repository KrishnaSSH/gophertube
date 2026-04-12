package app

import (
	"fmt"
	"sort"
	"strings"

	"gophertube/internal/services"
	"gophertube/internal/types"

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
	in.Placeholder = "Search YouTube..."
	in.Prompt = uiIndent() + textEmphasis.Render("> ")
	in.Focus()

	f := textinput.New()
	f.Placeholder = "Filter..."
	f.Prompt = uiIndent() + textEmphasis.Render("/ ")
	f.CharLimit = 80

	sp := spinner.New()
	sp.Spinner = spinner.Line

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
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.videos)-1 {
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
		return m.frame(WithMargin(m.input.View() + "\n"))
	case searchStateLoading:
		return m.frame(WithMargin(uiIndent() + textMuted.Render(m.spin.View()+" Searching...") + "\n"))
	case searchStateResults:
		return m.frame(WithMargin(m.renderResults()))
	case searchStateError:
		return m.frame(WithMargin(uiIndent() + textError.Render(m.errMsg) + "\n" + uiIndent() + textMuted.Render("Press Enter to search again") + "\n"))
	}
	return ""
}

func (m searchModel) renderResults() string {
	filtered := m.filteredIndices()
	if m.cursor >= len(filtered) {
		m.cursor = len(filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	totalW := m.width
	if totalW <= 0 {
		totalW = 100
	}
	totalH := m.height
	if totalH <= 0 {
		totalH = 30
	}

	header := []string{
		uiIndent() + textEmphasis.Render(fmt.Sprintf("Found %d results", len(m.videos))),
		uiIndent() + textMuted.Render("↑/↓ to navigate • Enter to select • Tab to load more • / to filter • Esc to back"),
		"",
	}

	left, leftLines := m.renderLeftPane()
	contentH := totalH - 2
	if contentH < 12 {
		contentH = 12
	}
	listHeight := contentH - len(header) - leftLines - 2
	if listHeight < 5 {
		listHeight = 5
	}

	start := m.cursor - listHeight/2
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

	lines := append([]string{}, header...)
	if m.filterOn {
		lines = append(lines, uiIndent()+textEmphasis.Render(m.filter.View()))
		lines = append(lines, "")
	}
	for i := start; i < end; i++ {
		idx := filtered[i]
		v := m.videos[idx]
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		title := v.Title
		if title == "" {
			title = v.URL
		}
		title = lipgloss.NewStyle().MaxWidth(max(20, totalW-10)).Render(title)
		cStyle := textPrimary
		if i == m.cursor {
			cStyle = textStrong
		}
		lines = append(lines, fmt.Sprintf("%s%s%s", uiIndent(), textAccent.Render(cursor), cStyle.Render(title)))
	}
	right := strings.Join(lines, "\n")

	leftW := min(42, totalW/2)
	if leftW < 30 {
		leftW = 30
	}
	rightW := totalW - leftW - 2
	if rightW < 30 {
		rightW = 30
		leftW = totalW - rightW - 2
		if leftW < 24 {
			leftW = 24
		}
	}

	leftBox := lipgloss.NewStyle().Width(leftW).Height(contentH).Align(lipgloss.Left).Render(left)
	rightBox := lipgloss.NewStyle().Width(rightW).Height(contentH).Align(lipgloss.Left).Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, "  ", rightBox)
}

func (m searchModel) renderLeftPane() (string, int) {
	filtered := m.filteredIndices()
	if len(filtered) == 0 || m.cursor < 0 || m.cursor >= len(filtered) {
		return "", 0
	}
	v := m.videos[filtered[m.cursor]]
	totalW := m.width
	if totalW <= 0 {
		totalW = 100
	}
	leftW := min(42, totalW/2)
	if leftW < 30 {
		leftW = 30
	}
	lines := []string{
		uiIndent() + textAccent.Render("Details"),
		uiIndent() + textStrong.Render("Channel: ") + textEmphasis.Render(v.Author),
		uiIndent() + textWarn.Render("Duration: ") + textAccent.Render(v.Duration),
		uiIndent() + textMuted.Render("Published: ") + textPrimary.Render(v.Published),
		uiIndent() + textMuted.Render("Views: ") + textPrimary.Render(v.Views),
	}
	content := strings.Join(lines, "\n")
	return content, strings.Count(content, "\n") + 1
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

func (m searchModel) frame(s string) string {
	if m.width <= 0 || m.height <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
