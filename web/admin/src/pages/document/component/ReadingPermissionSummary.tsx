import { Box, Stack, Typography } from '@mui/material';
import type { ReadingPermissionValue } from './ReadingPermissionFields';

type SummaryPermissions = Partial<
  Omit<ReadingPermissionValue, 'scope_type'>
> & {
  scope_type?: string;
  min_permission_level_rank?: number;
  denied_department_ids?: string[];
  denied_user_ids?: string[];
};

export default function ReadingPermissionSummary({
  permissions,
  levels,
  departments,
  users,
  levelsDisabled,
  label,
}: {
  permissions: SummaryPermissions;
  levels: { id: string; name: string; rank: number }[];
  departments: { id: string; name: string }[];
  users: { id: string; account: string }[];
  levelsDisabled?: boolean;
  label?: string;
}) {
  const names = (
    ids: string[] | undefined,
    options: { id: string; name: string }[],
  ) =>
    (ids || [])
      .map(id => options.find(item => item.id === id)?.name || '名称不可用')
      .join('、');
  const userOptions = users.map(user => ({ id: user.id, name: user.account }));
  const departmentNames = names(
    permissions.allowed_department_ids,
    departments,
  );
  const userNames = names(permissions.allowed_user_ids, userOptions);
  const scope = permissions.scope_type;
  const minimum = levels.find(
    level => level.id === permissions.min_permission_level_id,
  );
  const hasMinimum =
    !levelsDisabled &&
    (minimum
      ? minimum.rank > 0
      : !!permissions.min_permission_level_id ||
        (permissions.min_permission_level_rank || 0) > 0);
  let audience = '具备知识库访问权限的用户都可以查阅。';
  if (scope === 'permission_level') {
    audience = levelsDisabled
      ? '用户等级限制当前未启用，按知识库访问权限查阅。'
      : `仅以下等级的用户可查阅：${names(permissions.allowed_permission_level_ids, levels) || '尚未选择等级'}。`;
  } else if (scope === 'department') {
    audience = `仅以下部门的成员可查阅：${departmentNames || '尚未选择部门'}。`;
  } else if (scope === 'user') {
    audience = `仅以下用户可查阅：${userNames || '尚未选择用户'}。`;
  } else if (departmentNames || userNames) {
    audience = `可查阅对象：${[departmentNames && `部门成员（${departmentNames}）`, userNames && `指定用户（${userNames}）`].filter(Boolean).join('或')}。`;
  } else if (hasMinimum) {
    audience = '达到以下等级要求的用户可查阅。';
  }
  const deniedDepartments = names(
    permissions.denied_department_ids,
    departments,
  );
  const deniedUsers = names(permissions.denied_user_ids, userOptions);
  return (
    <Box sx={{ bgcolor: 'background.paper3', borderRadius: 1.5, p: 1.5 }}>
      <Stack gap={0.5}>
        {label && (
          <Typography fontSize={12} color='text.secondary'>
            {label}
          </Typography>
        )}
        <Typography fontSize={14}>{audience}</Typography>
        {hasMinimum && (
          <Typography fontSize={13} color='text.secondary'>
            还需满足等级要求：
            {minimum
              ? `${minimum.name}及以上`
              : permissions.min_permission_level_rank
                ? `等级值 ${permissions.min_permission_level_rank} 及以上`
                : '已设置最低等级（名称不可用）'}
            。
          </Typography>
        )}
        {(deniedDepartments || deniedUsers) && (
          <Typography fontSize={13} color='text.secondary'>
            以下对象不能查阅：
            {[
              deniedDepartments && `部门成员（${deniedDepartments}）`,
              deniedUsers && `用户（${deniedUsers}）`,
            ]
              .filter(Boolean)
              .join('、')}
            。
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
