import assert from 'node:assert/strict';
import { test } from 'node:test';
import { selectableReadingLevels } from './readingPermissionOptions.ts';

const levels = [
  { id: 'general', rank: 1, status: 'active' },
  { id: 'internal', rank: 2, status: 'active' },
  { id: 'confidential', rank: 3, status: 'active' },
  { id: 'disabled', rank: 4, status: 'disabled' },
];
const ids = parents =>
  selectableReadingLevels(levels, parents).map(level => level.id);

test('a public parent offers every enabled level even when nobody uses it', () => {
  assert.deepEqual(ids([{ scope_type: 'public' }]), [
    'general',
    'internal',
    'confidential',
  ]);
  assert.deepEqual(ids([{ scope_type: 'custom', allow_all_users: true }]), [
    'general',
    'internal',
    'confidential',
  ]);
  assert.deepEqual(ids([]), ['general', 'internal', 'confidential']);
});

test('department restrictions and personal exclusions do not invent grade restrictions', () => {
  assert.deepEqual(
    ids([
      {
        scope_type: 'custom',
        allowed_department_ids: ['engineering'],
        denied_user_ids: ['someone'],
      },
    ]),
    ['general', 'internal', 'confidential'],
  );
});

test('selected ancestor grades and minimum-grade constraints still narrow options', () => {
  assert.deepEqual(
    ids([
      {
        scope_type: 'custom',
        allowed_permission_level_ids: ['internal', 'confidential'],
      },
    ]),
    ['internal', 'confidential'],
  );
  assert.deepEqual(
    ids([
      {
        allow_all_users: true,
        constraints: [
          {
            scope_type: 'permission_level',
            allowed_permission_level_ids: ['general', 'confidential'],
          },
        ],
      },
    ]),
    ['general', 'confidential'],
  );
  assert.deepEqual(
    ids([
      {
        scope_type: 'custom',
        min_permission_level_id: 'internal',
      },
    ]),
    ['internal', 'confidential'],
  );
  assert.deepEqual(
    ids([
      { allowed_permission_level_ids: ['general', 'internal'] },
      { min_permission_level_rank: 2 },
    ]),
    ['internal'],
  );
});

test('personal exceptions stay independent while other ancestors remain restrictive', () => {
  assert.deepEqual(
    ids([
      {
        allowed_permission_level_ids: ['general'],
        allowed_user_ids: ['named-user'],
        constraints: [
          { allowed_permission_level_ids: ['internal', 'confidential'] },
        ],
      },
    ]),
    ['internal', 'confidential'],
  );
});

test('disabled grade enforcement does not hide active options', () => {
  assert.deepEqual(
    selectableReadingLevels(
      levels,
      [
        {
          scope_type: 'permission_level',
          allowed_permission_level_ids: ['general'],
        },
      ],
      true,
    ).map(level => level.id),
    ['general', 'internal', 'confidential'],
  );
});
