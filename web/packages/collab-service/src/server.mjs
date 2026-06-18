import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import jwt from 'jsonwebtoken';
import { WebSocketServer } from 'ws';
import * as Y from 'yjs';
import * as syncProtocol from 'y-protocols/sync';
import * as awarenessProtocol from 'y-protocols/awareness';
import * as encoding from 'lib0/encoding';
import * as decoding from 'lib0/decoding';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const dataRoot = path.resolve(
  process.env.COLLAB_DATA_DIR || path.resolve(__dirname, '../data'),
);
fs.mkdirSync(dataRoot, { recursive: true });

const port = Number(process.env.COLLAB_PORT || 1234);
const host = process.env.COLLAB_HOST || '0.0.0.0';
const jwtSecret =
  process.env.JWT_SECRET ||
  (process.env.JWT_SECRET_FILE
    ? fs.readFileSync(process.env.JWT_SECRET_FILE, 'utf8').trim()
    : '') ||
  'panda-wiki-dev-secret';
const jwtSecretFingerprint = crypto
  .createHash('sha256')
  .update(jwtSecret)
  .digest('hex')
  .slice(0, 12);

const messageSync = 0;
const messageAwareness = 1;
const heartbeatIntervalMs = 15000;
const compactDebounceMs = Number(
  process.env.COLLAB_COMPACT_DEBOUNCE_MS || 1000,
);
const compactIntervalMs = Number(
  process.env.COLLAB_COMPACT_INTERVAL_MS || 5 * 60 * 1000,
);
const compactUpdateThreshold = Number(
  process.env.COLLAB_COMPACT_UPDATE_THRESHOLD || 100,
);
const compactBytesThreshold = Number(
  process.env.COLLAB_COMPACT_BYTES_THRESHOLD || 5 * 1024 * 1024,
);
const emptyRoomTtlMs = Number(
  process.env.COLLAB_EMPTY_ROOM_TTL_MS || 2 * 60 * 1000,
);
const cleanupIntervalMs = Number(
  process.env.COLLAB_CLEANUP_INTERVAL_MS || 60 * 1000,
);
const maxLoadedRooms = Number(process.env.COLLAB_MAX_LOADED_ROOMS || 500);
const maxBlockRoomsPerUserDoc = Number(
  process.env.COLLAB_MAX_BLOCK_ROOMS_PER_USER_DOC || 48,
);

const rooms = new Map();
const userDocBlockRooms = new Map();

function roomPath(roomId) {
  return path.join(dataRoot, `${encodeURIComponent(roomId)}.bin`);
}

function roomDir(roomId) {
  return path.join(dataRoot, encodeURIComponent(roomId));
}

function roomFiles(roomId) {
  const dir = roomDir(roomId);
  return {
    dir,
    legacySnapshot: roomPath(roomId),
    snapshot: path.join(dir, 'snapshot.bin'),
    updates: path.join(dir, 'updates.log'),
    meta: path.join(dir, 'meta.json'),
  };
}

function readMeta(files) {
  if (!fs.existsSync(files.meta)) {
    return {
      updateCount: 0,
      updateBytes: 0,
      snapshotBytes: 0,
      compactCount: 0,
      createdAt: new Date().toISOString(),
      updatedAt: null,
      lastCompactAt: null,
    };
  }
  try {
    return JSON.parse(fs.readFileSync(files.meta, 'utf8'));
  } catch (err) {
    console.warn(
      'failed to read collab room meta:',
      err instanceof Error ? err.message : err,
    );
    return {
      updateCount: 0,
      updateBytes: 0,
      snapshotBytes: 0,
      compactCount: 0,
      createdAt: new Date().toISOString(),
      updatedAt: null,
      lastCompactAt: null,
    };
  }
}

function writeMeta(files, meta) {
  fs.mkdirSync(files.dir, { recursive: true });
  const nextMeta = {
    ...meta,
    updatedAt: new Date().toISOString(),
  };
  const tempPath = `${files.meta}.${process.pid}.tmp`;
  fs.writeFileSync(tempPath, JSON.stringify(nextMeta, null, 2));
  fs.renameSync(tempPath, files.meta);
  return nextMeta;
}

