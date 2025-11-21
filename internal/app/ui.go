package app

// Shared UI constants and helpers

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[1;31m"
	colorGreen   = "\033[1;32m"
	colorYellow  = "\033[1;33m"
	colorMagenta = "\033[1;35m"
	colorCyan    = "\033[1;36m"
	colorWhite   = "\033[1;37m"
)

// Decorative bar reused in sections
const barMagenta = "\033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m"

// fzf default UI options
const (
	fzfBorder      = "rounded"
	fzfMargin      = "1,1"
	fzfPreviewWrap = "wrap"
	// Thumbnail size ratios relative to preview area
	previewWidthNum  = 9
	previewWidthDen  = 10
	previewHeightNum = 2
	previewHeightDen = 5
)
