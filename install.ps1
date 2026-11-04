# Applies this repo's agent config to Codex and Claude Code. Only adds and
# updates: it never deletes skills, agents, hooks or plugins. Safe to re-run.
$ErrorActionPreference = "Stop"
$repo   = $PSScriptRoot
$u      = $env:USERPROFILE
$codex  = Join-Path $u ".codex"
$claude = Join-Path $u ".claude"
$utf8   = New-Object System.Text.UTF8Encoding $false

# Sets each key from $values in the top-level (pre-[table]) part of a TOML file,
# leaving every other line as it was.
function Set-TomlTopKeys($path, $values) {
  $text  = [IO.File]::ReadAllText($path)
  $nl    = if ($text -match "`r`n") { "`r`n" } else { "`n" }
  $lines = [Collections.Generic.List[string]]($text -split "\r?\n")
  $end   = $lines.FindIndex([Predicate[string]]{ param($l) $l -match '^\s*\[' })
  if ($end -lt 0) { $end = $lines.Count }
  foreach ($key in $values.Keys) {
    $line = "$key = $($values[$key])"
    $i = -1
    for ($j = 0; $j -lt $end; $j++) { if ($lines[$j] -match "^\s*$([regex]::Escape($key))\s*=") { $i = $j; break } }
    if ($i -ge 0) {
      if ($lines[$i] -ne $line) { "  config.toml: $($lines[$i].Trim()) -> $line" }
      $lines[$i] = $line
    } else { $lines.Insert(0, $line); $end++; "  config.toml: added $line" }
  }
  [IO.File]::WriteAllText($path, ($lines -join $nl), $utf8)
}

"Codex"
Copy-Item -Force (Join-Path $repo "AGENTS.md") (Join-Path $codex "AGENTS.md"); "  AGENTS.md updated"
foreach ($p in "deep","quick") { Copy-Item -Force (Join-Path $repo "codex\$p.config.toml") (Join-Path $codex "$p.config.toml") }
"  profiles deep, quick updated"
$want = [ordered]@{}
foreach ($l in Get-Content (Join-Path $repo "codex\default.toml")) {
  if ($l -match '^\s*(model|model_reasoning_effort)\s*=\s*(.+?)\s*$') { $want[$matches[1]] = $matches[2] }
}
Set-TomlTopKeys (Join-Path $codex "config.toml") $want

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
Copy-Item -Force (Join-Path $repo "claude\agents\*.md") $agents
"  subagents: $((Get-ChildItem (Join-Path $repo 'claude\agents') -Filter *.md).BaseName -join ', ')"
# Skills from .agents\skills are linked into .claude\skills with junctions.
Get-ChildItem $shared -Directory | ForEach-Object {
  $link = Join-Path $claude "skills\$($_.Name)"
  if (-not (Test-Path $link)) { cmd /c mklink /J "$link" "$($_.FullName)" | Out-Null; "  linked   $($_.Name)" }
}

# modelSettings is merged per model id; entries not in the repo are kept.
$sp = Join-Path $claude "settings.json"
$s  = Get-Content $sp -Raw | ConvertFrom-Json
$ms = Get-Content (Join-Path $repo "claude\model-settings.json") -Raw | ConvertFrom-Json
if (-not $s.modelSettings) { $s | Add-Member -NotePropertyName modelSettings -NotePropertyValue ([pscustomobject]@{}) }
foreach ($p in $ms.PSObject.Properties) {
  $s.modelSettings | Add-Member -Force -NotePropertyName $p.Name -NotePropertyValue $p.Value
}
[IO.File]::WriteAllText($sp, ($s | ConvertTo-Json -Depth 20), $utf8)
"  modelSettings: $(($ms.PSObject.Properties | ForEach-Object { "$($_.Name) $($_.Value.effortLevel)" }) -join ', ')"

"Done. Start a new Codex and Claude Code session to pick up the changes."
