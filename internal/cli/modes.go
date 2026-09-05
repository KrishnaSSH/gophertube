package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/krishnassh/gophertube/internal/types"

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
	p = os.ExpandEnv(p)
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

// checkAvailablePlayer checks for MPV.
func checkAvailablePlayer() bool {
	_, err := exec.LookPath("mpv")
	return err == nil
}

func pause(msg string) {
	fmt.Println("  " + msg)
	fmt.Println("  " + S.Muted.Render("Press any key to return..."))
	os.Stdin.Read(make([]byte, 1))
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
			query, videos, selected, back, exit, err = runSearchTeaWithState(int(cmd.Int(FlagSearchLimit)), lastQuery, lastVideos, lastCursor)
		} else {
			query, videos, selected, back, exit, err = runSearchTea(int(cmd.Int(FlagSearchLimit)))
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
		choice, back, exit, errAct := runMenuTea("Action", menu)
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
			selectedQ, backQ, exitQ, errQ := runMenuTea("Quality", qualities)
			if errQ != nil {
				return false
			}
			if exitQ {
				return true
			}
			if backQ || selectedQ == "" {
				continue
			}

			format := qualityToFormat(selectedQ)
			dlPath := expandPath(cmd.String(FlagDownloadsPath))
			os.MkdirAll(dlPath, 0755)
			filename := sanitizeFilename(videos[selected].Title)
			outputPath := fmt.Sprintf("%s/%s.%%(ext)s", dlPath, filename)
			fmt.Println("  " + S.Accent.Render(fmt.Sprintf("Downloading %q as %s...", videos[selected].Title, selectedQ)))

			ytDlpArgs := []string{"-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}

			// override the default args with an audio only version.
			// Note: this downloads it as a .webm, then converts it to a .opus file.
			if format == "bestaudio" {
				ytDlpArgs = []string{"-x", "-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}
			} else {
				if !hasFFmpeg() {
					fmt.Println("  " + S.Warning.Render("Warning: ffmpeg not found. Install ffmpeg to merge video+audio properly."))
					fmt.Println("  " + S.Muted.Render("On Ubuntu: sudo apt install ffmpeg | macOS: brew install ffmpeg | Arch: pacman -S ffmpeg"))
				}
				ytDlpArgs = append([]string{"-f", format}, append([]string{"-o", outputPath, "--merge-output-format", "mp4", "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg"}, videos[selected].URL)...)
			}
			actionDl := exec.Command("yt-dlp", ytDlpArgs...)
			actionDl.Stdout = os.Stdout
			actionDl.Stderr = os.Stderr
			err := actionDl.Run()
			if err == nil {
				pause(S.Success.Render("Download complete!") + " " + S.Muted.Render("Saved to: "+dlPath))
			} else {
				pause(S.Danger.Render("Download failed!"))
			}
			continue
		}

		if choice == "Listen" {
			if !checkAvailablePlayer() {
				pause(S.Danger.Render("No media player found. Install mpv to play audio."))
				continue
			}

			audioCmd := exec.Command("yt-dlp", "-f", "bestaudio[ext=m4a]/bestaudio", "-g", videos[selected].URL)
			streamURLBytes, err := audioCmd.Output()
			if err != nil {
				pause(S.Danger.Render("Failed to get direct audio URL. Make sure yt-dlp is installed."))
				continue
			}
			streamURL := strings.TrimSpace(string(streamURLBytes))

			args := []string{"--no-video", "--input-terminal=yes", "--no-terminal", "--msg-level=all=no", streamURL}
			exit, back, _ = runPlaybackTea(videos[selected].Title, videos[selected].Author, videos[selected].Duration, videos[selected].Published, "Playing Audio: ", args)
			if exit {
				return true
			}
			continue
		}

		// Watch
		quality := cmd.String(FlagQuality)
		var mpvArgs []string

		if cmd.Bool(FlagFullscreen) {
			mpvArgs = append(mpvArgs, "--fs")
		}
		mpvArgs = append(mpvArgs, "--input-terminal=yes", "--no-terminal", "--msg-level=all=no")

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
		continue
	}
}

func gophertubeDownloadsMode(cmd *cli.Command) bool {
	fullscreen := cmd.Bool(FlagFullscreen)
	dlPath := expandPath(cmd.String(FlagDownloadsPath))
	for {
		files, err := os.ReadDir(dlPath)
		if err != nil || len(files) == 0 {
			fmt.Println("  " + S.Muted.Render("No downloaded videos found."))
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
			fmt.Println("  " + S.Muted.Render("No downloaded videos found."))
			time.Sleep(600 * time.Millisecond)
			return false
		}
		selected, back, exit, err := runMenuTea("Downloads", videoFiles)
		if err != nil || exit {
			return exit
		}
		if back || selected == "" {
			return false
		}
		filePath := filepath.Join(dlPath, selected)
		var mpvArgs []string
		if fullscreen {
			mpvArgs = append(mpvArgs, "--fs")
		}
		mpvArgs = append(mpvArgs, "--input-terminal=yes", "--no-terminal", "--msg-level=all=no", filePath)
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
	names := ThemeNames()
	if len(names) == 0 {
		fmt.Println("  " + S.Muted.Render("No themes available."))
		time.Sleep(600 * time.Millisecond)
		return false
	}
	prompt := "Theme (" + CurrentThemeName() + ")"
	selected, back, exit, err := runMenuTea(prompt, names)
	if err != nil || exit {
		return exit
	}
	if back || selected == "" {
		return false
	}
	if ApplyTheme(selected) {
		SaveConfig(cmd)
	}
	return false
}
