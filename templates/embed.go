// Package templates exposes the starter files embedded in the application binary.
package templates

import "embed"

// Files contains the canonical project starter templates.
//
//go:embed *.yaml story_project.gitignore
var Files embed.FS
