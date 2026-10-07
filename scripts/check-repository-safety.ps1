[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$failures = [System.Collections.Generic.List[string]]::new()

function Add-Failure {
    param([string]$Message)

    $script:failures.Add($Message)
    Write-Output "FAIL: $Message"
}

Push-Location $repositoryRoot
try {
    if ((git rev-parse --is-inside-work-tree 2>$null) -ne 'true') {
        Add-Failure 'repository is not a Git worktree'
    }

    $ignoreTargets = [ordered]@{
        'reference/'    = 'reference/fim_server-main/fim_server-main/go.mod'
        '.env.local'    = '.env.local'
        '*.key'         = 'server/config/repository-safety.key'
        '*.pem'         = 'server/config/repository-safety.pem'
        'node_modules/' = 'web/node_modules/repository-safety/package.json'
        'dist/'         = 'admin/dist/repository-safety.js'
        'logs/'         = 'logs/repository-safety.log'
        'uploads/'      = 'server/uploads/repository-safety.bin'
        'local data'    = '.local-data/mysql/repository-safety'
    }

    foreach ($entry in $ignoreTargets.GetEnumerator()) {
        git check-ignore -q -- $entry.Value
        if ($LASTEXITCODE -eq 0) {
            Write-Output "PASS: ignore rule $($entry.Key)"
        } else {
            Add-Failure "missing ignore coverage for $($entry.Key)"
        }
    }

    if (-not (Test-Path -LiteralPath '.env.local' -PathType Leaf)) {
        Add-Failure '.env.local is missing'
    } else {
        $config = @{}
        foreach ($line in [System.IO.File]::ReadAllLines((Resolve-Path '.env.local'))) {
            $trimmed = $line.Trim()
            if (-not $trimmed -or $trimmed.StartsWith('#')) {
                continue
            }

            $separator = $trimmed.IndexOf('=')
            if ($separator -lt 1) {
                continue
            }

            $key = $trimmed.Substring(0, $separator).Trim()
            $value = $trimmed.Substring($separator + 1).Trim()
            if (($value.StartsWith('"') -and $value.EndsWith('"')) -or
                ($value.StartsWith("'") -and $value.EndsWith("'"))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            $config[$key] = $value
        }

        $requiredKeys = @(
            'MYSQL_HOST',
            'MYSQL_PORT',
            'MYSQL_USER',
            'MYSQL_PASSWORD',
            'MYSQL_DATABASE',
            'REDIS_HOST',
            'REDIS_PORT',
            'REDIS_PASSWORD',
            'ETCD_ENDPOINTS',
            'JWT_SECRET',
            'GATEWAY_PORT'
        )

        foreach ($key in $requiredKeys) {
            if (-not $config.ContainsKey($key) -or
                [string]::IsNullOrWhiteSpace($config[$key]) -or
                $config[$key] -eq 'CHANGE_ME') {
                Add-Failure "local configuration is incomplete: $key"
            } elseif ($key -match 'PASSWORD|SECRET') {
                Write-Output "PASS: $key=[REDACTED]"
            } else {
                Write-Output "PASS: $key is configured"
            }
        }

        if ($config.ContainsKey('JWT_SECRET')) {
            $secretByteCount = [System.Text.Encoding]::UTF8.GetByteCount($config['JWT_SECRET'])
            if ($secretByteCount -lt 32) {
                Add-Failure 'JWT_SECRET is shorter than 32 bytes'
            } else {
                Write-Output 'PASS: JWT_SECRET has at least 32 bytes [REDACTED]'
            }
        }
    }

    $forbiddenPaths = @(
        git ls-files |
            Where-Object {
                $_ -eq '.env.local' -or
                $_ -like 'reference/*' -or
                $_ -like '*.key' -or
                $_ -like '*.pem' -or
                $_ -like '*.p12' -or
                $_ -like '*.pfx'
            }
    )

    if ($forbiddenPaths.Count -gt 0) {
        foreach ($path in $forbiddenPaths) {
            Add-Failure "forbidden tracked path: $path"
        }
    } else {
        Write-Output 'PASS: no forbidden paths are tracked'
    }

    if ($failures.Count -gt 0) {
        Write-Output "Repository safety check failed with $($failures.Count) issue(s)."
        exit 1
    }

    Write-Output 'Repository safety check passed.'
} finally {
    Pop-Location
}
