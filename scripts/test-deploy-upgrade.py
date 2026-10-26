"""Regression tests for upgrade failure paths; never invoke real Docker."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


DEPLOY_SCRIPT = Path(__file__).resolve().parents[1] / "deploy/server/deploy.sh"
HARNESS = r'''
source "$DEPLOY_SCRIPT"
COMPOSE_BIN=(mock-compose)
project_name() { echo pandawiki; }
docker() {
  echo "docker $*" >> "$CALL_LOG"
  case "$1" in
    exec)
      # Model an actual schema version rather than returning an empty string.
      # Unknown versions must continue to block unsafe automatic rollback.
      if [[ -f "${UP_COUNT}" ]]; then
        printf '%s\n' "${SCHEMA_AFTER:-55}"
      else
        printf '%s\n' "${SCHEMA_BEFORE:-55}"
      fi
      ;;
    inspect)
      local container="${@: -1}"
      [[ "$container" != pandawiki-pdf-parser ]] || return 1
      if [[ "${2:-}" == --format ]]; then
        if [[ "$3" == '{{.Image}}' ]]; then
          echo "sha256:old-${container}"
        else
          echo "server_${container#pandawiki-}:latest"
        fi
      fi
      ;;
    tag)
      if [[ "${FAIL_RETAG:-0}" == 1 && "$2" == *-rollback-* ]]; then
        return 1
      fi
      ;;
  esac
  return 0
}
compose() {
  echo "compose $*" >> "$CALL_LOG"
  if [[ "$1" == build && "${FAIL_BUILD:-0}" == 1 ]]; then
    return 1
  fi
  if [[ "$1" == up ]]; then
    local count=0
    [[ ! -f "$UP_COUNT" ]] || read -r count < "$UP_COUNT"
    count=$((count + 1))
    echo "$count" > "$UP_COUNT"
    if [[ "${FAIL_UP:-0}" == 1 && "$count" == 1 ]] ||
       [[ "${FAIL_ROLLBACK:-0}" == 1 && "$count" -gt 1 ]]; then
      return 1
    fi
  fi
  return 0
}
wait_for_services() {
  echo "wait $*" >> "$CALL_LOG"
  [[ "${FAIL_HEALTH:-0}" != 1 ]]
}
upgrade_services "${TEST_TARGET:-backend}"
'''


class UpgradeTest(unittest.TestCase):
    def run_upgrade(self, **flags):
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / "calls"
            env = dict(os.environ, DEPLOY_SCRIPT=str(DEPLOY_SCRIPT),
                       CALL_LOG=str(log), UP_COUNT=str(Path(directory) / "up"), **flags)
            result = subprocess.run(["bash", "-c", HARNESS], env=env,
                                    capture_output=True, text=True)
            return result, log.read_text().splitlines()

    def test_build_failure_keeps_old_containers(self):
        result, calls = self.run_upgrade(FAIL_BUILD="1")
        self.assertNotEqual(result.returncode, 0, result.stderr)
        self.assertIn("compose build pdf-parser api consumer", calls)
        self.assertFalse(any(line.startswith(("compose rm", "compose up")) for line in calls))
        self.assertTrue(any("server_api:latest" in line and "rollback" in line for line in calls))

    def test_success_builds_before_stopping_and_disables_implicit_build(self):
        result, calls = self.run_upgrade()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertLess(calls.index("compose build pdf-parser api consumer"),
                        calls.index("compose rm -sf pdf-parser api consumer"))
        self.assertIn("compose up -d --no-build --no-deps pdf-parser api consumer", calls)

    def test_rollback_excludes_new_parser_and_checks_original_services(self):
        result, calls = self.run_upgrade(FAIL_UP="1")
        self.assertNotEqual(result.returncode, 0, result.stderr)
        self.assertIn("compose up -d --no-build --no-deps api consumer", calls)
        self.assertIn("wait api consumer", calls)
        self.assertEqual(sum(line.startswith("compose build") for line in calls), 1)
        self.assertTrue(any(line.startswith("docker rmi") for line in calls))

    def test_failed_rollback_preserves_backup_tags(self):
        result, calls = self.run_upgrade(FAIL_UP="1", FAIL_ROLLBACK="1")
        self.assertNotEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(line.startswith("docker rmi") for line in calls))
        self.assertIn("保留以下镜像", result.stdout)

    def test_unhealthy_rollback_preserves_backup_tags(self):
        result, calls = self.run_upgrade(FAIL_HEALTH="1")
        self.assertNotEqual(result.returncode, 0, result.stderr)
        self.assertIn("wait api consumer", calls)
        self.assertFalse(any(line.startswith("docker rmi") for line in calls))

    def test_failed_tag_restore_does_not_remove_containers_again(self):
        result, calls = self.run_upgrade(FAIL_UP="1", FAIL_RETAG="1")
        self.assertNotEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls.count("compose rm -sf pdf-parser api consumer"), 1)
        self.assertFalse(any(line.startswith("docker rmi") for line in calls))

    def test_base_upgrade_builds_only_monitor_proxy(self):
        result, calls = self.run_upgrade(TEST_TARGET="base")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([line for line in calls if line.startswith("compose build")], ["compose build nats-monitor"])

    def test_security_migration_blocks_binary_only_rollback(self):
        result, calls = self.run_upgrade(FAIL_UP="1", SCHEMA_AFTER="56")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("compose up -d --no-build --no-deps api consumer", calls)
        self.assertFalse(any(line.startswith("docker rmi") for line in calls))

    def test_unknown_schema_blocks_automatic_rollback(self):
        result, calls = self.run_upgrade(FAIL_UP="1", SCHEMA_AFTER="unknown")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("compose up -d --no-build --no-deps api consumer", calls)
        self.assertFalse(any(line.startswith("docker rmi") for line in calls))

    def test_internal_credentials_generated_once_without_printing_secrets(self):
        with tempfile.TemporaryDirectory() as directory:
            env_path = Path(directory) / "deployment.env"
            env_path.write_text("EXISTING=value\nPREVIEW_PASSWORD=\n")
            command = 'source "$DEPLOY_SCRIPT"; ENV_FILE="$TEST_ENV_FILE"; ensure_internal_credentials'
            env = dict(os.environ, DEPLOY_SCRIPT=str(DEPLOY_SCRIPT), TEST_ENV_FILE=str(env_path))
            first = subprocess.run(["bash", "-c", command], env=env, capture_output=True, text=True)
            self.assertEqual(first.returncode, 0, first.stderr)
            contents = env_path.read_text()
            values = dict(line.split("=", 1) for line in contents.splitlines() if "=" in line)
            self.assertEqual(values["EXISTING"], "value")
            for key in ["PREVIEW_PASSWORD", "NATS_MONITOR_PASSWORD"]:
                self.assertRegex(values[key], r"^[0-9a-f]{48}$")
                self.assertNotIn(values[key], first.stdout + first.stderr)
            second = subprocess.run(["bash", "-c", command], env=env, capture_output=True, text=True)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertEqual(env_path.read_text(), contents)
            self.assertEqual(env_path.stat().st_mode & 0o777, 0o600)


if __name__ == "__main__":
    unittest.main()
