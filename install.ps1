# Installs the malveon binary for Windows: downloads the release exe,
# places it in %LOCALAPPDATA%\malveon, and adds that folder to the
# user's PATH (no admin rights needed). Run via:
#   irm https://raw.githubusercontent.com/LadsonDavid/Malveon/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "LadsonDavid/Malveon"
$installDir = "$env:LOCALAPPDATA\malveon"
$exePath = "$installDir\malveon.exe"
$asset = "malveon-windows-amd64.exe"
# Overridable only for testing.
$github = if ($env:MALVEON_RELEASE_URL) { $env:MALVEON_RELEASE_URL } else { "https://github.com/$repo/releases/latest/download" }
$counter = if ($env:MALVEON_COUNTER_URL) { $env:MALVEON_COUNTER_URL } else { "https://get.malveon.com/latest" }  # counts the download, then forwards to GitHub

New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("malveon-" + [Guid]::NewGuid() + ".exe")

Write-Host "Downloading malveon..."
try {
    Invoke-WebRequest -UseBasicParsing -Uri "$counter/$($asset)?src=install.ps1" -OutFile $tmp
} catch {
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$github/$asset" -OutFile $tmp
    } catch {
        Write-Host "malveon: download failed - check https://github.com/$repo/releases for available builds" -ForegroundColor Red
        exit 1
    }
}

# Verify before installing anything: the checksum list comes straight from
# GitHub (never through the counter), and the download must match it.
try {
    $sums = (Invoke-WebRequest -UseBasicParsing -Uri "$github/SHA256SUMS").Content
    if ($sums -is [byte[]]) { $sums = [Text.Encoding]::UTF8.GetString($sums) }
} catch {
    Remove-Item $tmp -ErrorAction SilentlyContinue
    Write-Host "malveon: this release has no signed checksum list - not installing it" -ForegroundColor Red
    exit 1
}
$want = ($sums -split "`n" | Where-Object { $_ -match "\s$([regex]::Escape($asset))\s*$" } | ForEach-Object { ($_ -split '\s+')[0] }) | Select-Object -First 1
$got = (Get-FileHash -Algorithm SHA256 $tmp).Hash.ToLower()
if (-not $want -or $got -ne $want.ToLower()) {
    Remove-Item $tmp -ErrorAction SilentlyContinue
    Write-Host "malveon: the download doesn't match the release's checksum - not installing it" -ForegroundColor Red
    exit 1
}
Move-Item -Force $tmp $exePath
# Windows has no built-in way to check the release signature; say plainly what was checked.
Write-Host "Verified: checksum matches (signature not checked here - 'malveon update' checks it from now on)."

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "Added $installDir to your PATH."
    Write-Host "Open a NEW terminal window for that to take effect."
} else {
    Write-Host "Already on PATH."
}

Write-Host "Installed to $exePath"
Write-Host "Run 'malveon' to verify (open a new terminal first if your PATH was just updated above)."

Write-Host ""
Write-Host "Want a custom check rule for your stack, or found something broken? Run 'malveon register' -"
Write-Host "opens a real GitHub Discussion, no email or signup required."
