package cli

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

var NAME = "Gophertube"

//go:embed VERSION
var VERSION string

//go:embed DESCRIPTION
var DESCRIPTION string

//go:embed USAGE
var USAGE string

func New() cli.Command {
	return cli.Command{
		Name: NAME,
		Usage: USAGE,
		Authors: []any {
			"KrishnaSSH <krishna.pytech@gmail.com>",
		},
		Version: VERSION,
		Description: DESCRIPTION,
		Flags: Flags(),
		Action: Action,
	}

}

// Action is the equivalent of main except that all flags/configs
// have already been parsed and sanitized by the cli package.
func Action(ctx context.Context, cmd *cli.Command) error {
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
			continue
		}
	}
}
