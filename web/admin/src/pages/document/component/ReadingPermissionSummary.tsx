import { Modal } from '@/components/ui';
import type { ReadingPermissionSummary as EffectiveSummary } from '@/request/ReadingAudience';
import type { DomainNodePermissions } from '@/request/types';
import { Button, Stack, Typography } from '@mui/material';
import { useState, type ReactNode } from 'react';

type NamedOption = { id: string; name: string };
type DepartmentOption = NamedOption & {
  path?: string;
  parent_id?: string | null;
  ancestor_ids?: string[];
};

// Read-only single-column rules do not request an audience preview. Multi-level
// policies must use the server's merged summary rather than approximate users.
function singleRuleSummary(
  rule: DomainNodePermissions,
  levelsDisabled: boolean,
  levels: { id: string; rank: number }[],
): EffectiveSummary | undefined {
  if (rule.constraints?.length) return undefined;
  const scope = rule.scope_type;
  const all =
    rule.allow_all_users || (levelsDisabled && scope === 'permission_level');
  const departmentIDs =
    all || scope === 'user' || scope === 'permission_level'
      ? []
      : rule.allowed_department_ids || [];
  const levelIDs =
    all || levelsDisabled || scope === 'user' || scope === 'department'
      ? []
      : rule.allowed_permission_level_ids || [];
  const minimum =
    all || levelsDisabled || scope === 'user'
      ? 0
      : rule.min_permission_level_rank ||
        levels.find(item => item.id === rule.min_permission_level_id)?.rank ||
        0;
  const levelRestricted =
    !all &&
    !levelsDisabled &&
    (scope === 'permission_level' || levelIDs.length > 0);
  const eligibleLevels = levelIDs.filter(
    id => (levels.find(item => item.id === id)?.rank || 0) >= minimum,
  );
  const allowedUsers = (rule.allowed_user_ids || []).filter(
    id => !rule.denied_user_ids?.includes(id),
  );
  const groupAllowed =
    !!all ||
    (scope !== 'user' &&
      (scope !== 'department' || departmentIDs.length > 0) &&
      (!levelRestricted || eligibleLevels.length > 0) &&
      (!!departmentIDs.length ||
        levelRestricted ||
        minimum > 0 ||
        !rule.allowed_user_ids?.length));
  return {
    group_allowed: groupAllowed,
    department_groups: departmentIDs.length ? [departmentIDs] : [],
    level_restricted: levelRestricted,
    permission_level_ids: eligibleLevels,
    min_permission_level_id: rule.min_permission_level_id,
    min_permission_level_rank: minimum,
    allowed_user_ids: allowedUsers,
    denied_user_ids: rule.denied_user_ids || [],
    denied_department_ids: rule.denied_department_ids || [],
  };
}

