#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/dev-env.sh"

load_external_oss_env

find_listening_pid() {
  local port="$1"
  lsof -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null | head -n 1
}

print_port_status() {
  local label="$1"
  local port="$2"
  local pid

  pid="$(find_listening_pid "${port}")"
  if [[ -n "${pid}" ]]; then
    echo "  ${label}: 监听中 (port=${port}, pid=${pid})"
  else
    echo "  ${label}: 未监听 (port=${port})"
  fi
}

print_process_status() {
  local label="$1"
  local pattern="$2"
  local pid

  pid="$(pgrep -f "${pattern}" 2>/dev/null | head -n 1 || true)"
  if [[ -n "${pid}" ]]; then
    echo "  ${label}: 运行中 (pid=${pid})"
  else
    echo "  ${label}: 未检测到"
  fi
}

print_docker_status() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "  Docker 未安装，无法检查容器状态"
    return 0
  fi

  if ! docker compose -f "${PANDA_WIKI_COMPOSE_FILE}" ps "${PANDA_WIKI_DEPENDENCY_SERVICES[@]}"; then
    echo "  无法读取 Docker 容器状态，请确认 Docker Desktop 已启动且当前用户有权限访问 Docker socket"
  fi
}

echo "==> Docker 基础依赖"
print_docker_status

echo
echo "==> 本机命令进程"
print_process_status "backend-api" "cmd/api/main.go"
print_process_status "backend-consumer" "cmd/consumer/main.go"
print_process_status "web-dev" "pnpm --parallel --filter panda-wiki-admin --filter panda-wiki-app dev"

echo
echo "==> 关键端口"
print_port_status "backend-api" 8000
print_port_status "admin-web" 5173
print_port_status "app-web" 3010
print_port_status "postgres" 15432
print_port_status "redis" 16379
print_port_status "nats" 4222
print_port_status "qdrant" 6333
print_port_status "crawler" 8080
print_port_status "raglite" 5050
print_port_status "loki" 13100

echo
echo "==> 常用启动命令"
echo "  ./scripts/dev-deps.sh"
echo "  ./scripts/dev-api.sh"
echo "  ./scripts/dev-consumer.sh"
echo "  ./scripts/dev-web.sh"
