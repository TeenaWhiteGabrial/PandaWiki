#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
ROOT_ENV_FILE="${SCRIPT_DIR}/../../.env"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"

MODE="${1:-deploy}"
UPGRADE_TARGET="${2:-all}"

# 默认兼容没有 buildx 的 Docker Compose V1；如服务器安装了 buildx，
# 仍可通过 DOCKER_BUILDKIT=1 手动开启 BuildKit。
export DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-0}"
# 该变量对支持 BuildKit 的环境提供纯文本输出。
export BUILDKIT_PROGRESS="${BUILDKIT_PROGRESS:-plain}"

SERVICES_BASE=(
  postgres
  redis
  nats
  qdrant
  loki
  alloy
  raglite
)

SERVICES_BACKEND=(
  api
  consumer
)

SERVICES_FRONTEND=(
  admin
  app
)

SERVICES_RUNTIME=(
  api
  consumer
  admin
  app
)

required_env_keys=(
  SERVER_IP
  POSTGRES_PASSWORD
  RAGLITE_POSTGRES_DB
  JWT_SECRET
  ADMIN_PASSWORD
  QDRANT_API_KEY
  RAGLITE_API_KEY
)

required_oss_env_keys=(
  OSS_ENDPOINT
  OSS_ACCESS_KEY
  OSS_SECRET_KEY
  OSS_BUCKET_NAME
  OSS_DOWNLOAD_URL
  OSS_REGION
)

load_oss_env() {
  if [[ ! -f "${ROOT_ENV_FILE}" ]]; then
    echo "缺少根目录 OSS 配置：${ROOT_ENV_FILE}"
    echo "请参考 ${SCRIPT_DIR}/../../.env.example"
    exit 1
  fi
  local key value
  for key in "${required_oss_env_keys[@]}"; do
    value="$(grep -E "^${key}=" "${ROOT_ENV_FILE}" | tail -n 1 | cut -d= -f2- || true)"
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
}