export default function ReadingPermissionSummary({
  permissions,
  summary,
  levels,
  departments,
  users,
  levelsDisabled = false,
}: {
  permissions: DomainNodePermissions;
  summary?: EffectiveSummary;
  levels: { id: string; name: string; rank: number }[];
  departments: DepartmentOption[];
  users: { id: string; account: string }[];
  levelsDisabled?: boolean;
}) {
  const [details, setDetails] = useState<{
    title: string;
    names: string[];
  } | null>(null);
  const effective =
    summary || singleRuleSummary(permissions, levelsDisabled, levels);
  const userOptions = users.map(user => ({ id: user.id, name: user.account }));
  const names = (
    ids: string[],
    options: NamedOption[],
    kind: string,
  ): ReactNode => {
    const values = ids.map(
      id =>
        (kind === '人员' && effective?.user_names?.[id]) ||
        options.find(item => item.id === id)?.name ||
        `未知${kind}（${id}）`,
    );
    if (values.length <= 3) return values.join('、');
    return (
      <>
        {values.slice(0, 3).join('、')}
        <Button
          size='small'
          sx={{ minWidth: 0, px: 0.5, py: 0 }}
          onClick={() => setDetails({ title: kind, names: values })}
        >
          等 {values.length} 项
        </Button>
      </>
    );
  };
  const isWithin = (child: DepartmentOption, parent: DepartmentOption) => {
    if (child.id === parent.id) return true;
    if (child.ancestor_ids?.includes(parent.id)) return true;
    if (parent.path && child.path?.startsWith(`${parent.path}/`)) return true;
    const seen = new Set<string>();
    let parentID = child.parent_id;
    while (parentID && !seen.has(parentID)) {
      if (parentID === parent.id) return true;
      seen.add(parentID);
      parentID = departments.find(item => item.id === parentID)?.parent_id;
    }
    return false;
  };
  // Only abbreviate a subtree when all its known descendants are authorized.
  // Historical partial department sets must not be described as whole subtrees.
  const departmentNames = (ids: string[]): ReactNode => {
    const roots = ids.filter(id => {
      const child = departments.find(item => item.id === id);
      return (
        !child ||
        !ids.some(otherID => {
          const parent = departments.find(item => item.id === otherID);
          return (
            parent &&
            parent.id !== id &&
            isWithin(child, parent) &&
            departments
              .filter(item => isWithin(item, parent))
              .every(item => ids.includes(item.id))
          );
        })
      );
    });
    const options = departments.map(item => ({
      id: item.id,
      name:
        item.name +
        (ids.includes(item.id) &&
        departments
          .filter(child => isWithin(child, item))
          .every(child => ids.includes(child.id))
          ? '（含下级部门）'
          : ''),
    }));
    return names(roots, options, '部门');
  };
  const groups = effective?.department_groups || [];
  const level = effective?.level_restricted ? (
    <>等级为「{names(effective.permission_level_ids, levels, '等级')}」</>
  ) : effective?.min_permission_level_rank ? (
    <>
      等级为「
      {levels.find(item => item.id === effective.min_permission_level_id)
        ?.name || `等级 ${effective.min_permission_level_rank}`}
      及以上」
    </>
  ) : null;
  const hasGroup = !!effective?.group_allowed;
  const unrestrictedGroup = hasGroup && !groups.length && !level;
  const hasExclusions =
    !!effective?.denied_user_ids.length ||
    !!effective?.denied_department_ids.length;
  const hasNamed = !!effective?.allowed_user_ids.length && !unrestrictedGroup;
  return (
    <>
      <Stack gap={1.25}>
        {!effective ? (
          <Typography fontSize={14} color='text.secondary'>
            暂时无法计算最终查阅权限，请刷新后重试。
          </Typography>
        ) : (
          <>
            <Typography fontSize={13} color='text.secondary'>
              最终查阅权限
            </Typography>
            {hasGroup && (
              <Typography fontSize={14} sx={{ lineHeight: 1.85 }}>
                {unrestrictedGroup ? (
                  hasExclusions ? (
                    '除以下排除对象外，其他已登录用户均可查阅。'
                  ) : (
                    '所有已登录用户可查阅，不包括未登录游客。'
                  )
                ) : (
                  <>
                    {groups.length > 1 ? '同时属于' : ''}
                    {groups.map((group, index) => (
                      <span key={index}>
                        {index > 0 ? '和' : ''}
                        {groups.length > 1 ? '「' : ''}
                        {departmentNames(group)}
                        {groups.length > 1 ? '」' : ''}
                      </span>
                    ))}
                    {groups.length > 0 && level ? '中，' : ''}
                    {level}
                    {groups.length > 0 && !level ? '的成员' : '的用户'}
                    可以查阅。
                  </>
                )}
              </Typography>
            )}
            {hasNamed && (
              <Typography fontSize={14} sx={{ lineHeight: 1.85 }}>
                {hasGroup ? '另行授权：' : '仅以下人员可查阅：'}
                {names(effective.allowed_user_ids, userOptions, '人员')}
              </Typography>
            )}
            {!hasGroup && !hasNamed && (
              <Typography fontSize={14} color='warning.main'>
                当前没有用户可查阅。
              </Typography>
            )}
            {!!effective.denied_user_ids.length && (
              <Typography fontSize={13} color='text.secondary'>
                排除人员：
                {names(effective.denied_user_ids, userOptions, '人员')}
              </Typography>
            )}
            {!!effective.denied_department_ids.length && (
              <Typography fontSize={13} color='text.secondary'>
                排除部门：{departmentNames(effective.denied_department_ids)}
              </Typography>
            )}
          </>
        )}
      </Stack>
      <Modal
        title={details?.title}
        open={!!details}
        onCancel={() => setDetails(null)}
        footer={null}
        width={480}
      >
        <Typography
          sx={{
            maxHeight: 360,
            overflowY: 'auto',
            overflowWrap: 'anywhere',
            lineHeight: 2,
          }}
        >
          {details?.names.join('、')}
        </Typography>
      </Modal>
    </>
  );
}
