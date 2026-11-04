# Runs at logon (see register-task.ps1). Checks the benchmarks with a headless
# Claude run that has web tools but no edit tools; if the reading changed, a
# second run with edit tools but no web tools updates the repo, then this script
# runs install.ps1 and commits locally. Never pushes.
param([switch]$Force)   # -Force: run the update phase even if the check says unchanged
$ErrorActionPreference = "Stop"
$here  = $PSScriptRoot
$repo  = Split-Path $here -Parent
$today = Get-Date -Format "yyyy-MM-dd"
$logDir = Join-Path $env:LOCALAPPDATA "agent-config\logs"
New-Item -ItemType Directory -Force $logDir | Out-Null
$log = Join-Path $logDir "run-$(Get-Date -Format 'yyyy-MM-dd_HHmmss').log"
Get-ChildItem $logDir -Filter run-*.log | Sort-Object Name -Descending | Select-Object -Skip 30 | Remove-Item -Force
$OutputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false

# Files the update phase may change; anything else it touches is rolled back.
$allowed = @("profiles/home/AGENTS.md", "packs/core/agents/scout.md", "packs/core/agents/mechanic.md",
  "packs/core/agents/implement.md", "packs/core/agents/review.md", "profiles/home/claude/model-settings.json",
  "profiles/home/codex/default.toml", "profiles/home/codex/deep.config.toml", "profiles/home/codex/quick.config.toml",
  "auto-update/check-model.json")

. (Join-Path $here "common.ps1")

function Invoke-Git { & git -C $repo @args; if ($LASTEXITCODE -ne 0) { throw "git $args failed" } }

function Get-RepoStatus { (& git -C $repo status --porcelain --untracked-files=all) -join "`n" }

