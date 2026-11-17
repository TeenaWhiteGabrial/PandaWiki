import { Box, Button, Stack, Tooltip, Typography } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';

export function PermissionDialogTabs({
  value,
  onReading,
  onManagement,
  managementDisabled,
  readingDisabled,
}: {
  value: 'reading' | 'management';
  onReading: () => void;
  onManagement: () => void;
  managementDisabled?: boolean;
  readingDisabled?: boolean;
}) {
  return (
    <Stack
      direction='row'
      gap={2}
      role='tablist'
      aria-label='权限类型'
      sx={{ borderBottom: '1px solid', borderColor: 'divider' }}
    >
      {(
        [
          { key: 'reading', name: '查阅权限', action: onReading },
          { key: 'management', name: '管理权限', action: onManagement },
        ] as const
      ).map(tab => (
        <Tooltip
          key={tab.key}
          title={
            tab.key === 'management' && managementDisabled
              ? '请先保存查阅设置'
              : tab.key === 'reading' && readingDisabled
                ? '请先保存管理设置或取消编辑'
                : ''
          }
        >
          <span>
            <Button
              role='tab'
              aria-selected={value === tab.key}
              disabled={
                tab.key === 'management' ? managementDisabled : readingDisabled
              }
              onClick={tab.action}
              size='small'
              sx={{
                borderRadius: 0,
                px: 0,
                py: 1.25,
                minWidth: 64,
                color: value === tab.key ? 'text.primary' : 'text.secondary',
                fontWeight: value === tab.key ? 600 : 400,
                borderBottom: '2px solid',
                borderColor: value === tab.key ? 'primary.main' : 'transparent',
                mb: '-1px',
              }}
            >
              {tab.name}
            </Button>
          </span>
        </Tooltip>
      ))}
    </Stack>
  );
}

export function PermissionObjectName({ names }: { names: string[] }) {
  const text =
    names.length === 1
      ? names[0]
      : `${names.slice(0, 2).join('、')}${names.length > 2 ? '等' : ''}（${names.length} 项）`;
  return (
    <Stack direction='row' gap={1.5} alignItems='center' sx={{ minWidth: 0 }}>
      <Typography fontSize={13} color='text.secondary' flexShrink={0}>
        对象
      </Typography>
      <Tooltip title={names.join('、')}>
        <Typography fontSize={14} fontWeight={500} noWrap>
          {text}
        </Typography>
      </Tooltip>
    </Stack>
  );
}

export function PermissionHelp({
  management = false,
}: {
  management?: boolean;
}) {
  return (
    <Tooltip
      arrow
      placement='bottom-end'
      title={
        <Box sx={{ fontSize: 12, lineHeight: 1.8, p: 0.5 }}>
          {management ? (
            <>
              <div>上级授予的管理权持续生效，需到授权来源修改。</div>
              <div>新增管理员可管理当前及全部下级内容。</div>
              <div>管理权不自动授予用户端查阅和问答权限。</div>
              <div>超级管理员始终拥有全部管理权限。</div>
            </>
          ) : (
            <>
              <div>同一规则：部门与等级同时满足；多条规则：满足任意一条。</div>
              <div>部门包含子部门，等级按所选具体等级匹配。</div>
              <div>“仅当前目录”包含直接文件；下级独立设置优先。</div>
              <div>排除人员优先。所有用户须登录，超级管理员不受限制。</div>
            </>
          )}
        </Box>
      }
    >
      <Button
        size='small'
        color='inherit'
        sx={{ color: 'text.secondary', fontSize: 12, minWidth: 0 }}
      >
        规则说明
      </Button>
    </Tooltip>
  );
}

export const permissionTableSx: SxProps<Theme> = {
  tableLayout: 'fixed',
  '& th': {
    bgcolor: 'background.paper3',
    color: 'text.secondary',
    fontSize: 12,
    fontWeight: 400,
    py: 1,
  },
  '& td': {
    fontSize: 13,
    py: 1.25,
    verticalAlign: 'top',
    overflowWrap: 'anywhere',
  },
  '& th, & td': { px: 1.5, borderColor: 'divider' },
  '& tr:last-child td': { borderBottom: 0 },
};
