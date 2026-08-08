package cli

import (
	_ "embed"
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
	}
	
}
