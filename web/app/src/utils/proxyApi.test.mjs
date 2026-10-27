import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import http from 'node:http';
import { test } from 'node:test';
import ts from 'typescript';

const source = ts.transpileModule(
  await readFile(new URL('./proxyApi.ts', import.meta.url), 'utf8'),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const { proxyApiRequest } = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`
);

async function backend(t, handler) {
  const server = http.createServer(handler);
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(
    () =>
      new Promise(resolve => {
        server.closeAllConnections();
        server.close(resolve);
      }),
  );
  return new URL(
    `http://127.0.0.1:${server.address().port}/share/v1/auth/login/account`,
  );
}

const publicOrigin = 'http://100.120.3.46:3010';
const loginRequest = headers =>
  new Request(`${publicOrigin}/share/v1/auth/login/account`, {
    method: 'POST',
    headers: {
      Host: '100.120.3.46:3010',
      Origin: publicOrigin,
      'Content-Type': 'application/json',
      ...headers,
    },
    body: '{}',
  });

test('preserves public Host, original Origin and CSRF marker through an internal target', async t => {
  const target = await backend(t, (req, res) => {
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Set-Cookie', [
      'pw_reader_session=fixture; HttpOnly; Path=/',
      'legacy=; Max-Age=0; Path=/',
    ]);
    res.end(
      JSON.stringify({
        host: req.headers.host,
        origin: req.headers.origin,
        csrf: req.headers['x-pw-csrf'],
        site: req.headers['sec-fetch-site'],
      }),
    );
  });
  const result = await proxyApiRequest(
    loginRequest({ 'X-PW-CSRF': '1', 'Sec-Fetch-Site': 'same-origin' }),
    target,
  );
  assert.deepEqual(await result.json(), {
    host: '100.120.3.46:3010',
    origin: publicOrigin,
    csrf: '1',
    site: 'same-origin',
  });
  assert.equal(result.headers.getSetCookie().length, 2);
});

test('does not bless a cross-origin request or synthesize a missing CSRF marker', async t => {
  const target = await backend(t, (req, res) => {
    res.setHeader('Content-Type', 'application/json');
    res.end(
      JSON.stringify({
        origin: req.headers.origin,
        csrf: req.headers['x-pw-csrf'],
        site: req.headers['sec-fetch-site'],
      }),
    );
  });
  const result = await proxyApiRequest(
    loginRequest({
      Origin: 'https://untrusted.example.test',
      'Sec-Fetch-Site': 'cross-site',
    }),
    target,
  );
  assert.deepEqual(await result.json(), {
    origin: 'https://untrusted.example.test',
    site: 'cross-site',
  });
});

test('streams binary uploads without changing their bytes', async t => {
  const chunks = [Buffer.alloc(96 * 1024, 0x81), Buffer.alloc(37 * 1024, 0x02)];
  const target = await backend(t, async (req, res) => {
    const received = [];
    for await (const part of req) received.push(part);
    res.end(Buffer.concat(received));
  });
  const upload = new Request(`${publicOrigin}/share/v1/file/upload`, {
    method: 'POST',
    headers: {
      Host: '100.120.3.46:3010',
      'Content-Type': 'application/octet-stream',
    },
    body: new ReadableStream({
      start(controller) {
        chunks.forEach(chunk => controller.enqueue(chunk));
        controller.close();
      },
    }),
    duplex: 'half',
  });
  const result = await proxyApiRequest(upload, target);
  assert.deepEqual(
    Buffer.from(await result.arrayBuffer()),
    Buffer.concat(chunks),
  );
});

test(
  'delivers the first SSE chunk before the upstream finishes',
  { timeout: 5000 },
  async t => {
    let finish;
    const lastChunk = new Promise(resolve => {
      finish = resolve;
    });
    const target = await backend(t, (_req, res) => {
      res.setHeader('Content-Type', 'text/event-stream');
      res.write('data: first\n\n');
      lastChunk.then(() => res.end('data: last\n\n'));
    });
    const result = await proxyApiRequest(
      new Request(publicOrigin, { headers: { Host: '100.120.3.46:3010' } }),
      target,
    );
    const reader = result.body.getReader();
    const first = await reader.read();
    assert.equal(new TextDecoder().decode(first.value), 'data: first\n\n');
    finish();
    const rest = await reader.read();
    assert.equal(new TextDecoder().decode(rest.value), 'data: last\n\n');
    assert.equal((await reader.read()).done, true);
  },
);

test('returns redirects and bodyless responses without following redirects', async t => {
  const target = await backend(t, (_req, res) => {
    res.writeHead(302, { Location: '/auth/login' });
    res.end();
  });
  const result = await proxyApiRequest(new Request(publicOrigin), target);
  assert.equal(result.status, 302);
  assert.equal(result.headers.get('location'), '/auth/login');
  await result.text();
  const emptyTarget = await backend(t, (_req, res) => {
    res.writeHead(204);
    res.end();
  });
  const empty = await proxyApiRequest(new Request(publicOrigin), emptyTarget);
  assert.equal(empty.status, 204);
  assert.equal(empty.body, null);
});

test(
  'aborting a request closes the upstream connection',
  { timeout: 5000 },
  async t => {
    let arrived;
    const arrival = new Promise(resolve => {
      arrived = resolve;
    });
    const target = await backend(t, () => arrived());
    const controller = new AbortController();
    const result = proxyApiRequest(
      new Request(publicOrigin, { signal: controller.signal }),
      target,
    );
    // Install the rejection handler before aborting.
    const rejected = assert.rejects(result, { name: 'AbortError' });
    await arrival;
    controller.abort();
    await rejected;
  },
);
