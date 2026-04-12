package app

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/urfave/cli/v3"
	"os"
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
	// Apply theme from config/flags
	ApplyTheme(cmd.String(FlagTheme))

	defer ShowCursor()

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
			if gophertubeYouTubeMode(cmd) {
				return nil
			}
		case "Search Downloads":
			if gophertubeDownloadsMode(cmd) {
				return nil
			}
		case "Settings":
			if gophertubeSettingsMode(cmd) {
				return nil
			}
		default:
			// Unknown/empty selection: continue loop and ask again
			continue
		}
	}
}
