package app

import (
	"fmt"
	"gophertube/internal/services"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

// sanitizeFilename converts a video title into a filesystem-safe filename.
func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	allowed := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(allowed, r) {
			return r
		}
		return '_'
	}, s)
}

// qualityToFormat maps a human-readable quality to yt-dlp/mpv format selectors.
func qualityToFormat(q string) string {
	switch q {
	case "1080p":
		return "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
	case "720p":
		return "bestvideo[height<=720]+bestaudio/best[height<=720]"
	case "480p":
		return "bestvideo[height<=480]+bestaudio/best[height<=480]"
	case "360p":
		return "bestvideo[height<=360]+bestaudio/best[height<=360]"
	case "Audio":
		return "bestaudio"
	default:
		return "best"
	}
}

// hasFFmpeg checks if ffmpeg is available for merging video/audio.
func hasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// expandPath expands env vars like $HOME and user home shorthand ~.
func expandPath(p string) string {
	if p == "" {
		return p
	}
	// Expand $VAR
	p = os.ExpandEnv(p)
	// Expand ~
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				p = home
			} else if strings.HasPrefix(p, "~/") {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	return p
}

// buildDownloadsPreview returns the fzf preview command for the downloads list.
func buildDownloadsPreview(downloadsPath string) string {
	const tpl = `sh -c 'file="$1"; base="%s/${file%%%%.*}"; thumb="$base.jpg"; w=$((FZF_PREVIEW_COLUMNS * 9 / 10)); h=$((FZF_PREVIEW_LINES * 3 / 5)); if [ -f "$thumb" ]; then chafa --size=${w}x${h} "$thumb" 2>/dev/null; else echo "No image preview available"; fi' sh {}`
	return fmt.Sprintf(tpl, downloadsPath)
}

// MediaPlayer represents available media players
type MediaPlayer struct {
	Name string
	Path string
}

// checkAvailablePlayer checks for MPV.
func checkAvailablePlayer() *MediaPlayer {
	// Prefer MPV for better performance and terminal integration
	if path, err := exec.LookPath("mpv"); err == nil {
		return &MediaPlayer{
			Name: "mpv",
			Path: path,
		}
	}
	return nil
}

