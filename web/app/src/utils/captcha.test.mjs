import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const source = ts.transpileModule(
  await readFile(new URL('./captcha.ts', import.meta.url), 'utf8'),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const { createCaptchaFetch } = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`
);
const origin = 'http://100.120.3.46:3010';

test('logged-in challenge and redemption carry CSRF while retaining request options', async () => {
  for (const path of [
    '/share/v1/captcha/challenge',
    '/kb/share/v1/captcha/redeem',
  ]) {
    const inputHeaders = { 'Content-Type': 'application/json' };
    const controller = new AbortController();
    const fetch = createCaptchaFetch(origin, async (input, init) => {
      assert.equal(input, path);
      assert.equal(init.headers.get('X-PW-CSRF'), '1');
      assert.equal(init.headers.get('Content-Type'), 'application/json');
      assert.equal(init.credentials, 'same-origin');
      assert.equal(init.body, '{"token":"test"}');
      assert.equal(init.signal, controller.signal);
      return Response.json({ success: true });
    });
    const response = await fetch(path, {
      method: 'POST',
      headers: inputHeaders,
      credentials: 'same-origin',
      body: '{"token":"test"}',
      signal: controller.signal,
    });
    assert.equal(response.status, 200);
    assert.equal(inputHeaders['X-PW-CSRF'], undefined);
  }
});

test('does not manufacture CSRF for external URLs or unrelated endpoints', async () => {
  for (const path of [
    'https://other.test/share/v1/captcha/challenge',
    '/share/v1/auth/login/account',
    '/share/v1/captcha/challenge/extra',
  ]) {
    const init = { method: 'POST', headers: { Origin: 'https://other.test' } };
    const fetch = createCaptchaFetch(origin, async (input, options) => {
      assert.equal(input, path);
      assert.equal(options, init);
      return Response.json({});
    });
    await fetch(path, init);
  }
});

test('preserves Origin and supports Request inputs', async () => {
  const request = new Request(origin + '/share/v1/captcha/redeem', {
    method: 'POST',
    body: '{}',
    headers: { Origin: origin, 'Content-Type': 'application/json' },
  });
  await createCaptchaFetch(origin, async (input, init) => {
    assert.equal(input, request);
    assert.equal(init.headers.get('Origin'), origin);
    assert.equal(init.headers.get('X-PW-CSRF'), '1');
    assert.equal(init.headers.get('Content-Type'), 'application/json');
    return Response.json({});
  })(request);
});

test('failed validation retains HTTP status and endpoint for diagnostics', async () => {
  await assert.rejects(
    createCaptchaFetch(origin, async () =>
      Response.json({ message: 'private server detail' }, { status: 403 }),
    )('/share/v1/captcha/challenge', { method: 'POST' }),
    error =>
      error.status === 403 &&
      error.path === '/share/v1/captcha/challenge' &&
      !error.message.includes('private server detail'),
  );
});

test('every Cap entry point uses the shared CSRF integration', async () => {
  for (const file of [
    '../views/node/DocContent.tsx',
    '../views/widget/SearchDocContent.tsx',
    '../views/widget/AiQaContent.tsx',
    '../components/commentInput/index.tsx',
    '../components/QaModal/SearchDocContent.tsx',
    '../components/QaModal/AiQaContent.tsx',
  ]) {
    const text = await readFile(new URL(file, import.meta.url), 'utf8');
    assert.match(text, /await loadCaptcha\(\)/);
    assert.doesNotMatch(text, /import\(['`"]@cap\.js\/widget/);
  }
});