function readUpdatesLog(updatesPath, applyUpdate) {
  if (!fs.existsSync(updatesPath)) return { count: 0, bytes: 0 };

  const data = fs.readFileSync(updatesPath);
  let offset = 0;
  let count = 0;
  let bytes = 0;
  while (offset + 4 <= data.length) {
    const length = data.readUInt32BE(offset);
    offset += 4;
    if (length <= 0 || offset + length > data.length) {
      throw new Error(`corrupted updates log at offset ${offset - 4}`);
    }
    const update = data.subarray(offset, offset + length);
    applyUpdate(update);
    offset += length;
    count += 1;
    bytes += length;
  }
  if (offset !== data.length) {
    throw new Error(
      `corrupted updates log trailing bytes: ${data.length - offset}`,
    );
  }
  return { count, bytes };
}

function appendUpdate(files, update) {
  fs.mkdirSync(files.dir, { recursive: true });
  const bytes = Buffer.from(update);
  const header = Buffer.allocUnsafe(4);
  header.writeUInt32BE(bytes.length, 0);
  fs.appendFileSync(files.updates, Buffer.concat([header, bytes]));
  return bytes.length;
}

function compactRoom(room) {
  if (room.compacting) return;
  room.compacting = true;
  try {
    const snapshot = Buffer.from(Y.encodeStateAsUpdate(room.doc));
    fs.mkdirSync(room.files.dir, { recursive: true });

    const snapshotTemp = `${room.files.snapshot}.${process.pid}.tmp`;
    fs.writeFileSync(snapshotTemp, snapshot);
    fs.renameSync(snapshotTemp, room.files.snapshot);

    const updatesTemp = `${room.files.updates}.${process.pid}.tmp`;
    fs.writeFileSync(updatesTemp, Buffer.alloc(0));
    fs.renameSync(updatesTemp, room.files.updates);

    room.meta = writeMeta(room.files, {
      ...room.meta,
      updateCount: 0,
      updateBytes: 0,
      snapshotBytes: snapshot.length,
      compactCount: (room.meta.compactCount || 0) + 1,
      lastCompactAt: new Date().toISOString(),
    });
    console.log(
      `collab compacted room=${room.roomId} snapshotBytes=${snapshot.length} compactCount=${room.meta.compactCount}`,
    );
  } catch (err) {
    console.warn(
      'collab compact failed:',
      room.roomId,
      err instanceof Error ? err.message : err,
    );
  } finally {
    room.compacting = false;
  }
}

function scheduleCompact(room) {
  if (room.compactTimer) return;
  room.compactTimer = setTimeout(() => {
    room.compactTimer = null;
    compactRoom(room);
  }, compactDebounceMs);
}

function shouldCompact(room) {
  return (
    room.meta.updateCount >= compactUpdateThreshold ||
    room.meta.updateBytes >= compactBytesThreshold
  );
}

function roomKind(roomId) {
  if (roomId.startsWith('block:')) return 'block';
  if (roomId.startsWith('doc:')) return 'doc';
  return 'legacy';
}

function userDocKey(session) {
  return `${session.userId}:${session.kbId}:${session.nodeId}`;
}

function countLoadedClients() {
  let clients = 0;
  for (const room of rooms.values()) {
    clients += room.clients.size;
  }
  return clients;
}

function countTrackedUserDocBlockRooms() {
  let tracked = 0;
  for (const roomSet of userDocBlockRooms.values()) {
    tracked += roomSet.size;
  }
  return tracked;
}

function removeBlockRoomAccounting(room, client) {
  if (!client.blockUserDocKey) return;
  const roomSet = userDocBlockRooms.get(client.blockUserDocKey);
  if (!roomSet) return;
  roomSet.delete(room.roomId);
  if (roomSet.size === 0) {
    userDocBlockRooms.delete(client.blockUserDocKey);
  }
}

function unloadRoom(room, reason = 'idle', force = false) {
  if (!rooms.has(room.roomId) || (!force && room.clients.size > 0))
    return false;
  if (force) {
    for (const client of room.clients.values()) {
      removeBlockRoomAccounting(room, client);
      try {
        client.ws.close(1001, reason);
      } catch {
        // ignore close errors during shutdown
      }
    }
    room.clients.clear();
  }
  if (room.unloadTimer) {
    clearTimeout(room.unloadTimer);
    room.unloadTimer = null;
  }
  if (room.interval) {
    clearInterval(room.interval);
    room.interval = null;
  }
  if (room.compactTimer) {
    clearTimeout(room.compactTimer);
    room.compactTimer = null;
  }
  if (room.meta.updateCount > 0) {
    compactRoom(room);
  }
  room.awareness.destroy();
  room.doc.destroy();
  rooms.delete(room.roomId);
  console.log(`collab room unloaded room=${room.roomId} reason=${reason}`);
  return true;
}

