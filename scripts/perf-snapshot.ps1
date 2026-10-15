param(
    [string]$Label = 'snapshot'
)

$ErrorActionPreference = 'Stop'

Write-Output "label=$Label timestamp=$([DateTimeOffset]::Now.ToString('o'))"
Write-Output '=== docker stats ==='
docker stats --no-stream --format 'table {{.Name}}`t{{.CPUPerc}}`t{{.MemUsage}}`t{{.PIDs}}'

Write-Output '=== restart / oom ==='
docker compose ps -q | ForEach-Object {
    docker inspect --format '{{.Name}} RestartCount={{.RestartCount}} OOMKilled={{.State.OOMKilled}} Health={{if .State.Health}}{{.State.Health.Status}}{{else}}n/a{{end}}' $_
}

Write-Output '=== internal runtime metrics ==='
foreach ($container in @('gim-chat-api-1', 'gim-group-api-1', 'gim-file-api-1', 'gim-logs-api-1')) {
    Write-Output "[$container]"
    docker exec $container wget -q -O - http://127.0.0.1:6060/debug/metrics
}

Write-Output '=== mysql connections ==='
docker exec gim-mysql-1 sh -c 'mysql -N -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e "SHOW GLOBAL STATUS WHERE Variable_name IN (''Threads_connected'',''Threads_running'',''Connections'',''Aborted_connects'');"'

Write-Output '=== redis clients ==='
docker exec gim-redis-1 sh -c 'REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli INFO clients | grep -E "^(connected_clients|blocked_clients|tracking_clients):"'

Write-Output '=== kafka consumer group ==='
docker exec gim-kafka-1 /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server 127.0.0.1:29092 --describe --group gim-logs-v1
