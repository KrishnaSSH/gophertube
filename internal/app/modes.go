package app

import (
	"bufio" // Re-added for bufio.Scanner
	"context"
	"fmt"
	"gophertube/internal/services"
	"io" // Re-added for io.Pipe
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/urfave/cli/v3"
)

// ANSI colors and bar constants are defined in ui.go

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
	const tpl = `sh -c '
		f="$1"; d="$2"
		
		# Path Construction (Fix for dot names and path injection)
		# ${f%%.*} in Go becomes ${f%.*} in Shell (removes only the last extension)
		base="$d/${f%%.*}"
		thumb="$base.jpg"
		video="$d/$f"

		# 1. Dimensions (40%% Height)
		h=$((FZF_PREVIEW_LINES*2/5))
		w=$((FZF_PREVIEW_COLUMNS))
		
	   if [ -f "$thumb" ]; then 
            # Draw the Image (High Quality)
            chafa --size="${w}x${h}" --bg="#000000" --work=9 "$thumb" 2>/dev/null; 
            
            # 2. FORCE CURSOR DOWN
            i=0
            while [ $i -lt $h ]; do 
                echo; 
                i=$((i+1)); 
            done
        else 
            echo "No image preview available";
            echo; 
        fi;

		# 3. Get Duration (ffprobe)
		dur=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 -sexagesimal "$video" 2>/dev/null | cut -d. -f1)
		if [ -z "$dur" ]; then dur="N/A"; fi

		# 4. Print Info
		echo "--------------------------------"
		printf "\033[1;36mFile:      \033[0m%%s\n" "$f"
		printf "\033[1;36mDuration:  \033[0m%%s\n" "$dur"
		printf "\033[1;36mDirectory: \033[0m%%s\n" "$d"
	' sh {} "%[1]s"`

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
func playWithPlayer(ctx context.Context, player *MediaPlayer, url string, isAudioOnly bool) error {
	var args []string

	if isAudioOnly {
		args = []string{"--no-video"}
	}

	args = append(args, url)

	cmd := exec.CommandContext(ctx, player.Path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to play with %s: %w", player.Name, err)
	}
	return nil
}

// convertSpeedToMBps converts speed string (e.g., "1.23KiB/s") to float64 MB/s.
func convertSpeedToMBps(speedStr string) float64 {
	re := regexp.MustCompile(`(\d+\.?\d*)(K|M|G)?iB/s`)
	match := re.FindStringSubmatch(speedStr)

	if len(match) < 2 {
		return 0.0 // Could not parse speed
	}

	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0.0
	}

	unitPrefix := match[2] // K, M, G or empty

	switch unitPrefix {
	case "K":
		return value / 1024 // KiB/s to MiB/s
	case "M":
		return value // MiB/s (already in MB/s equivalent)
	case "G":
		return value * 1024 // GiB/s to MiB/s
	default: // Assume B/s, convert to MiB/s
		return value / (1024 * 1024)
	}
}

