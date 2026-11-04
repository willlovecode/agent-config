# Weekly skills check, run by the logon task after update-config.ps1. The script
# gathers upstream commits and unmanaged local skills; a headless Claude run with
# web tools only judges them. Findings are never applied here: they go into an
# HTML report for the owner to approve in a Claude Code session.
param([switch]$Force)   # -Force: run even if the last check is less than 7 days old
$ErrorActionPreference = "Stop"
$here   = $PSScriptRoot
$repo   = Split-Path $here -Parent
$state  = Join-Path $env:LOCALAPPDATA "agent-config"
$logDir = Join-Path $state "logs"
New-Item -ItemType Directory -Force $logDir | Out-Null
$log = Join-Path $logDir "skills-$(Get-Date -Format 'yyyy-MM-dd_HHmmss').log"
Get-ChildItem $logDir -Filter skills-*.* | Sort-Object Name -Descending | Select-Object -Skip 30 | Remove-Item -Force
$OutputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false
. (Join-Path $here "common.ps1")

$last = Join-Path $state "skills-check-last.txt"
if (-not $Force -and (Test-Path $last) -and ((Get-Date) - [datetime](Get-Content $last -Raw).Trim()).TotalDays -lt 7) { exit 0 }

function Get-GitHub($path) {
  if (Get-Command gh -ErrorAction SilentlyContinue) {
    $out = cmd /c "gh api `"$path`" 2>nul"
    if ($LASTEXITCODE -eq 0) { return ($out -join "`n") | ConvertFrom-Json }
  }
  Invoke-RestMethod -TimeoutSec 30 -Headers @{ "User-Agent" = "agent-config-skills-check" } "https://api.github.com/$path"
}

try {
  Set-Location $repo
  Log "start"
  $base = Get-Content (Join-Path $repo "skills\baseline.json") -Raw | ConvertFrom-Json

  # Upstream commits touching each pinned skill since its pin.
  $upstream = foreach ($s in $base.skills) {
    try {
      $pinDate = (Get-GitHub "repos/$($s.repo)/commits/$($s.commit)").commit.committer.date
      $since = ([datetime]$pinDate).ToUniversalTime().AddSeconds(1).ToString("yyyy-MM-ddTHH:mm:ssZ")
      $commits = @(Get-GitHub "repos/$($s.repo)/commits?path=$($s.path)&since=$since&per_page=20")
      if ($commits.Count) {
        "### $($s.name) ($($s.repo)/$($s.path), pinned $($s.commit.Substring(0,7)) of $pinDate)"
        "compare: https://github.com/$($s.repo)/compare/$($s.commit)...$($commits[0].sha)"
        foreach ($c in $commits) { "- $($c.sha) $($c.commit.committer.date) $(($c.commit.message -split "`n")[0])" }
      } else { "### $($s.name): no upstream commits since pin" }
    } catch { "### $($s.name): GitHub lookup failed ($_)"; Log "github $($s.name): $_" }
  }

  # Skills on disk that the baseline does not manage.
  $known = @($base.skills.name) + @("synced", ".trash")
  $unmanaged = foreach ($d in (Join-Path $env:USERPROFILE ".agents\skills"), (Join-Path $env:USERPROFILE ".claude\skills")) {
    Get-ChildItem -Force $d -ErrorAction SilentlyContinue | Where-Object { $known -notcontains $_.Name } | ForEach-Object { "- $($_.FullName)" }
  }
  $plugins = (cmd /c "claude plugin list 2>&1") -join "`n"

  $prompt = (Get-Content (Join-Path $here "skills-check-prompt.md") -Raw) +
    "`n`nToday is $(Get-Date -Format 'yyyy-MM-dd').`n`n## Baseline (skills/baseline.json)`n" + (Get-Content (Join-Path $repo "skills\baseline.json") -Raw) +
    "`n`n## Upstream commits since each pin (from the GitHub API)`n" + ($upstream -join "`n") +
    "`n`n## Installed Claude Code plugins`n$plugins`n`n## Local skills not in the baseline`n" + $(if ($unmanaged) { $unmanaged -join "`n" } else { "(none)" }) + "`n"

  # Judgement, not number-reading: the model the table assigns to the implement agent.
  $impl = Get-Content (Join-Path $repo "claude\agents\implement.md") -Raw
  $model  = [regex]::Match($impl, '(?m)^model:\s*(\S+)').Groups[1].Value
  $effort = [regex]::Match($impl, '(?m)^effort:\s*(\S+)').Groups[1].Value
  $res = Invoke-Claude $prompt (Join-Path $here "skills-check.schema.json") $model $effort "WebFetch,WebSearch" @("--allowedTools", "WebFetch,WebSearch")
  $res | ConvertTo-Json -Depth 10 | Add-Content -Encoding utf8 $log
  (Get-Date -Format "yyyy-MM-dd HH:mm") | Set-Content $last

  $findings = @($res.findings)
  if (-not $findings.Count) { Log "no findings; exit"; exit 0 }

  $i = 0
  $numbered = foreach ($f in $findings) { $i++; $f | Add-Member -Force -NotePropertyName id -NotePropertyValue $i -PassThru }
  $json = $log -replace '\.log$', '.json'
  [IO.File]::WriteAllText($json, ($numbered | ConvertTo-Json -Depth 10), (New-Object System.Text.UTF8Encoding $false))

  $cards = foreach ($f in $numbered) {
    $links = ($f.evidence | ForEach-Object { "<li><a href=`"$(Esc $_)`">$(Esc $_)</a></li>" }) -join ""
    "<div class=`"f`"><p class=`"k`">#$($f.id) &middot; $(Esc $f.kind) &middot; recommend: $(Esc $f.recommendation)</p>" +
    "<h3>$(Esc $f.target)</h3><p>$(Esc $f.summary)</p><p>$(Esc $f.details)</p>" +
    $(if ($f.new_commit) { "<p>New commit: <code>$(Esc $f.new_commit)</code></p>" } else { "" }) +
    $(if ($links) { "<ul>$links</ul>" } else { "" }) + "</div>"
  }
  $apply = "Apply skill findings 1, 2 from $json and skip the rest."
  $report = Write-Report "Skill findings: $($findings.Count)" ([ordered]@{
    "How to answer" = @("Nothing has been changed. Open Claude Code in C:\Users\walbe\Code\agent-config and say which to apply, for example:", $apply,
      "That session updates skills/baseline.json, runs install.ps1 and commits locally.")
    "What was checked" = @($res.checked)
  }) ("<h2>Findings</h2>" + ($cards -join "`n"))
  Toast "skill findings: $($findings.Count)" (($findings | Select-Object -First 2 | ForEach-Object { "$($_.kind) $($_.target)" }) -join "; ") $report -sticky
  exit 0
} catch {
  Log "FAILED: $_"
  $report = Write-Report "Skills check failed" ([ordered]@{ "Error" = "$_" })
  Toast "skills check failed" "$_" $report -sticky
  exit 1
}