// playWithPlayer plays media using the detected player.
func playWithPlayer(player *MediaPlayer, url string, isAudioOnly bool) error {
	var args []string

	if isAudioOnly {
		args = []string{"--no-video"}
	}

	args = append(args, url)

	cmd := exec.Command(player.Path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func gophertubeYouTubeMode(cmd *cli.Command) {
	query, esc := readQuery()
	if esc || query == "" {
		fmt.Print("\033[2J\033[H")
		return
	}
	for {
		// Spinner/progress state
		progressCurrent := 0
		progressTotal := 1
		progressDone := make(chan struct{})

		// Start spinner goroutine
		go func() {
			for {
				select {
				case <-progressDone:
					return
				default:
					printProgressBar(progressCurrent, progressTotal)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()

		videos, err := services.SearchYouTube(query, cmd.Int(FlagSearchLimit), func(current, total int) {
			progressCurrent = current
			progressTotal = total
		})

		close(progressDone)
		fmt.Print("\033[2K\r\n") // Clear progress bar/spinner line
		fmt.Println()
		fmt.Println()

		if err != nil || len(videos) == 0 {
			fmt.Println("    " + textMuted.Render("No results found."))
			fmt.Println()
			fmt.Println("    " + textMuted.Render("Press any key to search again..."))
			os.Stdin.Read(make([]byte, 1))
			return
		}

		fmt.Printf("    %s\n", textEmphasis.Render(fmt.Sprintf("Found %d results!", len(videos))))
		printSearchStats(videos)
		printSearchTips()
		// Reduced delay for faster response
		time.Sleep(200 * time.Millisecond)

		for {
			selected := runFzf(videos, cmd.Int(FlagSearchLimit), query)
			if selected == -2 {
				// User pressed escape, go back to new search
				return
			}
			if selected < 0 || selected >= len(videos) {
				continue // Stay in the same list
			}

			// Show Watch/Download/Audio menu
			menu := []string{"Watch", "Download", "Listen"}
			action := exec.Command("fzf", "--prompt=Action: ")
			action.Stdin = strings.NewReader(strings.Join(menu, "\n"))
			out, errAct := action.Output()
			choice := strings.TrimSpace(string(out))
			if errAct != nil || choice == "" {
				// ESC/cancel -> back to results list
				continue
			}

			if choice == "Download" {
				qualities := []string{"1080p", "720p", "480p", "360p", "Audio"}
				actionQ := exec.Command("fzf", "--prompt=Quality: ")
				actionQ.Stdin = strings.NewReader(strings.Join(qualities, "\n"))
				outQ, errQ := actionQ.Output()
				selectedQ := strings.TrimSpace(string(outQ))
				if errQ != nil || selectedQ == "" {
					// ESC/cancel -> back to results list
					continue
				}

				// Map quality to yt-dlp format
				format := qualityToFormat(selectedQ)
				dlPath := expandPath(cmd.String(FlagDownloadsPath))
				os.MkdirAll(dlPath, 0755)
				// Sanitize filename
				filename := sanitizeFilename(videos[selected].Title)
				outputPath := fmt.Sprintf("%s/%s.%%(ext)s", dlPath, filename)
				fmt.Printf("    %s\n", textEmphasis.Render(fmt.Sprintf("Downloading '%s' as %s...", videos[selected].Title, selectedQ)))

				ytDlpArgs := []string{"-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}

				// override the default args with an audio only version.
				// Note: this downloads it as a .webm, then converts it to a .opus file.
				if format == "bestaudio" {
					ytDlpArgs = []string{"-x", "-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}
				} else {
					// For video+audio, ensure merge to mp4 when possible
					// Warn if ffmpeg is missing (yt-dlp needs it to merge)
					if !hasFFmpeg() {
						fmt.Println("    " + textWarn.Render("Warning: ffmpeg not found. Install ffmpeg to merge video+audio properly."))
						fmt.Println("    " + textMuted.Render("On Ubuntu: sudo apt install ffmpeg | macOS: brew install ffmpeg | Arch: pacman -S ffmpeg"))
					}
					ytDlpArgs = append([]string{"-f", format}, append([]string{"-o", outputPath, "--merge-output-format", "mp4", "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg"}, videos[selected].URL)...)
				}
				actionDl := exec.Command("yt-dlp", ytDlpArgs...)
				actionDl.Stdout = os.Stdout
				actionDl.Stderr = os.Stderr
				err := actionDl.Run()
				if err == nil {
					fmt.Printf("    %s\n", textEmphasis.Render("Download complete!"))
					fmt.Printf("    %s\n", textMuted.Render("Saved to: "+dlPath))
				} else {
					fmt.Printf("    %s\n", textError.Render("Download failed!"))
				}
				fmt.Println("    " + textMuted.Render("Press any key to return..."))
				os.Stdin.Read(make([]byte, 1))
				// After handling download, return to results list
				continue
			}

			// New Audio playback logic
			if choice == "Listen" {
				player := checkAvailablePlayer()
				if player == nil {
					fmt.Println("    " + textError.Render("No media player found!"))
					fmt.Println("    " + textMuted.Render("Please install MPV to play audio."))
					fmt.Println("    " + textMuted.Render("Install MPV: sudo apt install mpv (Ubuntu) | brew install mpv (macOS)"))
					fmt.Println("    " + textMuted.Render("Press any key to return..."))
					os.Stdin.Read(make([]byte, 1))
					continue // Go back to the search results
				}

				fmt.Printf("    %s\n", textEmphasis.Render(fmt.Sprintf("Playing Audio with %s: %s", strings.ToUpper(player.Name), videos[selected].Title)))
				fmt.Printf("    %s\n", textMuted.Render("Channel: "+videos[selected].Author))
				fmt.Printf("    %s\n", textMuted.Render("Duration: "+videos[selected].Duration))
				fmt.Printf("    %s\n", textMuted.Render("Published: "+videos[selected].Published))
				fmt.Println("    " + textEmphasis.Render(dividerLine))
				fmt.Println("    " + textMuted.Render("Controls: 'q' to quit, SPACE to pause/resume, ←→ to seek"))
				fmt.Println("    " + textEmphasis.Render(dividerLine))
				fmt.Println()

				// Extract direct audio stream URL
				audioCmd := exec.Command("yt-dlp", "-f", "bestaudio[ext=m4a]/bestaudio", "-g", videos[selected].URL)
				streamURLBytes, err := audioCmd.Output()
				if err != nil {
					fmt.Println("    " + textError.Render("Failed to get direct audio URL."))
					fmt.Println("    " + textMuted.Render("Make sure yt-dlp is installed."))
					fmt.Println("    " + textMuted.Render("Press any key to return..."))
					os.Stdin.Read(make([]byte, 1))
					continue // Go back to the search results
				}
				streamURL := strings.TrimSpace(string(streamURLBytes))

				if err := playWithPlayer(player, streamURL, true); err != nil {
					fmt.Printf("    %s\n", textError.Render(fmt.Sprintf("Failed to play audio with %s.", player.Name)))
				}

				fmt.Println("    " + textMuted.Render("Press Enter to return."))
				os.Stdin.Read(make([]byte, 1))
				continue // Return to the search results
			}

			// Watch as before
			fmt.Printf("    %s\n", textEmphasis.Render("Playing: "+videos[selected].Title))
			fmt.Printf("    %s\n", textMuted.Render("Channel: "+videos[selected].Author))
			fmt.Printf("    %s\n", textMuted.Render("Duration: "+videos[selected].Duration))
			fmt.Printf("    %s\n", textMuted.Render("Published: "+videos[selected].Published))
			fmt.Println()
			fmt.Println("    " + textEmphasis.Render(dividerLine))
			fmt.Println()
			mpvPath := "mpv"
			quality := cmd.String(FlagQuality)
			var mpvArgs []string

			// Add the fullscreen flag for video playback
			mpvArgs = append(mpvArgs, "--fs")

			if quality != "" {
				f := qualityToFormat(quality)
				if f == "bestaudio" {
					mpvArgs = append(mpvArgs, "--no-video")
				}
				mpvArgs = append(mpvArgs, "--ytdl-format="+f)
			}

			mpvArgs = append(mpvArgs, videos[selected].URL)
			exec.Command(mpvPath, mpvArgs...).Run()
			continue
		}
	}
}

func gophertubeDownloadsMode(cmd *cli.Command) {
	dlPath := expandPath(cmd.String(FlagDownloadsPath))
	files, err := os.ReadDir(dlPath)
	if err != nil || len(files) == 0 {
		fmt.Println("    " + textMuted.Render("No downloaded videos found."))
		time.Sleep(600 * time.Millisecond)
		return
	}
	var videoFiles []string
	for _, f := range files {
		if !f.IsDir() && (strings.HasSuffix(f.Name(), ".mp4") || strings.HasSuffix(f.Name(), ".mkv") || strings.HasSuffix(f.Name(), ".webm") || strings.HasSuffix(f.Name(), ".avi") || strings.HasSuffix(f.Name(), ".m4a") || strings.HasSuffix(f.Name(), ".mp3") || strings.HasSuffix(f.Name(), ".opus")) {
			videoFiles = append(videoFiles, f.Name())
		}
	}
	if len(videoFiles) == 0 {
		fmt.Println("    " + textMuted.Render("No downloaded videos found."))
		time.Sleep(600 * time.Millisecond)
		return
	}
	fzfPreview := buildDownloadsPreview(dlPath)
	action := exec.Command("fzf", "--ansi", "--preview-window=wrap", "--prompt=Downloads: ", "--preview", fzfPreview)
	action.Stdin = strings.NewReader(strings.Join(videoFiles, "\n"))
	out, _ := action.Output()
	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return
	}
	filePath := filepath.Join(dlPath, selected)
	fmt.Printf("    %s\n", textEmphasis.Render("Playing: "+selected))
	fmt.Println()
	fmt.Println("    " + textEmphasis.Render(dividerLine))
	fmt.Println()
	mpvPath := "mpv"
	exec.Command(mpvPath, filePath).Run()
}
