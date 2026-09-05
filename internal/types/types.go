package types

// Video represents a YouTube video with all its metadata
type Video struct {
	Title         string
	Author        string
	Duration      string
	Views         string
	URL           string
	Thumbnail     string
	ThumbnailPath string // local path for preview
	Description   string
	Published     string // relative published/upload date
}
