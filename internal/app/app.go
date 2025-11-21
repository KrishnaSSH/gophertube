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
func Action(cliCtx context.Context, cmd *cli.Command) error {
	// Create a cancellable context for the application's lifecycle
	appCtx, cancel := context.WithCancel(cliCtx)
	defer cancel() // Ensure cancel is called if function exits normally

	// Handle Ctrl+C gracefully by canceling the context
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		// Newline for clean display after ^C
		fmt.Println()
		// Only print a message if the appCtx hasn't been cancelled yet
		select {
		case <-appCtx.Done():
			// Context already cancelled, likely exiting. Do nothing further.
		default:
			fmt.Println("\033[1;33mReturning to previous menu... (Press Ctrl+C again to exit)\033[0m")
			cancel() // Cancel the context to signal return/exit
		}
	}()

	for {
		// Check if context was cancelled (e.g., by Ctrl+C)
		select {
		case <-appCtx.Done():
			fmt.Println("\033[1;33mExiting GopherTube.\033[0m") // Final exit message
			return nil // Exit the application
		default:
			// Continue
		}

		mainMenu := []string{"Search YouTube", "Search Downloads"}

		// Check if fzf is installed
		path, err := exec.LookPath("fzf")
		if err != nil {
			fmt.Fprintln(os.Stderr, "fzf not found. Please install fzf and ensure it is on PATH.")
			return nil
		}
		var choice string
		// Pass appCtx to fzf command to allow cancellation
		action := exec.CommandContext(appCtx, path, "--prompt=Select mode: ")
		action.Stdin = strings.NewReader(strings.Join(mainMenu, "\n"))
		out, err := action.Output()

		if err != nil {
			// If fzf was cancelled by context (Ctrl+C), or if user pressed ESC from main menu
			if appCtx.Err() != nil {
				// Context was cancelled (e.g., by Ctrl+C). The signal handler already printed a message.
				// The select at the top of the loop will catch appCtx.Done() and exit.
				continue // Continue to let the outer select handle the exit
			}
			// If fzf error is due to ESC or other fzf-internal cancellation (not Ctrl+C context cancellation),
			// and we are in the main menu, it means the user wants to exit.
			fmt.Println("\033[1;33mExiting GopherTube.\033[0m")
			return nil
		}
		choice = strings.TrimSpace(string(out))
		if choice == "" {
			// Empty selection (e.g., ESC): if in main menu, exit app
			fmt.Println("\033[1;33mExiting GopherTube.\033[0m")
			return nil
		}

		switch choice {
		case "Search YouTube":
			// Pass appCtx to sub-modes
			gophertubeYouTubeMode(appCtx, cmd)
		case "Search Downloads":
			// Pass appCtx to sub-modes
			gophertubeDownloadsMode(appCtx, cmd)
		default:
			// Unknown/empty selection: continue loop and ask again
			continue
		}
	}
	return nil
}