function evictIdleRooms() {
  let evicted = 0;
  for (const room of rooms.values()) {
    if (rooms.size < maxLoadedRooms) break;
    if (room.clients.size === 0 && unloadRoom(room, 'capacity')) {
      evicted += 1;
    }
  }
  return evicted;
}

function cleanupIdleRooms() {
  const now = Date.now();
  let unloaded = 0;
  for (const room of rooms.values()) {
    if (
      room.clients.size === 0 &&
      room.emptySince &&
      now - room.emptySince >= emptyRoomTtlMs
    ) {
      if (unloadRoom(room, 'cleanup-interval')) {
        unloaded += 1;
      }
    }
  }
  if (unloaded > 0) {
    console.log(
      `collab cleanup unloaded=${unloaded} loadedRooms=${rooms.size}`,
    );
  }
}

function ensureRoom(roomId) {
  let room = rooms.get(roomId);
  if (room) {
    if (room.unloadTimer) {
      clearTimeout(room.unloadTimer);
      room.unloadTimer = null;
    }
    return room;
  }

  if (rooms.size >= maxLoadedRooms) {
    evictIdleRooms();
  }
  if (rooms.size >= maxLoadedRooms) {
    throw new Error(`too many loaded rooms: ${rooms.size}`);
  }

  const doc = new Y.Doc();
  const awareness = new awarenessProtocol.Awareness(doc);
  const clients = new Map();
  const files = roomFiles(roomId);
  fs.mkdirSync(files.dir, { recursive: true });
  let meta = readMeta(files);

  if (fs.existsSync(files.snapshot)) {
    const persisted = fs.readFileSync(files.snapshot);
    if (persisted.length > 0) {
      Y.applyUpdate(doc, new Uint8Array(persisted), 'persisted:snapshot');
      meta.snapshotBytes = persisted.length;
    }
  } else if (fs.existsSync(files.legacySnapshot)) {
    const legacy = fs.readFileSync(files.legacySnapshot);
    if (legacy.length > 0) {
      Y.applyUpdate(doc, new Uint8Array(legacy), 'persisted:legacy-snapshot');
      fs.writeFileSync(files.snapshot, legacy);
      meta.snapshotBytes = legacy.length;
    }
  }

  try {
    const restored = readUpdatesLog(files.updates, update => {
      Y.applyUpdate(doc, new Uint8Array(update), 'persisted:update');
    });
    meta.updateCount = restored.count;
    meta.updateBytes = restored.bytes;
  } catch (err) {
    console.warn(
      'collab room restore updates failed:',
      roomId,
      err instanceof Error ? err.message : err,
    );
  }

  meta = writeMeta(files, meta);

  room = {
    roomId,
    doc,
    awareness,
    clients,
    files,
    meta,
    compactTimer: null,
    unloadTimer: null,
    compacting: false,
    emptySince: null,
  };
  doc.on('update', (update, origin) => {
    if (typeof origin === 'string' && origin.startsWith('persisted:')) return;
    try {
      const bytes = appendUpdate(files, update);
      room.meta = writeMeta(files, {
        ...room.meta,
        updateCount: room.meta.updateCount + 1,
        updateBytes: room.meta.updateBytes + bytes,
      });
      if (shouldCompact(room)) {
        scheduleCompact(room);
      }
    } catch (err) {
      console.warn(
        'collab update persist failed:',
        roomId,
        err instanceof Error ? err.message : err,
      );
    }
  });
  room.interval = setInterval(() => {
    if (room.meta.updateCount > 0) {
      scheduleCompact(room);
    }
  }, compactIntervalMs);
  rooms.set(roomId, room);
  console.log(
    `collab room restored room=${roomId} snapshotBytes=${room.meta.snapshotBytes || 0} updateCount=${room.meta.updateCount || 0} updateBytes=${room.meta.updateBytes || 0}`,
  );
  return room;
}

function send(ws, bytes) {
  if (ws.readyState === ws.OPEN) {
    ws.send(bytes);
  }
}

function broadcast(room, bytes, except = null) {
  for (const client of room.clients.values()) {
    if (client.ws !== except) {
      send(client.ws, bytes);
    }
  }
}

function encodeAwareness(room, changedClients) {
  const encoder = encoding.createEncoder();
  encoding.writeVarUint(encoder, messageAwareness);
  encoding.writeVarUint8Array(
    encoder,
    awarenessProtocol.encodeAwarenessUpdate(room.awareness, changedClients),
  );
  return encoding.toUint8Array(encoder);
}

function sendJson(res, status, data) {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(data));
}

