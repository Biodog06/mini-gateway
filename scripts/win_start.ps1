<#
.SYNOPSIS
    Windows 启动脚本，用于启动 mini-gateway 的开发和测试环境。
.DESCRIPTION
    此脚本模拟了 Makefile 中 start-envs 的行为，适配 Windows 环境。
    它会启动 Redis, Consul, Elasticsearch, Kibana, Grafana, Prometheus, Jaeger 和 Filebeat。
    并自动配置 Grafana 数据源和 Dashboard。
.EXAMPLE
    .\scripts\win_start.ps1
#>

$ErrorActionPreference = "Stop"

# 获取脚本所在目录的上一级目录（即项目根目录）
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir

# 切换到项目根目录
Push-Location $ProjectRoot

try {
    Write-Host "正在启动测试环境 (Windows)..." -ForegroundColor Green

    $DockerComposeFile = "test/docker/docker-compose.yml"

    # 检查 docker-compose 是否可用
    if (-not (Get-Command "docker-compose" -ErrorAction SilentlyContinue)) {
        Write-Error "未找到 docker-compose 命令，请先安装 Docker Desktop。"
        exit 1
    }

    # 1. 启动基础服务
    Write-Host "`n[1/4] 启动基础服务 (Redis, Consul, ES, Kibana)..." -ForegroundColor Cyan
    docker-compose -f $DockerComposeFile up -d mg-redis mg-consul mg-elasticsearch mg-kibana

    # 2. 配置 Grafana
    Write-Host "`n[2/4] 配置 Grafana..." -ForegroundColor Cyan
    $GrafanaProvDir = Join-Path "test" "docker\grafana\provisioning"
    $DashboardSource = Join-Path "test" "docker\prometheus.dashboard.json"
    $DashboardDestDir = Join-Path "test" "docker\grafana\dashboards"
    $DashboardDest = Join-Path $DashboardDestDir "gateway.json"

    # 创建必要目录
    $Dirs = @(
        $DashboardDestDir,
        (Join-Path $GrafanaProvDir "datasources"),
        (Join-Path $GrafanaProvDir "dashboards")
    )
    foreach ($Dir in $Dirs) {
        if (-not (Test-Path $Dir)) {
            New-Item -ItemType Directory -Force -Path $Dir | Out-Null
            Write-Host "  已创建目录: $Dir" -ForegroundColor Gray
        }
    }

    # 生成数据源配置
    $DatasourceContent = @"
apiVersion: 1
datasources:
  - name: mini-gateway-Prometheus
    type: prometheus
    url: http://127.0.0.1:8390
    access: proxy
    isDefault: true
    editable: false
"@
    Set-Content -Path (Join-Path $GrafanaProvDir "datasources\datasource.yml") -Value $DatasourceContent -Encoding Ascii

    # 生成 Dashboard Provider 配置
    $DashboardProvContent = @"
apiVersion: 1
providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    options:
      path: /var/lib/grafana/dashboards
"@
    Set-Content -Path (Join-Path $GrafanaProvDir "dashboards\dashboards.yml") -Value $DashboardProvContent -Encoding Ascii

    # 生成 Preferences 配置
    $PrefContent = @"
apiVersion: 1
preferences:
  homeDashboardUID: "gateway-monitoring"
"@
    Set-Content -Path (Join-Path $GrafanaProvDir "preferences.yml") -Value $PrefContent -Encoding Ascii

    # 复制 Dashboard 文件
    if (Test-Path $DashboardSource) {
        Copy-Item -Path $DashboardSource -Destination $DashboardDest -Force
        Write-Host "  Dashboard 文件已配置" -ForegroundColor Gray
    } else {
        Write-Warning "  未找到 Dashboard 源文件: $DashboardSource"
    }

    # 3. 启动监控服务
    Write-Host "`n[3/4] 启动监控服务 (Grafana, Prometheus, Jaeger)..." -ForegroundColor Cyan
    docker-compose -f $DockerComposeFile up -d mg-grafana mg-prometheus mg-jaeger

    # 4. 启动 Filebeat
    Write-Host "`n[4/4] 启动 Filebeat..." -ForegroundColor Cyan
    docker-compose -f $DockerComposeFile up -d mg-filebeat

    # 获取本机 IP 用于显示链接
    $IP = "127.0.0.1"
    try {
        $NetIP = (Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.InterfaceAlias -notlike "*Loopback*" -and $_.IPAddress -notlike "169.254.*" } | Select-Object -ExpandProperty IPAddress -First 1)
        if ($NetIP) { $IP = $NetIP }
    } catch {
        # 忽略获取 IP 失败的错误，使用 localhost
    }

    Write-Host "`n===============================================" -ForegroundColor Green
    Write-Host "环境启动完成！" -ForegroundColor Green
    Write-Host "服务访问地址："
    Write-Host "  - Jaeger UI:        http://$($IP):8330"
    Write-Host "  - Jaeger OTLP HTTP: http://$($IP):8331"
    Write-Host "  - Grafana:          http://$($IP):8350/d/gateway-monitoring (login: admin/admin123)"
    Write-Host "  - Prometheus:       http://$($IP):8390"
    Write-Host "  - Kibana:           http://$($IP):5601"
    Write-Host "===============================================" -ForegroundColor Green

} finally {
    # 恢复目录
    Pop-Location
}
