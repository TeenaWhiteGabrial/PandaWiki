#!/usr/bin/env bash

PANDA_WIKI_ROOT_DIR="${PANDA_WIKI_ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
PANDA_WIKI_COMPOSE_FILE="${PANDA_WIKI_ROOT_DIR}/docker-compose.local.yml"
PANDA_WIKI_LOCAL_DATA_DIR="${PANDA_WIKI_ROOT_DIR}/.local-data"
PANDA_WIKI_SSL_DIR="${PANDA_WIKI_LOCAL_DATA_DIR}/ssl"

PANDA_WIKI_DEPENDENCY_SERVICES=(
  postgres
  redis
  nats
  qdrant
  crawler
  raglite
  loki
  alloy
)

PANDA_WIKI_DEPENDENCY_PORTS=(
  15432
  16379
  4222
  6333
  8080
  5050
  13100
)

ensure_dev_dirs() {
  mkdir -p \
    "${PANDA_WIKI_ROOT_DIR}/.logs" \
    "${PANDA_WIKI_ROOT_DIR}/.pids" \
    "${PANDA_WIKI_ROOT_DIR}/.cache/go-build" \
    "${PANDA_WIKI_ROOT_DIR}/.cache/gomod" \
    "${PANDA_WIKI_SSL_DIR}"
}

strip_env_quotes() {
  local value="$1"
  value="${value%$'\r'}"
  if [[ "${value}" == \"*\" && "${value}" == *\" ]]; then
    value="${value:1:${#value}-2}"
  elif [[ "${value}" == \'*\' && "${value}" == *\' ]]; then
    value="${value:1:${#value}-2}"
  fi
  printf '%s' "${value}"
}

read_env_value() {
  local env_file="$1"
  local key="$2"
  local value

  value="$(grep -E "^${key}=" "${env_file}" | tail -n 1 | cut -d= -f2- || true)"
  strip_env_quotes "${value}"
}

load_external_oss_env() {
  local env_file="${PANDA_WIKI_ROOT_DIR}/.env"
  local key value
  local required_keys=(OSS_ENDPOINT OSS_ACCESS_KEY OSS_SECRET_KEY OSS_BUCKET_NAME OSS_DOWNLOAD_URL OSS_REGION)

  if [[ ! -f "${env_file}" ]]; then
    echo "缺少根目录 OSS 配置：${env_file}"
    echo "请参考 ${PANDA_WIKI_ROOT_DIR}/.env.example"
    exit 1
  fi

  for key in "${required_keys[@]}"; do
    value="$(read_env_value "${env_file}" "${key}")"
    if [[ -z "${value}" ]] || [[ "${value}" == CHANGE_ME_* ]]; then
      echo "根目录 .env 缺少或未正确配置：${key}"
      exit 1
    fi
    export "${key}=${value}"
  done

  case "${OSS_ENDPOINT}" in
    https://*)
      export OSS_USE_SSL=true
      export OSS_S3_ENDPOINT="${OSS_ENDPOINT#https://}"
      ;;
    http://*)
      export OSS_USE_SSL=false
      export OSS_S3_ENDPOINT="${OSS_ENDPOINT#http://}"
      ;;
    *)
      export OSS_USE_SSL=false
      export OSS_S3_ENDPOINT="${OSS_ENDPOINT}"
      ;;
  esac

  OSS_S3_ENDPOINT="${OSS_S3_ENDPOINT%/}"
  export OSS_S3_ENDPOINT
  export STATIC_FILE_TARGET="${OSS_DOWNLOAD_URL}"
}

resolve_go_bin() {
  PANDA_WIKI_GO_BIN="${PANDA_WIKI_GO_BIN:-/opt/homebrew/opt/go@1.24/bin/go}"
  if [[ ! -x "${PANDA_WIKI_GO_BIN}" ]]; then
    PANDA_WIKI_GO_BIN="$(command -v go || true)"
  fi

  if [[ -z "${PANDA_WIKI_GO_BIN}" ]]; then
    echo "未找到 Go，请先安装 Go 1.24。"
    exit 1
  fi

  export PANDA_WIKI_GO_BIN
}

ensure_command() {
  local command_name="$1"
  local install_hint="$2"

  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "未找到 ${command_name}。${install_hint}"
    exit 1
  fi
}
