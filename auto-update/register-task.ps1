# Registers the logon task that runs update-config.ps1: 2 minutes after logon,
# only with a network connection. Re-run to update the task.
$ErrorActionPreference = "Stop"
$script = Join-Path $PSScriptRoot "update-config.ps1"
$user   = "$env:USERDOMAIN\$env:USERNAME"
# Two actions, run in order: the benchmark check, then the weekly skills check.
$action = foreach ($s in $script, (Join-Path $PSScriptRoot "skills-check.ps1")) {
  New-ScheduledTaskAction -Execute "powershell.exe" `
    -Argument "-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$s`""
}
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $user
$trigger.Delay = "PT2M"
$settings = New-ScheduledTaskSettingsSet -RunOnlyIfNetworkAvailable -StartWhenAvailable `
  -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Hours 1) -MultipleInstances IgnoreNew
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
Register-ScheduledTask -TaskName "agent-config auto-update" -Action $action -Trigger $trigger `
  -Settings $settings -Principal $principal -Force `
  -Description "Checks coding-agent benchmarks and updates C:\Users\walbe\Code\agent-config (local commits only)." | Out-Null
"registered: agent-config auto-update (logon +2 min, network only)"
