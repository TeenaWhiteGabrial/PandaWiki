import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import { FormItem } from '@/pages/setting/component/Common';
import type { Department } from '@/request/Enterprise';
import { buildDepartmentTree, flattenDepartmentTree } from '@/utils/enterprise';
import { Alert, Stack, Switch, Typography } from '@mui/material';
import { Select } from 'antd';
import { useMemo } from 'react';
import type { DomainNodePermissions } from '@/request/types';

export type ReadingScope =
  | 'public'
  | 'custom'
  | 'permission_level'
  | 'department'
  | 'user';
export type ReadingLevelRestrictionMode = 'none' | 'minimum' | 'selected';
export interface ReadingPermissionValue {
  scope_type: ReadingScope;
  level_restriction_mode: ReadingLevelRestrictionMode;
  level_restriction_needs_review?: boolean;
  min_permission_level_id: string;
  allowed_permission_level_ids: string[];
  allowed_department_ids: string[];
  allowed_user_ids: string[];
  allow_all_users: boolean;
  denied_user_ids: string[];
}

export function normalizeReadingPermissionValue(
  permissions: {
    scope_type?: string;
    min_permission_level_id?: string;
    min_permission_level_rank?: number;
    allowed_permission_level_ids?: string[];
    allowed_department_ids?: string[];
    allowed_user_ids?: string[];
    allow_all_users?: boolean;
    denied_user_ids?: string[];
  },
  levels: { id: string; rank: number }[] = [],
): ReadingPermissionValue {
  const minimumLevel = levels.find(
    level => level.id === permissions.min_permission_level_id,
  );
  const minimumRank =
    minimumLevel?.rank || permissions.min_permission_level_rank || 0;
  const configuredLevelIDs = permissions.allowed_permission_level_ids || [];
  const hasMinimum =
    !!permissions.min_permission_level_id ||
    !!permissions.min_permission_level_rank;
  const hasSelectedLevels = configuredLevelIDs.length > 0;
  // Older rules could combine a threshold with an explicit list. Present them
  // as the equivalent explicit list so the editor now exposes one level rule.
  const hasLegacyCombination = hasMinimum && hasSelectedLevels;
  const allowedPermissionLevelIDs = hasLegacyCombination
    ? minimumRank > 0
      ? configuredLevelIDs.filter(id => {
          const level = levels.find(item => item.id === id);
          return level && level.rank >= minimumRank;
        })
      : []
    : configuredLevelIDs;
  const needsLevelRuleReview =
    (hasLegacyCombination &&
      (minimumRank <= 0 || allowedPermissionLevelIDs.length === 0)) ||
    (hasMinimum &&
      !permissions.min_permission_level_id &&
      !levels.some(level => level.rank === minimumRank));
  const levelRestrictionMode: ReadingLevelRestrictionMode =
    hasMinimum && hasSelectedLevels
      ? 'selected'
      : hasMinimum
        ? 'minimum'
        : hasSelectedLevels
          ? 'selected'
          : 'none';
  const hasAudience =
    !!permissions.allow_all_users ||
    hasMinimum ||
    hasSelectedLevels ||
    !!permissions.allowed_department_ids?.length ||
    !!permissions.allowed_user_ids?.length;
  const scope =
    (permissions.scope_type || 'public') === 'public' && !hasAudience
      ? 'public'
      : 'custom';
  return {
    scope_type: scope,
    level_restriction_mode: levelRestrictionMode,
    level_restriction_needs_review: needsLevelRuleReview,
    min_permission_level_id:
      levelRestrictionMode === 'minimum'
        ? permissions.min_permission_level_id ||
          levels.find(level => level.rank === minimumRank)?.id ||
          ''
        : '',
    allowed_permission_level_ids:
      levelRestrictionMode === 'selected' ? allowedPermissionLevelIDs : [],
    allowed_department_ids: permissions.allowed_department_ids || [],
    allowed_user_ids: permissions.allowed_user_ids || [],
    allow_all_users:
      permissions.allow_all_users ??
      (!hasMinimum &&
        !hasSelectedLevels &&
        !permissions.allowed_department_ids?.length &&
        !permissions.allowed_user_ids?.length &&
        !['department', 'permission_level', 'user'].includes(
          permissions.scope_type || '',
        ) &&
        (permissions.scope_type === 'public' ||
          permissions.scope_type === 'custom' ||
          !permissions.scope_type)),
    denied_user_ids: permissions.denied_user_ids || [],
  };
}

