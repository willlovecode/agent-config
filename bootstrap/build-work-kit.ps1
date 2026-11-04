# Builds dist/agent-kit-work.zip: everything the work profile needs and nothing
# personal (no home profile, research, reports or auto-update). Fails if a file in
# the kit mentions a personal marker. For a work machine that can't clone this repo.
$ErrorActionPreference = "Stop"
$repo = Split-Path $PSScriptRoot
$prof = Get-Content (Join-Path $repo "profiles\work\profile.json") -Raw | ConvertFrom-Json
$stage = Join-Path $repo "dist\agent-kit-work"
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Force $stage | Out-Null

$include = @("install.ps1", "base", "profiles\work", "enforcement", "skills\install-skills.ps1", "skills\baseline.json", "checks")
foreach ($pack in $prof.packs) { $include += "packs\$pack" }
foreach ($p in $include) {
  $dest = Join-Path $stage $p
  New-Item -ItemType Directory -Force (Split-Path $dest) | Out-Null
  Copy-Item -Recurse (Join-Path $repo $p) $dest
}
@"
# Agent kit (work profile)

Run: powershell -ExecutionPolicy Bypass -File install.ps1 -Profile work
Then open Codex once and trust the hook with /hooks. See profiles\work\AGENTS.md
for the rules this installs. Built from agent-config $(git -C $repo rev-parse --short HEAD) on $(Get-Date -Format yyyy-MM-dd).
"@ | Set-Content -Encoding utf8 (Join-Path $stage "README.md")

# Personal markers that must never reach a work machine.
$markers = "walbe", "walbert", "Sol\b", "Vega\b", "infoStacker", "vbgym", "Ryzen", "last30days"
$hits = Get-ChildItem -Recurse -File $stage | Select-String -Pattern $markers -CaseSensitive |
  Where-Object { $_.Path -notmatch '\\packs\\[^\\]+\\skills\\' }   # vendored third-party text is checked by review, not markers
if ($hits) { $hits | ForEach-Object { "  $($_.Path.Substring($stage.Length)):$($_.LineNumber): $($_.Line.Trim())" }; throw "personal markers found in the work kit" }

$zip = Join-Path $repo "dist\agent-kit-work.zip"
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip
"Built $zip ($([math]::Round((Get-Item $zip).Length / 1KB)) KB, $((Get-ChildItem -Recurse -File $stage).Count) files)"
