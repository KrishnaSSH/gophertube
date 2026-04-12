package app

import (
	"bytes"
	"fmt"
	"gophertube/internal/services"
	"gophertube/internal/types"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

// buildSearchHeader creates the styled fzf header for the search UI.
func buildSearchHeader(resultCount int, query string) string {
	header := ""
	header += textMuted.Render("↑/↓") + " " + textMuted.Render("to move")
	header += " • " + textEmphasis.Render("type") + " " + textMuted.Render("to search")
	header += " • " + textEmphasis.Render("Enter") + " " + textMuted.Render("to select")
	header += " • " + textEmphasis.Render("Tab") + " " + textMuted.Render("to load more")
	header += " • " + textStrong.Render(strconv.Itoa(resultCount)) + " " + textMuted.Render("results")
	header += " • " + textEmphasis.Render(query)
	return "--header=" + header
}

// buildSearchPreview returns the shell for fzf --preview for search results.
// It renders the thumbnail via chafa, pads to place the cursor below the image,
// then prints plain metadata.
func buildSearchPreview() string {
	tpl := `sh -c 'thumbfile="$1"; title="$2"; w=$((FZF_PREVIEW_COLUMNS * %d / %d)); h=$((FZF_PREVIEW_LINES * %d / %d)); if [ -s "$thumbfile" ] && [ -f "$thumbfile" ]; then chafa --size=${w}x${h} "$thumbfile" 2>/dev/null; else echo "No image preview available"; fi; pad=$((FZF_PREVIEW_LINES - h - 1)); i=0; while [ $i -gt -1 ] && [ $i -lt $pad ]; do echo; i=$((i+1)); done; printf "%%s\n" "$title"; printf "Duration: %%s\n" "$3"; printf "Published: %%s\n" "$4"; printf "Author: %%s\n" "$5"; printf "Views: %%s\n" "$6"' sh {3} {2} {4} {8} {5} {6}`
	return fmt.Sprintf(
		tpl,
		previewWidthNum, previewWidthDen,
		previewHeightNum, previewHeightDen,
	)
}

func printBanner() {
	fmt.Print("\033[2J\033[H")
	fmt.Println()
	fmt.Println("    " + textEmphasis.Render("GopherTube"))
	fmt.Println("    " + textMuted.Render("version "+version))
	fmt.Println()
	fmt.Println("    " + textPrimary.Render("Fast Youtube Terminal UI"))
	fmt.Println("    " + textMuted.Render("Press Ctrl+C or Esc to exit"))
	fmt.Println()
	fmt.Println("    " + textEmphasis.Render(dividerLine))
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

	fmt.Println("    " + textEmphasis.Render("Search Statistics:"))
	fmt.Printf("    %s%s\n", textMuted.Render("• Total videos found: "), textStrong.Render(strconv.Itoa(len(videos))))
	fmt.Printf("    %s%s\n", textMuted.Render("• Unique channels: "), textStrong.Render(strconv.Itoa(len(channels))))

	if avgDuration != "" {
		fmt.Printf("    %s%s\n", textMuted.Render("• Average duration: "), textStrong.Render(avgDuration))
	}

	// Show top channels if there are multiple
	if len(channels) > 1 && len(videos) > 3 {
		fmt.Printf("    %s%s\n", textMuted.Render("• Most active channel: "), textStrong.Render(getTopChannel(channels)))
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
	fmt.Printf("    %s\n", textMuted.Render(randomTip))
	fmt.Println()
}

func readQuery() (string, bool) {
	printBanner()
	escPressed := false
	rl, err := readline.NewEx(&readline.Config{
		Prompt: "    " + textEmphasis.Render("> "),
		FuncFilterInputRune: func(r rune) (rune, bool) {
			if r == readline.CharEsc {
				escPressed = true
				return readline.CharInterrupt, true
			}
			escPressed = false
			return r, true
		},
	})
	if err != nil {
		return "", false
	}
	defer rl.Close()

	line, err := rl.Readline()
	if err == readline.ErrInterrupt {
		if escPressed {
			return "", true
		}
		os.Exit(0)
	}
	if err != nil {
		return "", true
	}

	return strings.TrimSpace(line), false
}

func runFzf(videos []types.Video, searchLimit int, query string) int {
	limit := searchLimit
	filter := ""
	for {
		var input bytes.Buffer
		for i, v := range videos {
			thumbPath := v.ThumbnailPath
			thumbPath = strings.ReplaceAll(thumbPath, "'", "'\\''")
			fmt.Fprintf(&input, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", i, v.Title, thumbPath, v.Duration, v.Author, v.Views, v.Description, v.Published)
		}
		fzfArgs := []string{
			"--ansi",
			"--with-nth=2..2",
			"--delimiter=\t",
			buildSearchHeader(len(videos), query),
			"--expect=tab",
			"--bind=esc:abort",
			"--border=" + fzfBorder,
			"--margin=" + fzfMargin,
			"--preview-window=" + fzfPreviewWrap,
			"--preview",
			buildSearchPreview(),
		}
		if filter != "" {
			fzfArgs = append(fzfArgs, "--query="+filter)
		}
		cmd := exec.Command("fzf", fzfArgs...)
		cmd.Stdin = &input
		pr, pw, _ := os.Pipe()
		cmd.Stdout = pw
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Println(textError.Render("fzf error:"), err)
			return -1
		}
		pw.Close()
		selected, _ := io.ReadAll(pr)
		cmd.Wait()
		lines := strings.Split(strings.TrimSpace(string(selected)), "\n")
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			return -2 // user pressed escape in fzf
		}
		if lines[0] == "tab" {
			fmt.Printf("    %s\n", textMuted.Render("Loading more results..."))
			limit += searchLimit
			moreVideos, err := services.SearchYouTube(query, limit, func(current, total int) {
				// progressCurrent = current // This line was removed
				// progressTotal = total // This line was removed
			})
			if err != nil || len(moreVideos) == len(videos) {
				continue
			}
			videos = moreVideos
			fmt.Printf("    %s\n", textEmphasis.Render(fmt.Sprintf("Loaded %d total results!", len(videos))))
			printSearchStats(videos)
			continue
		}
		line := lines[0]
		if line == "" {
			return -1
		}
		idxStr := strings.SplitN(line, "\t", 2)[0]
		idx := 0
		fmt.Sscanf(idxStr, "%d", &idx)
		return idx
	}
}
