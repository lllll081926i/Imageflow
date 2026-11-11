# OxiPNG 依赖

用途：PNG 无损压缩。采用官方版本 9.1.5，优化级别 2、单线程。
不启用 `--alpha` 或 `--scale16`，保留全透明像素的 RGB 和不可无损缩减的 16 位精度。
直接处理源 PNG 时保留 ICC、EXIF 及其他附加块。

- 上游：https://github.com/oxipng/oxipng
- 发布包：https://github.com/oxipng/oxipng/releases/download/v9.1.5/oxipng-9.1.5-x86_64-pc-windows-msvc.zip
- 内置文件：`assets/oxipng.exe`，1,127,424 字节。
- SHA-256：`421ce9013342db4a2d9ee6c595adc673d4fe0c69bb0b5c6b9a8794c83226093d`。
- 许可证：MIT，随附原文 `assets/LICENSE.txt`。

Windows x64 使用 `go:embed` 打包程序，按版本和哈希释放到用户缓存目录。
释放采用原子写入，缓存已有文件先校验哈希。子进程隐藏窗口，参数不经过 shell。
每个任务使用单线程；OxiPNG 软超时 60 秒，进程硬超时 120 秒。

其他平台从 PATH 查找系统安装的 `oxipng`。缺少依赖时返回明确错误。
可通过 `cargo install oxipng --version 9.1.5 --locked` 安装。

升级时下载官方发布包，替换内置程序与许可证，并同步更新代码中的版本和 SHA-256。
执行 PNG 无损回归测试、界面批处理测试与发布构建。
