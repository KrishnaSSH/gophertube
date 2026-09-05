package cli

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme is a small, role-based palette. Every screen is built out of these
// eight roles so a theme swap never leaves a screen half-styled.
type Theme struct {
	Name      string
	Accent    lipgloss.Color // primary brand / selection color
	Secondary lipgloss.Color // secondary accent, used sparingly
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Danger    lipgloss.Color
	Text      lipgloss.Color
	Subtext   lipgloss.Color
	Border    lipgloss.Color
}

var themes = []Theme{
	{Name: "Slate", Accent: "#7C9EF5", Secondary: "#A78BFA", Success: "#34D399", Warning: "#FBBF24", Danger: "#F87171", Text: "#E5E7EB", Subtext: "#8B93A7", Border: "#3A3F4B"},
	{Name: "Tokyo Night", Accent: "#7AA2F7", Secondary: "#BB9AF7", Success: "#9ECE6A", Warning: "#E0AF68", Danger: "#F7768E", Text: "#C0CAF5", Subtext: "#6B7394", Border: "#3B4261"},
	{Name: "Catppuccin Mocha", Accent: "#89B4FA", Secondary: "#CBA6F7", Success: "#A6E3A1", Warning: "#F9E2AF", Danger: "#F38BA8", Text: "#CDD6F4", Subtext: "#7D8296", Border: "#45475A"},
	{Name: "Gruvbox", Accent: "#83A598", Secondary: "#D3869B", Success: "#B8BB26", Warning: "#FABD2F", Danger: "#FB4934", Text: "#EBDBB2", Subtext: "#9A8F73", Border: "#504945"},
	{Name: "Nord", Accent: "#88C0D0", Secondary: "#B48EAD", Success: "#A3BE8C", Warning: "#EBCB8B", Danger: "#BF616A", Text: "#E5E9F0", Subtext: "#8894AC", Border: "#434C5E"},
	{Name: "Dracula", Accent: "#8BE9FD", Secondary: "#BD93F9", Success: "#50FA7B", Warning: "#F1FA8C", Danger: "#FF5555", Text: "#F8F8F2", Subtext: "#7C87B3", Border: "#44475A"},
	{Name: "Rose Pine", Accent: "#C4A7E7", Secondary: "#EBBCBA", Success: "#9CCFD8", Warning: "#F6C177", Danger: "#EB6F92", Text: "#E0DEF4", Subtext: "#82809E", Border: "#403D52"},
	{Name: "Everforest", Accent: "#A7C080", Secondary: "#83C092", Success: "#A7C080", Warning: "#DBBC7F", Danger: "#E67E80", Text: "#D3C6AA", Subtext: "#8F9A88", Border: "#4F585E"},
}

var currentTheme Theme

// S holds every derived style used by the screens. Rebuilt by ApplyTheme.
var S struct {
	Logo     lipgloss.Style
	Subtitle lipgloss.Style
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Bold     lipgloss.Style
	Accent   lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Danger   lipgloss.Style
	Marker   lipgloss.Style
	HintKey  lipgloss.Style
	HintDesc lipgloss.Style
}

func ApplyTheme(name string) bool {
	for _, t := range themes {
		if t.Name != name {
			continue
		}
		currentTheme = t

		S.Logo = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
		S.Subtitle = lipgloss.NewStyle().Foreground(t.Subtext)
		S.Text = lipgloss.NewStyle().Foreground(t.Text)
		S.Muted = lipgloss.NewStyle().Foreground(t.Subtext)
		S.Bold = lipgloss.NewStyle().Bold(true).Foreground(t.Text)
		S.Accent = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
		S.Success = lipgloss.NewStyle().Foreground(t.Success)
		S.Warning = lipgloss.NewStyle().Foreground(t.Warning)
		S.Danger = lipgloss.NewStyle().Bold(true).Foreground(t.Danger)
		S.Marker = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
		S.HintKey = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
		S.HintDesc = lipgloss.NewStyle().Foreground(t.Subtext)
		return true
	}
	return false
}

func ThemeNames() []string {
	out := make([]string, 0, len(themes))
	for _, t := range themes {
		out = append(out, t.Name)
	}
	return out
}

func CurrentThemeName() string {
	return currentTheme.Name
}

func init() {
	ApplyTheme(themes[0].Name)
}

const escShowCursor = "\033[?25h"

func ShowCursor() {
	print(escShowCursor)
}

// --- shared layout chrome -------------------------------------------------

func divider(width int) string {
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().Foreground(currentTheme.Border).Render(strings.Repeat("─", width))
}

const (
	outerMarginX = 2
	outerMarginY = 1
)

// frameSize returns the usable content width/height inside screen()'s outer
// margin, given the raw terminal size.
func frameSize(termWidth, termHeight int) (int, int) {
	if termWidth <= 0 {
		termWidth = 100
	}
	if termHeight <= 0 {
		termHeight = 32
	}
	w := termWidth - outerMarginX*2
	h := termHeight - outerMarginY*2
	if w < 24 {
		w = 24
	}
	if h < 8 {
		h = 8
	}
	return w, h
}

func header(width int, subtitle string) string {
	head := S.Logo.Render("GopherTube") + "  " + S.Muted.Render("v"+VERSION)
	if subtitle != "" {
		head += "   " + S.Subtitle.Render("· "+subtitle)
	}
	return lipgloss.NewStyle().Width(width).Render(head)
}

func hintBar(width int, hints [][2]string) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, S.HintKey.Render(h[0])+" "+S.HintDesc.Render(h[1]))
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(parts, S.Muted.Render("   ")))
}

// screen composes a single-line header, a divider, the body, another
// divider, and a keybind hint line, then sets the whole thing off from the
// terminal edge with a plain margin. No borders here on purpose — borders
// are reserved for the results split-view, where they separate two real
// panels rather than just framing text.
func screen(termWidth, termHeight int, subtitle string, body string, hints [][2]string) string {
	w, _ := frameSize(termWidth, termHeight)
	content := lipgloss.JoinVertical(lipgloss.Left,
		header(w, subtitle),
		divider(w),
		body,
		divider(w),
		hintBar(w, hints),
	)
	return lipgloss.NewStyle().Margin(outerMarginY, outerMarginX).Render(content)
}

// chromeOverhead is the number of fixed lines screen() adds around a body:
// header, two dividers, and the hint line.
const chromeOverhead = 4

// panel draws a rounded, bordered box with an optional small caption above it.
func panel(title string, w, h int, content string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Border).
		Padding(0, 1).
		Width(w).
		Height(h).
		Render(content)
	if title == "" {
		return box
	}
	return lipgloss.JoinVertical(lipgloss.Left, S.Subtitle.Render(" "+title), box)
}

// listItem renders one selectable row with a colored marker bar when active.
func listItem(label string, selected bool) string {
	if selected {
		return S.Marker.Render("▎ ") + S.Bold.Render(label)
	}
	return "  " + S.Text.Render(label)
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