export function makeInheritedReadingPermissionValue(
  permissions: DomainNodePermissions,
  levels: { id: string; rank: number }[] = [],
): ReadingPermissionValue {
  const rules: DomainNodePermissions[] = [];
  const collect = (rule: DomainNodePermissions) => {
    rules.push(rule);
    (rule.constraints || []).forEach(collect);
  };
  collect(permissions);
  const concrete = rules.find(
    rule =>
      !rule.allow_all_users &&
      (!!rule.allowed_department_ids?.length ||
        !!rule.allowed_user_ids?.length ||
        !!rule.allowed_permission_level_ids?.length ||
        !!rule.min_permission_level_id ||
        !!rule.min_permission_level_rank),
  );
  const normalized = normalizeReadingPermissionValue(
    concrete || permissions,
    levels,
  );
  return {
    ...normalized,
    scope_type: 'custom',
    // Remaining compound parent rules are enforced by the server's snapshot.
    allow_all_users: !concrete,
    denied_user_ids: [
      ...new Set(rules.flatMap(rule => rule.denied_user_ids || [])),
    ],
  };
}

export function readingPermissionPayload(
  value: ReadingPermissionValue,
  inherit = false,
): DomainNodePermissions {
  if (inherit && value.scope_type === 'public') return { inherit: true };
  const custom = value.scope_type === 'custom';
  return {
    ...(inherit ? { inherit: false } : {}),
    scope_type: custom ? 'custom' : 'public',
    min_permission_level_id:
      custom && value.level_restriction_mode === 'minimum'
        ? value.min_permission_level_id || undefined
        : undefined,
    allowed_permission_level_ids:
      custom && value.level_restriction_mode === 'selected'
        ? value.allowed_permission_level_ids
        : [],
    allowed_department_ids: custom ? value.allowed_department_ids : [],
    allowed_user_ids: custom ? value.allowed_user_ids : [],
    allow_all_users: custom ? value.allow_all_users : false,
    denied_user_ids: custom ? value.denied_user_ids : [],
  };
}

