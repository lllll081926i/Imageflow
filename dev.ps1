#Requires -Version 5.1
<#
.SYNOPSIS
    一键构建并启动 ImageFlow 开发版。

.DESCRIPTION
    三步：结束正在运行的 ImageFlow 进程（单实例锁会挡住新进程）→ 编译
    build\ImageFlow-dev.exe → 启动窗口并记录 pid。

    默认打开窗口。带 -Capture 时改为无人值守截图：画完第一帧、存盘、自动退出，
    这条路径不抢单实例锁。

.PARAMETER Open
    启动时预载的文件或目录，可给多个。等价于 ImageFlow-dev.exe -open a,b。

.PARAMETER Tool
    启动时停在哪一页：0 格式转换、1 尺寸调整、2 图片裁剪、3 宫格切图、
    4 图片布局、5 长图拼接、6 批量水印、7 色彩提取、8 图标生成、9 转 PDF、
    10 GIF 工具、11 富文本制图。省略则停在默认页。

.PARAMETER Capture
    截图模式：把首帧存到该路径后退出。用于验收与 CI。

.PARAMETER Hidden
    只在托盘启动，不显示窗口。

.PARAMETER SkipBuild
    跳过编译，直接启动已有的 build\ImageFlow-dev.exe。

.PARAMETER KeepRunning
    不结束已有进程。修图到一半想再开一个时用，但单实例锁会让新进程立刻退出。

.EXAMPLE
    .\dev.ps1
    结束旧进程、编译、打开窗口。

.EXAMPLE
    .\dev.ps1 -Tool 6 -Open .\photos
    停在水印页，并预载 photos 目录。

.EXAMPLE
    .\dev.ps1 -Tool 3 -Capture "$env:TEMP\shot.png"
    无人值守截图，画完自动退出。

.EXAMPLE
    .\dev.ps1 -SkipBuild
    不重新编译，只重启窗口。
#>
param(
    [string[]]$Open,
    [int]$Tool = -1,
    [string]$Capture,
    [switch]$Hidden,
    [switch]$SkipBuild,
    [switch]$KeepRunning
)

$ErrorActionPreference = 'Stop'

# ── 定位项目根：脚本放在 imageflow-go 根目录 ─────────────────────────────
$root = $PSScriptRoot
$exe = Join-Path $root 'build\ImageFlow-dev.exe'
$pidFile = Join-Path $root 'build\ImageFlow-dev.pid'

function Write-Step([string]$text) {
    Write-Host "`n▸ $text" -ForegroundColor Cyan
}

function Write-Ok([string]$text) {
    Write-Host "  ✓ $text" -ForegroundColor Green
}

function Write-Warn([string]$text) {
    Write-Host "  ! $text" -ForegroundColor Yellow
}

# Fail stops the script with one readable line. A thrown exception prints a stack
# trace and the offending line, which buries the message the user needs.
function Fail([string]$text) {
    Write-Host "  ✗ $text" -ForegroundColor Red
    Write-Host ''
    exit 1
}

# ── 前置检查 ────────────────────────────────────────────────────────────
if (-not (Test-Path (Join-Path $root 'go.mod'))) {
    Fail "没找到 go.mod，脚本必须放在 imageflow-go 根目录。当前：$root"
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Fail '找不到 go 命令，请先安装 Go 并加入 PATH。'
}

# 依赖走国内镜像；已有 GOPROXY 时不覆盖，尊重调用方的配置。
if (-not $env:GOPROXY) {
    $env:GOPROXY = 'https://goproxy.cn,direct'
}

$capturing = -not [string]::IsNullOrWhiteSpace($Capture)

# ── 1. 结束正在运行的实例 ───────────────────────────────────────────────
# 截图模式不抢单实例锁，不需要先杀进程。
if (-not $KeepRunning -and -not $capturing) {
    Write-Step '结束正在运行的实例'
    $running = Get-Process -Name 'ImageFlow*' -ErrorAction SilentlyContinue
    if ($running) {
        foreach ($proc in $running) {
            $path = ''
            try { $path = $proc.Path } catch { $path = '(无权限读取路径)' }
            try {
                Stop-Process -Id $proc.Id -Force -ErrorAction Stop
                Write-Ok "已结束 pid=$($proc.Id)  $path"
            } catch {
                Write-Warn "无法结束 pid=$($proc.Id)：$($_.Exception.Message)"
            }
        }
        # 等句柄真正释放，否则单实例锁可能还没放开。
        Start-Sleep -Milliseconds 400
    } else {
        Write-Ok '没有正在运行的实例'
    }
    if (Test-Path $pidFile) { Remove-Item $pidFile -Force -ErrorAction SilentlyContinue }
}

