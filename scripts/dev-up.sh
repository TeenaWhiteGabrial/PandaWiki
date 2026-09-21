#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/.logs"
PID_DIR="${ROOT_DIR}/.pids"
LOCAL_DATA_DIR="${ROOT_DIR}/.local-data"
SSL_DIR="${LOCAL_DATA_DIR}/ssl"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.local.yml"
DEPENDENCY_SERVICES=(
  postgres
  redis
  nats
  minio
  qdrant
  crawler
  raglite
  loki
  alloy
)
DEPENDENCY_PORTS=(
  15432
  16379
  4222
  9000
  6333
  8080
  5050
  13100
)

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

if ! command -v docker >/dev/null 2>&1; then
  echo "未找到 Docker，请先安装并启动 Docker Desktop。"
  exit 1
fi

if ! command -v pnpm >/dev/null 2>&1; then
  echo "未找到 pnpm，请先安装 pnpm。"
  exit 1
fi

find_listening_pid() {
  local port="$1"
  lsof -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null | head -n 1
}

is_process_running() {
  local pattern="$1"
  pgrep -f "${pattern}" >/dev/null 2>&1
}

is_port_listening() {
  local port="$1"
  [[ -n "$(find_listening_pid "${port}")" ]]
}

is_service_running() {
  local probe_type="$1"
  local probe_target="$2"

  case "${probe_type}" in
    port)
      is_port_listening "${probe_target}"
      ;;
    pattern)
      is_process_running "${probe_target}"
      ;;
    ports)
      local port
      IFS=',' read -r -a ports <<<"${probe_target}"
      for port in "${ports[@]}"; do
        if ! is_port_listening "${port}"; then
          return 1
        fi
      done
      return 0
      ;;
    none)
      return 1
      ;;
    *)
      echo "未知探测类型: ${probe_type}"
      exit 1
      ;;
  esac
}

validate_frontend_ports() {
  local admin_running=0
  local app_running=0

  if is_port_listening 5173; then
    admin_running=1
  fi
  if is_port_listening 3010; then
    app_running=1
  fi

  if [[ "${admin_running}" -eq 1 && "${app_running}" -eq 0 ]]; then
    echo "检测到管理台端口 5173 已在运行，前台端口 3010 尚未启动，将继续尝试启动前端工作区。"
  fi

  if [[ "${admin_running}" -eq 0 && "${app_running}" -eq 1 ]]; then
    echo "检测到前台端口 3010 已在运行，管理台端口 5173 尚未启动，将继续尝试启动前端工作区。"
  fi
}

validate_no_containerized_app_conflicts() {
  if is_port_listening 8000 && ! is_process_running "cmd/api/main.go"; then
    echo "检测到 8000 端口已被非本地 go run 的后端占用。"
    echo "这通常表示旧的容器化 API 仍在运行，请先停止该应用容器后再启动本地开发。"
    echo "可先执行: docker stop pandawiki-api pandawiki-consumer"
    exit 1
  fi
}

all_dependency_ports_ready() {
  local port
  for port in "${DEPENDENCY_PORTS[@]}"; do
    if ! is_port_listening "${port}"; then
      return 1
    fi
  done
  return 0
}

