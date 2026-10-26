import DescriptionOutlinedIcon from '@mui/icons-material/DescriptionOutlined';
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined';
import ManageAccountsOutlinedIcon from '@mui/icons-material/ManageAccountsOutlined';
import VisibilityOutlinedIcon from '@mui/icons-material/VisibilityOutlined';
import { Box, Button, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';

export function PermissionTarget({
  names,
  kind,
}: {
  names: string[];
  kind: '文档' | '目录' | '栏目';
}) {
  const Icon = kind === '文档' ? DescriptionOutlinedIcon : FolderOutlinedIcon;
  return (
    <Stack direction='row' gap={1.5} alignItems='flex-start'>
      <Box sx={{ p: 1, borderRadius: 2, bgcolor: 'background.paper3' }}>
        <Icon sx={{ display: 'block', color: 'text.secondary' }} />
      </Box>
      <Box sx={{ minWidth: 0 }}>
        <Typography fontSize={12} color='text.secondary'>
          {names.length > 1
            ? `为 ${names.length} 个文档 / 目录设置权限`
            : `正在配置${kind}`}
        </Typography>
        <Typography
          fontSize={16}
          fontWeight={600}
          sx={{ overflowWrap: 'anywhere' }}
        >
          {names.length === 1 ? names[0] : names.slice(0, 3).join('、')}
          {names.length > 3 && ` 等 ${names.length} 项`}
        </Typography>
      </Box>
    </Stack>
  );
}

export function ReadingPermissionSection({
  children,
  action,
}: {
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <Stack
      gap={2}
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        p: 2,
      }}
    >
      <Stack direction='row' gap={1} alignItems='center'>
        <VisibilityOutlinedIcon
          sx={{ fontSize: 20, color: 'text.secondary' }}
        />
        <Typography fontWeight={600}>谁可以查阅</Typography>
        <Box sx={{ ml: 'auto' }}>{action}</Box>
      </Stack>
      <Typography fontSize={12} color='text.secondary'>
        文档查阅和智能问答均需先登录；全员指所有已登录用户，不包括游客。
      </Typography>
      {children}
    </Stack>
  );
}

export function ManagementPermissionSection({
  onConfigure,
  disabled,
}: {
  onConfigure: () => void;
  disabled?: boolean;
}) {
  return (
    <Stack
      direction={{ xs: 'column', sm: 'row' }}
      gap={2}
      alignItems={{ sm: 'center' }}
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        p: 2,
      }}
    >
      <Box sx={{ flex: 1 }}>
        <Stack direction='row' gap={1} alignItems='center' sx={{ mb: 0.75 }}>
          <ManageAccountsOutlinedIcon
            sx={{ fontSize: 20, color: 'text.secondary' }}
          />
          <Typography fontWeight={600}>谁可以管理</Typography>
        </Stack>
        <Typography fontSize={13} color='text.secondary'>
          设置哪些人可以编辑、移动、删除文档，以及可管理的目录范围。
          仅管理员账号可管理文档。
        </Typography>
        <Typography fontSize={12} color='text.secondary' sx={{ mt: 0.5 }}>
          查阅权限与管理权限分别保存。
        </Typography>
      </Box>
      <Button
        variant='outlined'
        size='small'
        onClick={onConfigure}
        disabled={disabled}
      >
        配置管理权限
      </Button>
    </Stack>
  );
}