# ── 2. 编译 ─────────────────────────────────────────────────────────────
if ($SkipBuild) {
    Write-Step '跳过编译'
    if (-not (Test-Path $exe)) {
        Fail "找不到 $exe，不能跳过编译。去掉 -SkipBuild 重新运行。"
    }
    Write-Ok "使用已有文件 $(Get-Item $exe | Select-Object -ExpandProperty Length | ForEach-Object { '{0:N1} MB' -f ($_ / 1MB) })"
} else {
    Write-Step '编译开发版'
    $buildDir = Join-Path $root 'build'
    if (-not (Test-Path $buildDir)) {
        New-Item -ItemType Directory -Path $buildDir -Force | Out-Null
    }

    Push-Location $root
    try {
        # 输出重定向到 stderr 变量，避免编译警告被当成失败；退出码才是判据。
        $buildOutput = & go build -o $exe . 2>&1
        $code = $LASTEXITCODE
    } finally {
        Pop-Location
    }

    if ($buildOutput) {
        $buildOutput | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
    }
    if ($code -ne 0) {
        Fail "编译失败（退出码 $code），窗口未启动。"
    }
    $size = '{0:N1} MB' -f ((Get-Item $exe).Length / 1MB)
    Write-Ok "编译完成  $size"
}

# ── 3. 启动 ─────────────────────────────────────────────────────────────
Write-Step '启动'

# 参数按 ImageFlow-dev.exe 的 flag 逐个拼装，避免空值被当成位置参数。
$argList = @()
if ($Open) {
    $argList += '-open'
    $argList += ($Open -join ',')
}
if ($Tool -ge 0) {
    $argList += '-tool'
    $argList += "$Tool"
}
if ($capturing) {
    $capturePath = [System.IO.Path]::GetFullPath($Capture)
    $captureDir = Split-Path -Parent $capturePath
    if ($captureDir -and -not (Test-Path $captureDir)) {
        New-Item -ItemType Directory -Path $captureDir -Force | Out-Null
    }
    $argList += '-capture'
    $argList += $capturePath
}
if ($Hidden) {
    $argList += '-hidden'
}

if ($capturing) {
    # 截图模式：前台等待退出，再把结果报出来。
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $proc = Start-Process -FilePath $exe -ArgumentList $argList -PassThru -Wait -NoNewWindow
    $sw.Stop()
    $capturePath = $argList[-1]
    if (Test-Path $capturePath) {
        $kb = '{0:N0} KB' -f ((Get-Item $capturePath).Length / 1KB)
        Write-Ok "截图已保存  $kb  （耗时 $([math]::Round($sw.Elapsed.TotalSeconds, 1)) s）"
        Write-Host "    $capturePath" -ForegroundColor DarkGray
    } else {
        Fail "截图模式退出码 $($proc.ExitCode)，但没有生成文件：$capturePath"
    }
    Write-Host ''
    exit 0
}

$proc = Start-Process -FilePath $exe -ArgumentList $argList -PassThru
$proc.Id | Set-Content -Path $pidFile -Encoding ASCII

# 等窗口真正出现再报成功，否则 Start-Process 返回时窗口可能还在初始化。
$deadline = (Get-Date).AddSeconds(15)
$ready = $null
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 300
    $candidate = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
    if (-not $candidate) {
        break
    }
    if ($candidate.MainWindowTitle) {
        $ready = $candidate
        break
    }
}

if (-not $ready) {
    if (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue) {
        Write-Warn "进程 pid=$($proc.Id) 在运行，但 15 s 内没有出现窗口标题。"
        Write-Warn '可能停在托盘（-Hidden），或被其他实例挡住。'
    } else {
        Fail "进程启动后立刻退出。常见原因：另一个 ImageFlow 实例占着单实例锁（去掉 -KeepRunning 重试）。"
    }
} else {
    Write-Ok "窗口已就绪  pid=$($ready.Id)  标题='$($ready.MainWindowTitle)'"
}

Write-Host ''
Write-Host "  停止：Stop-Process -Id $($proc.Id)" -ForegroundColor DarkGray
Write-Host "  重编并重启：.\dev.ps1" -ForegroundColor DarkGray
Write-Host ''