usage() {
  cat <<'EOF'
用法：
  bash deploy.sh [mode] [target]

可选 mode：
  deploy     删除旧容器和旧镜像后重建部署（默认）
  upgrade    按 target 升级，并在启动失败时自动回滚
  restart    仅重启服务，不清理现有容器
  fresh-db   清空 PostgreSQL 数据后重建部署
  status     查看当前容器状态
  logs       查看核心服务最近日志

示例：
  bash deploy.sh
  bash deploy.sh deploy
  bash deploy.sh restart
  bash deploy.sh upgrade admin    # 只升级管理端
  bash deploy.sh upgrade app      # 只升级用户端
  bash deploy.sh upgrade backend  # 只升级 API 和 Consumer
  bash deploy.sh upgrade base     # 只重建基础服务容器，不删除数据卷
  bash deploy.sh upgrade all      # 完整升级（默认）
  bash deploy.sh fresh-db
EOF
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

detect_compose() {
  if has_cmd docker-compose; then
    COMPOSE_BIN=(docker-compose)
    return 0
  fi

  if docker compose version >/dev/null 2>&1; then
    COMPOSE_BIN=(docker compose)
    return 0
  fi

  echo "未找到 docker-compose 或 docker compose，请先安装 Docker Compose。"
  exit 1
}

compose() {
  "${COMPOSE_BIN[@]}" --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
}

require_env_file() {
  if [[ ! -f "${ENV_FILE}" ]]; then
    echo "缺少环境文件：${ENV_FILE}"
    echo "请先执行：cp ${SCRIPT_DIR}/.env.example ${ENV_FILE}"
    exit 1
  fi
}

check_docker() {
  echo "==> 检查 Docker"
  docker info >/dev/null
}

check_required_envs() {
  echo "==> 检查关键环境变量"

  local missing=0
  local key
  local value

  for key in "${required_env_keys[@]}"; do
    value="$(grep -E "^${key}=" "${ENV_FILE}" | tail -n 1 | cut -d= -f2- || true)"
    if [[ -z "${value}" ]] || [[ "${value}" == CHANGE_ME_* ]] || [[ "${value}" == YOUR_SERVER_IP ]]; then
      echo "缺少或未正确配置：${key}"
      missing=1
    fi
  done

  if [[ "${missing}" -ne 0 ]]; then
    echo
    echo "请先完善 ${ENV_FILE} 后重试。"
    exit 1
  fi
}

print_summary() {
  local server_ip
  local admin_port
  local app_port
  local api_port

  server_ip="$(grep '^SERVER_IP=' "${ENV_FILE}" | cut -d= -f2-)"
  admin_port="$(grep '^ADMIN_PORT=' "${ENV_FILE}" | cut -d= -f2-)"
  app_port="$(grep '^APP_PORT=' "${ENV_FILE}" | cut -d= -f2-)"
  api_port="$(grep '^API_PORT=' "${ENV_FILE}" | cut -d= -f2-)"

  echo
  echo "部署完成，可访问："
  echo "  管理台:  http://${server_ip}:${admin_port}"
  echo "  前台:    http://${server_ip}:${app_port}"
  echo "  后端:    http://${server_ip}:${api_port}"
  echo "  对象存储: ${OSS_DOWNLOAD_URL}"
  echo
  echo "常用检查命令："
  echo "  ${COMPOSE_BIN[*]} --env-file ${ENV_FILE} -f ${COMPOSE_FILE} ps"
  echo "  ${COMPOSE_BIN[*]} --env-file ${ENV_FILE} -f ${COMPOSE_FILE} logs --tail=100 api"
  echo "  ${COMPOSE_BIN[*]} --env-file ${ENV_FILE} -f ${COMPOSE_FILE} logs --tail=100 consumer"
}

remove_container_if_exists() {
  local container_name="$1"

  if docker inspect "${container_name}" >/dev/null 2>&1; then
    echo "删除旧容器: ${container_name}"
    docker rm -f "${container_name}" >/dev/null
  fi
}

remove_image_if_exists() {
  local image_ref="$1"

  if [[ -n "${image_ref}" ]] && docker image inspect "${image_ref}" >/dev/null 2>&1; then
    echo "删除旧镜像: ${image_ref}"
    docker rmi -f "${image_ref}" >/dev/null || true
  fi
}

cleanup_old_runtime_artifacts() {
  local project_name
  local service
  local container_name
  local image_id

  project_name="$(grep '^PROJECT_NAME=' "${ENV_FILE}" | cut -d= -f2-)"

  echo "==> 清理旧运行容器和镜像"
  for service in "${SERVICES_RUNTIME[@]}"; do
    container_name="${project_name}-${service}"
    image_id=""

    if docker inspect "${container_name}" >/dev/null 2>&1; then
      image_id="$(docker inspect --format '{{.Image}}' "${container_name}" 2>/dev/null || true)"
    fi

    remove_container_if_exists "${container_name}"
    remove_image_if_exists "${image_id}"
  done

  echo "==> 清理旧构建容器"
  remove_container_if_exists "${project_name}-admin-build"
  remove_container_if_exists "${project_name}-app-build"
}

run_restart() {
  echo "==> 仅重启服务"
  # Docker Compose V1.29.2 在重建旧镜像容器时可能因缺少
  # ContainerConfig 字段失败。先移除应用容器再重建，不删除任何数据卷。
  compose rm -sf api consumer admin app raglite qdrant loki alloy preview \
    admin-build app-build >/dev/null 2>&1 || true
  compose up -d "${SERVICES_BASE[@]}"
  compose up -d --build "${SERVICES_BACKEND[@]}"
  compose up -d --build --no-deps "${SERVICES_FRONTEND[@]}"
  compose ps
}

run_frontend_builds() {
  echo "==> 构建前端产物"
  compose rm -f admin-build app-build >/dev/null 2>&1 || true
  compose run --rm admin-build
  compose run --rm app-build
}

project_name() {
  grep '^PROJECT_NAME=' "${ENV_FILE}" | cut -d= -f2-
}

declare -a ROLLBACK_SERVICES=()
declare -a ROLLBACK_BACKUPS=()
declare -a ROLLBACK_IMAGE_REFS=()

snapshot_for_rollback() {
  local service container image_id image_ref backup_ref stamp
  stamp="$(date +%Y%m%d%H%M%S)"
  ROLLBACK_SERVICES=("$@")
  ROLLBACK_BACKUPS=()
  ROLLBACK_IMAGE_REFS=()

  for service in "${ROLLBACK_SERVICES[@]}"; do
    container="$(project_name)-${service}"
    image_id=""
    image_ref=""
    backup_ref=""

    if docker inspect "${container}" >/dev/null 2>&1; then
      image_id="$(docker inspect --format '{{.Image}}' "${container}" 2>/dev/null || true)"
      image_ref="$(docker inspect --format '{{.Config.Image}}' "${container}" 2>/dev/null || true)"
    fi

    if [[ -n "${image_id}" && -n "${image_ref}" ]]; then
      backup_ref="${PROJECT_NAME:-$(project_name)}-rollback-${stamp}-${service}:latest"
      docker tag "${image_id}" "${backup_ref}"
      echo "已保存 ${service} 回滚镜像：${backup_ref}"
    fi

    ROLLBACK_BACKUPS+=("${backup_ref}")
    ROLLBACK_IMAGE_REFS+=("${image_ref}")
  done
}

cleanup_rollback_backups() {
  local backup_ref
  for backup_ref in "${ROLLBACK_BACKUPS[@]}"; do
    [[ -n "${backup_ref}" ]] && docker rmi "${backup_ref}" >/dev/null 2>&1 || true
  done
}

wait_for_services() {
  local service container state attempt

  for attempt in $(seq 1 12); do
    for service in "$@"; do
      container="$(project_name)-${service}"
      if ! docker inspect "${container}" >/dev/null 2>&1; then
        echo "服务容器未创建：${container}"
        return 1
      fi
      state="$(docker inspect --format '{{.State.Status}}' "${container}" 2>/dev/null || true)"
      case "${state}" in
        running)
          ;;
        restarting|exited|dead)
          echo "服务未正常运行：${container}（状态：${state}）"
          return 1
          ;;
      esac
    done
    sleep 2
  done
}

