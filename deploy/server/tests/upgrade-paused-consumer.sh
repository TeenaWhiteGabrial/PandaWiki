#!/usr/bin/env bash
# Exercise upgrade/rollback control flow with stubs; never invokes Docker.
set -euo pipefail
TEST_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TEST_TEMP_DIR}"' EXIT

run_case() (
  local scenario="$1"
  source "${TEST_SCRIPT_DIR}/../deploy.sh"
  local log_file="${TEST_TEMP_DIR}/${scenario}.log"
  local failed_once=0
  export KEEP_CONSUMER_STOPPED=1
  [[ "${scenario}" != normal ]] || export KEEP_CONSUMER_STOPPED=0
  compose() {
    printf '%s\n' "$*" >> "${log_file}"
    if [[ "${scenario}" == rollback && "${failed_once}" == 0 && "$*" == 'up -d --no-build --no-deps pdf-parser api' ]]; then
      failed_once=1
      return 1
    fi
    return 0
  }
  snapshot_for_rollback() {
    ROLLBACK_SERVICES=("$@")
    ROLLBACK_BACKUPS=(old-parser old-api old-consumer)
    return 0
  }
  current_schema_version() { printf '56\n'; }
  security_schema_rollback_allowed() { return 0; }
  restore_rollback_image_tags() { return 0; }
  cleanup_rollback_backups() { return 0; }
  wait_for_services() { printf 'wait %s\n' "$*" >> "${log_file}"; }
  SCHEMA_VERSION_BEFORE=56
  local succeeded=0
  if upgrade_services backend; then succeeded=1; fi
  if [[ "${scenario}" == rollback ]]; then
    [[ "${succeeded}" == 0 ]]
    [[ "$(rg -c '^up --no-start --no-build --no-deps consumer$' "${log_file}")" == 2 ]]
  else
    [[ "${succeeded}" == 1 ]]
  fi
  if [[ "${scenario}" == normal ]]; then
    rg -q '^up -d --no-build --no-deps pdf-parser api consumer$' "${log_file}"
    rg -q '^wait pdf-parser api consumer$' "${log_file}"
  else
    rg -q '^up --no-start --no-build --no-deps consumer$' "${log_file}"
    if rg -q '^(up -d|wait).*consumer' "${log_file}"; then
      cat "${log_file}"
      return 1
    fi
    rg -q '^wait pdf-parser api$' "${log_file}"
  fi
  printf 'PASS %s\n' "${scenario}"
)

run_case normal
run_case paused
run_case rollback