function getRoomStats(roomId) {
  const existing = rooms.get(roomId);
  if (existing) {
    return {
      roomId,
      loaded: true,
      clients: existing.clients.size,
      updateCount: existing.meta.updateCount || 0,
      updateBytes: existing.meta.updateBytes || 0,
      snapshotBytes: existing.meta.snapshotBytes || 0,
      compactCount: existing.meta.compactCount || 0,
      kind: roomKind(roomId),
      lastCompactAt: existing.meta.lastCompactAt || null,
      updatedAt: existing.meta.updatedAt || null,
    };
  }

  const files = roomFiles(roomId);
  const meta = readMeta(files);
  return {
    roomId,
    loaded: false,
    clients: 0,
    updateCount: meta.updateCount || 0,
    updateBytes: meta.updateBytes || 0,
    snapshotBytes: meta.snapshotBytes || 0,
    compactCount: meta.compactCount || 0,
    kind: roomKind(roomId),
    lastCompactAt: meta.lastCompactAt || null,
    updatedAt: meta.updatedAt || null,
  };
}

function closeClient(room, clientKey) {
  const existing = room.clients.get(clientKey);
  if (!existing) return;
  room.clients.delete(clientKey);
  removeBlockRoomAccounting(room, existing);
  awarenessProtocol.removeAwarenessStates(
    room.awareness,
    [existing.clientId],
    'disconnect',
  );
  const states = Array.from(room.awareness.getStates().keys());
  broadcast(room, encodeAwareness(room, states));

  if (room.clients.size === 0) {
    room.emptySince = Date.now();
    scheduleCompact(room);
    if (!room.unloadTimer) {
      room.unloadTimer = setTimeout(() => {
        room.unloadTimer = null;
        unloadRoom(room, 'empty-ttl');
      }, emptyRoomTtlMs);
    }
  }
}

function authorize(reqUrl) {
  if (!jwtSecret) {
    throw new Error('missing JWT_SECRET');
  }
  const url = new URL(reqUrl, 'http://localhost');
  const token = url.searchParams.get('token');
  const pathRoomId = decodeURIComponent(
    url.pathname.replace(/^\/collab\/ws\/?/, ''),
  );
  const roomId = url.searchParams.get('roomId') || pathRoomId;
  if (!token || !roomId) {
    throw new Error('missing token or roomId');
  }
  const payload = jwt.verify(token, jwtSecret);
  const expectedRoomId = `${payload.kb_id}:${payload.node_id}`;
  const expectedDocRoomId = `doc:${expectedRoomId}`;
  const expectedProseMirrorRoomId = `pm-v1:${expectedRoomId}`;
  const expectedProseMirrorDocRoomId = `doc:${expectedProseMirrorRoomId}`;
  const expectedBlockRoomPrefix = `block:${expectedRoomId}:`;
  const allowedRoom =
    expectedRoomId === roomId ||
    expectedDocRoomId === roomId ||
    expectedProseMirrorRoomId === roomId ||
    expectedProseMirrorDocRoomId === roomId ||
    roomId.startsWith(expectedBlockRoomPrefix);
  if (!allowedRoom || payload.scope !== 'collab') {
    throw new Error('invalid collab token');
  }
  return {
    roomId,
    userId: String(payload.id || ''),
    kbId: String(payload.kb_id || ''),
    nodeId: String(payload.node_id || ''),
    kind: roomKind(roomId),
  };
}

function handleMessage(room, ws, message) {
  const decoder = decoding.createDecoder(message);
  const messageType = decoding.readVarUint(decoder);

  if (messageType === messageSync) {
    const encoder = encoding.createEncoder();
    encoding.writeVarUint(encoder, messageSync);
    syncProtocol.readSyncMessage(decoder, encoder, room.doc, ws);
    const reply = encoding.toUint8Array(encoder);
    if (reply.length > 1) {
      send(ws, reply);
    }
    broadcast(room, message, ws);
    return;
  }

  if (messageType === messageAwareness) {
    const update = decoding.readVarUint8Array(decoder);
    awarenessProtocol.applyAwarenessUpdate(room.awareness, update, ws);
    const states = Array.from(room.awareness.getStates().keys());
    broadcast(room, encodeAwareness(room, states), null);
  }
}