rollback_upgrade() {
  local index service backup_ref image_ref

  echo
  echo "==> 升级失败，开始回滚本次涉及的服务"
  compose rm -sf "${ROLLBACK_SERVICES[@]}" >/dev/null 2>&1 || true

  for index in "${!ROLLBACK_SERVICES[@]}"; do
    service="${ROLLBACK_SERVICES[$index]}"
    backup_ref="${ROLLBACK_BACKUPS[$index]}"
    image_ref="${ROLLBACK_IMAGE_REFS[$index]}"
    if [[ -n "${backup_ref}" && -n "${image_ref}" ]]; then
      docker tag "${backup_ref}" "${image_ref}"
    fi
  done

  if compose up -d --no-deps "${ROLLBACK_SERVICES[@]}"; then
    echo "==> 已恢复旧服务容器"
  else
    echo "回滚容器启动失败，请执行：${COMPOSE_BIN[*]} --env-file ${ENV_FILE} -f ${COMPOSE_FILE} ps"
  fi
}

upgrade_services() {
  local target="$1"
  local -a services=()

  case "${target}" in
    admin)
      services=(admin)
      echo "==> 仅升级管理端"
      snapshot_for_rollback "${services[@]}"
      if ! run_frontend_build admin; then
        rollback_upgrade
        cleanup_rollback_backups
        return 1
      fi
      ;;
    app)
      services=(app)
      echo "==> 仅升级用户端"
      snapshot_for_rollback "${services[@]}"
      if ! run_frontend_build app; then
        rollback_upgrade
        cleanup_rollback_backups
        return 1
      fi
      ;;
    backend)
      services=(api consumer)
      echo "==> 仅升级后端 API 和 Consumer"
      snapshot_for_rollback "${services[@]}"
      ;;
    base)
      services=("${SERVICES_BASE[@]}")
      echo "==> 仅重建基础服务容器（不删除数据卷）"
      snapshot_for_rollback "${services[@]}"
      ;;
    all)
      services=("${SERVICES_BASE[@]}" api consumer admin app)
      echo "==> 完整无损升级"
      snapshot_for_rollback "${services[@]}"
      ;;
    *)
      echo "不支持的升级目标：${target}"
      echo "可选目标：all、admin、app、backend、base"
      return 1
      ;;
  esac

  if [[ "${target}" == "admin" || "${target}" == "app" ]]; then
    compose rm -sf "${services[@]}" >/dev/null 2>&1 || true
    if ! compose up -d --build --no-deps "${services[@]}" ||
      ! wait_for_services "${services[@]}"; then
      rollback_upgrade
      cleanup_rollback_backups
      return 1
    fi
  elif [[ "${target}" == "backend" ]]; then
    compose rm -sf "${services[@]}" >/dev/null 2>&1 || true
    if ! compose up -d --build --no-deps "${services[@]}" ||
      ! wait_for_services "${services[@]}"; then
      rollback_upgrade
      cleanup_rollback_backups
      return 1
    fi
  elif [[ "${target}" == "base" ]]; then
    compose rm -sf "${services[@]}" >/dev/null 2>&1 || true
    if ! compose up -d "${services[@]}" ||
      ! wait_for_services "${services[@]}"; then
      rollback_upgrade
      cleanup_rollback_backups
      return 1
    fi
  else
    # Base services are restarted without deleting their named volumes.
    if ! compose rm -sf "${SERVICES_BASE[@]}" >/dev/null 2>&1 ||
      ! compose up -d "${SERVICES_BASE[@]}" ||
      ! run_frontend_builds ||
      ! verify_frontend_artifacts ||
      ! compose rm -sf api consumer admin app >/dev/null 2>&1 ||
      ! compose up -d --build --no-deps api consumer ||
      ! compose up -d --build --no-deps admin app ||
      ! wait_for_services "${services[@]}"; then
      rollback_upgrade
      cleanup_rollback_backups
      return 1
    fi
  fi

  compose ps
  cleanup_rollback_backups
}

