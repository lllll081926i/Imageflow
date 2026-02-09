//go:build !windows

package main

import "embed"

// Non-Windows builds keep an empty embedded FS and rely on external runtime setup.
var embeddedPythonFS embed.FS
