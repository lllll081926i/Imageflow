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

$root = Find-WailsRoot -Start $PSScriptRoot
if (-not $root) {
    $root = Find-WailsRoot -Start (Get-Location).Path
}
if (-not $root) {
    throw "wails.json not found"
}

function Find-PythonDir {
    param([string]$Start)
    $candidates = @(
        (Join-Path $Start "python"),
        (Join-Path $Start "..\\python"),
        (Join-Path $Start "..\\..\\python")
    )
    foreach ($candidate in $candidates) {
        $full = [System.IO.Path]::GetFullPath($candidate)
        if (Test-Path (Join-Path $full "converter.py")) {
            return $full
        }
    }
    return $null
}

function Trim-PythonRuntime {
    param([string]$RuntimeRoot)
    if (-not (Test-Path $RuntimeRoot)) {
        return
    }

    $scriptsDir = Join-Path $RuntimeRoot "Scripts"
    if (Test-Path $scriptsDir) {
        Remove-Item -Recurse -Force $scriptsDir
    }

    $removeAtRoot = @(
        ".keep",
        "LICENSE.txt",
        "python.cat"
    )
    foreach ($name in $removeAtRoot) {
        $p = Join-Path $RuntimeRoot $name
        if (Test-Path $p) {
            Remove-Item -Force $p -ErrorAction SilentlyContinue
        }
    }

    $sitePackages = Join-Path $RuntimeRoot "Lib\\site-packages"
    if (Test-Path $sitePackages) {
        $removeTop = @(
            "pip",
            "pip-*.dist-info",
            "wheel",
            "wheel-*.dist-info",
            "setuptools",
            "setuptools-*.dist-info",
            "pkg_resources",
            "__pycache__"
        )
        foreach ($pattern in $removeTop) {
            Get-ChildItem -Path $sitePackages -Force -Filter $pattern |
                Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
        }

        Get-ChildItem -Path $sitePackages -Force -Directory |
            Where-Object { $_.Name -like "*.dist-info" } |
            Remove-Item -Recurse -Force -ErrorAction SilentlyContinue

        Get-ChildItem -Path $sitePackages -Recurse -Force -Directory |
            Where-Object { $_.Name -in @("tests", "test", "__pycache__") } |
            Remove-Item -Recurse -Force -ErrorAction SilentlyContinue

        Get-ChildItem -Path $sitePackages -Recurse -Force -Directory |
            Where-Object { $_.Name -in @("include", "includes", "__pyinstaller") } |
            Remove-Item -Recurse -Force -ErrorAction SilentlyContinue

        Get-ChildItem -Path $sitePackages -Recurse -Force -File -Include *.pyi, *.pyx, *.pxd, *.h, *.c, *.cpp, *.lib |
            Remove-Item -Force -ErrorAction SilentlyContinue

        Get-ChildItem -Path $sitePackages -Recurse -Force -File -Include py.typed |
            Remove-Item -Force -ErrorAction SilentlyContinue
    }

    Get-ChildItem -Path $RuntimeRoot -Recurse -Force -Include *.pyc, *.pyo |
        Remove-Item -Force -ErrorAction SilentlyContinue

    Get-ChildItem -Path $RuntimeRoot -Force -Filter "python*._pth" -ErrorAction SilentlyContinue | ForEach-Object {
        $pthPath = $_.FullName
        $lines = Get-Content -LiteralPath $pthPath -ErrorAction SilentlyContinue
        if ($lines -and ($lines -notcontains "..\\python")) {
            $out = New-Object System.Collections.Generic.List[string]
            foreach ($l in $lines) {
                if ($l -eq "import site") {
                    $out.Add("..\\python")
                }
                $out.Add($l)
            }
            if ($out -notcontains "..\\python") {
                $out.Insert([Math]::Min(2, $out.Count), "..\\python")
            }
            Set-Content -LiteralPath $pthPath -Value $out -Encoding ASCII
        }
    }
}

$src = Find-PythonDir -Start $root
if (-not $src) {
    throw "python scripts dir not found near $root"
}

$buildBin = Join-Path $root "build\bin"
if (-not (Test-Path $buildBin)) {
    throw "build/bin not found; run wails build first"
}

$wailsConfig = Get-Content -Raw (Join-Path $root "wails.json") | ConvertFrom-Json
$outputName = $wailsConfig.outputfilename
if (-not $outputName) {
    $outputName = $wailsConfig.name
}

$exeName = "$outputName.exe"
$exePath = Join-Path $buildBin $exeName
if (-not (Test-Path $exePath)) {
    $exe = Get-ChildItem -Path $buildBin -Filter *.exe |
        Where-Object { $_.Name -notmatch "installer" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if (-not $exe) {
        throw "app executable not found in $buildBin"
    }
    $exePath = $exe.FullName
    $exeName = $exe.Name
    $outputName = [System.IO.Path]::GetFileNameWithoutExtension($exeName)
}

$runtimeSrc = Join-Path $root "embedded_python_runtime"
if (-not (Test-Path $runtimeSrc)) {
    throw "embedded_python_runtime not found at $runtimeSrc"
}
$runtimeDst = Join-Path $buildBin "python_runtime"
if (Test-Path $runtimeDst) {
    Remove-Item -Recurse -Force $runtimeDst
}
Copy-Item -Recurse -Force $runtimeSrc $runtimeDst
Trim-PythonRuntime -RuntimeRoot $runtimeDst

$scriptsDst = Join-Path $buildBin "python"
if (Test-Path $scriptsDst) {
    Remove-Item -Recurse -Force $scriptsDst
}
Copy-Item -Recurse -Force $src $scriptsDst
Get-ChildItem -Path $scriptsDst -Recurse -Force -Directory |
    Where-Object { $_.Name -eq "__pycache__" } |
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue

$zipPath = Join-Path $buildBin "$outputName-portable.zip"
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
}

Push-Location $buildBin
try {
    Compress-Archive -Path $exeName, "python_runtime", "python" -DestinationPath $zipPath
} finally {
    Pop-Location
}

Write-Host "Portable package created: $zipPath"
