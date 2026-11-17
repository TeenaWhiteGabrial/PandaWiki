import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';
const source = ts.transpileModule(
  await readFile(new URL('./permissionTags.ts', import.meta.url), 'utf8'),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const { getDocumentPermissionTags } = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`
);
test('management tag counts the union and describes inherited access without intersecting it', () => {
  const grant = ids => ({
    subject_type: 'user',
    subject_ids: ids,
    directory_scope: 'current_and_descendants',
  });
  const tags = getDocumentPermissionTags(
    {},
    { users: { a: '张三', b: '李四' }, levels: {}, departments: {} },
    [[grant(['a'])], [grant(['a', 'b'])]],
  );
  const tag = tags[1];
  assert.match(tag.label, /2 人/);
  assert.match(tag.title, /合并生效/);
  assert.doesNotMatch(tag.title, /都必须满足/);
});
test('reading-rule labels describe OR rules and never report a closed policy as public', () => {
  const catalog = { users: { a: '甲' }, levels: {}, departments: {} };
  const closed = getDocumentPermissionTags(
    { reading_version: 2, reading_rules: [] },
    catalog,
  )[0];
  assert.match(closed.label, /未授权/);
  assert.doesNotMatch(closed.label, /全员/);
  const open = getDocumentPermissionTags(
    {
      reading_version: 2,
      reading_rules: [
        {
          permissions: { scope_type: 'user', allowed_user_ids: ['a'] },
          include_descendants: false,
        },
      ],
    },
    catalog,
  )[0];
  assert.match(open.label, /1 条查阅规则/);
  assert.match(open.title, /任意一条.*甲.*仅当前目录及直接文件/);
});
