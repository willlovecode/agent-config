# Wires the Stop-hook check runner into one project, for both tools:
#   add-project-checks.ps1 -Project C:\path\to\repo -Command "uv run ruff check . && uv run pytest -q" [-Timeout 300]
# Writes .claude/settings.local.json and .codex/hooks.json in the project and adds both
# to .git/info/exclude, so nothing personal gets committed to the project's repo.
# The command runs in cmd.exe. Codex needs the project trusted and the hook trusted (/hooks).
param([Parameter(Mandatory)][string]$Project, [Parameter(Mandatory)][string]$Command, [int]$Timeout = 300)
$ErrorActionPreference = "Stop"
$utf8 = New-Object System.Text.UTF8Encoding $false
$runner = Join-Path $env:USERPROFILE ".agent-config\hooks\stop-checks.mjs"
if (-not (Test-Path $runner)) { throw "run install.ps1 first ($runner missing)" }
$runnerFwd = $runner -replace '\\', '/'

function Read-Json($path) { if (Test-Path $path) { Get-Content $path -Raw -Encoding UTF8 | ConvertFrom-Json } else { [pscustomobject]@{} } }
function Set-Prop($obj, $name, $value) { $obj | Add-Member -Force -NotePropertyName $name -NotePropertyValue $value }
function Set-StopHook($path, $group) {
  New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
  $j = Read-Json $path
  if (-not $j.hooks) { Set-Prop $j "hooks" ([pscustomobject]@{}) }
  $keep = @($j.hooks.Stop | Where-Object { $_ -and ((ConvertTo-Json -InputObject $_ -Depth 10) -notmatch 'stop-checks\.mjs') })
  Set-Prop $j.hooks "Stop" (@($keep) + @($group))
  [IO.File]::WriteAllText($path, (ConvertTo-Json -InputObject $j -Depth 20), $utf8)
}

Set-StopHook (Join-Path $Project ".claude\settings.local.json") ([pscustomobject]@{ hooks = @([pscustomobject]@{
  type = "command"; command = "node"; args = @($runner, $Command, "$Timeout"); timeout = $Timeout + 30 }) })
Set-StopHook (Join-Path $Project ".codex\hooks.json") ([pscustomobject]@{ hooks = @([pscustomobject]@{
  type = "command"; command = "node `"$runnerFwd`" `"$($Command -replace '"', '\"')`" $Timeout"; timeout = $Timeout + 30 }) })

$exclude = Join-Path $Project ".git\info\exclude"
if (Test-Path (Split-Path $exclude)) {
  $have = if (Test-Path $exclude) { Get-Content $exclude } else { @() }
  foreach ($line in ".claude/settings.local.json", ".codex/hooks.json") { if ($have -notcontains $line) { Add-Content -Encoding utf8 $exclude $line } }
}
"Stop checks for $Project`: $Command (timeout $Timeout s). Trust in Codex with /hooks."
