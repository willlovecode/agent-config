# Installs the skills listed in baseline.json for the given packs, from the copies
# vendored in packs/<pack>/skills (pinned commits; nothing is fetched from the network).
# Adds and updates only: skills not in the baseline are left alone.
# Shared skills live in ~/.agents/skills (Codex reads it); Claude Code gets a
# junction per skill. Claude-only skills live in ~/.claude/skills directly.
param([string[]]$Packs = @("core"))
$ErrorActionPreference = "Stop"
$u      = $env:USERPROFILE
$repo   = Split-Path $PSScriptRoot
$base   = Get-Content (Join-Path $PSScriptRoot "baseline.json") -Raw | ConvertFrom-Json
$shared = Join-Path $u ".agents\skills"
$claude = Join-Path $u ".claude\skills"
$utf8   = New-Object System.Text.UTF8Encoding $false
New-Item -ItemType Directory -Force $shared, $claude | Out-Null

function Set-ExplicitOnly($dir) {
  # Claude Code: hide from automatic invocation; the user can still call it by name.
  $md = Join-Path $dir "SKILL.md"
  $text = [IO.File]::ReadAllText($md)
  if ($text -notmatch '(?m)^disable-model-invocation:') {
    $text = [regex]::Replace($text, '\A---\r?\n', "---`ndisable-model-invocation: true`n", 1)
    [IO.File]::WriteAllText($md, $text, $utf8)
  }
  # Codex: same, via agents/openai.yaml.
  $yaml = Join-Path $dir "agents\openai.yaml"
  New-Item -ItemType Directory -Force (Split-Path $yaml) | Out-Null
  if (-not (Test-Path $yaml) -or (Get-Content $yaml -Raw) -notmatch 'allow_implicit_invocation') {
    [IO.File]::AppendAllText($yaml, "policy:`n  allow_implicit_invocation: false`n", $utf8)
  }
}

# Pre-approved tools in a third-party skill are never wanted: strip allowed-tools.
function Remove-AllowedTools($dir) {
  $md = Join-Path $dir "SKILL.md"
  $text = [IO.File]::ReadAllText($md)
  $new = [regex]::Replace($text, '(?m)^allowed-tools:.*\r?\n', '')
  if ($new -ne $text) { [IO.File]::WriteAllText($md, $new, $utf8); "           (removed allowed-tools)" }
}

"Skills ($($Packs -join ', '))"
foreach ($s in $base.skills | Where-Object { $Packs -contains $_.pack }) {
  $src = Join-Path $repo "packs\$($s.pack)\skills\$($s.name)"
  if (-not (Test-Path (Join-Path $src "SKILL.md"))) { throw "$($s.name): no SKILL.md in $src" }
  $hash = (Get-ChildItem -Recurse -File $src | Sort-Object FullName | ForEach-Object { (Get-FileHash $_.FullName -Algorithm SHA256).Hash }) -join ""
  $hash = [BitConverter]::ToString([Security.Cryptography.SHA256]::Create().ComputeHash([Text.Encoding]::ASCII.GetBytes($hash))).Replace("-", "").Substring(0, 16)
  $home_ = if ($s.tools -contains "codex") { $shared } else { $claude }
  $dest  = Join-Path $home_ $s.name
  $pin   = Join-Path $dest ".agent-config-pin"
  $want  = "$($s.repo)@$($s.commit) explicit_only=$($s.explicit_only) files=$hash"
  if ((Test-Path $pin) -and ((Get-Content $pin -Raw).Trim() -eq $want)) {
    "  ok       $($s.name)"
  } else {
    # Replace only a folder this script installed before (it has a pin file).
    if (Test-Path $dest) {
      if (-not (Test-Path $pin)) { throw "$($s.name): $dest exists but was not installed from the baseline; move it away first" }
      Remove-Item -Recurse -Force $dest
    }
    Copy-Item -Recurse $src $dest
    Remove-AllowedTools $dest
    if ($s.explicit_only) { Set-ExplicitOnly $dest }
    [IO.File]::WriteAllText($pin, $want, $utf8)
    "  install  $($s.name)  ($($s.repo)@$($s.commit.Substring(0,7))$(if ($s.explicit_only) { ', explicit-only' }))"
  }
  if ($s.tools -contains "claude" -and $home_ -eq $shared) {
    $link = Join-Path $claude $s.name
    if (-not (Test-Path $link)) { cmd /c mklink /J "$link" "$dest" | Out-Null; "  linked   $($s.name) -> Claude Code" }
  }
}
