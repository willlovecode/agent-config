# Installs agtk on Windows (x64 and arm64) to %USERPROFILE%\.agtk\bin\agtk.exe.
# No admin rights, no Go needed. It downloads the release archive, checks it against
# SHA256SUMS, and prints the command that adds the folder to your user PATH; it does not run it.
# Works in Windows PowerShell 5.1 and PowerShell 7. This file is ASCII on purpose.
#
#   irm https://raw.githubusercontent.com/willlovecode/agent-config/master/get-agtk.ps1 | iex
#
# Environment variables:
#   AGTK_VERSION   release to install, e.g. 1.2.3 or v1.2.3 (default: the latest release)
#   AGTK_BASE_URL  folder holding SHA256SUMS and the archives (default: the GitHub release)
#   AGTK_FILE      install this archive (.zip) instead of downloading (offline or private repo)
#   AGTK_SUMS      SHA256SUMS file to verify AGTK_FILE against (without it AGTK_FILE is unverified)

function Install-Agtk {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    $repo = 'willlovecode/agent-config'

    # Old Windows PowerShell defaults to TLS 1.0/1.1, which GitHub refuses.
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

    $cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    switch ($cpu) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'arm64' }
        default { throw "unsupported CPU '$cpu': releases exist for x64 and arm64" }
    }

    function Test-Archive($archive, $sums) {
        $name = Split-Path -Leaf $archive
        $want = $null
        foreach ($line in Get-Content -LiteralPath $sums) {
            $parts = $line.Trim() -split '\s+', 2
            if ($parts.Count -eq 2 -and $parts[1].TrimStart('*') -eq $name) { $want = $parts[0]; break }
        }
        if (-not $want) { throw "$name is not listed in $sums" }
        $have = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash
        if ($have -ne $want) { throw "checksum mismatch for $name (expected $want, got $have); not installing" }
        Write-Host "Checksum ok: $name"
    }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('get-agtk-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        if ($env:AGTK_FILE) {
            $archive = $env:AGTK_FILE
            if (-not (Test-Path -LiteralPath $archive -PathType Leaf)) { throw "AGTK_FILE $archive does not exist" }
            if ($env:AGTK_SUMS) {
                if (-not (Test-Path -LiteralPath $env:AGTK_SUMS -PathType Leaf)) { throw "AGTK_SUMS $($env:AGTK_SUMS) does not exist" }
                Test-Archive $archive $env:AGTK_SUMS
            } else {
                Write-Host "Warning: AGTK_SUMS not set, so $archive is not verified."
            }
        } else {
            if ($env:AGTK_BASE_URL) {
                $base = $env:AGTK_BASE_URL.TrimEnd('/')
            } elseif ($env:AGTK_VERSION) {
                $base = "https://github.com/$repo/releases/download/v$($env:AGTK_VERSION.TrimStart('v'))"
            } else {
                $base = "https://github.com/$repo/releases/latest/download"
            }
            Write-Host "Downloading agtk for windows/$arch from $base"
            $sums = Join-Path $tmp 'SHA256SUMS'
            try {
                Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $sums
            } catch {
                throw "cannot download $base/SHA256SUMS ($($_.Exception.Message)). If the repository is private or you are offline, download the zip by hand and set AGTK_FILE and AGTK_SUMS."
            }
            # The asset name carries the version, so find it in the checksum list.
            $suffix = "_windows_$arch.zip"
            $name = $null
            foreach ($line in Get-Content -LiteralPath $sums) {
                $parts = $line.Trim() -split '\s+', 2
                if ($parts.Count -eq 2 -and $parts[1].TrimStart('*').EndsWith($suffix)) { $name = $parts[1].TrimStart('*'); break }
            }
            if (-not $name) { throw "no windows/$arch archive listed in $base/SHA256SUMS" }
            $archive = Join-Path $tmp $name
            try {
                Invoke-WebRequest -UseBasicParsing -Uri "$base/$name" -OutFile $archive
            } catch {
                throw "cannot download $base/$name ($($_.Exception.Message))"
            }
            Test-Archive $archive $sums
        }

        $extract = Join-Path $tmp 'x'
        Expand-Archive -LiteralPath $archive -DestinationPath $extract
        $exe = Join-Path $extract 'agtk.exe'
        if (-not (Test-Path -LiteralPath $exe)) { throw "$archive does not contain agtk.exe" }

        $bin = Join-Path $env:USERPROFILE '.agtk\bin'
        New-Item -ItemType Directory -Force -Path $bin | Out-Null
        $target = Join-Path $bin 'agtk.exe'
        try {
            Copy-Item -LiteralPath $exe -Destination $target -Force
        } catch {
            throw "cannot write $target ($($_.Exception.Message)). Close any running agtk and try again."
        }

        $version = & $target version 2>$null
        $ran = ($LASTEXITCODE -eq 0) -and $version
        if ($ran) { Write-Host "Installed agtk $version to $target" } else { Write-Host "Installed $target, but it did not run here." }

        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $onPath = ($userPath -split ';') + ($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -ieq $bin }
        $cmd = 'agtk'
        if ($onPath) {
            Write-Host "$bin is already on your PATH."
        } else {
            $cmd = $target
            Write-Host ''
            Write-Host "$bin is not on your PATH yet. To add it to your user PATH (this script does not change it), run:"
            Write-Host "  [Environment]::SetEnvironmentVariable('Path', '$bin;' + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')"
            Write-Host 'then open a new terminal.'
        }
        # Offer the wizard when someone is at the console. It is started by full path after the
        # temp files are gone, so PATH need not be set yet.
        $answer = $false
        try {
            if ($ran -and [Environment]::UserInteractive -and -not [Console]::IsInputRedirected -and -not [Console]::IsOutputRedirected) {
                Write-Host ''
                Write-Host -NoNewline 'Run the setup wizard now? [Y/n] '
                $key = [Console]::ReadKey($true)
                Write-Host ($(if ($key.Key -eq 'Enter') { '' } else { $key.KeyChar }))
                $answer = ($key.Key -eq 'Enter') -or ($key.KeyChar -eq 'y') -or ($key.KeyChar -eq 'Y')
            }
        } catch { }
        if ($answer) {
            $script:AgtkWizard = $target
        } else {
            Write-Host ''
            Write-Host "Next: $cmd wizard"
        }
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }
}

$script:AgtkWizard = $null
Install-Agtk
if ($script:AgtkWizard) { & $script:AgtkWizard wizard }
