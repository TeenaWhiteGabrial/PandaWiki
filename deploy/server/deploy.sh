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
  nats-monitor
  preview
  qdrant
  loki
  alloy
  raglite
)

SERVICES_BACKEND=(
  pdf-parser
  api
  consumer
)

SERVICES_FRONTEND=(
  admin
  app
)

SERVICES_RUNTIME=(
  pdf-parser
  api
  consumer
  admin
  app
)

required_env_keys=(
  SERVER_IP
  POSTGRES_PASSWORD
  REDIS_PASSWORD
  NATS_PASSWORD
  NATS_MONITOR_PASSWORD
  PREVIEW_PASSWORD
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
  bash deploy.sh upgrade backend  # 升级 PDF 解析服务、API 和 Consumer
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

# Generate separate internal-service secrets once, preserving existing values.
ensure_internal_credentials() {
  local internal_key internal_value
  for internal_key in NATS_MONITOR_PASSWORD PREVIEW_PASSWORD; do
    internal_value="$(grep -E "^${internal_key}=" "${ENV_FILE}" | tail -n 1 | cut -d= -f2- || true)"
    if [[ -z "${internal_value}" || "${internal_value}" == CHANGE_ME_* ]]; then
      # Remove a blank/example entry before appending the generated value.
      local internal_tmp
      internal_tmp="$(mktemp "${ENV_FILE}.XXXXXX")"
      chmod 600 "${internal_tmp}"
      awk -F= -v key="${internal_key}" '$1 != key' "${ENV_FILE}" > "${internal_tmp}"
      printf '\n%s=%s\n' "${internal_key}" "$(openssl rand -hex 24)" >> "${internal_tmp}"
      mv "${internal_tmp}" "${ENV_FILE}"
      echo "==> 已生成并保存内部服务凭据：${internal_key}"
    fi
  done
  chmod 600 "${ENV_FILE}"
}

check_required_envs() {
  ensure_internal_credentials
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
  compose rm -sf pdf-parser api consumer admin app raglite qdrant loki alloy preview \
    admin-build app-build >/dev/null 2>&1 || true
  compose up -d "${SERVICES_BASE[@]}"
  compose up -d --build "${SERVICES_BACKEND[@]}"
  compose up -d --build --no-deps "${SERVICES_FRONTEND[@]}"
  compose ps
}

run_frontend_builds() {
  echo "==> 构建前端产物"
  compose rm -f admin-build app-build >/dev/null 2>&1 || true
  compose run --rm admin-build || return 1
  compose run --rm app-build || return 1
}

project_name() {
  grep '^PROJECT_NAME=' "${ENV_FILE}" | cut -d= -f2-
}

SCHEMA_VERSION_BEFORE=""

current_schema_version() {
  local pg_user pg_database
  pg_user="$(grep '^POSTGRES_USER=' "${ENV_FILE}" | tail -n 1 | cut -d= -f2-)"
  pg_database="$(grep '^POSTGRES_DB=' "${ENV_FILE}" | tail -n 1 | cut -d= -f2-)"
  docker exec "$(project_name)-postgres" psql -U "${pg_user}" -d "${pg_database}" -Atqc 'SELECT version FROM schema_migrations WHERE NOT dirty LIMIT 1' 2>/dev/null || true
}

# Security migration 56 hashes reusable keys. A binary rollback alone would
# silently break MCP authentication; restore the operator's backup explicitly.
security_schema_rollback_allowed() {
  local after="$1" before="$2"
  [[ "${before}" =~ ^[0-9]+$ && "${after}" =~ ^[0-9]+$ ]] || return 1
  ! (( before < 56 && after >= 56 ))
}

declare -a ROLLBACK_SERVICES=()
declare -a ROLLBACK_BACKUPS=()
declare -a ROLLBACK_IMAGE_REFS=()

snapshot_for_rollback() {
  local service container image_id image_ref backup_ref stamp
  stamp="$(date +%Y%m%d%H%M%S)"
  SCHEMA_VERSION_BEFORE="$(current_schema_version)"
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
      if ! docker tag "${image_id}" "${backup_ref}"; then
        echo "保存 ${service} 回滚镜像失败，取消升级"
        return 1
      fi
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
  local service container state health attempt ready

  for attempt in $(seq 1 12); do
    ready=true
    for service in "$@"; do
      container="$(project_name)-${service}"
      if ! docker inspect "${container}" >/dev/null 2>&1; then
        echo "服务容器未创建：${container}"
        return 1
      fi
      state="$(docker inspect --format '{{.State.Status}}' "${container}" 2>/dev/null || true)"
      case "${state}" in
        running)
          health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${container}")"
          if [[ "${health}" == "unhealthy" ]]; then
            echo "服务健康检查失败：${container}"
            return 1
          fi
          [[ "${health}" == "starting" ]] && ready=false
          ;;
        restarting|exited|dead)
          echo "服务未正常运行：${container}（状态：${state}）"
          return 1
          ;;
        *)
          ready=false
          ;;
      esac
    done
    if [[ "${ready}" == true && "${attempt}" -ge 3 ]]; then
      return 0
    fi
    sleep 2
  done
  echo "服务启动或健康检查超时"
  return 1
}

