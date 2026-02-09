//go:build windows

package main

import "embed"

//go:embed all:python all:embedded_python_runtime
var embeddedPythonFS embed.FS
