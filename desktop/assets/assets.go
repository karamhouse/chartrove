package assets

import "embed"

// Frontend is populated by desktop/build.sh from the current fork's build.
//
//go:embed all:frontend
var Frontend embed.FS