restore_rollback_image_tags() {
  local index backup_ref image_ref
  for index in "${!ROLLBACK_SERVICES[@]}"; do
    backup_ref="${ROLLBACK_BACKUPS[$index]}"
    image_ref="${ROLLBACK_IMAGE_REFS[$index]}"
    if [[ -n "${backup_ref}" && -n "${image_ref}" ]]; then
      if ! docker tag "${backup_ref}" "${image_ref}"; then
        echo "恢复旧镜像标签失败：${backup_ref} -> ${image_ref}"
        return 1
      fi
    fi
  done
}

rollback_upgrade() {
  local index service
  local -a existing_services=()

  local schema_after="$(current_schema_version)"
  for service in "${ROLLBACK_SERVICES[@]}"; do
    if [[ "${service}" == "api" ]] && ! security_schema_rollback_allowed "${schema_after}" "${SCHEMA_VERSION_BEFORE}"; then
      echo "数据库已跨越安全迁移 56，或迁移版本无法确认，已停止自动镜像回滚。"
      echo "请保留当前数据库与回滚镜像；回退旧版前须人工恢复升级前的数据库备份。"
      return 1
    fi
  done
  echo
  echo "==> 升级失败，开始回滚本次涉及的服务"
  # 首次新增的服务没有旧镜像，不能在回滚时触发其构建。
  if ! restore_rollback_image_tags; then
    return 1
  fi

  for index in "${!ROLLBACK_SERVICES[@]}"; do
    service="${ROLLBACK_SERVICES[$index]}"
    if [[ -n "${ROLLBACK_BACKUPS[$index]}" ]]; then
      existing_services+=("${service}")
    else
      echo "跳过此前未部署的服务：${service}"
    fi
  done

  if ! compose rm -sf "${ROLLBACK_SERVICES[@]}"; then
    echo "清理失败容器失败，保留回滚镜像，请检查容器状态"
    return 1
  fi
  if [[ "${#existing_services[@]}" -eq 0 ]]; then
    echo "==> 此前没有已部署服务，已清理本次新增容器"
    return 0
  fi
  if ! compose up -d --no-build --no-deps "${existing_services[@]}" ||
    ! wait_for_services "${existing_services[@]}"; then
    echo "回滚容器启动失败，请执行：${COMPOSE_BIN[*]} --env-file ${ENV_FILE} -f ${COMPOSE_FILE} ps"
    return 1
  fi
  echo "==> 已恢复旧服务容器并通过状态检查"
}

