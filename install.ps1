# Installs the malveon binary for Windows: downloads the release exe,
# places it in %LOCALAPPDATA%\malveon, and adds that folder to the
# user's PATH (no admin rights needed). Run via:
#   irm https://raw.githubusercontent.com/LadsonDavid/beta-test/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "LadsonDavid/beta-test"
$installDir = "$env:LOCALAPPDATA\malveon"
$exePath = "$installDir\malveon.exe"
$url = "https://github.com/$repo/releases/latest/download/malveon-windows-amd64.exe"

New-Item -ItemType Directory -Force -Path $installDir | Out-Null

Write-Host "Downloading malveon..."
try {
    Invoke-WebRequest -Uri $url -OutFile $exePath
} catch {
    Write-Host "malveon: download failed - check https://github.com/$repo/releases for available builds" -ForegroundColor Red
    exit 1
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "Added $installDir to your PATH."
    Write-Host "Open a NEW terminal window for that to take effect."
} else {
    Write-Host "Already on PATH."
}

Write-Host "Installed to $exePath"
Write-Host "Windows may warn 'unrecognized publisher' the first time you run it (unsigned binary) - click 'More info' then 'Run anyway'."
