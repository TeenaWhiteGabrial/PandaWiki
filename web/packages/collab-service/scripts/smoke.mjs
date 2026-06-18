import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import jwt from 'jsonwebtoken';
import WebSocket from 'ws';
import * as Y from 'yjs';
import * as syncProtocol from 'y-protocols/sync';
import * as encoding from 'lib0/encoding';
import * as decoding from 'lib0/decoding';

const serviceRoot = path.resolve(import.meta.dirname, '..');
const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'panda-collab-smoke-'));
const port = 22345 + Math.floor(Math.random() * 1000);
const jwtSecret = 'panda-wiki-smoke-secret';
const kbId = 'kb-smoke';
const nodeId = 'node-smoke';
const roomId = `${kbId}:${nodeId}`;
const endpoint = `ws://127.0.0.1:${port}/collab/ws`;
const messageSync = 0;

const token = jwt.sign(
  {
    id: 'user-smoke',
    kb_id: kbId,
    node_id: nodeId,
    scope: 'collab',
    exp: Math.floor(Date.now() / 1000) + 3600,
  },
  jwtSecret,
);

const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

const waitFor = async (fn, label, timeoutMs = 5000) => {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    const value = await fn();
    if (value) return value;
    await sleep(100);
  }
  throw new Error(`timeout waiting for ${label}`);
};

const requestJson = pathname =>
  new Promise((resolve, reject) => {
    const req = http.get(
      {
        hostname: '127.0.0.1',
        port,
        path: pathname,
      },
      res => {
        let body = '';
        res.setEncoding('utf8');
        res.on('data', chunk => {
          body += chunk;
        });
        res.on('end', () => {
          if ((res.statusCode || 0) >= 400) {
            reject(
              new Error(`GET ${pathname} failed: ${res.statusCode} ${body}`),
            );
            return;
          }
          resolve(JSON.parse(body));
        });
      },
    );
    req.on('error', reject);
  });

const startServer = async () => {
  const child = spawn(process.execPath, ['src/server.mjs'], {
    cwd: serviceRoot,
    env: {
      ...process.env,
      COLLAB_HOST: '127.0.0.1',
      COLLAB_PORT: String(port),
      COLLAB_DATA_DIR: dataDir,
      COLLAB_COMPACT_DEBOUNCE_MS: '50',
      COLLAB_COMPACT_INTERVAL_MS: '200',
      COLLAB_COMPACT_UPDATE_THRESHOLD: '1',
      COLLAB_COMPACT_BYTES_THRESHOLD: '1',
      COLLAB_EMPTY_ROOM_TTL_MS: '500',
      COLLAB_CLEANUP_INTERVAL_MS: '200',
      JWT_SECRET: jwtSecret,
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  child.stdout.setEncoding('utf8');
  child.stderr.setEncoding('utf8');
  child.stdout.on('data', chunk => process.stdout.write(`[collab] ${chunk}`));
  child.stderr.on('data', chunk => process.stderr.write(`[collab] ${chunk}`));

  await waitFor(
    () =>
      requestJson('/healthz')
        .then(() => true)
        .catch(() => false),
    'collab service healthz',
  );
  return child;
};

const stopServer = async child => {
  if (child.exitCode !== null) return;
  child.kill('SIGTERM');
  await new Promise(resolve => {
    child.once('exit', resolve);
    setTimeout(resolve, 3000);
  });
};

const encodeSyncStep1 = doc => {
  const encoder = encoding.createEncoder();
  encoding.writeVarUint(encoder, messageSync);
  syncProtocol.writeSyncStep1(encoder, doc);
  return encoding.toUint8Array(encoder);
};

const encodeUpdate = update => {
  const encoder = encoding.createEncoder();
  encoding.writeVarUint(encoder, messageSync);
  syncProtocol.writeUpdate(encoder, update);
  return encoding.toUint8Array(encoder);
};

const applyServerMessage = (doc, raw) => {
  const decoder = decoding.createDecoder(new Uint8Array(raw));
  const messageType = decoding.readVarUint(decoder);
  if (messageType !== messageSync) return;
  const encoder = encoding.createEncoder();
  encoding.writeVarUint(encoder, messageSync);
  syncProtocol.readSyncMessage(decoder, encoder, doc, null);
};

const connectDoc = async () => {
  const doc = new Y.Doc();
  const ws = new WebSocket(
    `${endpoint}?token=${encodeURIComponent(token)}&roomId=${encodeURIComponent(roomId)}`,
  );
  ws.on('message', raw => {
    applyServerMessage(doc, raw);
  });
  await waitFor(() => ws.readyState === WebSocket.OPEN, 'websocket connection');
  ws.send(encodeSyncStep1(doc));
  await sleep(100);
  return { doc, ws };
};

let server;
try {
  server = await startServer();

  const first = await connectDoc();
  Y.transact(first.doc, () => {
    first.doc.getText('content').insert(0, 'hello persisted collab');
  });
  first.ws.send(encodeUpdate(Y.encodeStateAsUpdate(first.doc)));
  await waitFor(async () => {
    const stats = await requestJson(
      `/rooms/${encodeURIComponent(roomId)}/stats`,
    );
    return stats.compactCount > 0 && stats.snapshotBytes > 0;
  }, 'room compact');
  first.ws.close();
  first.doc.destroy();

  await stopServer(server);
  server = await startServer();

  const second = await connectDoc();
  second.ws.send(encodeSyncStep1(second.doc));
  await waitFor(
    () => second.doc.getText('content').toString() === 'hello persisted collab',
    'persisted document restore',
  );
  const restoredStats = await requestJson(
    `/rooms/${encodeURIComponent(roomId)}/stats`,
  );
  if (!restoredStats.loaded || restoredStats.snapshotBytes <= 0) {
    throw new Error(
      `unexpected restored stats: ${JSON.stringify(restoredStats)}`,
    );
  }
  second.ws.close();
  second.doc.destroy();

  console.log('collab smoke passed');
} finally {
  if (server) {
    await stopServer(server);
  }
  fs.rmSync(dataDir, { recursive: true, force: true });
}
