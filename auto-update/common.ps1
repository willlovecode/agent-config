# Helpers shared by update-config.ps1 and skills-check.ps1. Callers set $log
# (log file path) before dot-sourcing this file.

function Log($msg) { "$(Get-Date -Format 'HH:mm:ss') $msg" | Add-Content -Encoding utf8 $log }

function Esc($s) { [Net.WebUtility]::HtmlEncode([string]$s) }

# Writes a self-contained HTML report next to the log; $sections is an ordered
# hashtable of heading -> string or string[]. Returns the report path.
# $extraHtml is trusted markup the caller built (already escaped), appended at the end.
function Write-Report($title, $sections, $extraHtml = "") {
  $path = $log -replace '\.log$', '.html'
  $body = foreach ($h in $sections.Keys) {
    $v = @($sections[$h] | Where-Object { $_ })
    if (-not $v) { continue }
    "<h2>$(Esc $h)</h2>"
    if ($v.Count -eq 1) { "<p>$(Esc $v[0])</p>" } else { "<ul>" + (($v | ForEach-Object { "<li>$(Esc $_)</li>" }) -join "") + "</ul>" }
  }
  $html = @"
<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>$(Esc $title)</title>
<style>
:root{color-scheme:light dark;--bg:#fafafa;--fg:#1d1d1f;--mute:#6e6e73;--card:#fff;--line:#e5e5ea}
@media (prefers-color-scheme:dark){:root{--bg:#1c1c1e;--fg:#f5f5f7;--mute:#a1a1a6;--card:#2c2c2e;--line:#3a3a3c}}
body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.5 "Segoe UI",system-ui,sans-serif}
main{max-width:760px;margin:32px auto;padding:0 16px}
h1{font-size:22px;margin:0 0 4px}h2{font-size:15px;margin:24px 0 6px;color:var(--mute);font-weight:600}
p,ul{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:12px 16px;margin:0}
ul{padding-left:32px}li{margin:4px 0}.meta{color:var(--mute);font-size:13px;background:none;border:0;padding:0}
.f{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:12px 16px;margin:0 0 10px}
.f h3{margin:0 0 4px;font-size:15px}.f .k{font-size:12px;font-weight:700;color:var(--mute);text-transform:uppercase}
.f p,.f ul{background:none;border:0;padding:0;margin:4px 0}.f ul{padding-left:20px}a{color:inherit}
code{font:13px Consolas,monospace}
</style><main><h1>$(Esc $title)</h1><p class="meta">$(Get-Date -Format 'yyyy-MM-dd HH:mm') &middot; log: $(Esc $log)</p>
$($body -join "`n")
$extraHtml
</main></html>
"@
  [IO.File]::WriteAllText($path, $html, (New-Object System.Text.UTF8Encoding $false))
  return $path
}

# $report: optional HTML file, opened in the browser right after the toast.
# $sticky: stays on screen until dismissed (reminder scenario).
function Toast($title, $body, $report, [switch]$sticky) {
  try {
    [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
    [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null
    # Toasts from unpackaged scripts can't open links (file: or custom), so the report is opened below instead.
    $scenario = if ($sticky) { " scenario=`"reminder`"" } else { "" }
    $actions = if ($sticky) { "<action content=`"Dismiss`" activationType=`"system`" arguments=`"dismiss`"/>" } else { "" }
    $note = if ($report) { "<text>Full report opened in your browser.</text>" } else { "" }
    $xmlText = "<toast$scenario><visual><binding template=`"ToastGeneric`"><text>$(Esc $title)</text>" +
      "<text>$(Esc $body)</text>$note</binding></visual><actions>$actions</actions></toast>"
    $xml = [Windows.Data.Xml.Dom.XmlDocument]::new(); $xml.LoadXml($xmlText)
    $app = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
    [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($app).Show([Windows.UI.Notifications.ToastNotification]::new($xml))
  } catch { Log "toast failed: $_" }
  if ($report) { try { Open-InBrowser $report } catch { Log "open report failed: $_" } }
}

# .html files may have no default app (here only https links do), so use the
# https handler's command line with the report path.
function Open-InBrowser($path) {
  # Windows 11 keeps the current choice in UserChoiceLatest\ProgId; older builds in UserChoice.
  $assoc = 'HKCU:\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\https'
  $progId = (Get-ItemProperty "$assoc\UserChoiceLatest\ProgId" -ErrorAction SilentlyContinue).ProgId
  if (-not $progId) { $progId = (Get-ItemProperty "$assoc\UserChoice" -ErrorAction SilentlyContinue).ProgId }
  $cmd = if ($progId) { (Get-ItemProperty -LiteralPath "Registry::HKEY_CLASSES_ROOT\$progId\shell\open\command" -ErrorAction SilentlyContinue).'(default)' }
  if ($cmd -match '^\s*"([^"]+)"\s*(.*)$') {
    $exe = $matches[1]; $rest = $matches[2]
    # A file: URI has no spaces, so it needs no quotes; Chromium's --single-argument would keep them literally.
    $uri = ([Uri]$path).AbsoluteUri
    $argLine = if ($rest.Contains('%1')) { $rest.Replace('"%1"', $uri).Replace('%1', $uri) } else { "$rest $uri" }
    Start-Process -FilePath $exe -ArgumentList $argLine
  } else { Start-Process -FilePath "explorer.exe" -ArgumentList "`"$path`"" }
}

# Headless Claude run; returns the parsed structured output or throws.
function Invoke-Claude($prompt, $schemaFile, $model, $effort, $tools, $extra) {
  # Windows PowerShell 5.1 drops bare quotes when passing arguments to a native exe.
  $schema = ((Get-Content $schemaFile -Raw) -replace '\s+', ' ') -replace '"', '\"'
  $cargs = @("-p", "--model", $model, "--effort", $effort, "--tools", $tools,
    "--setting-sources", "project", "--strict-mcp-config", "--no-session-persistence",
    "--output-format", "json", "--json-schema", $schema) + $extra
  Log "claude $model/$effort tools=[$tools]"
  $raw = $prompt | & claude @cargs
  if ($LASTEXITCODE -ne 0) { throw "claude exited $LASTEXITCODE`: $raw" }
  $r = ($raw | Out-String) | ConvertFrom-Json
  Log "  cost `$$([Math]::Round($r.total_cost_usd, 4)), turns $($r.num_turns), $($r.subtype)"
  if ($r.is_error -or -not $r.structured_output) { throw "claude returned no structured output: $($r.result)" }
  return $r.structured_output
}
