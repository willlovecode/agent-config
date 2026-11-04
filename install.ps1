# Applies this repo's agent config to Codex and Claude Code for one profile.
# Only adds and updates: it never deletes skills, agents, hooks or plugins. Safe to re-run.
#   install.ps1                  profile from the last run, else home
#   install.ps1 -Profile work    work machine (see docs/portable-toolkit-design.md)
#   install.ps1 -Tier B          force the no-hooks fallback
param([ValidateSet("home", "work")][string]$Profile, [ValidateSet("A", "B")][string]$Tier)
$ErrorActionPreference = "Stop"
$repo   = $PSScriptRoot
$u      = $env:USERPROFILE
$codex  = Join-Path $u ".codex"
$claude = Join-Path $u ".claude"
$state  = Join-Path $u ".agent-config"
$utf8   = New-Object System.Text.UTF8Encoding $false
New-Item -ItemType Directory -Force $codex, $claude, $state | Out-Null

$profileFile = Join-Path $state "profile"
if (-not $Profile) {
  $Profile = if (Test-Path $profileFile) { (Get-Content $profileFile -Raw).Trim() }
             elseif (Test-Path (Join-Path $repo "profiles\home")) { "home" } else { "work" }   # the work kit has no home profile
}
$pdir = Join-Path $repo "profiles\$Profile"
$prof = Get-Content (Join-Path $pdir "profile.json") -Raw | ConvertFrom-Json
"Profile: $Profile ($($prof.description))"

# --- Tier: can hooks run here? A = yes, B = plain files and permission rules only.
$why = @()
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { $why += "node is not on PATH" }
foreach ($m in "$env:ProgramFiles\ClaudeCode\managed-settings.json", "$env:ProgramData\ClaudeCode\managed-settings.json") {
  if (Test-Path $m) {
    $ms_ = Get-Content $m -Raw | ConvertFrom-Json
    if ($ms_.disableAllHooks -or $ms_.allowManagedHooksOnly) { $why += "Claude Code managed settings block user hooks ($m)" }
  }
}
if (-not $Tier) { $Tier = if ($why) { "B" } else { "A" } }
"Tier: $Tier$(if ($why) { " ($($why -join '; '))" })"

