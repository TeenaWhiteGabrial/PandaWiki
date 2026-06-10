#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_DIR="${ROOT_DIR}/.pids"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.local.yml"
DEPENDENCY_SERVICES=(
  postgres
  redis
  nats
  minio
  qdrant
  crawler
  raglite
)

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

print_pid_file_status() {
  local label="$1"
  local pid_file="$2"

  if [[ ! -f "${pid_file}" ]]; then
    echo "  ${label}: 未托管（缺少 PID 文件）"
    return 0
  fi

  local pid
  pid="$(cat "${pid_file}")"
  if kill -0 "${pid}" >/dev/null 2>&1; then
    echo "  ${label}: 运行中 (pid=${pid})"
  else
    echo "  ${label}: PID 文件残留 (pid=${pid})"
  fi
}

print_docker_status() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "  Docker 未安装，无法检查容器状态"
    return 0
  fi

  if ! docker compose -f "${COMPOSE_FILE}" ps "${DEPENDENCY_SERVICES[@]}"; then
    echo "  无法读取 Docker 容器状态，请确认 Docker Desktop 已启动且当前用户有权限访问 Docker socket"
  fi
}

echo "==> Docker 依赖"
print_docker_status

echo
echo "==> 脚本托管进程"
mkdir -p "${PID_DIR}"
print_pid_file_status "backend-api" "${PID_DIR}/backend-api.pid"
print_pid_file_status "backend-consumer" "${PID_DIR}/backend-consumer.pid"
print_pid_file_status "web-dev" "${PID_DIR}/web-dev.pid"

echo
echo "==> 关键端口"
print_port_status "backend-api" 8000
print_port_status "admin-web" 5173
print_port_status "app-web" 3010
print_port_status "postgres" 15432
print_port_status "redis" 16379
print_port_status "nats" 4222
print_port_status "minio" 9000
print_port_status "crawler" 8080
print_port_status "raglite" 5050

echo
echo "==> 日志文件"
echo "  ${ROOT_DIR}/.logs/backend-api.log"
echo "  ${ROOT_DIR}/.logs/backend-consumer.log"
echo "  ${ROOT_DIR}/.logs/web-dev.log"