handle_upgrade_failure() {
  if rollback_upgrade; then
    cleanup_rollback_backups
  else
    echo "==> 回滚未完成，保留以下镜像供手动恢复："
    local backup_ref
    for backup_ref in "${ROLLBACK_BACKUPS[@]}"; do
      [[ -z "${backup_ref}" ]] || echo "  ${backup_ref}"
    done
  fi
}

upgrade_services() {
  local target="$1"
  local -a services=()
  local -a build_services=()

  case "${target}" in
    admin)
      services=(admin)
      echo "==> 仅升级管理端"
      build_services=(admin)
      ;;
    app)
      services=(app)
      echo "==> 仅升级用户端"
      build_services=(app)
      ;;
    backend)
      services=(pdf-parser api consumer)
      echo "==> 升级 PDF 解析服务、API 和 Consumer"
      build_services=("${services[@]}")
      ;;
    base)
      services=("${SERVICES_BASE[@]}")
      build_services=(nats-monitor)
      echo "==> 仅重建基础服务容器（不删除数据卷）"
      ;;
    all)
      services=("${SERVICES_BASE[@]}" pdf-parser api consumer admin app)
      echo "==> 完整无损升级"
      build_services=(nats-monitor pdf-parser api consumer admin app)
      ;;
    *)
      echo "不支持的升级目标：${target}"
      echo "可选目标：all、admin、app、backend、base"
      return 1
      ;;
  esac

  if ! snapshot_for_rollback "${services[@]}"; then
    return 1
  fi

  # 旧容器保持运行，全部构建成功之后才切换。兼容 Compose V1，
  # 切换时仍先 rm 避免 ContainerConfig 错误，但不再边停服务边构建。
  if { [[ "${target}" == "admin" || "${target}" == "app" ]] &&
       ! run_frontend_build "${target}"; } ||
     { [[ "${target}" == "all" ]] &&
       { ! run_frontend_builds || ! verify_frontend_artifacts; }; } ||
     { [[ "${#build_services[@]}" -gt 0 ]] &&
       ! compose build "${build_services[@]}"; }; then
    echo "==> 构建失败，未停止或删除现有服务容器"
    if restore_rollback_image_tags; then
      cleanup_rollback_backups
    else
      echo "旧镜像标签恢复失败，回滚镜像已保留"
    fi
    return 1
  fi

  if [[ "${target}" != "all" ]]; then
    if ! compose rm -sf "${services[@]}" ||
      ! compose up -d --no-build --no-deps "${services[@]}" ||
      ! wait_for_services "${services[@]}"; then
      handle_upgrade_failure
      return 1
    fi
  else
    # Base services are restarted without deleting their named volumes.
    if ! compose rm -sf "${SERVICES_BASE[@]}" >/dev/null 2>&1 ||
      ! compose up -d "${SERVICES_BASE[@]}" ||
      ! compose rm -sf pdf-parser api consumer admin app >/dev/null 2>&1 ||
      ! compose up -d --no-build --no-deps pdf-parser api consumer ||
      ! compose up -d --no-build --no-deps admin app ||
      ! wait_for_services "${services[@]}"; then
      handle_upgrade_failure
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
  if [[ "${UPGRADE_TARGET}" != "all" ]] && docker inspect "$(project_name)-redis" >/dev/null 2>&1; then
    local anonymous_ping
    anonymous_ping="$(docker exec "$(project_name)-redis" env -u REDISCLI_AUTH redis-cli ping 2>/dev/null || true)"
    if [[ "${anonymous_ping}" == "PONG" ]]; then
      echo "检测到旧版无认证 Redis。首次安全升级必须使用 upgrade all，同步更新 Redis、NATS 和全部客户端。"
      return 1
    fi
  fi
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
    return 1
  fi

  if [[ ! -d "${app_dist}" ]]; then
    echo "缺少前端构建产物：${app_dist}"
    echo "app 镜像构建依赖该目录，请先检查 app-build 日志。"
    return 1
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
  compose logs --tail=100 postgres redis nats qdrant raglite pdf-parser loki alloy api consumer admin app
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

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
