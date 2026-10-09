#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/dev-env.sh"

ensure_dev_dirs
load_external_oss_env
resolve_go_bin

cd "${PANDA_WIKI_ROOT_DIR}/backend/store/pg"

env \
  PATH="${PANDA_WIKI_GO_BIN%/*}:${PATH}" \
  GOCACHE="${PANDA_WIKI_ROOT_DIR}/.cache/go-build" \
  GOMODCACHE="${PANDA_WIKI_ROOT_DIR}/.cache/gomod" \
  GOPROXY="https://goproxy.cn,direct" \
  ENV="local" \
  SERVICE_NAME="consumer" \
  PDF_PARSER_URL="http://127.0.0.1:18090" \
  PDF_PARSER_TOKEN="${PDF_PARSER_TOKEN:-}" \
  LOG_LEVEL="-4" \
  LOG_STORE_URL="http://127.0.0.1:13100" \
  PG_DSN="host=127.0.0.1 user=panda-wiki password=panda-wiki-secret dbname=panda-wiki port=15432 sslmode=disable TimeZone=Asia/Shanghai" \
  PANDA_WIKI_CONFIG_FILE="${PANDA_WIKI_CONFIG_FILE:-${PANDA_WIKI_ROOT_DIR}/backend/config.yml}" \
  MQ_NATS_SERVER="nats://127.0.0.1:4222" \
  REDIS_ADDR="127.0.0.1:16379" \
  OSS_ENDPOINT="${OSS_ENDPOINT}" \
  OSS_ACCESS_KEY="${OSS_ACCESS_KEY}" \
  OSS_SECRET_KEY="${OSS_SECRET_KEY}" \
  OSS_BUCKET_NAME="${OSS_BUCKET_NAME}" \
  OSS_DOWNLOAD_URL="${OSS_DOWNLOAD_URL}" \
  OSS_REGION="${OSS_REGION}" \
  RAG_CT_RAG_BASE_URL="http://127.0.0.1:5050" \
  SENTRY_ENABLED="false" \
  FILE_BASE_URL="${OSS_DOWNLOAD_URL}/static-file" \
  ANYDOC_BASE_URL="http://127.0.0.1:8080" \
  PANDA_WIKI_MACHINE_ID_FILE="${PANDA_WIKI_LOCAL_DATA_DIR}/.machine_id" \
  PANDA_WIKI_SSL_DIR="${PANDA_WIKI_SSL_DIR}" \
  "${PANDA_WIKI_GO_BIN}" run ../../cmd/consumer/main.go ../../cmd/consumer/wire_gen.go \
  2>&1 | tee "${PANDA_WIKI_ROOT_DIR}/.logs/backend-consumer.log"
