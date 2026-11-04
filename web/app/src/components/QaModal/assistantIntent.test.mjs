import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  assistantFileIntent,
  assistantTemplateIntent,
} from './assistantIntent.mjs';

const cases = JSON.parse(
  readFileSync(
    new URL(
      '../../../../../docs/fixtures/assistant-file-intents.json',
      import.meta.url,
    ),
    'utf8',
  ),
);
test('natural file requests and how-to questions use the correct route', () => {
  for (const item of cases)
    assert.equal(assistantFileIntent(item.message), item.format, item.message);
});

test('a PPT template is a preference, not a mode that captures every question', () => {
  for (const q of ['你好', '这个产品做什么？', '解释第二条', '如何制作PPT？'])
    assert.equal(assistantTemplateIntent(q), false, q);
  for (const q of ['用这个模板做个产品介绍', '按模板生成一份演示材料'])
    assert.equal(assistantTemplateIntent(q), true, q);
});
