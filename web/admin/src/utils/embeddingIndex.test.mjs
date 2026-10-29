import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const source = ts.transpileModule(
  await readFile(new URL('./embeddingIndex.ts', import.meta.url), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.ESNext } },
).outputText;
const { autoEmbeddingIndexConfig, embeddingIndexNeedsRebuild } = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`
);

const config = {
  provider: 'CompatiblePlatform',
  api_key: 'old',
  base_url: 'http://old/v1',
  models: { embedding: 'embed-v1', chat: 'chat-v1' },
};
const baseline = autoEmbeddingIndexConfig(config);

test('credentials, endpoint and non-embedding changes do not show a rebuild confirmation', () => {
  const next = {
    ...config,
    api_key: 'new',
    base_url: 'http://new/v1',
    base_urls: { embedding: 'http://embed/v1' },
    models: {
      ...config.models,
      chat: 'chat-v2',
      rerank: 'rank-v2',
      analysis: 'analysis-v2',
    },
  };
  assert.equal(
    embeddingIndexNeedsRebuild(baseline, autoEmbeddingIndexConfig(next)),
    false,
  );
  assert.equal(
    embeddingIndexNeedsRebuild(
      baseline,
      autoEmbeddingIndexConfig({
        ...next,
        provider: 'Other',
        models: { embedding: ' embed-v1 ' },
      }),
    ),
    false,
  );
  assert.ok(
    !JSON.stringify(autoEmbeddingIndexConfig(next)).includes('api_key'),
  );
});

test('changed model, provider or index parameters still require a rebuild', () => {
  for (const next of [
    { ...baseline, model: 'embed-v2' },
    { ...baseline, provider: 'Xinference' },
    { ...baseline, context_window: 4096 },
    { ...baseline, max_tokens: 4096 },
  ])
    assert.equal(embeddingIndexNeedsRebuild(baseline, next), true);
  assert.equal(embeddingIndexNeedsRebuild(null, baseline), true);
});

test('equivalent defaults across mode switches keep the same index', () => {
  assert.equal(
    embeddingIndexNeedsRebuild({ ...baseline, max_tokens: 0 }, baseline),
    false,
  );
  assert.equal(embeddingIndexNeedsRebuild(baseline, baseline), false);
});