export default function ReadingPermissionFields({
  value,
  onChange,
  levelsDisabled,
  levels,
  permissionLevels,
  allPermissionLevels,
  departments,
  users,
  restrictedPermissionLevelIDs = [],
  restrictedDepartmentIDs = [],
  restrictedUserIDs = [],
  publicAllowed = true,
  neutralAudienceAllowed = false,
  allUsersAllowed = true,
  inheritedValue,
  disabled = false,
}: {
  value: ReadingPermissionValue;
  onChange: (patch: Partial<ReadingPermissionValue>) => void;
  levelsDisabled?: boolean;
  levels: { id: string; name: string; rank: number }[];
  permissionLevels?: { id: string; name: string; rank: number }[];
  minimumPermissionLevels?: { id: string; name: string; rank: number }[];
  allPermissionLevels?: { id: string; name: string; rank: number }[];
  departments: Department[];
  users: { id: string; account: string }[];
  restrictedPermissionLevelIDs?: string[];
  restrictedMinimumPermissionLevelIDs?: string[];
  restrictedDepartmentIDs?: string[];
  restrictedUserIDs?: string[];
  publicAllowed?: boolean;
  neutralAudienceAllowed?: boolean;
  allUsersAllowed?: boolean;
  inheritedValue?: ReadingPermissionValue;
  disabled?: boolean;
}) {
  const departmentTree = useMemo(() => {
    const selected = new Set(value.allowed_department_ids);
    const restricted = new Set(restrictedDepartmentIDs);
    const visible = new Set(
      departments
        .filter(item => !restricted.has(item.id) || selected.has(item.id))
        .map(item => item.id),
    );
    const byID = new Map(departments.map(item => [item.id, item]));
    // Keep ancestors as navigation paths, without making them selectable.
    for (const id of [...visible]) {
      const seen = new Set<string>();
      let parentID = byID.get(id)?.parent_id;
      while (parentID && !seen.has(parentID)) {
        seen.add(parentID);
        visible.add(parentID);
        parentID = byID.get(parentID)?.parent_id;
      }
    }
    return buildDepartmentTree(
      departments
        .filter(item => visible.has(item.id))
        .map(department =>
          restrictedDepartmentIDs.includes(department.id) &&
          value.allowed_department_ids.includes(department.id)
            ? { ...department, name: `${department.name}（超出上级范围）` }
            : department,
        ),
    );
  }, [departments, restrictedDepartmentIDs, value.allowed_department_ids]);
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );
  const visibleLevels = allPermissionLevels || levels;
  const minimumRank = levels.find(
    level => level.id === value.min_permission_level_id,
  )?.rank;
  const selectedLevels =
    value.level_restriction_mode === 'minimum'
      ? visibleLevels
          .filter(
            level => minimumRank !== undefined && level.rank >= minimumRank,
          )
          .map(level => level.id)
      : value.allowed_permission_level_ids;
  const eligibleLevelIDs = new Set(
    (permissionLevels || levels).map(level => level.id),
  );
  const levelOptions = visibleLevels
    .filter(
      level =>
        eligibleLevelIDs.has(level.id) || selectedLevels.includes(level.id),
    )
    .map(level => ({
      value: level.id,
      label: restrictedPermissionLevelIDs.includes(level.id)
        ? `${level.name}（超出上级范围）`
        : level.name,
      disabled: restrictedPermissionLevelIDs.includes(level.id),
    }));
  const userOptions = [
    ...users.map(user => ({
      value: user.id,
      label: user.account,
      disabled: restrictedUserIDs.includes(user.id),
    })),
    ...value.allowed_user_ids
      .filter(id => !users.some(user => user.id === id))
      .map(id => ({ value: id, label: `未知用户（${id}）`, disabled: true })),
  ];
  const selectScope = (custom: boolean) => {
    if (custom) {
      onChange(
        inheritedValue || {
          ...value,
          scope_type: 'custom',
          allow_all_users:
            value.allow_all_users || value.scope_type === 'public',
        },
      );
    } else {
      onChange({
        scope_type: 'public',
        level_restriction_mode: 'none',
        level_restriction_needs_review: false,
        min_permission_level_id: '',
        allowed_permission_level_ids: [],
        allowed_department_ids: [],
        allowed_user_ids: [],
        allow_all_users: false,
        denied_user_ids: [],
      });
    }
  };
  const changeAudience = (patch: Partial<ReadingPermissionValue>) => {
    const next = { ...value, ...patch };
    const hasAudience =
      next.allowed_department_ids.length > 0 ||
      next.allowed_user_ids.length > 0 ||
      next.level_restriction_mode !== 'none';
    onChange({
      ...patch,
      allow_all_users:
        !hasAudience &&
        (neutralAudienceAllowed || allUsersAllowed || value.allow_all_users),
    });
  };
  return (
    <>
      <Stack direction='row' alignItems='center' gap={1}>
        <Switch
          size='small'
          slotProps={{
            input: {
              'aria-label': neutralAudienceAllowed
                ? '使用上级设置'
                : '自定义查阅范围',
            },
          }}
          checked={
            neutralAudienceAllowed
              ? value.scope_type === 'public'
              : value.scope_type === 'custom'
          }
          disabled={
            disabled ||
            (!neutralAudienceAllowed &&
              !publicAllowed &&
              value.scope_type === 'custom')
          }
          onChange={event =>
            selectScope(
              neutralAudienceAllowed
                ? !event.target.checked
                : event.target.checked,
            )
          }
        />
        <Typography fontSize={14}>
          {neutralAudienceAllowed ? '使用上级设置' : '自定义查阅范围'}
        </Typography>
      </Stack>
      {value.scope_type === 'custom' && (
        <>
          <FormItem label='授权部门'>
            <DepartmentTreeSelect
              disabled={disabled}
              cascadeDisplay
              departments={departmentTree}
              flatDepartments={flatDepartments}
              value={value.allowed_department_ids}
              excludeIds={restrictedDepartmentIDs}
              multiple
              size='small'
              label='不限制部门；选择时包含下级部门'
              onChange={allowed_department_ids =>
                changeAudience({ allowed_department_ids })
              }
            />
          </FormItem>
          {!levelsDisabled && (
            <FormItem label='可查阅等级'>
              <Select
                disabled={disabled}
                mode='multiple'
                style={{ width: '100%' }}
                value={selectedLevels}
                options={levelOptions}
                placeholder='不限制用户等级'
                onChange={allowed_permission_level_ids =>
                  changeAudience({
                    allowed_permission_level_ids,
                    level_restriction_mode: allowed_permission_level_ids.length
                      ? 'selected'
                      : 'none',
                    min_permission_level_id: '',
                    level_restriction_needs_review: false,
                  })
                }
              />
              {value.level_restriction_needs_review && (
                <Alert severity='warning' icon={false}>
                  历史等级规则已失效，请重新选择可查阅等级。
                </Alert>
              )}
            </FormItem>
          )}
          <FormItem label='单独授权人员'>
            <Select
              disabled={disabled}
              mode='multiple'
              showSearch
              optionFilterProp='label'
              style={{ width: '100%' }}
              value={value.allowed_user_ids}
              options={userOptions}
              placeholder='可选；单独允许这些人员查阅'
              onChange={allowed_user_ids =>
                changeAudience({ allowed_user_ids })
              }
            />
          </FormItem>
          <FormItem label='排除人员'>
            <Select
              disabled={disabled}
              mode='multiple'
              showSearch
              optionFilterProp='label'
              style={{ width: '100%' }}
              value={value.denied_user_ids}
              placeholder='可选；这些人员不可查阅'
              options={[
                ...users.map(user => ({ value: user.id, label: user.account })),
                ...value.denied_user_ids
                  .filter(id => !users.some(user => user.id === id))
                  .map(id => ({ value: id, label: `未知用户（${id}）` })),
              ]}
              onChange={denied_user_ids =>
                onChange({
                  denied_user_ids,
                  allowed_user_ids: value.allowed_user_ids.filter(
                    id => !denied_user_ids.includes(id),
                  ),
                })
              }
            />
          </FormItem>
          <Typography fontSize={12} color='text.secondary'>
            部门和等级需同时满足；单独授权人员可独立获准；排除人员不可查阅。
          </Typography>
        </>
      )}
    </>
  );
}
