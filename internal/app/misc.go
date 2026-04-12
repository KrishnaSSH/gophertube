package app

import (
	"fmt"
	"gophertube/internal/types"
	"strconv"
	"time"
)

func printBanner() {
	fmt.Print("\033[2J\033[H")
	fmt.Println()
	fmt.Println(uiIndent() + textEmphasis.Render("GopherTube"))
	fmt.Println(uiIndent() + textMuted.Render("version "+version))
	fmt.Println()
	fmt.Println(uiIndent() + textPrimary.Render("Fast Youtube Terminal UI"))
	fmt.Println(uiIndent() + textMuted.Render("Press Ctrl+C or Esc to exit"))
	fmt.Println()
	fmt.Println(uiIndent() + textEmphasis.Render(dividerLine))
	fmt.Println()
}

func printProgressBar(current, total int) {
	width := 40
	filled := (current * width) / total
	percentage := (current * 100) / total

	// Create animated progress bar with original cyan color
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += textEmphasis.Render("█")
		} else {
			bar += textMuted.Render("░")
		}
	}

	// Add spinning animation
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := spinners[time.Now().UnixNano()/100000000%int64(len(spinners))]

	// Format percentage with proper padding
	percentStr := fmt.Sprintf("%3d%%", percentage)

	fmt.Printf("\033[2K\r    %s %s %s", spinner, bar, percentStr)
}

func printSearchStats(videos []types.Video) {
	if len(videos) == 0 {
		return
	}

	channels := make(map[string]int)
	hasDuration := 0

	for _, v := range videos {
		channels[v.Author]++
		if v.Duration != "" {
			hasDuration++
		}
	}

	// Calculate average duration if available
	var avgDuration string
	if hasDuration > 0 {
		avgDuration = "~" + videos[0].Duration // Simple approximation
	}

	fmt.Println(uiIndent() + textEmphasis.Render("Search Statistics:"))
	fmt.Printf("%s%s%s\n", uiIndent(), textMuted.Render("• Total videos found: "), textStrong.Render(strconv.Itoa(len(videos))))
	fmt.Printf("%s%s%s\n", uiIndent(), textMuted.Render("• Unique channels: "), textStrong.Render(strconv.Itoa(len(channels))))

	if avgDuration != "" {
		fmt.Printf("%s%s%s\n", uiIndent(), textMuted.Render("• Average duration: "), textStrong.Render(avgDuration))
	}

	// Show top channels if there are multiple
	if len(channels) > 1 && len(videos) > 3 {
		fmt.Printf("%s%s%s\n", uiIndent(), textMuted.Render("• Most active channel: "), textStrong.Render(getTopChannel(channels)))
	}

	fmt.Println()
}

func getTopChannel(channels map[string]int) string {
	var topChannel string
	maxCount := 0

	for channel, count := range channels {
		if count > maxCount {
			maxCount = count
			topChannel = channel
		}
	}

	if len(topChannel) > 30 {
		return topChannel[:27] + "..."
	}
	return topChannel
}

func printSearchTips() {
	tips := []string{
		"Tip: Press Tab to load more results, Esc to go back",
		"Tip: Use ↑/↓ to navigate, Enter to select, Ctrl+C to exit",
	}

	randomTip := tips[time.Now().Unix()%int64(len(tips))]
	fmt.Printf("%s%s\n", uiIndent(), textMuted.Render(randomTip))
	fmt.Println()
}

// (fzf/search helpers removed)
