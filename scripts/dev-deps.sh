#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/scripts/dev-env.sh"

ensure_dev_dirs
load_external_oss_env
ensure_command docker "请先安装并启动 Docker Desktop。"

echo "==> 启动 Docker 基础依赖"
echo "    pdf-parser postgres redis nats qdrant crawler raglite loki alloy"

cd "${PANDA_WIKI_ROOT_DIR}"
docker compose -f "${PANDA_WIKI_COMPOSE_FILE}" up -d --wait --remove-orphans "${PANDA_WIKI_DEPENDENCY_SERVICES[@]}"

echo
echo "Docker 基础依赖已启动。"
echo "接下来请分别打开三个终端运行："
echo "  ./scripts/dev-api.sh"
echo "  ./scripts/dev-consumer.sh"
echo "  ./scripts/dev-web.sh"
