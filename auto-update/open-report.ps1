# Handler for agentcfg:report/<file>.html links from the auto-update toasts.
# Opens only an existing .html report in the log folder; anything else is ignored.
param([string]$uri)
$logDir = Join-Path $env:LOCALAPPDATA "agent-config\logs"
$name = [Uri]::UnescapeDataString(($uri -replace '^agentcfg:(//)?report/', '')).TrimEnd('/')
if ($name -notmatch '^[\w.-]+\.html$') { exit 1 }
$path = Join-Path $logDir $name
if (Test-Path -LiteralPath $path -PathType Leaf) { Invoke-Item -LiteralPath $path }
