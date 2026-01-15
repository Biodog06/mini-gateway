#!/bin/bash
set -e

# 确保脚本在项目根目录运行
# 假设脚本位于 scripts/ 目录下，所以跳到上一级
cd "$(dirname "$0")/.."

echo "Starting test environment in WSL..."

# 修复可能的换行符问题
if [ -f "test/docker/setup_grafana.sh" ]; then
    sed -i 's/\r$//' test/docker/setup_grafana.sh
    chmod +x test/docker/setup_grafana.sh
fi

if [ -f "test/docker/setup_monitoring.sh" ]; then
    sed -i 's/\r$//' test/docker/setup_monitoring.sh
    chmod +x test/docker/setup_monitoring.sh
fi

echo "Starting Redis, Consul, Elasticsearch, Kibana..."
docker-compose -f test/docker/docker-compose.yml up -d mg-redis mg-consul mg-elasticsearch mg-kibana

echo "Starting monitoring (Grafana|Prometheus|Jaeger)..."
./test/docker/setup_monitoring.sh

echo "Starting Filebeat..."
docker-compose -f test/docker/docker-compose.yml up -d mg-filebeat

echo "Environment started successfully!"
