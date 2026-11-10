"""Render production Compose with synthetic values; never start Docker services."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
COMPOSE = ROOT / 'deploy/server/docker-compose.yml'


class ProductionAPINetworkTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not shutil.which('docker') or subprocess.run(
            ['docker', 'compose', 'version'], capture_output=True,
        ).returncode:
            raise unittest.SkipTest('Docker Compose plugin required for config-only rendering')
        cls.values = {}
        for line in (ROOT / 'deploy/server/.env.example').read_text().splitlines():
            if line and not line.startswith('#') and '=' in line:
                key, value = line.split('=', 1)
                cls.values[key] = value
        cls.values.update({
            'SERVER_IP': '192.0.2.10', 'PROJECT_NAME': 'network-fixture',
            'API_PORT': '18000',  # Legacy .env values must not republish the API.
            'REDIS_PASSWORD': 'fixture-redis', 'NATS_PASSWORD': 'fixture-nats',
            'OSS_ENDPOINT': 'https://storage.example.test', 'OSS_ACCESS_KEY': 'fixture-access',
            'OSS_SECRET_KEY': 'fixture-secret', 'OSS_BUCKET_NAME': 'fixture-bucket',
            'OSS_DOWNLOAD_URL': 'https://storage.example.test/fixture-bucket', 'OSS_REGION': 'fixture',
        })
        # Do not inherit deployment credentials or shell overrides.
        variables = set(re.findall(r'\$\{([A-Z][A-Z_0-9]*)', COMPOSE.read_text()))
        cls.env = {k: v for k, v in os.environ.items() if k not in variables}
        cls.env.update(cls.values)
        with tempfile.NamedTemporaryFile(mode='w') as empty_env:
            empty_env.write('# synthetic configuration only\n')
            empty_env.flush()
            result = subprocess.run([
                'docker', 'compose', '-p', 'network-fixture', '--env-file', empty_env.name,
                '-f', str(COMPOSE), 'config', '--format', 'json',
            ], env=cls.env, text=True, capture_output=True)
        if result.returncode:
            raise AssertionError('Synthetic Compose configuration failed: ' + result.stderr)
        cls.config = json.loads(result.stdout)

    def test_api_has_no_host_binding_even_with_legacy_api_port(self):
        self.assertFalse(self.config['services']['api'].get('ports'))
        self.assertNotIn('${API_PORT}', COMPOSE.read_text())

    def test_frontends_still_publish_their_existing_ports(self):
        for service, port in [('admin', '18080'), ('app', '3010')]:
            self.assertEqual(str(self.config['services'][service]['ports'][0]['published']), port)

    def test_runtime_and_build_targets_stay_on_compose_network(self):
        for service in ['app', 'admin-build', 'app-build']:
            self.assertEqual(self.config['services'][service]['environment']['TARGET'], 'http://api:8000')
        for service in ['admin-build', 'app-build']:
            self.assertEqual(self.config['services'][service]['environment']['SWAGGER_BASE_URL'], 'http://api:8000')
        nginx = (ROOT / 'web/admin/server.conf').read_text()
        self.assertIn('server api:8000;', nginx)
        self.assertIn('proxy_buffering off;', nginx)

    def test_data_volumes_keep_existing_project_scoped_names_and_mounts(self):
        for service, suffix, destination in [
            ('postgres', 'postgres', '/var/lib/postgresql/data'),
            ('redis', 'redis', '/data'), ('nats', 'nats', '/data'),
            ('qdrant', 'qdrant', '/qdrant/storage'), ('raglite', 'raglite', '/data'),
        ]:
            source = 'pandawiki_' + suffix
            mounts = self.config['services'][service]['volumes']
            self.assertTrue(any(v['type'] == 'volume' and v['source'] == source and v['target'] == destination for v in mounts))
            self.assertEqual(self.config['volumes'][source]['name'], 'network-fixture_' + source)

    def test_summary_no_longer_requires_a_published_api_port(self):
        with tempfile.NamedTemporaryFile(mode='w') as env_file:
            env_file.write('SERVER_IP=192.0.2.10\nADMIN_PORT=18080\nAPP_PORT=3010\n')
            env_file.flush()
            env = dict(self.env, TEST_ENV_FILE=env_file.name)
            result = subprocess.run(['bash', '-c', '''
set -euo pipefail
source deploy/server/deploy.sh
ENV_FILE="$TEST_ENV_FILE"
COMPOSE_BIN=(docker-compose)
print_summary
'''], cwd=ROOT, env=env, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('http://192.0.2.10:18080', result.stdout)
        self.assertIn('http://192.0.2.10:3010', result.stdout)
        self.assertNotIn('http://192.0.2.10:8000', result.stdout)
        self.assertIn('api:8000', result.stdout)


if __name__ == '__main__':
    unittest.main()
