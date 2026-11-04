# Installs the skills and plugins listed in baseline.json, pinned to the listed
# commits. Adds and updates only: skills not in the baseline are left alone.
# Shared skills live in ~/.agents/skills (Codex reads it); Claude Code gets a
# junction per skill. Claude-only skills live in ~/.claude/skills directly.
$ErrorActionPreference = "Stop"
$u       = $env:USERPROFILE
$base    = Get-Content (Join-Path $PSScriptRoot "baseline.json") -Raw | ConvertFrom-Json
$shared  = Join-Path $u ".agents\skills"
$claude  = Join-Path $u ".claude\skills"
$cache   = Join-Path $u ".cache\agent-config\repos"
$utf8    = New-Object System.Text.UTF8Encoding $false
New-Item -ItemType Directory -Force $shared, $claude, $cache | Out-Null

function Invoke-Native { & $args[0] @($args | Select-Object -Skip 1); if ($LASTEXITCODE -ne 0) { throw "$($args -join ' ') exited $LASTEXITCODE" } }

# Fetches one commit of a repo into the cache and returns the cache path.
function Get-Repo($repo, $commit) {
  $dir = Join-Path $cache ($repo -replace '/', '__')
  if (-not (Test-Path (Join-Path $dir ".git"))) {
    Invoke-Native git clone -q --filter=blob:none --no-checkout "https://github.com/$repo.git" $dir
  }
  $ErrorActionPreference = "Continue"   # 2> on a native command would throw under Stop in PS 5.1
  & git -C $dir cat-file -e "$commit^{commit}" 2>$null
  if ($LASTEXITCODE -ne 0) { Invoke-Native git -C $dir fetch -q origin $commit }
  return $dir
}

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

"Skills"
foreach ($s in $base.skills) {
  $home_ = if ($s.tools -contains "codex") { $shared } else { $claude }
  $dest  = Join-Path $home_ $s.name
  $pin   = Join-Path $dest ".agent-config-pin"
  $want  = "$($s.repo)@$($s.commit) explicit_only=$($s.explicit_only)"
  if ((Test-Path $pin) -and ((Get-Content $pin -Raw).Trim() -eq $want)) {
    "  ok       $($s.name)"
  } else {
    $repo = Get-Repo $s.repo $s.commit
    $tmp  = Join-Path $cache "_checkout"
    if (Test-Path $tmp) { Remove-Item -Recurse -Force $tmp }
    New-Item -ItemType Directory -Force $tmp | Out-Null
    Invoke-Native git -C $repo "--work-tree=$tmp" checkout -q $s.commit -- $s.path
    & cmd /c "git -C `"$repo`" reset -q 2>nul"   # checkout with --work-tree also stages into the cache's index
    $src = Join-Path $tmp ($s.path -replace '/', '\')
    if (-not (Test-Path (Join-Path $src "SKILL.md"))) { throw "$($s.name): no SKILL.md at $($s.repo)/$($s.path)@$($s.commit)" }
    # Replace only a folder this script installed before (it has a pin file).
    if (Test-Path $dest) {
      if (-not (Test-Path $pin)) { throw "$($s.name): $dest exists but was not installed from the baseline; move it away first" }
      Remove-Item -Recurse -Force $dest
    }
    Copy-Item -Recurse $src $dest
    if ($s.explicit_only) { Set-ExplicitOnly $dest }
    [IO.File]::WriteAllText($pin, $want, $utf8)
    Remove-Item -Recurse -Force $tmp
    "  install  $($s.name)  ($($s.repo)@$($s.commit.Substring(0,7)))"
  }
  if ($s.tools -contains "claude" -and $home_ -eq $shared) {
    $link = Join-Path $claude $s.name
    if (-not (Test-Path $link)) { cmd /c mklink /J "$link" "$dest" | Out-Null; "  linked   $($s.name) -> Claude Code" }
  }
}

"Plugins"
if (Get-Command claude -ErrorAction SilentlyContinue) {
  $have = (cmd /c "claude plugin list 2>&1") -join "`n"
  $markets = (cmd /c "claude plugin marketplace list 2>&1") -join "`n"
  foreach ($p in $base.plugins.claude) {
    $mk = $p.id.Split("@")[1]
    if ($markets -notmatch [regex]::Escape($mk)) { Invoke-Native claude plugin marketplace add $p.marketplace; $markets += "`n$mk" }
    if ($have -match [regex]::Escape($p.id)) { "  ok       claude $($p.id)" }
    else { Invoke-Native claude plugin install $p.id; "  install  claude $($p.id)" }
  }
}
if (Get-Command codex -ErrorAction SilentlyContinue) {
  $have = (cmd /c "codex plugin list 2>&1") -join "`n"
  foreach ($p in $base.plugins.codex) {
    if ($have -match "$([regex]::Escape($p.id))\s+installed") { "  ok       codex $($p.id)" }
    else { Invoke-Native codex plugin add $p.id; "  install  codex $($p.id)" }
  }
}
