package app

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
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
		choice, exit, err := runMainMenuTea()
		if err != nil {
			fmt.Fprintln(os.Stderr, "menu error:", err)
			return nil
		}
		if exit {
			return nil
		}

		switch choice {
		case "Search YouTube":
			gophertubeYouTubeMode(cmd)
		case "Search Downloads":
			gophertubeDownloadsMode(cmd)
		case "Settings":
			gophertubeSettingsMode(cmd)
		default:
			// Unknown/empty selection: continue loop and ask again
			continue
		}
	}
}