const server = http.createServer((req, res) => {
  if (req.url?.startsWith('/healthz')) {
    sendJson(res, 200, {
      ok: true,
      rooms: rooms.size,
      clients: countLoadedClients(),
      trackedBlockRooms: countTrackedUserDocBlockRooms(),
      limits: {
        emptyRoomTtlMs,
        cleanupIntervalMs,
        maxLoadedRooms,
        maxBlockRoomsPerUserDoc,
      },
    });
    return;
  }
  if (req.url?.startsWith('/rooms/')) {
    const url = new URL(req.url, 'http://localhost');
    const match = url.pathname.match(/^\/rooms\/(.+)\/stats$/);
    const roomId = match ? decodeURIComponent(match[1]) : '';
    if (!roomId) {
      sendJson(res, 404, { error: 'expected /rooms/:roomId/stats' });
      return;
    }
    sendJson(res, 200, getRoomStats(roomId));
    return;
  }
  res.writeHead(404);
  res.end('not found');
});

const wss = new WebSocketServer({ noServer: true });

wss.on('connection', (ws, req, session) => {
  if (session.kind === 'block') {
    const key = userDocKey(session);
    const roomSet = userDocBlockRooms.get(key) || new Set();
    if (
      !roomSet.has(session.roomId) &&
      roomSet.size >= maxBlockRoomsPerUserDoc
    ) {
      ws.close(1013, 'too many block rooms');
      return;
    }
  }

  let room;
  try {
    room = ensureRoom(session.roomId);
  } catch (err) {
    console.warn(
      'collab room rejected:',
      err instanceof Error ? err.message : err,
    );
    ws.close(1013, 'room capacity exceeded');
    return;
  }
  const clientKey = `${session.userId}:${Date.now()}:${Math.random().toString(36).slice(2)}`;
  let blockUserDocKey = null;
  if (session.kind === 'block') {
    blockUserDocKey = userDocKey(session);
    const roomSet = userDocBlockRooms.get(blockUserDocKey) || new Set();
    roomSet.add(session.roomId);
    userDocBlockRooms.set(blockUserDocKey, roomSet);
  }
  room.clients.set(clientKey, {
    ws,
    clientId: Math.floor(Math.random() * Number.MAX_SAFE_INTEGER),
    blockUserDocKey,
  });
  room.emptySince = null;

  const syncStep1 = encoding.createEncoder();
  encoding.writeVarUint(syncStep1, messageSync);
  syncProtocol.writeSyncStep1(syncStep1, room.doc);
  send(ws, encoding.toUint8Array(syncStep1));

  const syncStep2 = encoding.createEncoder();
  encoding.writeVarUint(syncStep2, messageSync);
  syncProtocol.writeSyncStep2(syncStep2, room.doc);
  send(ws, encoding.toUint8Array(syncStep2));

  const states = Array.from(room.awareness.getStates().keys());
  if (states.length > 0) {
    send(ws, encodeAwareness(room, states));
  }

  const heartbeat = setInterval(() => {
    const encoder = encoding.createEncoder();
    encoding.writeVarUint(encoder, messageSync);
    syncProtocol.writeSyncStep1(encoder, room.doc);
    send(ws, encoding.toUint8Array(encoder));
  }, heartbeatIntervalMs);

  ws.on('message', raw => {
    handleMessage(room, ws, new Uint8Array(raw));
  });
  ws.on('close', () => {
    clearInterval(heartbeat);
    closeClient(room, clientKey);
  });
  ws.on('error', () => {
    clearInterval(heartbeat);
    closeClient(room, clientKey);
  });
});

server.on('upgrade', (req, socket, head) => {
  try {
    if (!req.url?.startsWith('/collab/ws')) {
      socket.destroy();
      return;
    }
    const session = authorize(req.url);
    wss.handleUpgrade(req, socket, head, ws => {
      wss.emit('connection', ws, req, session);
    });
  } catch (err) {
    console.warn(
      'collab websocket unauthorized:',
      err instanceof Error ? err.message : err,
    );
    socket.write('HTTP/1.1 401 Unauthorized\r\n\r\n');
    socket.destroy();
  }
});

const cleanupInterval = setInterval(cleanupIdleRooms, cleanupIntervalMs);

function shutdown() {
  clearInterval(cleanupInterval);
  for (const room of rooms.values()) {
    if (room.interval) {
      clearInterval(room.interval);
    }
    if (room.compactTimer) {
      clearTimeout(room.compactTimer);
      room.compactTimer = null;
    }
    unloadRoom(room, 'shutdown', true);
  }
}

process.on('SIGINT', () => {
  shutdown();
  process.exit(0);
});

process.on('SIGTERM', () => {
  shutdown();
  process.exit(0);
});

server.listen(port, host, () => {
  console.log(`collab service listening on ${host}:${port}`);
  console.log(`collab jwt secret sha256:${jwtSecretFingerprint}`);
});
