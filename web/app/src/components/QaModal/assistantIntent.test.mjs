import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  assistantFileIntent,
  assistantFileOptions,
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

test('explicit format wins and a personal PPT can provide its layout, content or both', () => {
  const personal = { kind: 'upload', id: 'slides' };
  const opts = {
    template: personal,
    attachments: [{ id: 'slides' }, { id: 'sheet' }],
  };
  assert.deepEqual(assistantFileOptions('总结这些资料', opts), {
    format: undefined,
    template: undefined,
    attachmentIds: ['slides', 'sheet'],
  });
  assert.deepEqual(
    assistantFileOptions('做个介绍', { ...opts, format: 'pptx' }),
    {
      format: 'pptx',
      template: personal,
      attachmentIds: ['sheet'],
    },
  );
  assert.deepEqual(
    assistantFileOptions('做成PPT', { ...opts, includeTemplateContent: true }),
    {
      format: 'pptx',
      template: personal,
      attachmentIds: ['slides', 'sheet'],
    },
  );
  assert.deepEqual(
    assistantFileOptions('做成PPT', { ...opts, format: 'xlsx' }),
    {
      format: 'xlsx',
      template: undefined,
      attachmentIds: ['slides', 'sheet'],
    },
  );
});

test('a PPT template is a preference, not a mode that captures every question', () => {
  for (const q of ['你好', '这个产品做什么？', '解释第二条', '如何制作PPT？'])
    assert.equal(assistantTemplateIntent(q), false, q);
  for (const q of ['用这个模板做个产品介绍', '按模板生成一份演示材料'])
    assert.equal(assistantTemplateIntent(q), true, q);
});
