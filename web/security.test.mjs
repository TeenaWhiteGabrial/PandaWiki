import test from 'node:test';
import assert from 'node:assert/strict';
import { allowedHost, pageSecurityHeaders } from './security.mjs';
test('host boundary rejects arbitrary DNS and malformed Host values', () => {
  for (const host of [
    'audit.invalid',
    'evil@localhost',
    'localhost/path',
    'localhost#x',
    '',
  ])
    assert.equal(allowedHost(host, ''), false, host);
  for (const host of [
    'localhost:3010',
    '127.0.0.1:8000',
    '[::1]:3010',
    '[2001:db8::1]:3010',
  ])
    assert.equal(allowedHost(host, ''), true, host);
  assert.equal(
    allowedHost(
      'wiki.example.test:3010',
      'wiki.example.test,admin.example.test',
    ),
    true,
  );
  assert.equal(allowedHost('audit.invalid', 'wiki.example.test'), false);
});
test('production page policy requires a nonce, widget embedding remains deliberate', () => {
  const normal = pageSecurityHeaders({ nonce: 'testnonce' });
  assert.ok(normal['Content-Security-Policy'].includes("'nonce-testnonce'"));
  assert.ok(!normal['Content-Security-Policy'].includes("'unsafe-eval'"));
  assert.ok(
    normal['Content-Security-Policy'].includes("frame-ancestors 'self'"),
  );
  const widget = pageSecurityHeaders({ widget: true, nonce: 'testnonce' });
  assert.equal(widget['X-Frame-Options'], undefined);
  assert.ok(widget['Content-Security-Policy'].includes('frame-ancestors *'));
});