start_if_not_running() {
  local name="$1"
  local pid_file="$2"
  local log_file="$3"
  local probe_type="$4"
  local probe_target="$5"
  local startup_port="${6:-}"
  if [[ $# -ge 6 ]]; then
    shift 6
  else
    shift 5
  fi

  if [[ -f "${pid_file}" ]]; then
    local pid
    pid="$(cat "${pid_file}")"
    if kill -0 "${pid}" >/dev/null 2>&1; then
      echo "${name} 已在运行，PID=${pid}"
      return 0
    fi
    rm -f "${pid_file}"
  fi

  if is_service_running "${probe_type}" "${probe_target}"; then
    echo "${name} 已在运行（非脚本托管），跳过启动"
    return 0
  fi

  echo "启动 ${name}..."
  nohup "$@" >"${log_file}" 2>&1 &
  echo $! >"${pid_file}"
  sleep 2

  local new_pid
  new_pid="$(cat "${pid_file}")"
  if ! kill -0 "${new_pid}" >/dev/null 2>&1; then
    rm -f "${pid_file}"
    if is_service_running "${probe_type}" "${probe_target}"; then
      echo "${name} 启动期间检测到已有同类服务在运行，跳过启动"
      return 0
    fi
    echo "${name} 启动失败，请查看日志：${log_file}"
    exit 1
  fi

  if [[ -n "${startup_port}" ]]; then
    local ready=0
    for _ in {1..20}; do
      if is_port_listening "${startup_port}"; then
        ready=1
        break
      fi
      if ! kill -0 "${new_pid}" >/dev/null 2>&1; then
        break
      fi
      sleep 1
    done

    if [[ "${ready}" -ne 1 ]]; then
      rm -f "${pid_file}"
      echo "${name} 未在预期时间内监听端口 ${startup_port}，请查看日志：${log_file}"
      exit 1
    fi
  fi

  echo "${name} 已启动，PID=${new_pid}"
}

wait_for_compose_services() {
  local services=("$@")

  echo "等待 Docker 依赖就绪..."
  if ! docker compose -f "${COMPOSE_FILE}" up -d --wait "${services[@]}"; then
    echo "Docker 依赖启动失败，请执行以下命令查看状态："
    echo "  docker compose -f ${COMPOSE_FILE} ps"
    exit 1
  fi
}

echo "==> 启动 Docker 依赖"
cd "${ROOT_DIR}"
if all_dependency_ports_ready; then
  echo "检测到本地依赖端口已就绪，复用现有依赖容器"
else
  wait_for_compose_services "${DEPENDENCY_SERVICES[@]}"
fi

validate_no_containerized_app_conflicts

echo "==> 启动后端 API"
cd "${ROOT_DIR}/backend/store/pg"
start_if_not_running \
  "backend-api" \
  "${PID_DIR}/backend-api.pid" \
  "${LOG_DIR}/backend-api.log" \
  pattern \
  "cmd/api/main.go" \
  8000 \
  env \
  PATH="/opt/homebrew/opt/go@1.24/bin:${PATH}" \
  GOCACHE="${ROOT_DIR}/.cache/go-build" \
  GOMODCACHE="${ROOT_DIR}/.cache/gomod" \
  GOPROXY="https://goproxy.cn,direct" \
  ENV="local" \
  SERVICE_NAME="api" \
  LOG_LEVEL="-4" \
  LOG_STORE_URL="http://127.0.0.1:13100" \
  ADMIN_PASSWORD="admin123" \
  PANDA_WIKI_CONFIG_FILE="${PANDA_WIKI_CONFIG_FILE:-${ROOT_DIR}/backend/config.yml}" \
  JWT_SECRET="dev-jwt-secret" \
  PG_DSN="host=127.0.0.1 user=panda-wiki password=panda-wiki-secret dbname=panda-wiki port=15432 sslmode=disable TimeZone=Asia/Shanghai" \
  MQ_NATS_SERVER="nats://127.0.0.1:4222" \
  REDIS_ADDR="127.0.0.1:16379" \
  S3_ENDPOINT="127.0.0.1:9000" \
  S3_ACCESS_KEY="s3panda-wiki" \
  S3_SECRET_KEY="panda-wiki-secret" \
  RAG_CT_RAG_BASE_URL="http://127.0.0.1:5050" \
  SENTRY_ENABLED="false" \
  CADDY_API="" \
  FILE_BASE_URL="http://127.0.0.1:8000/static-file" \
  ANYDOC_BASE_URL="http://127.0.0.1:8080" \
  ANYDOC_CRAWLER_BASE_URL="http://127.0.0.1:8080" \
  ANYDOC_API_UPLOADER_URL="http://host.docker.internal:8000/api/v1/file/upload/anydoc" \
  ANYDOC_STATIC_FILE_BASE_URL="http://host.docker.internal:9000/static-file" \
  PANDA_WIKI_MACHINE_ID_FILE="${LOCAL_DATA_DIR}/.machine_id" \
  PANDA_WIKI_SSL_DIR="${SSL_DIR}" \
  "${GO_BIN}" run ../../cmd/api/main.go ../../cmd/api/wire_gen.go

echo "==> 启动 consumer"
start_if_not_running \
  "backend-consumer" \
  "${PID_DIR}/backend-consumer.pid" \
  "${LOG_DIR}/backend-consumer.log" \
  pattern \
  "cmd/consumer/main.go" \
  "" \
  env \
  PATH="/opt/homebrew/opt/go@1.24/bin:${PATH}" \
  GOCACHE="${ROOT_DIR}/.cache/go-build" \
  GOMODCACHE="${ROOT_DIR}/.cache/gomod" \
  GOPROXY="https://goproxy.cn,direct" \
  ENV="local" \
  SERVICE_NAME="consumer" \
  LOG_LEVEL="-4" \
  LOG_STORE_URL="http://127.0.0.1:13100" \
  PG_DSN="host=127.0.0.1 user=panda-wiki password=panda-wiki-secret dbname=panda-wiki port=15432 sslmode=disable TimeZone=Asia/Shanghai" \
  PANDA_WIKI_CONFIG_FILE="${PANDA_WIKI_CONFIG_FILE:-${ROOT_DIR}/backend/config.yml}" \
  MQ_NATS_SERVER="nats://127.0.0.1:4222" \
  REDIS_ADDR="127.0.0.1:16379" \
  S3_ENDPOINT="127.0.0.1:9000" \
  S3_ACCESS_KEY="s3panda-wiki" \
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
validate_frontend_ports
start_if_not_running \
  "web-dev" \
  "${PID_DIR}/web-dev.pid" \
  "${LOG_DIR}/web-dev.log" \
  ports \
  "5173,3010" \
  3010 \
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
