$ErrorActionPreference = 'Stop'
if ($env:GITHUB_ACTIONS -ne 'true' -or -not $env:RUNNER_TEMP) {
    throw '安装验收只能在 GitHub Actions 临时机器运行。'
}
$root = [IO.Path]::GetFullPath((Join-Path $env:RUNNER_TEMP 'imageflow-acceptance'))
$runnerRoot = [IO.Path]::GetFullPath($env:RUNNER_TEMP).TrimEnd('\') + '\'
if (-not $root.StartsWith($runnerRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw '验收目录超出临时工作区。'
}
$evidence = Join-Path $root 'evidence'
New-Item -ItemType Directory -Path $evidence -Force | Out-Null
$config = Get-Content mygo.json -Raw | ConvertFrom-Json
$stage = Join-Path (Get-Location) 'build/windows-amd64'
$installer = Join-Path $stage ('imageflow-setup-' + $config.version + '-amd64.exe')
$expectedHash = (Get-FileHash (Join-Path $stage 'ImageFlow.exe') -Algorithm SHA256).Hash
$registry = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\' + $config.identifier
if (Test-Path $registry) {
    throw '临时机器已有同名安装，拒绝覆盖。'
}
$install = Join-Path $root 'installed'
$fixture = Join-Path (Get-Location) 'docs/Go_Image_Toolbox_Full_Delivery_v2.0/assets/fixtures/screenshot_text_1600x900.png'

function Invoke-Installer([string]$File, [string[]]$Arguments) {
    $process = Start-Process -FilePath $File -ArgumentList $Arguments -WindowStyle Hidden -PassThru
    if (-not $process.WaitForExit(120000) -or $process.ExitCode -ne 0) {
        throw "安装程序失败：$File，退出码 $($process.ExitCode)"
    }
}

function Assert-Installed {
    $exe = Join-Path $install 'ImageFlow.exe'
    if (-not (Test-Path $exe) -or (Get-FileHash $exe -Algorithm SHA256).Hash -ne $expectedHash) {
        throw '安装后的程序与本轮构建不一致。'
    }
    $entry = Get-ItemProperty $registry
    if ($entry.DisplayVersion -ne $config.version -or $entry.InstallLocation.TrimEnd('\') -ne $install.TrimEnd('\')) {
        throw '安装版本或安装目录登记不正确。'
    }
}

function Invoke-App([string]$Directory, [string]$Name) {
    $env:IMAGEFLOW_SETTINGS_FILE = Join-Path $root ($Name + '-settings.json')
    $capture = Join-Path $evidence ($Name + '.png')
    $log = Join-Path $evidence ($Name + '.log')
    $process = Start-Process -FilePath (Join-Path $Directory 'ImageFlow.exe') -ArgumentList @('-tool', '14', '-open', $fixture, '-capture', $capture) -WindowStyle Hidden -RedirectStandardError $log -PassThru
    if (-not $process.WaitForExit(60000) -or $process.ExitCode -ne 0 -or -not (Test-Path $capture)) {
        throw "原生启动验收失败：$Name"
    }
    if ((Get-Item $capture).Length -lt 10000 -or -not (Select-String -LiteralPath $log -Pattern '输入 1 张' -Quiet)) {
        throw "输入导入或截图验收失败：$Name"
    }
}

function Uninstall-App {
    Invoke-Installer (Join-Path $install 'Uninstall.exe') @('/S')
    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    while ((Test-Path $install) -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 200
    }
    if ((Test-Path $install) -or (Test-Path $registry)) {
        throw '卸载后仍有安装目录或卸载登记。'
    }
}

Invoke-Installer $installer @('/S', ('/D=' + $install))
Assert-Installed
Invoke-App $install 'fresh-install'
Uninstall-App

gh release download v3.1.0 --repo $env:GITHUB_REPOSITORY --pattern 'imageflow-setup-3.1.0-amd64.exe' --dir $root
if ($LASTEXITCODE -ne 0) { throw '下载旧版安装包失败。' }
Invoke-Installer (Join-Path $root 'imageflow-setup-3.1.0-amd64.exe') @('/S', ('/D=' + $install))
$models = Join-Path $install 'models'
New-Item -ItemType Directory -Path $models -Force | Out-Null
$marker = Join-Path $models 'upgrade-preservation.txt'
[IO.File]::WriteAllText($marker, '升级不得删除既有模型目录')
Invoke-Installer $installer @('/S')
Assert-Installed
if (-not (Test-Path $marker) -or [IO.File]::ReadAllText($marker) -ne '升级不得删除既有模型目录') {
    throw '升级破坏了既有模型目录。'
}
Invoke-App $install 'upgrade-install'
Uninstall-App

$portable = Join-Path $root 'portable'
New-Item -ItemType Directory -Path $portable -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $stage 'ImageFlow.exe') -Destination $portable
Copy-Item -LiteralPath (Join-Path $stage 'icon.png') -Destination $portable
Invoke-App $portable 'portable'
$moved = Join-Path $root 'portable-moved'
if (-not ([IO.Path]::GetFullPath($portable)).StartsWith($runnerRoot, [StringComparison]::OrdinalIgnoreCase) -or -not ([IO.Path]::GetFullPath($moved)).StartsWith($runnerRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw '便携移动路径超出临时工作区。'
}
Move-Item -LiteralPath $portable -Destination $moved
Invoke-App $moved 'portable-moved'
[IO.File]::WriteAllText((Join-Path $evidence 'result.txt'), '通过：全新安装、3.1.0 升级、模型目录保留、卸载、便携启动、移动后启动。')
Write-Output 'Windows 安装、升级、卸载与便携启动验收通过。'
