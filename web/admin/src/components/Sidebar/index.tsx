import Logo from '@/assets/images/logo.png';
import { Box, Button, Stack, useTheme } from '@mui/material';
import { ConstsUserKBPermission } from '@/request/types';
import { useMemo, useEffect } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import Avatar from '../Avatar';
import { useAppSelector } from '@/store';
import {
  IconNeirongguanli,
  IconTongjifenxi1,
  IconPaperFull,
  IconDuihualishi1,
  IconChilun,
} from '@panda-wiki/icons';

const MENUS = [
  {
    label: '文档',
    value: '/',
    pathname: 'document',
    icon: IconNeirongguanli,
    show: true,
    perms: [
      ConstsUserKBPermission.UserKBPermissionFullControl,
      ConstsUserKBPermission.UserKBPermissionDocManage,
    ],
  },
  {
    label: '统计',
    value: '/stat',
    pathname: 'stat',
    icon: IconTongjifenxi1,
    show: true,
    perms: [
      ConstsUserKBPermission.UserKBPermissionFullControl,
      ConstsUserKBPermission.UserKBPermissionDataOperate,
    ],
  },
  {
    label: '问答',
    value: '/conversation',
    pathname: 'conversation',
    icon: IconDuihualishi1,
    show: true,
    perms: [
      ConstsUserKBPermission.UserKBPermissionFullControl,
      ConstsUserKBPermission.UserKBPermissionDataOperate,
    ],
  },
  {
    label: '发布',
    value: '/release',
    pathname: 'release',
    icon: IconPaperFull,
    show: true,
    perms: [
      ConstsUserKBPermission.UserKBPermissionFullControl,
      ConstsUserKBPermission.UserKBPermissionDocManage,
    ],
  },
  {
    label: '设置',
    value: '/setting',
    pathname: 'application-setting',
    icon: IconChilun,
    show: true,
    perms: [ConstsUserKBPermission.UserKBPermissionFullControl],
  },
];

const Sidebar = () => {
  const { pathname } = useLocation();
  const { kbDetail } = useAppSelector(state => state.config);
  const theme = useTheme();
  const navigate = useNavigate();
  const menus = useMemo(() => {
    return MENUS.filter(it => {
      return it.perms.includes(kbDetail.perm!);
    });
  }, [kbDetail]);

  useEffect(() => {
    const menu = menus.find(it => {
      if (it.value === '/') {
        return pathname === '/';
      }
      return pathname.startsWith(it.value);
    });

    if (!menu && menus.length > 0) {
      navigate(menus[0].value);
    }
  }, [pathname, menus]);

  return (
    <Stack
      sx={{
        width: 138,
        m: 2,
        zIndex: 999,
        p: 2,
        height: 'calc(100vh - 32px)',
        bgcolor: '#FFFFFF',
        borderRadius: '10px',
        position: 'fixed',
        top: 0,
        left: 0,
        overflow: 'auto',
      }}
    >
      <Stack
        direction={'row'}
        alignItems={'center'}
        justifyContent={'center'}
        sx={{ flexShrink: 0 }}
      >
        <Avatar src={Logo} sx={{ width: 30, height: 30 }} />
      </Stack>
      <Box
        sx={{
          fontSize: '16px',
          fontWeight: 'bold',
          color: 'text.primary',
          textAlign: 'center',
          lineHeight: '36px',
          borderBottom: `1px solid ${theme.palette.divider}`,
        }}
      >
        知识库
      </Box>
      <Stack sx={{ py: 2, flexGrow: 1 }} gap={1}>
        {menus.map(it => {
          let isActive = false;
          if (it.value === '/') {
            isActive = pathname === '/';
          } else {
            isActive = pathname.includes(it.value);
          }
          if (!it.show) return null;
          const IconMenu = it.icon;
          return (
            <NavLink
              key={it.pathname}
              to={it.value}
              style={{
                zIndex: isActive ? 2 : 1,
              }}
            >
              <Button
                variant={isActive ? 'contained' : 'text'}
                color='dark'
                sx={{
                  width: '100%',
                  height: 50,
                  px: 2,
                  justifyContent: 'flex-start',
                  color: isActive ? '#FFFFFF' : 'text.primary',
                  fontWeight: isActive ? '500' : '400',
                  boxShadow: isActive
                    ? '0px 10px 25px 0px rgba(33,34,45,0.2)'
                    : 'none',
                  ':hover': {
                    boxShadow: isActive
                      ? '0px 10px 25px 0px rgba(33,34,45,0.2)'
                      : 'none',
                  },
                }}
              >
                <IconMenu
                  sx={{
                    fontSize: 14,
                    mr: 1,
                    color: isActive ? '#FFFFFF' : 'text.disabled',
                  }}
                />
                {it.label}
              </Button>
            </NavLink>
          );
        })}
      </Stack>
      <Stack gap={1} sx={{ flexShrink: 0 }}>
        <Button
          variant='outlined'
          color='dark'
          sx={{
            fontSize: 14,
            flexShrink: 0,
            fontWeight: 400,
            pr: 1.5,
            pl: 1.5,
            gap: 0.5,
            justifyContent: 'flex-start',
            border: `1px solid ${theme.palette.divider}`,
            '.MuiButton-startIcon': {
              mr: '3px',
            },
            '&:hover': {
              color: 'primary.main',
            },
          }}
          disabled
        >
          内部使用
        </Button>
      </Stack>
    </Stack>
  );
};

export default Sidebar;
