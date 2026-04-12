package app

import "github.com/charmbracelet/lipgloss"

// Shared UI styles
var (
	textPrimary  = lipgloss.NewStyle()
	textMuted    = lipgloss.NewStyle().Faint(true)
	textEmphasis = lipgloss.NewStyle().Bold(true)
	textStrong   = lipgloss.NewStyle().Bold(true)
	textAccent   = lipgloss.NewStyle().Bold(true)
	textWarn     = lipgloss.NewStyle().Bold(true)
	textError    = lipgloss.NewStyle().Bold(true)
)

// Decorative divider reused in sections
const dividerLine = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

// fzf default UI options
const (
	fzfBorder      = "rounded"
	fzfMargin      = "1,1"
	fzfPreviewWrap = "wrap"
	// Thumbnail size ratios relative to preview area
	previewWidthNum  = 9
	previewWidthDen  = 10
	previewHeightNum = 3
	previewHeightDen = 5
)
