package main

import "embed"

// Python runtime/scripts are shipped alongside the executable.
var embeddedPythonFS embed.FS
