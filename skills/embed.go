// Package skills embeds md365 agent skill files in the binary.
package skills

import "embed"

//go:embed md365
var FS embed.FS