func gophertubeYouTubeMode(ctx context.Context, cmd *cli.Command) {
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
				case <-ctx.Done(): // Listen for context cancellation
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
			fmt.Println("    " + colorRed + "No results found." + colorReset)
			fmt.Println()
			fmt.Println("    " + colorWhite + "Press any key to search again..." + colorReset)
			os.Stdin.Read(make([]byte, 1))
			return
		}

		fmt.Printf("    %sFound %d results!%s\n", colorGreen, len(videos), colorReset)
		printSearchStats(videos)
		printSearchTips()
		// Reduced delay for faster response
		time.Sleep(200 * time.Millisecond)

		for {
			selected := runFzf(ctx, videos, cmd.Int(FlagSearchLimit), query)
			if selected == -2 {
				// User pressed escape, go back to new search
				return
			}
			if selected < 0 || selected >= len(videos) {
				continue // Stay in the same list
			}

			// Show Watch/Download/Audio menu
			menu := []string{"Watch", "Download", "Listen"}
			action := exec.CommandContext(ctx, "fzf", "--prompt=Action: ")
			action.Stdin = strings.NewReader(strings.Join(menu, "\n"))
			out, errAct := action.Output()
			choice := strings.TrimSpace(string(out))
			if errAct != nil || choice == "" {
				// ESC/cancel -> back to results list
				continue
			}
			checkYTdlpVersion()
			if choice == "Download" {
				qualities := []string{"1080p", "720p", "480p", "360p", "Audio"}
				actionQ := exec.CommandContext(ctx, "fzf", "--prompt=Quality: ")
				actionQ.Stdin = strings.NewReader(strings.Join(qualities, "\n"))
				outQ, errQ := actionQ.Output()
				selectedQ := strings.TrimSpace(string(outQ))
				if errQ != nil || selectedQ == "" {
					continue
				}

				format := qualityToFormat(selectedQ)
				dlPath := expandPath(cmd.String(FlagDownloadsPath))
				os.MkdirAll(dlPath, 0755)
				filename := sanitizeFilename(videos[selected].Title)
				outputPath := fmt.Sprintf("%s/%s.%%(ext)s", dlPath, filename)

				fmt.Printf("    %sDownloading '%s' as %s...%s\n", colorGreen, videos[selected].Title, selectedQ, colorReset)

				// 1. SETUP YT-DLP ARGS
				ytDlpArgs := []string{"-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}

				if format == "bestaudio" {
					ytDlpArgs = []string{"-x", "-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}
				} else {
					if !hasFFmpeg() {
						fmt.Println("    " + colorYellow + "Warning: ffmpeg not found." + colorReset)
					}
					// Standard merge args
					ytDlpArgs = append([]string{"-f", format}, append([]string{"-o", outputPath, "--merge-output-format", "mp4", "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg"}, videos[selected].URL)...)
				}

				// 2. CRITICAL: Add --newline so bufio can scan it, and --progress
				ytDlpArgs = append(ytDlpArgs, "--progress", "--newline", "--no-color")

				actionDl := exec.CommandContext(ctx, "yt-dlp", ytDlpArgs...)
				dlReader, dlWriter := io.Pipe()
				dlErrReader, dlErrWriter := io.Pipe()
				actionDl.Stdout = dlWriter
				actionDl.Stderr = dlErrWriter

				downloadDone := make(chan error, 1)
				progressLineChan := make(chan string, 1)

				// Goroutine: Scan Stdout
				go func() {
					scanner := bufio.NewScanner(dlReader)
					// We look for lines starting with [download]
					progressLineRegex := regexp.MustCompile(`^\[download\].*`)
					for scanner.Scan() {
						line := scanner.Text()
						if progressLineRegex.MatchString(line) {
							progressLineChan <- line
						}
					}
					dlReader.Close()
				}()

				// Goroutine: Scan Stderr (silenced mostly)
				go func() {
					scanner := bufio.NewScanner(dlErrReader)
					for scanner.Scan() {
						// Optional: Un-comment below to debug errors
						// fmt.Fprintf(os.Stderr, "Debug: %s\n", scanner.Text())
					}
					dlErrReader.Close()
				}()

				// Goroutine: Run Command
				go func() {
					err := actionDl.Run()
					dlWriter.Close()
					dlErrWriter.Close()
					downloadDone <- err
					close(downloadDone)
					close(progressLineChan)
				}()

				// 3. DISPLAY PACMAN PROGRESS
				var finalDownloadErr error
				downloadFinished := false

				// Regex to extract percentage (e.g. "45.5%")
				// Regex to extract percentage (e.g. "45.5% of")
				pctRegex := regexp.MustCompile(`(\d+\.?\d*)% of`)
				// Regex to extract speed (e.g. "1.23MiB/s" or "414.07KiB/s")
				speedRegex := regexp.MustCompile(`at\s+(\d+\.?\d*(?:K|M|G)iB/s)`)

			progressLoop:
				for {
					select {
					case <-ctx.Done(): // Listen for context cancellation
						finalDownloadErr = ctx.Err()
						break progressLoop
					case progressLine, ok := <-progressLineChan:
						if !ok {
							break progressLoop
						}

						// Default to 0 if we can't parse
						pct := 0.0
						match := pctRegex.FindStringSubmatch(progressLine)
						if len(match) > 1 {
							// convert string "45.5" to float 45.5
							if val, err := strconv.ParseFloat(match[1], 64); err == nil {
								pct = val
							}
						}

						// Extract speed
						speedStr := "N/A"
						speedMatch := speedRegex.FindStringSubmatch(progressLine)
						if len(speedMatch) > 1 {
							speedStr = speedMatch[1]
						}

						currentSpeedMBps := convertSpeedToMBps(speedStr)
						formattedSpeed := fmt.Sprintf("%.1f MB/s", currentSpeedMBps)

						// Generate and print the new download progress bar
						// Width: 30 chars for the bar part
						progressBarOutput := drawDownloadProgressBar(pct, formattedSpeed, 30)

						// \r overwrites the current line
						fmt.Printf("\r    %s", progressBarOutput)

					case err := <-downloadDone:
						finalDownloadErr = err
						downloadFinished = true
						break progressLoop
					}
				}

				fmt.Print("\r" + strings.Repeat(" ", 100) + "\r") // Clear line
				if finalDownloadErr == nil {
					fmt.Printf("    %sDownload complete!%s\n", colorGreen, colorReset)
					fmt.Printf("    %sSaved to: %s%s\n", colorWhite, dlPath, colorReset)
				} else {
					fmt.Printf("    %sDownload failed! %v%s\n", colorRed, finalDownloadErr, colorReset)
				}

				fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)

				// ... Input blocking code ...
				oldState, _ := readline.MakeRaw(int(os.Stdin.Fd()))
				os.Stdin.Read(make([]byte, 1))
				readline.Restore(int(os.Stdin.Fd()), oldState)

				if downloadFinished {
					continue //Return to the Search Results
				}
			}
			// New Audio playback logic
			if choice == "Listen" {
				player := checkAvailablePlayer()
				if player == nil {
					fmt.Println("    " + colorRed + "No media player found!" + colorReset)
					fmt.Println("    " + colorWhite + "Please install MPV to play audio." + colorReset)
					fmt.Println("    " + colorYellow + "Install MPV: sudo apt install mpv (Ubuntu) | brew install mpv (macOS) | sudo pacman -S mpv (arch)" + colorReset)
					fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)
					os.Stdin.Read(make([]byte, 1))
					continue // Go back to the search results
				}

				fmt.Printf("    %sPlaying Audio with %s: %s%s\n", colorYellow, strings.ToUpper(player.Name), videos[selected].Title, colorReset)
				fmt.Printf("    %sChannel: %s%s\n", colorWhite, videos[selected].Author, colorReset)
				fmt.Printf("    %sDuration: %s%s\n", colorWhite, videos[selected].Duration, colorReset)
				fmt.Printf("    %sPublished: %s%s\n", colorCyan, videos[selected].Published, colorReset)
				fmt.Println("    " + barMagenta)
				fmt.Println("    " + colorYellow + "Controls: 'q' to quit, SPACE to pause/resume, ←→ to seek" + colorReset)
				fmt.Println("    " + barMagenta)
				fmt.Println()

				// Extract direct audio stream URL
				audioCmd := exec.CommandContext(ctx, "yt-dlp", "-f", "bestaudio[ext=m4a]/bestaudio", "-g", videos[selected].URL)
				streamURLBytes, err := audioCmd.Output()
				if err != nil {
					fmt.Println("    " + colorRed + "Failed to get direct audio URL." + colorReset)
					fmt.Println("    " + colorWhite + "Make sure yt-dlp is installed and the latest one." + colorReset)
					fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)
					os.Stdin.Read(make([]byte, 1))
					continue // Go back to the search results
				}
				streamURL := strings.TrimSpace(string(streamURLBytes))

				if err := playWithPlayer(ctx, player, streamURL, true); err != nil {
					fmt.Printf("    \033[1;31mFailed to play audio with %s.\033[0m\n", player.Name)
				}

				fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)

				// ... Input blocking code ...
				oldState, _ := readline.MakeRaw(int(os.Stdin.Fd()))
				os.Stdin.Read(make([]byte, 1))
				readline.Restore(int(os.Stdin.Fd()), oldState)
				continue // Return to the search results
			}

			if choice == "Watch" {
				player := checkAvailablePlayer()
				if player == nil {
					fmt.Println("    " + colorRed + "No media player (mpv) found!" + colorReset)
					fmt.Println("    " + colorWhite + "Please install mpv to play videos." + colorReset)
					fmt.Println("    " + colorYellow + "Install mpv: sudo apt install mpv (Ubuntu) | brew install mpv (macOS) | sudo pacman -S mpv (arch)" + colorReset)
					fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)
					os.Stdin.Read(make([]byte, 1))
					continue // Go back to the search results
				}
				fmt.Printf("    %sPlaying: %s%s\n", colorYellow, videos[selected].Title, colorReset)
			}
			fmt.Printf("    %sChannel: %s%s\n", colorWhite, videos[selected].Author, colorReset)
			fmt.Printf("    %sDuration: %s%s\n", colorWhite, videos[selected].Duration, colorReset)
			fmt.Printf("    %sPublished: %s%s\n", colorCyan, videos[selected].Published, colorReset)
			fmt.Println()
			fmt.Println("    " + barMagenta)
			fmt.Println()

			// New menu for playback mode
			fmt.Println("    " + barMagenta)
			fmt.Println("    " + colorYellow + "Play video in [t]erminal or [m]pv? " + colorReset)
			fmt.Println("    " + barMagenta)
			fmt.Print("    " + colorGreen + "> " + colorReset)

			oldState, err := readline.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				continue
			}
			defer readline.Restore(int(os.Stdin.Fd()), oldState)

			var playbackChoice rune
			buf := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					playbackChoice = 'm' // Default to MPV on error
					break
				}
				if buf[0] == 't' || buf[0] == 'T' {
					playbackChoice = 't'
					break
				} else if buf[0] == 'm' || buf[0] == 'M' {
					playbackChoice = 'm'
					break
				} else if buf[0] == 27 { // ESC key
					playbackChoice = ' ' // Indicate escape
					break
				}
			}
			fmt.Println() // Newline after input

			if playbackChoice == ' ' { // User pressed ESC
				continue
			}

			mpvPath := "mpv"
			quality := cmd.String(FlagQuality)
			var mpvArgs []string

			if playbackChoice == 't' {
				compat := checkTerminalCompatibility()
				if !compat.HasSixel && !compat.HasCaca {
					fmt.Println("    " + colorYellow + "Warning: In-terminal video playback may not be supported." + colorReset)
					fmt.Println("    " + colorYellow + "For best results, please install 'libsixel-bin' or 'caca-utils'." + colorReset)
					fmt.Println("    " + colorYellow + "Continuing playback attempt in 3 seconds..." + colorReset)
					time.Sleep(3 * time.Second)
				}
				mpvArgs = append(mpvArgs, "--vo=tct")       //  tct for terminal compatibility
				mpvArgs = append(mpvArgs, "--really-quiet") // Suppress initial logs
				mpvArgs = append(mpvArgs, "--loop=no")      // Ensure it doesn't loop
				fmt.Println("    " + colorYellow + "Controls: 'q' to quit, SPACE to pause/resume, ←→ to seek" + colorReset)
			} else { // Play in MPV (External)
				mpvArgs = append(mpvArgs, "--fs") // Fullscreen for external MPV
				mpvArgs = append(mpvArgs, "--really-quiet")
			}

			if quality != "" {
				f := qualityToFormat(quality)
				if f == "bestaudio" {
					mpvArgs = append(mpvArgs, "--no-video")
				}
				mpvArgs = append(mpvArgs, "--ytdl-format="+f)
			}

			mpvArgs = append(mpvArgs, videos[selected].URL)
			mpvCmd := exec.CommandContext(ctx, mpvPath, mpvArgs...)
			mpvCmd.Stdin = os.Stdin // Ensure mpv receives input for 'q'
			mpvCmd.Stdout = os.Stdout
			mpvCmd.Stderr = os.Stderr
			err = mpvCmd.Run()
			if err != nil {
				fmt.Printf("    \033[1;31mError playing video: %v\033[0m\n", err)
				if playbackChoice == 't' {
					fmt.Println("    " + colorYellow + "In-terminal video playback often has compatibility issues." + colorReset)
					fmt.Println("    " + colorYellow + "Try selecting 'm' for external mpv playback instead." + colorReset)
				} else {
					fmt.Println("    " + colorYellow + "Ensure mpv is installed, in your PATH, and yt-dlp is the lastest one." + colorReset)
				}
				fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)
				os.Stdin.Read(make([]byte, 1))
			} else {
				fmt.Println("    " + colorGreen + "Playback command executed successfully." + colorReset)
			}
			continue
		}
	}
}

