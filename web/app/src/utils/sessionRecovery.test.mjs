import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const asModule = source =>
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
let moduleID = 0;
async function compile(file, imports = {}) {
  let source = ts.transpileModule(
    await readFile(new URL(file, import.meta.url), 'utf8'),
    {
      compilerOptions: {
        module: ts.ModuleKind.ESNext,
        target: ts.ScriptTarget.ES2022,
      },
    },
  ).outputText;
  for (const [name, url] of Object.entries(imports)) {
    source = source.replaceAll(`from '${name}'`, `from '${url}'`);
  }
  return asModule(`${source}\n// isolated module ${moduleID++}`);
}
const authURL = await compile('./auth.ts');

async function setup(
  t,
  { base = '', pathname = '/node/doc', search = '?a=1' } = {},
) {
  const saved = Object.fromEntries(
    ['window', 'document', 'location', 'fetch'].map(key => [
      key,
      globalThis[key],
    ]),
  );
  const events = [],
    redirects = [],
    calls = [];
  const location = {
    pathname: base + pathname,
    search,
    get href() {
      return `http://localhost${this.pathname}${this.search}`;
    },
    set href(value) {
      redirects.push(value);
    },
  };
  globalThis.window = {
    _BASE_PATH_: base,
    location,
    localStorage: { removeItem() {}, setItem() {} },
    dispatchEvent: event => events.push(event.type),
  };
  globalThis.location = location;
  globalThis.document = { set cookie(_) {} };
  globalThis.fetch = async (...args) => {
    calls.push(args);
    return Response.json({ success: true, data: { id: 'reader' } });
  };
  t.after(() => {
    for (const [key, value] of Object.entries(saved)) {
      if (value === undefined) delete globalThis[key];
      else globalThis[key] = value;
    }
  });
  const recoveryURL = await compile('./sessionRecovery.ts', {
    './auth': authURL,
  });
  return {
    ...(await import(recoveryURL)),
    recoveryURL,
    events,
    redirects,
    calls,
  };
}

test('concurrent ambiguous 401s share verification and redirect once with the original path', async t => {
  const env = await setup(t, { base: '/wiki' });
  let release;
  globalThis.fetch = (...args) => {
    env.calls.push(args);
    return new Promise(resolve => {
      release = resolve;
    });
  };
  const pending = Array.from({ length: 10 }, () =>
    env.handleReaderAuthFailure(401),
  );
  assert.equal(env.calls.length, 1);
  assert.equal(env.calls[0][0], '/wiki/share/v1/auth/user');
  assert.equal(env.calls[0][1].credentials, 'same-origin');
  assert.equal(env.calls[0][1].cache, 'no-store');
  release(Response.json({ success: false }, { status: 401 }));
  await Promise.all(pending);
  assert.deepEqual(env.events, ['panda-wiki-auth-cleared']);
  assert.deepEqual(env.redirects, [
    '/wiki/auth/login?redirect=%2Fnode%2Fdoc%3Fa%3D1',
  ]);
});

test('valid identity preserves login and a later 401 performs a fresh check', async t => {
  const env = await setup(t);
  await env.handleReaderAuthFailure(401, 0);
  await env.handleReaderAuthFailure(401);
  assert.equal(env.calls.length, 2);
  assert.deepEqual(env.events, []);
  assert.deepEqual(env.redirects, []);
});

test('permission errors do not trigger identity verification', async t => {
  const env = await setup(t);
  await env.handleReaderAuthFailure(403, 40003);
  await env.handleReaderAuthFailure(200, 40003);
  assert.equal(env.calls.length, 0);
  assert.deepEqual(env.events, []);
});

for (const outcome of ['network', 'timeout', '503', '403', 'invalid-json']) {
  test(`identity ${outcome} does not erase login`, async t => {
    const env = await setup(t);
    globalThis.fetch = async () => {
      if (outcome === 'network') throw new TypeError('Failed to fetch');
      if (outcome === 'timeout')
        throw new DOMException('timed out', 'TimeoutError');
      if (outcome === 'invalid-json')
        return new Response('<html>unavailable</html>');
      return Response.json({ success: false }, { status: Number(outcome) });
    };
    await env.handleReaderAuthFailure(401);
    assert.deepEqual(env.events, []);
    assert.deepEqual(env.redirects, []);
  });
}

