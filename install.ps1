# Applies this repo's agent config to Codex and Claude Code, and clears out the
# skills and hooks retired on 2026-10-07. Safe to re-run. Backups of the old
# setup are in Code\_archive\codex-config-2026-10-07 and claude-config-2026-10-07.
$ErrorActionPreference = "Stop"
$repo   = $PSScriptRoot
$u      = $env:USERPROFILE
$codex  = Join-Path $u ".codex"
$claude = Join-Path $u ".claude"
$utf8   = New-Object System.Text.UTF8Encoding $false

function Remove-Path($p) {
  if (-not (Test-Path -LiteralPath $p) -and -not (Get-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue)) { return }
  $item = Get-Item -LiteralPath $p -Force
  if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { cmd /c rmdir "$p" | Out-Null }   # link: remove the link only
  else { Remove-Item -LiteralPath $p -Recurse -Force }
  "  removed  $p"
}

"Codex"
Copy-Item -Force (Join-Path $repo "AGENTS.md") (Join-Path $codex "AGENTS.md"); "  AGENTS.md updated"
Get-ChildItem -Force (Join-Path $codex "skills") -Directory -ErrorAction SilentlyContinue |
  Where-Object { $_.Name -ne ".system" } | ForEach-Object { Remove-Path $_.FullName }

"Shared skills folder"
$shared = Join-Path $u ".agents\skills"
New-Item -ItemType Directory -Force $shared | Out-Null
Get-ChildItem (Join-Path $repo "skills") -Directory -ErrorAction SilentlyContinue | ForEach-Object {
  Copy-Item -Recurse -Force $_.FullName $shared; "  skill    $($_.Name)"
}

"Claude Code"
Copy-Item -Force (Join-Path $repo "claude\CLAUDE.md") (Join-Path $claude "CLAUDE.md"); "  CLAUDE.md updated (imports .codex\AGENTS.md)"
$agents = Join-Path $claude "agents"
New-Item -ItemType Directory -Force $agents | Out-Null
Get-ChildItem $agents -Filter *.md | Remove-Item -Force
Copy-Item -Force (Join-Path $repo "claude\agents\*.md") $agents; "  subagents: $((Get-ChildItem $agents -Filter *.md).BaseName -join ', ')"
foreach ($s in "browser-automation","game-development",".trash") { Remove-Path (Join-Path $claude "skills\$s") }
# Skills from .agents\skills are linked into .claude\skills with junctions.
Get-ChildItem $shared -Directory | ForEach-Object {
  $link = Join-Path $claude "skills\$($_.Name)"
  if (-not (Test-Path $link)) { cmd /c mklink /J "$link" "$($_.FullName)" | Out-Null; "  linked   $($_.Name)" }
}

if (Get-Command claude -ErrorAction SilentlyContinue) {
  try { claude plugin uninstall "last30days@last30days-skill" 2>&1 | Out-Null } catch {}
  try { claude plugin marketplace remove "last30days-skill" 2>&1 | Out-Null } catch {}
}
$sp = Join-Path $claude "settings.json"
$s  = Get-Content $sp -Raw | ConvertFrom-Json
if ($s.PSObject.Properties.Name -contains "hooks") { $s.PSObject.Properties.Remove("hooks"); "  agent-flow hooks removed" }
if ($s.enabledPlugins -and ($s.enabledPlugins.PSObject.Properties.Name -contains "last30days@last30days-skill")) {
  $s.enabledPlugins.PSObject.Properties.Remove("last30days@last30days-skill"); "  last30days plugin disabled"
}
[IO.File]::WriteAllText($sp, ($s | ConvertTo-Json -Depth 20), $utf8)

"Done. Start a new Codex and Claude Code session to pick up the changes."
