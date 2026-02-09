//go:build windows

package main

import "embed"

//go:embed all:embedded_python all:embedded_python_runtime
var embeddedPythonFS embed.FS

