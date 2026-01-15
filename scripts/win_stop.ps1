<#
.SYNOPSIS
    Windows 停止脚本，用于停止 mini-gateway 的开发环境。
.DESCRIPTION
    此脚本模拟了 Makefile 中 stop-envs 的行为。
    它会停止 Docker 容器并清理相关的数据卷。
.EXAMPLE
    .\scripts\win_stop.ps1
#>

$ErrorActionPreference = "Stop"

# 获取脚本所在目录的上一级目录（即项目根目录）
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir

# 切换到项目根目录
Push-Location $ProjectRoot

try {
    Write-Host "正在停止服务..." -ForegroundColor Yellow
    
    $DockerComposeFile = "test/docker/docker-compose.yml"
    
    if (-not (Get-Command "docker-compose" -ErrorAction SilentlyContinue)) {
        Write-Error "未找到 docker-compose 命令。"
        exit 1
    }

    docker-compose -f $DockerComposeFile down

    Write-Host "`n正在清理数据卷..." -ForegroundColor Yellow
    $Volumes = @(
        "docker_mg-consul-data",
        "docker_mg-grafana-data",
        "docker_mg-redis-data",
        "docker_mg-prometheus-data",
        "docker_mg-jaeger-data",
        "docker_mg-elasticsearch-data",
        "docker_mg-filebeat-data"
    )

    foreach ($Vol in $Volumes) {
        # 检查卷是否存在，如果存在则删除
        if (docker volume ls -q -f name=$Vol) {
            docker volume rm $Vol | Out-Null
            Write-Host "  已删除卷: $Vol" -ForegroundColor Gray
        }
    }

    Write-Host "`n服务已停止且数据已清理。" -ForegroundColor Green

} finally {
    Pop-Location
}
