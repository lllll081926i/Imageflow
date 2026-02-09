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

function Assert-NoTestsInRuntime {
    param([string]$Target)

    if (-not (Test-Path $Target)) {
        return
    }

    $testDirs = Get-ChildItem -Path $Target -Recurse -Directory -Force |
        Where-Object { $_.Name -in @("test", "tests") } |
        ForEach-Object { $_.FullName }

    $testFiles = Get-ChildItem -Path $Target -Recurse -File -Force |
        Where-Object {
            $_.Name -like "test_*.py" -or
            $_.Name -like "*_test.py"
        } |
        ForEach-Object { $_.FullName }

    $offenders = @()
    if ($testDirs) {
        $offenders += $testDirs
    }
    if ($testFiles) {
        $offenders += $testFiles
    }
    if ($offenders.Count -gt 0) {
        $preview = ($offenders | Select-Object -First 30) -join "`n"
        throw "package audit failed: test files found in runtime payload:`n$preview"
    }
}

function Copy-RuntimePayload {
    param(
        [string]$Root,
        [string]$TargetRuntimeDir
    )

    $src = Join-Path $Root "embedded_python_runtime"
    if (-not (Test-Path $src)) {
        throw "embedded runtime not found: $src"
    }

    Assert-NoTestsInRuntime -Target $src

    if (Test-Path $TargetRuntimeDir) {
        Remove-Item -Recurse -Force $TargetRuntimeDir
    }

    New-Item -ItemType Directory -Path $TargetRuntimeDir -Force | Out-Null
    Copy-Item -Path (Join-Path $src "*") -Destination $TargetRuntimeDir -Recurse -Force
}

function Audit-PortableZip {
    param(
        [string]$ZipPath,
        [string]$ExpectedExeName,
        [string]$ExpectedRuntimeRoot = "runtime"
    )

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        $entries = @($zip.Entries | Where-Object { -not [string]::IsNullOrWhiteSpace($_.Name) })
        if ($entries.Count -lt 2) {
            $names = ($entries | ForEach-Object { $_.FullName }) -join ", "
            throw "portable zip audit failed: expected exe + runtime, got: $names"
        }

        $rootEntries = @($entries | ForEach-Object {
            $name = $_.FullName.Replace('\\', '/')
            if ($name -match '/') { ($name -split '/')[0] } else { $name }
        } | Sort-Object -Unique)

        $expectedRoots = @($ExpectedExeName, $ExpectedRuntimeRoot)
        $unexpectedRoots = @($rootEntries | Where-Object { $_ -notin $expectedRoots })
        if ($unexpectedRoots.Count -gt 0) {
            throw "portable zip audit failed: unexpected root entries: $($unexpectedRoots -join ', ')"
        }

        $exeEntry = @($entries | Where-Object { $_.FullName.Replace('\\', '/') -eq $ExpectedExeName })
        if ($exeEntry.Count -ne 1) {
            throw "portable zip audit failed: missing root exe $ExpectedExeName"
        }

        $runtimeEntries = @($entries | Where-Object { $_.FullName.Replace('\\', '/') -like "$ExpectedRuntimeRoot/*" })
        if ($runtimeEntries.Count -eq 0) {
            throw "portable zip audit failed: missing runtime payload"
        }
    }
    finally {
        $zip.Dispose()
    }
}

$root = Find-WailsRoot -Start $PSScriptRoot
if (-not $root) {
    $root = Find-WailsRoot -Start (Get-Location).Path
}
if (-not $root) {
    throw "wails.json not found"
}

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

$runtimeDir = Join-Path $buildBin "runtime"
Copy-RuntimePayload -Root $root -TargetRuntimeDir $runtimeDir

Push-Location $buildBin
try {
    Compress-Archive -Path @($exeName, "runtime") -DestinationPath $zipPath
}
finally {
    Pop-Location
}

Audit-PortableZip -ZipPath $zipPath -ExpectedExeName $exeName -ExpectedRuntimeRoot "runtime"
Write-Host "Portable package created: $zipPath"
