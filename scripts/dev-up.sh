#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/.logs"
PID_DIR="${ROOT_DIR}/.pids"
LOCAL_DATA_DIR="${ROOT_DIR}/.local-data"
SSL_DIR="${LOCAL_DATA_DIR}/ssl"

mkdir -p \
  "${LOG_DIR}" \
  "${PID_DIR}" \
  "${ROOT_DIR}/.cache/go-build" \
  "${ROOT_DIR}/.cache/gomod" \
  "${SSL_DIR}"

GO_BIN="/opt/homebrew/opt/go@1.24/bin/go"
if [[ ! -x "${GO_BIN}" ]]; then
  GO_BIN="$(command -v go || true)"
fi

if [[ -z "${GO_BIN}" ]]; then
  echo "未找到 Go，请先安装 Go 1.24。"
  exit 1
fi

start_if_not_running() {
  local name="$1"
  local pid_file="$2"
  local log_file="$3"
  shift 3

  if [[ -f "${pid_file}" ]]; then
    local pid
    pid="$(cat "${pid_file}")"
    if kill -0 "${pid}" >/dev/null 2>&1; then
      echo "${name} 已在运行，PID=${pid}"
      return 0
    fi
    rm -f "${pid_file}"
  fi

  echo "启动 ${name}..."
  nohup "$@" >"${log_file}" 2>&1 &
  echo $! >"${pid_file}"
  sleep 2

  local new_pid
  new_pid="$(cat "${pid_file}")"
  if ! kill -0 "${new_pid}" >/dev/null 2>&1; then
    echo "${name} 启动失败，请查看日志：${log_file}"
    exit 1
  fi
  echo "${name} 已启动，PID=${new_pid}"
}

echo "==> 启动 Docker 依赖"
cd "${ROOT_DIR}"
docker compose -f docker-compose.local.yml up -d

echo "==> 启动后端 API"
cd "${ROOT_DIR}/backend/store/pg"
start_if_not_running \
  "backend-api" \
  "${PID_DIR}/backend-api.pid" \
  "${LOG_DIR}/backend-api.log" \
  env \
  PATH="/opt/homebrew/opt/go@1.24/bin:${PATH}" \
  GOCACHE="${ROOT_DIR}/.cache/go-build" \
  GOMODCACHE="${ROOT_DIR}/.cache/gomod" \
  GOPROXY="https://goproxy.cn,direct" \
  ENV="local" \
  LOG_LEVEL="-4" \
  ADMIN_PASSWORD="admin123" \
  JWT_SECRET="dev-jwt-secret" \
  PG_DSN="host=127.0.0.1 user=panda-wiki password=panda-wiki-secret dbname=panda-wiki port=15432 sslmode=disable TimeZone=Asia/Shanghai" \
  MQ_NATS_SERVER="nats://127.0.0.1:4222" \
  REDIS_ADDR="127.0.0.1:16379" \
  S3_ENDPOINT="127.0.0.1:9000" \
  S3_SECRET_KEY="panda-wiki-secret" \
  RAG_CT_RAG_BASE_URL="http://127.0.0.1:5050" \
  SENTRY_ENABLED="false" \
  CADDY_API="" \
  FILE_BASE_URL="http://127.0.0.1:8000/static-file" \
  ANYDOC_BASE_URL="http://127.0.0.1:8080" \
  PANDA_WIKI_MACHINE_ID_FILE="${LOCAL_DATA_DIR}/.machine_id" \
  PANDA_WIKI_SSL_DIR="${SSL_DIR}" \
  "${GO_BIN}" run ../../cmd/api/main.go ../../cmd/api/wire_gen.go

echo "==> 启动 consumer"
start_if_not_running \
  "backend-consumer" \
  "${PID_DIR}/backend-consumer.pid" \
  "${LOG_DIR}/backend-consumer.log" \
  env \
  PATH="/opt/homebrew/opt/go@1.24/bin:${PATH}" \
  GOCACHE="${ROOT_DIR}/.cache/go-build" \
  GOMODCACHE="${ROOT_DIR}/.cache/gomod" \
  GOPROXY="https://goproxy.cn,direct" \
  ENV="local" \
  LOG_LEVEL="-4" \
  PG_DSN="host=127.0.0.1 user=panda-wiki password=panda-wiki-secret dbname=panda-wiki port=15432 sslmode=disable TimeZone=Asia/Shanghai" \
  MQ_NATS_SERVER="nats://127.0.0.1:4222" \
  REDIS_ADDR="127.0.0.1:16379" \
  S3_ENDPOINT="127.0.0.1:9000" \
  S3_SECRET_KEY="panda-wiki-secret" \
  RAG_CT_RAG_BASE_URL="http://127.0.0.1:5050" \
  SENTRY_ENABLED="false" \
  FILE_BASE_URL="http://127.0.0.1:8000/static-file" \
  ANYDOC_BASE_URL="http://127.0.0.1:8080" \
  PANDA_WIKI_MACHINE_ID_FILE="${LOCAL_DATA_DIR}/.machine_id" \
  PANDA_WIKI_SSL_DIR="${SSL_DIR}" \
  "${GO_BIN}" run ../../cmd/consumer/main.go ../../cmd/consumer/wire_gen.go

echo "==> 启动前端工作区"
cd "${ROOT_DIR}/web"
start_if_not_running \
  "web-dev" \
  "${PID_DIR}/web-dev.pid" \
  "${LOG_DIR}/web-dev.log" \
  pnpm dev

echo
echo "启动完成："
echo "  管理台: http://localhost:5173"
echo "  前台:   http://localhost:3010"
echo "  后端:   http://127.0.0.1:8000"
echo
echo "日志文件："
echo "  ${LOG_DIR}/backend-api.log"
echo "  ${LOG_DIR}/backend-consumer.log"
echo "  ${LOG_DIR}/web-dev.log"
