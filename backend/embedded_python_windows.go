//go:build windows

package main

import "embed"

// 仅嵌入运行所需 Python 脚本，运行时依赖通过安装包/绿色包落地到 runtime 目录。
//go:embed python/*.py python/requirements.txt
var embeddedPythonFS embed.FS