$phase = "check"
try {
  Set-Location $repo   # claude's acceptEdits scope is its working directory
  Log "start (repo $repo)"
  # --- Check phase: web tools, no edit tools.
  $cm = Get-Content (Join-Path $here "check-model.json") -Raw | ConvertFrom-Json
  try {
    $d = Invoke-RestMethod -TimeoutSec 60 "https://cognition.com/data/frontiercode-leaderboard/data.json"
    $rows = foreach ($p in $d.v1_1.data.PSObject.Properties) {
      if ($p.Name -notmatch '^(Claude|GPT)') { continue }
      foreach ($e in $p.Value.PSObject.Properties) {
        if ($e.Value.main) { "{0} | {1} | {2:N1}% | `${3:N2}" -f $p.Name, $e.Name, ($e.Value.main.new_score * 100), $e.Value.main.cost }
      }
    }
    $fc = "model | effort | new_score | cost per task`n" + ($rows -join "`n")
  } catch { $fc = "FETCH FAILED: $_"; Log "data.json fetch failed: $_" }

  $agents = Get-Content (Join-Path $repo "profiles\home\AGENTS.md") -Raw
  $section = [regex]::Match($agents, '(?s)## Model choice.*?(?=\r?\n## |\z)').Value
  $prompt = (Get-Content (Join-Path $here "check-prompt.md") -Raw) +
    "`n`nToday is $today.`n`n## Current config: AGENTS.md`n$section`n`n## Current check model`n" +
    (Get-Content (Join-Path $here "check-model.json") -Raw) +
    "`n## FrontierCode 1.1 main, Claude and OpenAI rows (fetched by the script)`n$fc`n"
  $check = Invoke-Claude $prompt (Join-Path $here "check.schema.json") $cm.model $cm.effort "WebFetch,WebSearch" @("--allowedTools", "WebFetch,WebSearch")
  $check | ConvertTo-Json -Depth 10 | Add-Content -Encoding utf8 $log
  Copy-Item $log (Join-Path $logDir "last-check.log") -Force

  if (-not $check.changed -and -not $Force) { Log "unchanged; exit"; exit 0 }

  # --- Update phase: edit tools, no web tools, only validated reading fields.
  $phase = "update"
  Toast "agent config updates needed, updating" (($check.changes | Select-Object -First 3) -join " ")
  $before = Get-RepoStatus
  $dirty = (& git -C $repo status --porcelain --untracked-files=no) | Where-Object { $_ }
  if ($dirty) { throw "repo has uncommitted changes to tracked files; not updating:`n$($dirty -join "`n")" }

  $impl = Get-Content (Join-Path $repo "packs\core\agents\implement.md") -Raw
  $umodel  = [regex]::Match($impl, '(?m)^model:\s*(\S+)').Groups[1].Value
  $ueffort = [regex]::Match($impl, '(?m)^effort:\s*(\S+)').Groups[1].Value
  if (-not $umodel) { $umodel = "sonnet" }; if (-not $ueffort) { $ueffort = "high" }

  $reading = [pscustomobject]@{ benchmark = $check.reading.benchmark; date = $check.reading.date
    recommendations = $check.reading.recommendations; never_use = $check.reading.never_use
    check_model = $check.check_model }
  $uprompt = (Get-Content (Join-Path $here "update-prompt.md") -Raw) +
    "`n`n## New reading`n" + ($reading | ConvertTo-Json -Depth 10)
  $upd = Invoke-Claude $uprompt (Join-Path $here "update.schema.json") $umodel $ueffort "Read,Edit,Write,Glob,Grep" @("--permission-mode", "acceptEdits")
  Log "update: done=$($upd.done) $($upd.summary)"

  $changed = @((& git -C $repo status --porcelain --untracked-files=all) | ForEach-Object { $_.Substring(3).Trim('"') } |
    Where-Object { $before -notmatch [regex]::Escape($_) -or $allowed -contains $_ })
  $bad = @($changed | Where-Object { $allowed -notcontains $_ })
  if ($bad) {
    foreach ($f in $bad) {
      if (& git -C $repo ls-files -- $f) { Invoke-Git checkout -- $f } else { Remove-Item -Force (Join-Path $repo $f) }
    }
    throw "update touched files outside the allowlist (rolled back): $($bad -join ', ')"
  }
  if (-not $upd.done) { throw "update model declined: $($upd.summary)" }
  $touched = @($changed | Where-Object { $allowed -contains $_ })
  if (-not $touched) { throw "update model changed no files although the check reported changes" }

  $phase = "install"
  & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repo "install.ps1") | Add-Content -Encoding utf8 $log
  if ($LASTEXITCODE -ne 0) { throw "install.ps1 exited $LASTEXITCODE" }

  $phase = "commit"
  Invoke-Git add -- @touched
  $msg = "Update model choice to $($check.reading.benchmark) reading of $($check.reading.date)`n`n" +
    (($check.changes | ForEach-Object { "- $_" }) -join "`n") + "`n`nApplied by auto-update/update-config.ps1."
  $msgFile = Join-Path $logDir "commit-msg.txt"
  [IO.File]::WriteAllText($msgFile, $msg, (New-Object System.Text.UTF8Encoding $false))
  Invoke-Git commit -q -F $msgFile
  $sha = & git -C $repo rev-parse --short HEAD
  Log "committed $sha"
  $report = Write-Report "Agent config updated" ([ordered]@{
    "What changed in the benchmark" = @($check.changes)
    "What was edited" = $upd.summary
    "Commit (local, not pushed)" = "$sha in $repo - undo with: git -C `"$repo`" revert $sha"
    "Files" = @($touched)
    "Cross-checks" = @($check.cross_checks | ForEach-Object { "$($_.source): $($_.status) - $($_.note)" })
  })
  Toast "agent config updated" ($upd.summary) $report -sticky
  exit 0
} catch {
  Log "FAILED in $phase phase: $_"
  $report = Write-Report "Agent config $phase failed" ([ordered]@{ "Error" = "$_"; "Benchmark changes found" = @($check.changes) })
  Toast "agent config $phase failed" "$_" $report -sticky
  exit 1
}
