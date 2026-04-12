package app

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/urfave/cli/v3"
)

var version = "dev"

//go:embed description.txt
var Desc string

func New() cli.Command {
	return cli.Command{
		Name: "GopherTube",
		Authors: []any{
			"KrishnaSSH <krishna.pytech@gmail.com>",
		},
		Usage:       "Terminal YouTube Search & Play",
		Description: Desc,
		Flags:       Flags(),
		Version:     version,
		Action:      Action,
	}
}

// Action is the equivalent of the main except that all flags/configs
// have already been parsed and sanitized.
func Action(ctx context.Context, cmd *cli.Command) error {
	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println()
		fmt.Println(textMuted.Render("Exiting..."))
		os.Exit(0)
	}()

	for {
		mainMenu := []string{"Search YouTube", "Search Downloads"}

		// Check if fzf is installed
		path, err := exec.LookPath("fzf")
		if err != nil {
			fmt.Fprintln(os.Stderr, "fzf not found. Please install fzf and ensure it is on PATH.")
			return nil
		}
		var choice string
		action := exec.CommandContext(ctx, path, "--prompt=Select mode: ")
		action.Stdin = strings.NewReader(strings.Join(mainMenu, "\n"))
		out, err := action.Output()
		if err != nil {
			// ESC/cancel or fzf error: exit app
			return nil
		}
		choice = strings.TrimSpace(string(out))
		if choice == "" {
			// Empty selection (e.g., ESC): exit app
			return nil
		}

		switch choice {
		case "Search YouTube":
			gophertubeYouTubeMode(cmd)
		case "Search Downloads":
			gophertubeDownloadsMode(cmd)
		default:
			// Unknown/empty selection: continue loop and ask again
			continue
		}
	}
}
