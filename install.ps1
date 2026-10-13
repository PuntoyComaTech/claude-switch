# Install claude-switch on Windows and hook it into the Claude Code status line.
#
#   irm https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.ps1 | iex
#
# Env:
#   CLAUDE_SWITCH_BIN_DIR   install dir (default %LOCALAPPDATA%\Programs\claude-switch)
#   CLAUDE_SWITCH_VERSION   release tag to install (default: latest)
#   CLAUDE_SWITCH_BASE_URL  where release files live (for mirrors and tests)
#   CLAUDE_SWITCH_NO_SETUP  set to 1 to leave the Claude Code status line alone
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'PuntoyComaTech/claude-switch'
$binDir = if ($env:CLAUDE_SWITCH_BIN_DIR) { $env:CLAUDE_SWITCH_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\claude-switch' }
$version = if ($env:CLAUDE_SWITCH_VERSION) { $env:CLAUDE_SWITCH_VERSION } else { 'latest' }
$base = if ($env:CLAUDE_SWITCH_BASE_URL) { $env:CLAUDE_SWITCH_BASE_URL }
        elseif ($version -eq 'latest') { "https://github.com/$repo/releases/latest/download" }
        else { "https://github.com/$repo/releases/download/$version" }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'amd64' }
  'ARM64' { 'arm64' }
  default { throw "unsupported CPU $env:PROCESSOR_ARCHITECTURE" }
}
$archive = "claude-switch_windows_$arch.zip"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Write-Host "downloading $archive"
  Invoke-WebRequest -UseBasicParsing "$base/$archive" -OutFile (Join-Path $tmp $archive)
  Invoke-WebRequest -UseBasicParsing "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

  $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { ($_ -split '\s+')[1] -eq $archive }
  if (-not $line) { throw "$archive is not listed in checksums.txt" }
  $want = ($line -split '\s+')[0]
  $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $archive)).Hash.ToLower()
  if ($got -ne $want) { throw "checksum mismatch for $archive" }

  Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath $tmp -Force
  New-Item -ItemType Directory -Force -Path $binDir | Out-Null
  $exe = Join-Path $binDir 'claude-switch.exe'
  Copy-Item (Join-Path $tmp 'claude-switch.exe') $exe -Force
  # Downloaded by a script, the file carries no Mark-of-the-Web; clear it anyway.
  Unblock-File $exe
  Write-Host "installed $exe ($(& $exe --version))"
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $binDir) {
  [Environment]::SetEnvironmentVariable('Path', (($userPath, $binDir) -join ';').Trim(';'), 'User')
  $env:Path = "$env:Path;$binDir"
  Write-Host "added $binDir to your PATH (open a new terminal to use it)"
}

if (-not (Get-Command claude -ErrorAction SilentlyContinue)) { Write-Host "warning: 'claude' (Claude Code) not found on PATH" }
if ($env:CLAUDE_SWITCH_NO_SETUP -ne '1') { & $exe setup }

Write-Host ''
Write-Host 'Next, save your accounts:'
Write-Host '  claude-switch add <name>            # the account you are logged in with now'
Write-Host '  claude-switch add <name> --login    # each other account, via the browser'
Write-Host '  claude-switch list'
