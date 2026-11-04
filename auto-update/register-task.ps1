# Registers the logon task that runs update-config.ps1: 2 minutes after logon,
# only with a network connection. Re-run to update the task.
$ErrorActionPreference = "Stop"
$script = Join-Path $PSScriptRoot "update-config.ps1"
$user   = "$env:USERDOMAIN\$env:USERNAME"
$action = New-ScheduledTaskAction -Execute "powershell.exe" `
  -Argument "-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$script`""
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $user
$trigger.Delay = "PT2M"
$settings = New-ScheduledTaskSettingsSet -RunOnlyIfNetworkAvailable -StartWhenAvailable `
  -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Hours 1) -MultipleInstances IgnoreNew
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
Register-ScheduledTask -TaskName "agent-config auto-update" -Action $action -Trigger $trigger `
  -Settings $settings -Principal $principal -Force `
  -Description "Checks coding-agent benchmarks and updates C:\Users\walbe\Code\agent-config (local commits only)." | Out-Null
"registered: agent-config auto-update (logon +2 min, network only)"

# agentcfg: link handler so a click on a toast opens its HTML report (current user only).
$key = "HKCU:\Software\Classes\agentcfg"
New-Item -Force $key | Out-Null
Set-ItemProperty $key "(default)" "URL:agent-config report"
Set-ItemProperty $key "URL Protocol" ""
New-Item -Force "$key\shell\open\command" | Out-Null
$handler = Join-Path $PSScriptRoot "open-report.ps1"
Set-ItemProperty "$key\shell\open\command" "(default)" "powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$handler`" `"%1`""
"registered: agentcfg: link handler"