func gophertubeDownloadsMode(ctx context.Context, cmd *cli.Command) {
	dlPath := expandPath(cmd.String(FlagDownloadsPath))
	files, err := os.ReadDir(dlPath)
	if err != nil || len(files) == 0 {
		fmt.Println("    " + colorRed + "No downloaded videos found." + colorReset)
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
		fmt.Println("    " + colorRed + "No downloaded videos found." + colorReset)
		time.Sleep(600 * time.Millisecond)
		return
	}
	fzfPreview := buildDownloadsPreview(dlPath)
	action := exec.CommandContext(ctx, "fzf", "--ansi", "--preview-window=wrap", "--prompt=Downloads: ", "--preview", fzfPreview)
	action.Stdin = strings.NewReader(strings.Join(videoFiles, "\n"))
	out, _ := action.Output()
	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return
	}
	filePath := filepath.Join(dlPath, selected)

	player := checkAvailablePlayer()
	if player == nil {
		fmt.Println("    " + colorRed + "No media player (mpv) found!" + colorReset)
		fmt.Println("    " + colorWhite + "Please install mpv to play videos." + colorReset)
		fmt.Println("    " + colorYellow + "Install mpv: sudo apt install mpv (Ubuntu) | brew install mpv (macOS)" + colorReset)
		fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)
		os.Stdin.Read(make([]byte, 1))
		return // Go back from downloads mode
	}

	fmt.Printf("    %sPlaying: %s%s\n", colorYellow, selected, colorReset)
	fmt.Println()
	fmt.Println("    " + barMagenta)
	fmt.Println()

	// New menu for playback mode
	fmt.Println("    " + barMagenta)
	fmt.Println("    " + colorYellow + "Play video in [t]erminal or [m]pv? " + colorReset)
	fmt.Println("    " + barMagenta)
	fmt.Print("    " + colorGreen + "> " + colorReset)

	oldState, err := readline.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	defer readline.Restore(int(os.Stdin.Fd()), oldState)

	var playbackChoice rune
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			playbackChoice = 'm' // Default to MPV on error
			break
		}
		if buf[0] == 't' || buf[0] == 'T' {
			playbackChoice = 't'
			break
		} else if buf[0] == 'm' || buf[0] == 'M' {
			playbackChoice = 'm'
			break
		} else if buf[0] == 27 { // ESC key
			playbackChoice = ' ' // Indicate escape
			break
		}
	}
	fmt.Println() // Newline after input

	if playbackChoice == ' ' { // User pressed ESC
		return
	}

	mpvPath := "mpv"
	var mpvArgs []string

	if playbackChoice == 't' {
		compat := checkTerminalCompatibility()
		if !compat.HasSixel && !compat.HasCaca {
			fmt.Println("    " + colorYellow + "Warning: In-terminal video playback may not be supported." + colorReset)
			fmt.Println("    " + colorYellow + "Continuing playback attempt in 3 seconds..." + colorReset)
			time.Sleep(3 * time.Second)
		}
		mpvArgs = append(mpvArgs, "--vo=tct")       // tct for terminal video compatibility
		mpvArgs = append(mpvArgs, "--really-quiet") // Suppress initial logs
		mpvArgs = append(mpvArgs, "--loop=no")      // Ensure it doesn't loop
	} else { // Play in MPV (External)
		mpvArgs = append(mpvArgs, "--fs") // Fullscreen for external MPV
		mpvArgs = append(mpvArgs, "--really-quiet")
	}

	mpvArgs = append(mpvArgs, filePath)
	mpvCmd := exec.CommandContext(ctx, mpvPath, mpvArgs...)
	mpvCmd.Stdin = os.Stdin // Ensure mpv receives input for 'q'
	mpvCmd.Stdout = os.Stdout
	mpvCmd.Stderr = os.Stderr
	err = mpvCmd.Run()
	if err != nil {
		fmt.Printf("    \033[1;31mError playing video: %v\033[0m\n", err)
		if playbackChoice == 't' {
			fmt.Println("    " + colorYellow + "In-terminal video playback often has compatibility issues." + colorReset)
			fmt.Println("    " + colorYellow + "Try selecting 'm' for external mpv playback instead." + colorReset)
		} else {
			fmt.Println("    " + colorYellow + "Ensure mpv is installed, in your PATH, and the video file is not corrupted." + colorReset)
		}
		fmt.Println("    " + colorWhite + "Press any key to return..." + colorReset)
		os.Stdin.Read(make([]byte, 1))
	} else {
		fmt.Println("    " + colorGreen + "Playback command executed successfully." + colorReset)
	}
}

// New function for download progress bar in the style of search progress
func drawDownloadProgressBar(percent float64, speed string, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filledWidth := int((percent / 100.0) * float64(width))
	emptyWidth := width - filledWidth

	barFilled := strings.Repeat("█", filledWidth)
	barEmpty := strings.Repeat("░", emptyWidth)

	// Add spinning animation
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := spinners[time.Now().UnixNano()/100000000%int64(len(spinners))]

	// Color codes for the bar
	bar := fmt.Sprintf("\033[1;36m%s\033[0;37m%s\033[0m", barFilled, barEmpty)

	// Format percentage
	percentStr := fmt.Sprintf("%5.1f%%", percent)

	// Combine spinner, bar, percentage and speed
	return fmt.Sprintf("%s %s %s %s", spinner, bar, percentStr, speed)
}