test('explicit session expiry redirects without another identity request', async t => {
  const env = await setup(t);
  await env.handleReaderAuthFailure(200, 40101);
  assert.equal(env.calls.length, 0);
  assert.equal(env.redirects.length, 1);
});

test('identity endpoint 401 is handled directly without recursion', async t => {
  const env = await setup(t);
  await env.handleReaderAuthFailure(401, 0, '/share/v1/auth/user');
  assert.equal(env.calls.length, 0);
  assert.equal(env.redirects.length, 1);
});

test('login failures keep the form, and expiry on login does not redirect to itself', async t => {
  const env = await setup(t, { base: '/wiki', pathname: '/auth/login' });
  await env.handleReaderAuthFailure(401, 40101, '/share/v1/auth/login/account');
  assert.equal(env.calls.length, 0);
  assert.deepEqual(env.events, []);
  await env.handleReaderAuthFailure(401, 40101);
  assert.deepEqual(env.redirects, []);
  assert.equal(env.events.length, 1);
});

const serverURL = asModule(
  `export const getServerHeader=async()=>({}); export const getServerBasePath=async()=>''; export const getServerPathname=async()=>'/'; export const getServerSearch=async()=>'';`,
);
const uiURL = asModule(`export const message={error(){}};`);
const navigationURL = asModule(
  `export const redirect=()=>{throw Error('unexpected server redirect')};`,
);
for (const client of [
  '../request/httpClient.ts',
  '../request/pro/httpClient.ts',
]) {
  for (const expired of [false, true]) {
    test(`${client}: ambiguous 401 rejects the original request and only logs out an invalid session (${expired})`, async t => {
      const env = await setup(t);
      globalThis.fetch = async (...args) => {
        env.calls.push(args);
        return expired
          ? Response.json({ success: false }, { status: 401 })
          : Response.json({ success: true, data: { id: 'reader' } });
      };
      const clientURL = await compile(client, {
        '@/utils/getServerHeader': serverURL,
        '@/utils/sessionRecovery': env.recoveryURL,
        '@ctzhian/ui': uiURL,
        'next/navigation': navigationURL,
        '../../../credentialTransport.mjs': new URL(
          '../../../credentialTransport.mjs',
          import.meta.url,
        ).href,
      });
      const { HttpClient } = await import(clientURL);
      let attempts = 0;
      const http = new HttpClient({
        customFetch: async () => {
          attempts++;
          const response = new Response('not JSON', { status: 401 });
          Object.defineProperty(response, 'url', {
            value: 'http://localhost/share/v1/node/detail',
          });
          return response;
        },
      });
      await assert.rejects(
        http.request({
          path: '/share/v1/node/detail',
          method: 'GET',
          isAlert: false,
        }),
        error => error.code === 401,
      );
      assert.equal(
        attempts,
        1,
        'do not automatically replay business requests',
      );
      assert.equal(env.calls.length, 1);
      assert.equal(env.events.length, expired ? 1 : 0);
      assert.equal(env.redirects.length, expired ? 1 : 0);
    });
  }
}

for (const expired of [false, true]) {
  test(`SSE: a non-JSON 401 verifies identity and keeps the original error (${expired})`, async t => {
    const env = await setup(t);
    globalThis.fetch = async (url, options) => {
      env.calls.push([url, options]);
      if (url === '/share/v1/chat/message')
        return new Response('Unauthorized', { status: 401 });
      return expired
        ? Response.json({ success: false, code: 40101 }, { status: 401 })
        : Response.json({ success: true, data: { id: 'reader' } });
    };
    const sseURL = await compile('./fetch.ts', {
      './sessionRecovery': env.recoveryURL,
    });
    const { default: SSEClient } = await import(sseURL);
    const error = await new Promise(resolve => {
      new SSEClient({
        url: '/share/v1/chat/message',
        onError: resolve,
      }).subscribe('{}', () => assert.fail('no stream data expected'));
    });
    assert.equal(error.status, 401);
    assert.equal(env.calls.length, 2);
    assert.equal(env.events.length, expired ? 1 : 0);
    assert.equal(env.redirects.length, expired ? 1 : 0);
  });
}
