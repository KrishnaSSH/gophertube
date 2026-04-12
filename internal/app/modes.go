package app

import (
	"fmt"
	"gophertube/internal/types"
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

func gophertubeYouTubeMode(cmd *cli.Command) bool {
	var lastQuery string
	var lastVideos []types.Video
	var lastCursor int
	for {
		var query string
		var videos []types.Video
		var selected int
		var back bool
		var exit bool
		var err error

		if lastQuery != "" && len(lastVideos) > 0 {
			query, videos, selected, back, exit, err = runSearchTeaWithState(cmd.Int(FlagSearchLimit), lastQuery, lastVideos, lastCursor)
		} else {
			query, videos, selected, back, exit, err = runSearchTea(cmd.Int(FlagSearchLimit))
		}
		if err != nil || exit {
			return exit
		}
		if back || selected < 0 {
			return false
		}
		lastQuery = query
		lastVideos = videos
		lastCursor = selected

		// Show Watch/Download/Audio menu
		menu := []string{"Watch", "Download", "Listen"}
		choice, back, exit, errAct := runMenuTea("Action", "Esc to back • Ctrl+C to exit", menu)
		if errAct != nil {
			return false
		}
		if exit {
			return true
		}
		if back || choice == "" {
			continue
		}

		if choice == "Download" {
			qualities := []string{"1080p", "720p", "480p", "360p", "Audio"}
			selectedQ, backQ, exitQ, errQ := runMenuTea("Quality", "Esc to back • Ctrl+C to exit", qualities)
			if errQ != nil {
				return false
			}
			if exitQ {
				return true
			}
			if backQ || selectedQ == "" {
				continue
			}

			// Map quality to yt-dlp format
			format := qualityToFormat(selectedQ)
			dlPath := expandPath(cmd.String(FlagDownloadsPath))
			os.MkdirAll(dlPath, 0755)
			// Sanitize filename
			filename := sanitizeFilename(videos[selected].Title)
			outputPath := fmt.Sprintf("%s/%s.%%(ext)s", dlPath, filename)
			fmt.Printf("%s%s\n", uiIndent(), textEmphasis.Render(fmt.Sprintf("Downloading '%s' as %s...", videos[selected].Title, selectedQ)))

			ytDlpArgs := []string{"-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}

			// override the default args with an audio only version.
			// Note: this downloads it as a .webm, then converts it to a .opus file.
			if format == "bestaudio" {
				ytDlpArgs = []string{"-x", "-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}
			} else {
				// For video+audio, ensure merge to mp4 when possible
				// Warn if ffmpeg is missing (yt-dlp needs it to merge)
				if !hasFFmpeg() {
					fmt.Println(uiIndent() + textWarn.Render("Warning: ffmpeg not found. Install ffmpeg to merge video+audio properly."))
					fmt.Println(uiIndent() + textMuted.Render("On Ubuntu: sudo apt install ffmpeg | macOS: brew install ffmpeg | Arch: pacman -S ffmpeg"))
				}
				ytDlpArgs = append([]string{"-f", format}, append([]string{"-o", outputPath, "--merge-output-format", "mp4", "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg"}, videos[selected].URL)...)
			}
			actionDl := exec.Command("yt-dlp", ytDlpArgs...)
			actionDl.Stdout = os.Stdout
			actionDl.Stderr = os.Stderr
			err := actionDl.Run()
			if err == nil {
				fmt.Printf("%s%s\n", uiIndent(), textEmphasis.Render("Download complete!"))
				fmt.Printf("%s%s\n", uiIndent(), textMuted.Render("Saved to: "+dlPath))
			} else {
				fmt.Printf("%s%s\n", uiIndent(), textError.Render("Download failed!"))
			}
			fmt.Println(uiIndent() + textMuted.Render("Press any key to return..."))
			os.Stdin.Read(make([]byte, 1))
			// After handling download, return to results
			continue
		}

		// New Audio playback logic
		if choice == "Listen" {
			player := checkAvailablePlayer()
			if player == nil {
				fmt.Println(uiIndent() + textError.Render("No media player found!"))
				fmt.Println(uiIndent() + textMuted.Render("Please install MPV to play audio."))
				fmt.Println(uiIndent() + textMuted.Render("Install MPV: sudo apt install mpv (Ubuntu) | brew install mpv (macOS)"))
				fmt.Println(uiIndent() + textMuted.Render("Press any key to return..."))
				os.Stdin.Read(make([]byte, 1))
				continue // Go back to the results
			}

			// Extract direct audio stream URL
			audioCmd := exec.Command("yt-dlp", "-f", "bestaudio[ext=m4a]/bestaudio", "-g", videos[selected].URL)
			streamURLBytes, err := audioCmd.Output()
			if err != nil {
				fmt.Println(uiIndent() + textError.Render("Failed to get direct audio URL."))
				fmt.Println(uiIndent() + textMuted.Render("Make sure yt-dlp is installed."))
				fmt.Println(uiIndent() + textMuted.Render("Press any key to return..."))
				os.Stdin.Read(make([]byte, 1))
				continue // Go back to the results
			}
			streamURL := strings.TrimSpace(string(streamURLBytes))

			args := []string{"--no-video", "--input-terminal=yes", "--no-terminal", "--msg-level=all=no", streamURL}
			exit, back, _ = runPlaybackTea(videos[selected].Title, videos[selected].Author, videos[selected].Duration, videos[selected].Published, "Playing Audio: ", args)
			if exit {
				return true
			}
			if back {
				continue
			}
			continue // Return to the results
		}

		// Watch as before
		quality := cmd.String(FlagQuality)
		var mpvArgs []string

		// Add the fullscreen flag for video playback
		mpvArgs = append(mpvArgs, "--fs", "--input-terminal=yes", "--no-terminal", "--msg-level=all=no")

		if quality != "" {
			f := qualityToFormat(quality)
			if f == "bestaudio" {
				mpvArgs = append(mpvArgs, "--no-video")
			}
			mpvArgs = append(mpvArgs, "--ytdl-format="+f)
		}

		mpvArgs = append(mpvArgs, videos[selected].URL)
		exit, back, _ = runPlaybackTea(videos[selected].Title, videos[selected].Author, videos[selected].Duration, videos[selected].Published, "Playing: ", mpvArgs)
		if exit {
			return true
		}
		if back {
			continue
		}
		continue
	}
}

func gophertubeDownloadsMode(cmd *cli.Command) bool {
	dlPath := expandPath(cmd.String(FlagDownloadsPath))
	for {
		files, err := os.ReadDir(dlPath)
		if err != nil || len(files) == 0 {
			fmt.Println(uiIndent() + textMuted.Render("No downloaded videos found."))
			time.Sleep(600 * time.Millisecond)
			return false
		}
		var videoFiles []string
		for _, f := range files {
			if !f.IsDir() && (strings.HasSuffix(f.Name(), ".mp4") || strings.HasSuffix(f.Name(), ".mkv") || strings.HasSuffix(f.Name(), ".webm") || strings.HasSuffix(f.Name(), ".avi") || strings.HasSuffix(f.Name(), ".m4a") || strings.HasSuffix(f.Name(), ".mp3") || strings.HasSuffix(f.Name(), ".opus")) {
				videoFiles = append(videoFiles, f.Name())
			}
		}
		if len(videoFiles) == 0 {
			fmt.Println(uiIndent() + textMuted.Render("No downloaded videos found."))
			time.Sleep(600 * time.Millisecond)
			return false
		}
		selected, back, exit, err := runMenuTea("Downloads", "Esc to back • Ctrl+C to exit", videoFiles)
		if err != nil || exit {
			return exit
		}
		if back || selected == "" {
			return false
		}
		filePath := filepath.Join(dlPath, selected)
		mpvArgs := []string{"--fs", "--input-terminal=yes", "--no-terminal", "--msg-level=all=no", filePath}
		exit, back, _ = runPlaybackTea(selected, "", "", "", "Playing: ", mpvArgs)
		if exit {
			return true
		}
		if back {
			continue
		}
	}
}

func gophertubeSettingsMode(cmd *cli.Command) bool {
	themes := ThemeNames()
	if len(themes) == 0 {
		fmt.Println(uiIndent() + textMuted.Render("No themes available."))
		time.Sleep(600 * time.Millisecond)
		return false
	}
	prompt := "Theme (" + CurrentThemeName() + ")"
	selected, back, exit, err := runMenuTea(prompt, "Esc to back • Ctrl+C to exit", themes)
	if err != nil || exit {
		return exit
	}
	if back || selected == "" {
		return false
	}
	if ApplyTheme(selected) {
		SaveConfig(cmd)
		fmt.Print("\r\033[2K    " + textEmphasis.Render("Theme set to "+selected))
		time.Sleep(600 * time.Millisecond)
		fmt.Print("\r\033[2K")
	}
	return false
}
