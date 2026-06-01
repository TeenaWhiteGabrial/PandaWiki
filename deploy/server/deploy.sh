#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "缺少 ${ENV_FILE}"
  echo "请先执行：cp ${SCRIPT_DIR}/.env.example ${ENV_FILE}"
  exit 1
fi

cd "${SCRIPT_DIR}"

echo "==> 检查 Docker"
docker info >/dev/null

echo "==> 使用环境文件: ${ENV_FILE}"
echo "==> 构建并启动 PandaWiki 单机部署"

docker-compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d --build postgres redis nats minio qdrant crawler raglite
docker-compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up --build admin-build app-build
docker-compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d --build api consumer admin app

echo
echo "部署完成。可访问地址："
SERVER_IP="$(grep '^SERVER_IP=' "${ENV_FILE}" | cut -d= -f2-)"
ADMIN_PORT="$(grep '^ADMIN_PORT=' "${ENV_FILE}" | cut -d= -f2-)"
APP_PORT="$(grep '^APP_PORT=' "${ENV_FILE}" | cut -d= -f2-)"
API_PORT="$(grep '^API_PORT=' "${ENV_FILE}" | cut -d= -f2-)"
MINIO_CONSOLE_PORT="$(grep '^MINIO_CONSOLE_PORT=' "${ENV_FILE}" | cut -d= -f2-)"

echo "  管理台:  http://${SERVER_IP}:${ADMIN_PORT}"
echo "  前台:    http://${SERVER_IP}:${APP_PORT}"
echo "  后端:    http://${SERVER_IP}:${API_PORT}"
echo "  MinIO:   http://${SERVER_IP}:${MINIO_CONSOLE_PORT}"
echo
echo "查看容器状态："
echo "  docker-compose --env-file ${ENV_FILE} -f ${COMPOSE_FILE} ps"
echo
echo "查看日志："
echo "  docker-compose --env-file ${ENV_FILE} -f ${COMPOSE_FILE} logs -f api"
