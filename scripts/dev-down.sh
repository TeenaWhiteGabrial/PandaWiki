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

stop_by_pid_file() {
  local name="$1"
  local pid_file="$2"

  if [[ ! -f "${pid_file}" ]]; then
    echo "${name} 未记录 PID，跳过"
    return 0
  fi

  local pid
  pid="$(cat "${pid_file}")"
  if kill -0 "${pid}" >/dev/null 2>&1; then
    echo "停止 ${name} (PID=${pid})"
    kill "${pid}" >/dev/null 2>&1 || true
    for _ in {1..10}; do
      if ! kill -0 "${pid}" >/dev/null 2>&1; then
        break
      fi
      sleep 1
    done
    if kill -0 "${pid}" >/dev/null 2>&1; then
      echo "${name} 未在预期时间内退出，发送 SIGKILL"
      kill -9 "${pid}" >/dev/null 2>&1 || true
    fi
  else
    echo "${name} 进程不存在，清理 PID 文件"
  fi

  rm -f "${pid_file}"
}

mkdir -p "${PID_DIR}"

stop_by_pid_file "web-dev" "${PID_DIR}/web-dev.pid"
stop_by_pid_file "backend-consumer" "${PID_DIR}/backend-consumer.pid"
stop_by_pid_file "backend-api" "${PID_DIR}/backend-api.pid"

echo "停止 Docker 依赖"
cd "${ROOT_DIR}"
docker compose -f "${COMPOSE_FILE}" stop "${DEPENDENCY_SERVICES[@]}" >/dev/null 2>&1 || true
docker compose -f "${COMPOSE_FILE}" rm -f "${DEPENDENCY_SERVICES[@]}" >/dev/null 2>&1 || true

echo "本地开发服务已停止。"
