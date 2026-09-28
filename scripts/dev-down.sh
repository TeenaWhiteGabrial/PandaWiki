#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/dev-env.sh"

load_external_oss_env

echo "停止 Docker 基础依赖"
cd "${PANDA_WIKI_ROOT_DIR}"
docker compose -f "${PANDA_WIKI_COMPOSE_FILE}" stop "${PANDA_WIKI_DEPENDENCY_SERVICES[@]}" >/dev/null 2>&1 || true
docker compose -f "${PANDA_WIKI_COMPOSE_FILE}" rm -f "${PANDA_WIKI_DEPENDENCY_SERVICES[@]}" >/dev/null 2>&1 || true

echo
echo "Docker 基础依赖已停止。"
echo "如果 API、consumer、前端仍在前台终端运行，请在对应终端按 Ctrl-C 停止。"
