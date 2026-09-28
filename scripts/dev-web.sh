#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/dev-env.sh"

load_external_oss_env
ensure_command pnpm "请先安装 pnpm。"

cd "${PANDA_WIKI_ROOT_DIR}/web"

env \
  STATIC_FILE_TARGET="${OSS_DOWNLOAD_URL}" \
  pnpm dev \
  2>&1 | tee "${PANDA_WIKI_ROOT_DIR}/.logs/web-dev.log"
