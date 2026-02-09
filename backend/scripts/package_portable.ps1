$ErrorActionPreference = "Stop"

function Find-WailsRoot {
    param([string]$Start)
    $root = $Start
    for ($i = 0; $i -lt 8; $i++) {
        if (Test-Path (Join-Path $root "wails.json")) {
            return $root
        }
        $parent = Split-Path $root -Parent
        if ($parent -eq $root) {
            break
        }
        $root = $parent
    }
    return $null
}

function Resolve-Executable {
    param(
        [string]$BuildBin,
        [string]$Root
    )

    $wailsConfig = Get-Content -Raw (Join-Path $Root "wails.json") | ConvertFrom-Json
    $outputName = $wailsConfig.outputfilename
    if (-not $outputName) {
        $outputName = $wailsConfig.name
    }

    $exeName = "$outputName.exe"
    $exePath = Join-Path $BuildBin $exeName
    if (Test-Path $exePath) {
        return @{ Name = $exeName; Path = $exePath; OutputName = $outputName }
    }

    $exe = Get-ChildItem -Path $BuildBin -Filter *.exe |
        Where-Object { $_.Name -notmatch "installer" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if (-not $exe) {
        throw "app executable not found in $BuildBin"
    }

    return @{
        Name = $exe.Name
        Path = $exe.FullName
        OutputName = [System.IO.Path]::GetFileNameWithoutExtension($exe.Name)
    }
}

function Audit-PortableZip {
    param(
        [string]$ZipPath,
        [string]$ExpectedExeName
    )

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        $entries = @($zip.Entries | Where-Object { -not [string]::IsNullOrWhiteSpace($_.Name) })
        if ($entries.Count -ne 1) {
            $names = ($entries | ForEach-Object { $_.FullName }) -join ", "
            throw "portable zip审查失败：包含多余文件 ($names)"
        }
        if ($entries[0].FullName -ne $ExpectedExeName) {
            throw "portable zip审查失败：期望仅包含 $ExpectedExeName，实际为 $($entries[0].FullName)"
        }
    }
    finally {
        $zip.Dispose()
    }
}

function Audit-EmbeddedPayloadNoTests {
    param([string]$Root)

    $targets = @(
        (Join-Path $Root "python"),
        (Join-Path $Root "embedded_python_runtime")
    )

    $offenders = @()
    foreach ($target in $targets) {
        if (-not (Test-Path $target)) {
            continue
        }

        $testDirs = Get-ChildItem -Path $target -Recurse -Directory -Force |
            Where-Object { $_.Name -in @("test", "tests") } |
            ForEach-Object { $_.FullName }

        $testFiles = Get-ChildItem -Path $target -Recurse -File -Force |
            Where-Object { $_.Name -like "test_*.py" -or $_.Name -like "*_test.py" } |
            ForEach-Object { $_.FullName }

        $offenders += @($testDirs)
        $offenders += @($testFiles)
    }

    if ($offenders.Count -gt 0) {
        $preview = ($offenders | Select-Object -First 30) -join "`n"
        throw "打包审查失败：检测到测试文件/目录将被打包。请先清理：`n$preview"
    }
}

$root = Find-WailsRoot -Start $PSScriptRoot
if (-not $root) {
    $root = Find-WailsRoot -Start (Get-Location).Path
}
if (-not $root) {
    throw "wails.json not found"
}

Audit-EmbeddedPayloadNoTests -Root $root

$buildBin = Join-Path $root "build\bin"
if (-not (Test-Path $buildBin)) {
    throw "build/bin not found; run wails build first"
}

$exeInfo = Resolve-Executable -BuildBin $buildBin -Root $root
$exeName = $exeInfo.Name
$outputName = $exeInfo.OutputName

$zipPath = Join-Path $buildBin "$outputName-portable.zip"
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
}

Push-Location $buildBin
try {
    Compress-Archive -Path $exeName -DestinationPath $zipPath
}
finally {
    Pop-Location
}

Audit-PortableZip -ZipPath $zipPath -ExpectedExeName $exeName
Write-Host "Portable package created: $zipPath"