run_frontend_build() {
  local target="$1"
  local build_service="${target}-build"
  local dist_path="${SCRIPT_DIR}/../../web/${target}/dist"

  echo "==> 构建 ${target} 前端产物"
  compose rm -f "${build_service}" >/dev/null 2>&1 || true
  if ! compose run --rm "${build_service}"; then
    return 1
  fi
  [[ -d "${dist_path}" ]]
}

run_upgrade() {
  echo "==> 无损升级：${UPGRADE_TARGET}（失败自动回滚，不删除数据卷）"
  upgrade_services "${UPGRADE_TARGET}"
}

verify_frontend_artifacts() {
  local admin_dist="${SCRIPT_DIR}/../../web/admin/dist"
  local app_dist="${SCRIPT_DIR}/../../web/app/dist"

  echo "==> 校验前端构建产物"

  if [[ ! -d "${admin_dist}" ]]; then
    echo "缺少前端构建产物：${admin_dist}"
    echo "admin 镜像构建依赖该目录，请先检查 admin-build 日志。"
    exit 1
  fi

  if [[ ! -d "${app_dist}" ]]; then
    echo "缺少前端构建产物：${app_dist}"
    echo "app 镜像构建依赖该目录，请先检查 app-build 日志。"
    exit 1
  fi
}

run_deploy() {
  echo "==> 删除旧容器和旧镜像后重建部署"
  compose down
  cleanup_old_runtime_artifacts
  compose up -d "${SERVICES_BASE[@]}"
  run_frontend_builds
  verify_frontend_artifacts
  compose up -d --build "${SERVICES_BACKEND[@]}"
  compose up -d --build --no-deps "${SERVICES_FRONTEND[@]}"
  compose ps
}

run_fresh_db() {
  local project_name
  local volume_name

  project_name="$(grep '^PROJECT_NAME=' "${ENV_FILE}" | cut -d= -f2-)"
  volume_name="server_${project_name}_postgres"

  echo "警告：即将清空 PostgreSQL 数据卷 ${volume_name}"
  compose down
  cleanup_old_runtime_artifacts

  if docker volume inspect "${volume_name}" >/dev/null 2>&1; then
    docker volume rm "${volume_name}"
  else
    echo "未找到数据卷 ${volume_name}，继续执行。"
  fi

  compose up -d "${SERVICES_BASE[@]}"
  run_frontend_builds
  verify_frontend_artifacts
  compose up -d --build "${SERVICES_BACKEND[@]}"
  compose up -d --build --no-deps "${SERVICES_FRONTEND[@]}"
  compose ps
}

run_status() {
  echo "==> 当前状态"
  compose ps
}

run_logs() {
  echo "==> 最近日志"
  compose logs --tail=100 postgres redis nats qdrant raglite loki alloy api consumer admin app
}

main() {
  if [[ "${MODE}" == "-h" ]] || [[ "${MODE}" == "--help" ]]; then
    usage
    exit 0
  fi

  require_env_file
  load_oss_env
  detect_compose
  check_docker

  case "${MODE}" in
    deploy)
      check_required_envs
      run_deploy
      print_summary
      ;;
    restart)
      check_required_envs
      run_restart
      print_summary
      ;;
    upgrade)
      check_required_envs
      run_upgrade
      print_summary
      ;;
    fresh-db)
      check_required_envs
      run_fresh_db
      print_summary
      ;;
    status)
      run_status
      ;;
    logs)
      run_logs
      ;;
    *)
      echo "未知 mode: ${MODE}"
      echo
      usage
      exit 1
      ;;
  esac
}

main "$@"
