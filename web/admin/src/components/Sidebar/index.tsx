import Logo from '@/assets/images/logo.png';
import { message, Modal } from '@/components/ui';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import {
  Box,
  Button,
  IconButton,
  Stack,
  Tooltip,
  useTheme,
} from '@mui/material';
import { ConstsUserKBPermission, ConstsUserRole } from '@/request/types';
import { useMemo, useEffect, useState } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import Avatar from '../Avatar';
import { useAppSelector } from '@/store';
import {
  IconNeirongguanli,
  IconTongjifenxi1,
  IconPaperFull,
  IconDuihualishi1,
  IconChilun,
  IconKehupingjia,
  IconTianjia,
  IconDengchu,
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
    label: '评论',
    value: '/comment',
    pathname: 'comment',
    icon: IconKehupingjia,
    show: true,
    perms: [
      ConstsUserKBPermission.UserKBPermissionFullControl,
      ConstsUserKBPermission.UserKBPermissionDataOperate,
    ],
  },
  {
    label: '贡献',
    value: '/contribute',
    pathname: 'contribute',
    icon: IconTianjia,
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
    perms: [
      ConstsUserKBPermission.UserKBPermissionFullControl,
      ConstsUserKBPermission.UserKBPermissionNotNull,
    ],
  },
];

const Sidebar = () => {
  const { pathname } = useLocation();
  const { kbDetail, user, appBrand } = useAppSelector(state => state.config);
  const theme = useTheme();
  const navigate = useNavigate();
  const [logoutConfirmOpen, setLogoutConfirmOpen] = useState(false);
  const menus = useMemo(() => {
    return MENUS.filter(it => {
      if (
        it.value === '/setting' &&
        user?.role === ConstsUserRole.UserRoleManager
      ) {
        return true;
      }
      return it.perms.includes(kbDetail.perm!);
    });
  }, [kbDetail, user]);

  useEffect(() => {
    if (pathname === '/') {
      return;
    }

    // 权限数据还没加载完，不执行重定向
    if (!kbDetail.perm) {
      return;
    }

    const menu = menus.find(it => {
      if (it.value === '/') {
        return pathname === '/';
      }
      return pathname.startsWith(it.value);
    });

    if (!menu && menus.length > 0) {
      navigate(menus[0].value);
    }
  }, [pathname, menus, kbDetail.perm, navigate]);

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
        <Avatar src={appBrand.logo || Logo} sx={{ width: 30, height: 30 }} />
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
        {appBrand.title || '知识库管理台'}
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
        <Stack
          direction='row'
          alignItems='center'
          justifyContent='space-between'
          sx={{
            px: 1,
            py: 1.25,
            borderTop: `1px solid ${theme.palette.divider}`,
            gap: 1,
          }}
        >
          <Box
            sx={{
              flexGrow: 1,
              minWidth: 0,
              fontSize: 13,
              color: 'text.secondary',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {user.account || '当前用户'}
          </Box>
          <Tooltip arrow title='退出登录'>
            <IconButton
              size='small'
              sx={{
                flexShrink: 0,
                color: 'text.secondary',
                '&:hover': {
                  color: 'primary.main',
                },
              }}
              onClick={() => {
                setLogoutConfirmOpen(true);
              }}
            >
              <IconDengchu sx={{ fontSize: 18 }} />
            </IconButton>
          </Tooltip>
        </Stack>
      </Stack>
      <Modal
        open={logoutConfirmOpen}
        onCancel={() => setLogoutConfirmOpen(false)}
        onOk={() => {
          message.success('退出登录成功，请重新登录');
          setTimeout(() => {
            localStorage.removeItem('panda_wiki_token');
            navigate('/login');
          }, 1500);
        }}
        cancelButtonProps={{
          variant: 'outlined',
          sx: { '&:hover': { borderColor: 'grey.300' } },
        }}
        okButtonProps={{
          variant: 'contained',
          sx: {
            bgcolor: 'primary.main',
            '&:hover': { bgcolor: 'primary.dark' },
          },
        }}
        title={
          <Stack direction='column' gap={3}>
            <Stack direction='row' alignItems='center' gap={1}>
              <ErrorOutlineIcon sx={{ color: 'error.main', fontSize: 24 }} />
              <span style={{ fontWeight: 'bold' }}>确定要退出当前账号？</span>
            </Stack>
          </Stack>
        }
        transitionDuration={300}
      />
    </Stack>
  );
};

export default Sidebar;
