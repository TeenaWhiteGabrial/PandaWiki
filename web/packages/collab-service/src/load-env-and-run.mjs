import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(__dirname, '../../..');
const repoRoot = path.resolve(webRoot, '..');

const candidates = [
  process.env.COLLAB_ENV_FILE,
  path.resolve(repoRoot, 'server/.env'),
  path.resolve(repoRoot, 'deploy/server/.env'),
].filter(Boolean);

for (const file of candidates) {
  if (!fs.existsSync(file)) continue;

  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const match = trimmed.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    if (!match) continue;

    const [, key, rawValue] = match;
    if (process.env[key]) continue;
    process.env[key] = rawValue.replace(/^['"]|['"]$/g, '');
  }
  console.log(`collab loaded env from ${file}`);
  break;
}

const child = spawn(process.execPath, [path.resolve(__dirname, 'server.mjs')], {
  stdio: 'inherit',
  env: process.env,
});

child.on('exit', code => {
  process.exit(code ?? 0);
});
