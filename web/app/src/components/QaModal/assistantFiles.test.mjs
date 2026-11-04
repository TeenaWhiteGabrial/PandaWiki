import test from 'node:test';
import assert from 'node:assert/strict';
import {
  assistantConversationFiles,
  assistantRevisionTarget,
} from './assistantFiles.mjs';
const item = (id, family, revision, name, format = 'pptx') => ({
  state: 'completed',
  artifact_id: id,
  artifact: {
    id,
    family_id: family,
    revision,
    name,
    format,
    created_at: `2026-11-03T00:0${revision}:00Z`,
  },
});
test('multiple versions are one file and revisions target its latest version', () => {
  const runs = [
    item('v1', 'deck', 1, '产品介绍.pptx'),
    item('v2', 'deck', 2, '产品介绍.pptx'),
  ];
  assert.equal(assistantConversationFiles(runs).length, 1);
  assert.equal(assistantRevisionTarget('修改第三页', runs).id, 'v2');
  assert.equal(assistantRevisionTarget('解释第三页', runs).id, 'v2');
  assert.equal(assistantRevisionTarget('总结刚才的PPT', runs).id, 'v2');
  assert.equal(assistantRevisionTarget('另外做个新PPT', runs).editing, false);
  assert.equal(assistantRevisionTarget('生成一份新的PPT', runs).fresh, true);
});
test('different files require a target unless the user clearly refers to one', () => {
  const runs = [
    item('a', 'a', 1, '产品介绍.pptx'),
    item('b', 'b', 2, '销售分析.pptx'),
  ];
  assert.equal(assistantRevisionTarget('精简第三页', runs).ambiguous, true);
  assert.equal(assistantRevisionTarget('修改刚才那份文件', runs).id, 'b');
  assert.equal(assistantRevisionTarget('精简产品介绍', runs).id, 'a');
  assert.equal(
    assistantRevisionTarget('把这个PPT导出成Word', [runs[0]]).id,
    'a',
  );
});
