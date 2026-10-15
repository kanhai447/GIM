param(
    [ValidateSet('full', 'focused', 'integration')]
    [string]$Scope = 'full'
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$envFile = Join-Path $repoRoot '.env.compose.local'
if (-not (Test-Path -LiteralPath $envFile)) {
    throw '.env.compose.local is required and must remain untracked'
}

$packages = './...'
if ($Scope -eq 'focused') {
    $packages = './internal/chat/... ./internal/group/... ./internal/file/... ./internal/platform/audit/... ./internal/platform/kafkaaudit/... ./internal/logs/... ./tools/loadtest'
}

$testCommand = '/usr/local/go/bin/go test -race ' + $packages + ' -count=1'
if ($Scope -eq 'integration') {
    $testCommand = @'
awk -F= '{
  if ($1 == "MYSQL_USER") { print "MYSQL_USER=root" }
  else if ($1 == "MYSQL_PASSWORD") { print "MYSQL_PASSWORD=" ENVIRON["MYSQL_ROOT_PASSWORD"] }
  else { print $0 }
}' /workspace/.env.compose.local > /tmp/gim-race.env
GIM_ENV_FILE=/tmp/gim-race.env GIM_KAFKA_INTEGRATION=1 /usr/local/go/bin/go test -race ./tests -count=1
'@
}

$arguments = @(
    'run', '--rm',
    '--network', 'gim',
    '--env-file', $envFile,
    '-v', ($repoRoot + ':/workspace'),
    '-v', 'gim-race-gomod:/go/pkg/mod',
    '-v', 'gim-race-gobuild:/root/.cache/go-build',
    '-w', '/workspace/server',
    'golang:1.25-bookworm',
    'bash', '-lc', $testCommand
)

& docker @arguments
if ($LASTEXITCODE -ne 0) {
    throw "Linux Docker race failed for scope $Scope"
}
