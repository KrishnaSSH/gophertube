package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Shared UI styles (set by ApplyTheme).
var (
	textPrimary  = lipgloss.NewStyle()
	textMuted    = lipgloss.NewStyle()
	textEmphasis = lipgloss.NewStyle()
	textStrong   = lipgloss.NewStyle()
	textAccent   = lipgloss.NewStyle()
	textWarn     = lipgloss.NewStyle()
	textError    = lipgloss.NewStyle()
)

// Decorative divider reused in sections.
const dividerLine = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

const (
	uiPadTop    = 1
	uiPadBottom = 0
	uiPadLeft   = 4
	uiPadRight  = 0
)

func uiIndent() string {
	return strings.Repeat(" ", uiPadLeft)
}

const (
	escClearScreen = "\033[2J\033[H"
	escHideCursor  = "\033[?25l"
	escShowCursor  = "\033[?25h"
)

func ClearScreen() {
	fmt.Print(escClearScreen)
}

func HideCursor() {
	fmt.Print(escHideCursor)
}

func ShowCursor() {
	fmt.Print(escShowCursor)
}

type Theme struct {
	Name       string
	Primary    lipgloss.Color
	Muted      lipgloss.Color
	Emphasis   lipgloss.Color
	Strong     lipgloss.Color
	Accent     lipgloss.Color
	Warn       lipgloss.Color
	Error      lipgloss.Color
	Monochrome bool
}

var (
	themes = []Theme{
		{Name: "Minimal", Monochrome: true},
		{Name: "Graphite", Primary: "#D0D0D0", Muted: "#8A8A8A", Emphasis: "#E6E6E6", Strong: "#FFFFFF", Accent: "#CFCFCF", Warn: "#C2C2C2", Error: "#B0B0B0"},
		{Name: "Ocean", Primary: "#CDE7F7", Muted: "#6D8CA1", Emphasis: "#8CC9F0", Strong: "#E8F5FF", Accent: "#5FB3D7", Warn: "#9CC6DA", Error: "#7AA0B5"},
		{Name: "Amber", Primary: "#F5E6C8", Muted: "#B49463", Emphasis: "#F2C37E", Strong: "#FFE7B5", Accent: "#E0A458", Warn: "#D8B47A", Error: "#C1884B"},
		{Name: "Forest", Primary: "#DCE9D1", Muted: "#7D9A79", Emphasis: "#9FD19A", Strong: "#EAF5E0", Accent: "#76B28C", Warn: "#A7C4A0", Error: "#7FA383"},
		{Name: "Rose", Primary: "#F1D6DD", Muted: "#A47D87", Emphasis: "#E6A9B5", Strong: "#FFE6EC", Accent: "#D88A9A", Warn: "#C99AA5", Error: "#B97786"},
		{Name: "Nord", Primary: "#E5E9F0", Muted: "#81A1C1", Emphasis: "#88C0D0", Strong: "#ECEFF4", Accent: "#8FBCBB", Warn: "#D8DEE9", Error: "#BF616A"},
		{Name: "Gruv", Primary: "#EBDBB2", Muted: "#A89984", Emphasis: "#D5C4A1", Strong: "#FBF1C7", Accent: "#BDAE93", Warn: "#D8A657", Error: "#FB4934"},
		{Name: "Mono", Monochrome: true},
	}
	currentThemeName = "Minimal"
)

func ApplyTheme(name string) bool {
	for _, t := range themes {
		if t.Name != name {
			continue
		}
		currentThemeName = t.Name
		if t.Monochrome {
			textPrimary = lipgloss.NewStyle()
			textMuted = lipgloss.NewStyle().Faint(true)
			textEmphasis = lipgloss.NewStyle().Bold(true)
			textStrong = lipgloss.NewStyle().Bold(true)
			textAccent = lipgloss.NewStyle().Bold(true)
			textWarn = lipgloss.NewStyle().Bold(true)
			textError = lipgloss.NewStyle().Bold(true)
			return true
		}
		textPrimary = lipgloss.NewStyle().Foreground(t.Primary)
		textMuted = lipgloss.NewStyle().Foreground(t.Muted)
		textEmphasis = lipgloss.NewStyle().Foreground(t.Emphasis).Bold(true)
		textStrong = lipgloss.NewStyle().Foreground(t.Strong).Bold(true)
		textAccent = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
		textWarn = lipgloss.NewStyle().Foreground(t.Warn).Bold(true)
		textError = lipgloss.NewStyle().Foreground(t.Error).Bold(true)
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
	return currentThemeName
}

func init() {
	ApplyTheme(currentThemeName)
}

// (fzf constants removed)
