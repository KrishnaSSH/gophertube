package app

import (
	"errors"
	"os"

	altsrc "github.com/urfave/cli-altsrc/v3"
	toml "github.com/urfave/cli-altsrc/v3/toml"
	"github.com/urfave/cli/v3"
)

// Flag names are constant since they are also used as keys to query the data.
// This ensures a single source of truth.
const (
	FlagQuality       = "quality"
	FlagSearchLimit   = "search-limit"
	FlagConfig        = "config"
	FlagDownloadsPath = "downloads-path"

	defaultConfigPath    = "$HOME/.config/gophertube/gophertube.toml"
	defaultDownloadsPath = "$HOME/Videos/GopherTube"
)

var (
	errQualityFormat = errors.New("invalid format for quality provided")
)

func Flags() []cli.Flag {
	var confDir string

	// --help and -version flags are free, no need to set them up :)
	return []cli.Flag{
		&cli.StringFlag{
			Name:        FlagConfig,
			Aliases:     []string{"c"},
			Sources:     cli.EnvVars("GOPHERTUBE_CONFIG"),
			Value:       os.ExpandEnv(defaultConfigPath),
			DefaultText: defaultConfigPath, // otherwise `--help` prints it expanded
			Destination: &confDir,
			TakesFile:   true,
		},
		&cli.StringFlag{
			Name:      FlagDownloadsPath,
			Aliases:   []string{"d"},
			TakesFile: true,
			Sources: cli.NewValueSourceChain(
				toml.TOML("downloads_path", altsrc.NewStringPtrSourcer(&confDir)),
			),
			Value:       os.ExpandEnv(defaultDownloadsPath),
			DefaultText: defaultDownloadsPath, // otherwise `--help` prints it expanded
		},
		&cli.IntFlag{
			Name:    FlagSearchLimit,
			Aliases: []string{"l"},
			Sources: cli.NewValueSourceChain(
				toml.TOML("search_limit", altsrc.NewStringPtrSourcer(&confDir)),
			),
			Value: 30,
		},
		&cli.StringFlag{
			Name:    FlagQuality,
			Aliases: []string{"q"},
			Sources: cli.NewValueSourceChain(
				toml.TOML("quality", altsrc.NewStringPtrSourcer(&confDir)),
			),
			Value:     "720p",
			Validator: IsValidQualityFmt,
		},
	}
}

// Ensure argument format follows the "<int>p" pattern.
// Ex: 720p, 1080p, etc...
func IsValidQualityFmt(s string) error {
	// Check if string ends with 'p'
	if len(s) < 2 || s[len(s)-1] != 'p' {
		return errQualityFormat
	}

	// Check if everything before 'p' is a valid integer
	numPart := s[:len(s)-1]
	for _, char := range numPart {
		if char < '0' || char > '9' {
			return errQualityFormat
		}
	}

	return nil
}
