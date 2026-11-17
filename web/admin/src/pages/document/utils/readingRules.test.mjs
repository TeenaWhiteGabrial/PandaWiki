import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';
const source = ts.transpileModule(
  await readFile(new URL('./readingRules.ts', import.meta.url), 'utf8'),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const {
  independentReadingPolicy,
  incompleteReadingRule,
  newReadingRule,
  readingRuleKind,
  describeReadingPolicy,
} = await import(
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`
);
const catalogs = {
  departments: { dev: '研发部', sales: '市场部' },
  levels: { secret: '涉密' },
  users: { a: '甲', b: '乙' },
};
test('switching to independent clones the complete parent rules and exclusions', () => {
  const parent = {
    reading_version: 2,
    reading_rules: [
      {
        permissions: { scope_type: 'user', allowed_user_ids: ['a'] },
        include_descendants: false,
      },
    ],
    denied_user_ids: ['b'],
    denied_department_ids: ['sales'],
  };
  const result = independentReadingPolicy(parent);
  assert.equal(result.inherit, false);
  assert.deepEqual(result.reading_rules, parent.reading_rules);
  result.reading_rules[0].permissions.allowed_user_ids.push('b');
  assert.deepEqual(parent.reading_rules[0].permissions.allowed_user_ids, ['a']);
  assert.deepEqual(result.denied_user_ids, ['b']);
  assert.deepEqual(result.denied_department_ids, ['sales']);
});
test('historical intersection stays one rule, and top-level exclusions can be edited', () => {
  const result = independentReadingPolicy({
    allowed_department_ids: ['dev'],
    denied_user_ids: ['b'],
    constraints: [
      {
        scope_type: 'permission_level',
        allowed_permission_level_ids: ['secret'],
      },
    ],
  });
  assert.equal(result.reading_rules.length, 1);
  assert.equal(readingRuleKind(result.reading_rules[0]), 'legacy');
  assert.deepEqual(
    result.reading_rules[0].permissions.constraints[0]
      .allowed_permission_level_ids,
    ['secret'],
  );
  assert.equal(result.reading_rules[0].permissions.denied_user_ids, undefined);
  assert.deepEqual(result.denied_user_ids, ['b']);
  assert.match(
    describeReadingPolicy(result, catalogs),
    /研发部.*且.*涉密.*排除：乙/,
  );
});
test('batch parent intersection never discards another selected target policy', () => {
  const result = independentReadingPolicy({
    reading_version: 2,
    reading_rules: [
      { permissions: { allow_all_users: true }, include_descendants: true },
    ],
    constraints: [{ scope_type: 'user', allowed_user_ids: ['a'] }],
  });
  assert.equal(result.reading_rules.length, 1);
  assert.equal(readingRuleKind(result.reading_rules[0]), 'legacy');
  assert.deepEqual(
    result.reading_rules[0].permissions.constraints[0].constraints[0]
      .allowed_user_ids,
    ['a'],
  );
});
test('empty saved policy is closed; blank editor rules require an audience', () => {
  const closed = independentReadingPolicy({
    reading_version: 2,
    reading_rules: [],
  });
  assert.equal(closed.reading_rules.length, 0);
  assert.match(describeReadingPolicy(closed, catalogs), /未授权普通用户/);
  assert.equal(incompleteReadingRule(newReadingRule()), true);
  assert.equal(
    incompleteReadingRule({
      permissions: { scope_type: 'user' },
      include_descendants: true,
    }),
    true,
  );
  assert.equal(
    incompleteReadingRule({
      permissions: { scope_type: 'public', allow_all_users: true },
      include_descendants: true,
    }),
    false,
  );
  assert.equal(newReadingRule().include_descendants, true);
});
test('unrestricted legacy parent copies as an explicit logged-in all-users rule', () => {
  const result = independentReadingPolicy({});
  assert.equal(readingRuleKind(result.reading_rules[0]), 'all');
  assert.equal(result.reading_rules[0].permissions.allow_all_users, true);
});
