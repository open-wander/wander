package uihtmx

import (
	"embed"
)

// StaticFS contains all static assets embedded at build time.
// This includes CSS, JavaScript, and icon files.
// Note: We use all:static to include all files, including dotfiles,
// but this will skip empty directories like icons/.
//
//go:embed all:static
var StaticFS embed.FS
