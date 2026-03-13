//go:build windows

package main

import "embed"

// Windows 单文件构建同时嵌入 Python 脚本和完整运行时，启动时再解压到本地目录。
//
//go:embed python/*.py python/requirements.txt embedded_python_runtime
var embeddedPythonFS embed.FS
