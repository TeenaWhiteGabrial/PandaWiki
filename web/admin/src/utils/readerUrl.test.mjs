import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const source = ts.transpileModule(
  await readFile(new URL('./readerUrl.ts', import.meta.url), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.ESNext } },
).outputText;
const { resolveReaderBaseUrl, getReaderDocumentUrl } = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`
);

test('HTTP deployment with empty HTTPS ports produces a reader URL', () => {
  const base = resolveReaderBaseUrl({
    hosts: ['100.120.3.46'],
    ports: [3010],
    ssl_ports: [],
  });
  assert.equal(base, 'http://100.120.3.46:3010');
  assert.equal(
    getReaderDocumentUrl(base, 'document-id'),
    'http://100.120.3.46:3010/node/document-id',
  );
  assert.equal(
    resolveReaderBaseUrl({ hosts: ['wiki.example.test'], ports: [80] }),
    'http://wiki.example.test',
  );
});

test('explicit reader URL and subpath take priority, with no double slash', () => {
  const base = resolveReaderBaseUrl({
    base_url: ' https://wiki.example.test/knowledge/ ',
    ports: [3010],
    ssl_ports: [8443],
  });
  assert.equal(
    getReaderDocumentUrl(base, 'document-id'),
    'https://wiki.example.test/knowledge/node/document-id',
  );
  assert.equal(
    resolveReaderBaseUrl({
      hosts: ['wiki.example.test'],
      ports: [80],
      ssl_ports: [443],
    }),
    'https://wiki.example.test',
  );
  assert.equal(
    resolveReaderBaseUrl({ hosts: ['wiki.example.test'], ssl_ports: [8443] }),
    'https://wiki.example.test:8443',
  );
});

test('missing or invalid addresses cannot fall back to the admin origin', () => {
  for (const settings of [
    undefined,
    {},
    { hosts: ['100.120.3.46'], ssl_ports: [] },
    { hosts: ['*'], ports: [3010] },
  ]) {
    assert.equal(resolveReaderBaseUrl(settings), '');
  }
  for (const base of [
    undefined,
    '',
    ' ',
    '/node',
    'javascript:alert(1)',
    'not-a-url',
  ]) {
    assert.equal(getReaderDocumentUrl(base, 'document-id'), '');
  }
});

test('an address removed after loading does not reuse the previous configuration', () => {
  assert.equal(
    resolveReaderBaseUrl({ base_url: 'https://wiki.example.test' }),
    'https://wiki.example.test',
  );
  assert.equal(resolveReaderBaseUrl({}), '');
});