# Sets each key from $values in the top-level (pre-[table]) part of a TOML file,
# leaving every other line as it was.
function Set-TomlTopKeys($path, $values) {
  if (-not (Test-Path $path)) { [IO.File]::WriteAllText($path, "", $utf8) }
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

function Read-Json($path) { if (Test-Path $path) { Get-Content $path -Raw | ConvertFrom-Json } else { [pscustomobject]@{} } }
function Write-Json($path, $obj) { [IO.File]::WriteAllText($path, (ConvertTo-Json -InputObject $obj -Depth 20), $utf8) }
function Set-Prop($obj, $name, $value) { $obj | Add-Member -Force -NotePropertyName $name -NotePropertyValue $value }

# Replaces the hook group whose command mentions $marker under $event, or appends it.
function Set-HookGroup($settings, $event, $group, $marker) {
  if (-not $settings.hooks) { Set-Prop $settings "hooks" ([pscustomobject]@{}) }
  $list = @($settings.hooks.$event | Where-Object { $_ -and ((ConvertTo-Json -InputObject $_ -Depth 10) -notmatch [regex]::Escape($marker)) })
  Set-Prop $settings.hooks $event (@($list) + @($group))
}

"Instructions"
$agentsText = (Get-Content (Join-Path $repo "base\AGENTS.md") -Raw).TrimEnd() + "`n`n" + (Get-Content (Join-Path $pdir "AGENTS.md") -Raw).TrimEnd() + "`n"
[IO.File]::WriteAllText((Join-Path $codex "AGENTS.md"), $agentsText, $utf8)
"  ~/.codex/AGENTS.md = base + profiles/$Profile ($([math]::Round($agentsText.Length / 4)) tokens approx.)"
$importPath = ($codex -replace '\\', '/') + "/AGENTS.md"
[IO.File]::WriteAllText((Join-Path $claude "CLAUDE.md"), "@$importPath`n", $utf8)
"  ~/.claude/CLAUDE.md imports $importPath"

"Codex"
if ($prof.codexDefaults) {
  $want = [ordered]@{}
  foreach ($l in Get-Content (Join-Path $pdir $prof.codexDefaults)) {
    if ($l -match '^\s*(model|model_reasoning_effort)\s*=\s*(.+?)\s*$') { $want[$matches[1]] = $matches[2] }
  }
  Set-TomlTopKeys (Join-Path $codex "config.toml") $want
}
foreach ($p in @($prof.codexProfiles)) { if ($p) { Copy-Item -Force (Join-Path $pdir $p) (Join-Path $codex (Split-Path $p -Leaf)); "  profile $(Split-Path $p -Leaf)" } }
$rules = Join-Path $codex "rules"
New-Item -ItemType Directory -Force $rules | Out-Null
Copy-Item -Force (Join-Path $repo "enforcement\codex-rules\agent-config.rules") $rules
"  execpolicy rules: git push, git reset --hard forbidden"

"Claude Code"
$agents = Join-Path $claude "agents"
New-Item -ItemType Directory -Force $agents | Out-Null
foreach ($pack in $prof.packs) {
  $src = Join-Path $repo "packs\$pack\agents"
  if (Test-Path $src) { Copy-Item -Force (Join-Path $src "*.md") $agents; "  subagents ($pack): $((Get-ChildItem $src -Filter *.md).BaseName -join ', ')" }
}
$sp = Join-Path $claude "settings.json"
$s  = Read-Json $sp
if ($prof.claudeModelSettings) {
  # modelSettings is merged per model id; entries not in the repo are kept.
  $ms = Get-Content (Join-Path $pdir $prof.claudeModelSettings) -Raw | ConvertFrom-Json
  if (-not $s.modelSettings) { Set-Prop $s "modelSettings" ([pscustomobject]@{}) }
  foreach ($p in $ms.PSObject.Properties) { Set-Prop $s.modelSettings $p.Name $p.Value }
  "  modelSettings: $(($ms.PSObject.Properties | ForEach-Object { "$($_.Name) $($_.Value.effortLevel)" }) -join ', ')"
}

"Enforcement"
$hooksDir = Join-Path $state "hooks"
New-Item -ItemType Directory -Force $hooksDir | Out-Null
Copy-Item -Force (Join-Path $repo "enforcement\hooks\*.mjs") $hooksDir
$guard = Join-Path $hooksDir "git-guard.mjs"
if ($Tier -eq "A") {
  Set-HookGroup $s "PreToolUse" ([pscustomobject]@{ matcher = "Bash|PowerShell"; hooks = @([pscustomobject]@{ type = "command"; command = "node"; args = @($guard); timeout = 10 }) }) "git-guard.mjs"
  "  Claude Code: git-guard PreToolUse hook"
  $ch = Join-Path $codex "hooks.json"
  $c = Read-Json $ch
  Set-HookGroup $c "PreToolUse" ([pscustomobject]@{ matcher = "^Bash$"; hooks = @([pscustomobject]@{ type = "command"; command = "node `"$($guard -replace '\\', '/')`""; timeout = 10; statusMessage = "git-guard" }) }) "git-guard.mjs"
  Write-Json $ch $c
  "  Codex: git-guard PreToolUse hook (trust it once: open Codex, run /hooks)"
} else {
  # No hooks: deny rules in Claude Code's permissions instead (prefix match, weaker).
  if (-not $s.permissions) { Set-Prop $s "permissions" ([pscustomobject]@{}) }
  $deny = @($s.permissions.deny) + @("Bash(git push:*)", "Bash(git reset --hard:*)", "PowerShell(git push:*)", "PowerShell(git reset --hard:*)")
  Set-Prop $s.permissions "deny" @($deny | Where-Object { $_ } | Select-Object -Unique)
  "  Claude Code: permissions.deny for git push / reset --hard (no hooks in tier B)"
}
Write-Json $sp $s

& (Join-Path $repo "skills\install-skills.ps1") -Packs $prof.packs

[IO.File]::WriteAllText($profileFile, $Profile, $utf8)
"Done ($Profile, tier $Tier). Start new Codex and Claude Code sessions to pick up the changes."
if ($Tier -eq "A") { "Codex skips untrusted hooks silently: run /hooks in Codex once now, and after every update of the hook." }
