import test from 'node:test';
import assert from 'node:assert/strict';
import { loadPreviewContent } from './load-content.mjs';

test('authenticated fetch validates PDF bytes and reports progress', async t => {
  let progress;
  t.mock.method(globalThis, 'fetch', async (_url, options) => {
    assert.equal(options.credentials, 'same-origin');
    assert.equal(options.cache, 'no-store');
    return new Response('%PDF-1.7\nfixture', {
      headers: { 'content-type': 'application/pdf', 'content-length': '16' },
    });
  });
  const bytes = await loadPreviewContent({
    url: '/local/file',
    kind: 'pdf',
    onProgress: (...p) => (progress = p),
  });
  assert.equal(new TextDecoder().decode(bytes), '%PDF-1.7\nfixture');
  assert.deepEqual(progress, [16, 16]);
});

test('permission errors preserve login and auth failures reach the session handler', async t => {
  for (const [status, code] of [
    [403, 40003],
    [401, 0],
    [200, 40101],
  ]) {
    const data = { success: false, code, message: '没有权限' };
    t.mock.method(
      globalThis,
      'fetch',
      async () =>
        new Response(JSON.stringify(data), {
          status,
          headers: { 'content-type': 'application/json' },
        }),
    );
    let access;
    await assert.rejects(
      loadPreviewContent({
        url: '/local/file',
        kind: 'pdf',
        onAccessError: e => (access = e),
      }),
      /没有权限/,
    );
    assert.deepEqual(access, { status, data });
    t.mock.restoreAll();
  }
});

test('HTML responses, invalid PDF and oversized files cannot reach the renderer', async t => {
  for (const [body, headers, message] of [
    ['<html/>', { 'content-type': 'text/html' }, /未返回 PDF/],
    ['<script/>', { 'content-type': 'application/pdf' }, /不是有效的 PDF/],
    [
      '%PDF-',
      {
        'content-type': 'application/pdf',
        'content-length': String(100 * 1024 * 1024 + 1),
      },
      /文件过大/,
    ],
    [
      '%PDF-',
      { 'content-type': 'application/pdf', 'content-length': '100' },
      /传输不完整/,
    ],
  ]) {
    t.mock.method(
      globalThis,
      'fetch',
      async () => new Response(body, { headers }),
    );
    await assert.rejects(
      loadPreviewContent({ url: '/local/file', kind: 'pdf' }),
      message,
    );
    t.mock.restoreAll();
  }
});

test('missing objects and timeouts produce actionable errors', async t => {
  t.mock.method(
    globalThis,
    'fetch',
    async () => new Response('', { status: 404 }),
  );
  await assert.rejects(
    loadPreviewContent({ url: '/local/file', kind: 'pdf' }),
    /尚未迁移/,
  );
  t.mock.restoreAll();
  t.mock.method(
    globalThis,
    'fetch',
    (_url, { signal }) =>
      new Promise((_, reject) =>
        signal.addEventListener('abort', () =>
          reject(new DOMException('abort', 'AbortError')),
        ),
      ),
  );
  await assert.rejects(
    loadPreviewContent({ url: '/local/file', kind: 'pdf', timeoutMs: 10 }),
    /加载超时/,
  );
});
