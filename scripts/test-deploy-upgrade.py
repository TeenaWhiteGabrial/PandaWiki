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

    def test_base_upgrade_does_not_build(self):
        result, calls = self.run_upgrade(TEST_TARGET="base")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(line.startswith("compose build") for line in calls))


if __name__ == "__main__":
    unittest.main()
