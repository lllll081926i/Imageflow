//go:build windows

package main

import "embed"

// 仅嵌入运行必需 Python 脚本；运行时依赖由安装包和便携包落地到 runtime 目录。
//go:embed python/*.py python/requirements.txt
var embeddedPythonFS embed.FS

