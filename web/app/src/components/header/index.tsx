'use client';

import { useBasePath } from '@/hooks';
import { useStore } from '@/provider';
import { clearCookie } from '@/utils/cookie';
import { getImagePath } from '@/utils/getImagePath';
import { Modal } from '@ctzhian/ui';
import ErrorIcon from '@mui/icons-material/Error';
import { alpha, Box, Button, IconButton, Stack, Tooltip } from '@mui/material';
import { IconDengchu } from '@panda-wiki/icons';
import {
  Header as CustomHeader,
  WelcomeHeader as WelcomeHeaderComponent,
} from '@panda-wiki/ui';
import { usePathname, useSearchParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
import QaModal from '../QaModal';
import ThemeSwitch from './themeSwitch';

interface HeaderProps {
  isDocPage?: boolean;
  isWelcomePage?: boolean;
}

const getHeaderButtons = (
  btns: any[] = [],
  basePath: string,
  redirectPath: string,
  isAuthenticated: boolean,
) => {
  return btns
    .filter(item => {
      const text = String(item?.text || '').toLowerCase();
      const url = String(item?.url || '').toLowerCase();
      const isLoginPlaceholder =
        text.includes('pandawiki') || url.includes('pandawiki');

      return (
        !text.includes('github') &&
        !url.includes('github.com') &&
        !(isAuthenticated && isLoginPlaceholder)
      );
    })
    .map((item: any) => {
      const text = String(item?.text || '').toLowerCase();
      const url = String(item?.url || '').toLowerCase();
      const isLoginPlaceholder =
        text.includes('pandawiki') || url.includes('pandawiki');
      const redirect = encodeURIComponent(redirectPath);

      return {
        ...item,
        text: isLoginPlaceholder ? '登录' : item.text,
        url: isLoginPlaceholder
          ? `${basePath || ''}/auth/login${redirect ? `?redirect=${redirect}` : ''}`
          : getImagePath(item.url, basePath),
        target: isLoginPlaceholder ? '_self' : item.target,
        icon: getImagePath(item.icon, basePath),
      };
    });
};

const LogoutButton = () => {
  const [open, setOpen] = useState(false);
  const handleLogout = () => {
    clearCookie();
    window.localStorage.removeItem('panda_wiki_token');
    window.localStorage.removeItem('panda_wiki_remember_login');
    const protocol = window.location.protocol;
    const host = window.location.host;
    window.location.href = `${protocol}//${host}/auth/login`;
  };
  return (
    <>
      <Modal
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <ErrorIcon sx={{ fontSize: 24, color: 'warning.main' }} />
            <Box sx={{ mt: '2px' }}>提示</Box>
          </Stack>
        }
        open={open}
        okText='确定'
        cancelText='取消'
        onCancel={() => setOpen(false)}
        onOk={handleLogout}
        closable={false}
      >
        <Box sx={{ pl: 4 }}>确定要退出登录吗？</Box>
      </Modal>
      <Tooltip title='退出登录' arrow>
        <IconButton size='small' onClick={() => setOpen(true)}>
          <IconDengchu
            sx={theme => ({
              cursor: 'pointer',
              color: alpha(theme.palette.text.primary, 0.65),
              fontSize: 24,
              '&:hover': { color: theme.palette.primary.main },
            })}
          />
        </IconButton>
      </Tooltip>
    </>
  );
};

const LoginButton = ({
  basePath,
  redirectPath,
}: {
  basePath: string;
  redirectPath: string;
}) => {
  const redirect = encodeURIComponent(redirectPath);
  const loginUrl = `${basePath || ''}/auth/login${redirect ? `?redirect=${redirect}` : ''}`;

  return (
    <Button
      href={loginUrl}
      variant='outlined'
      size='small'
      sx={{
        minWidth: 72,
        borderRadius: '999px',
      }}
    >
      登录
    </Button>
  );
};

const useMounted = () => {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  return mounted;
};

const Header = ({ isDocPage = false, isWelcomePage = false }: HeaderProps) => {
  const {
    mobile = false,
    kbDetail,
    catalogWidth,
    setQaModalOpen,
    authInfo,
  } = useStore();
  const mounted = useMounted();
  const basePath = useBasePath();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const docWidth = useMemo(() => {
    if (isWelcomePage) return 'full';
    return kbDetail?.settings?.theme_and_style?.doc_width || 'full';
  }, [kbDetail, isWelcomePage]);
  const currentRedirectPath = useMemo(() => {
    const search = searchParams?.toString();
    const normalizedPathname =
      pathname && basePath && pathname.startsWith(basePath)
        ? pathname.slice(basePath.length) || '/'
        : pathname || '/';

    return search ? `${normalizedPathname}?${search}` : normalizedPathname;
  }, [basePath, pathname, searchParams]);
  const headerLogo = kbDetail?.settings?.icon
    ? getImagePath(kbDetail.settings.icon, basePath)
    : '';
  const headerButtons = getHeaderButtons(
    kbDetail?.settings?.btns || [],
    basePath,
    currentRedirectPath,
    !!authInfo,
  );

  const handleSearch = (value?: string, type: 'chat' | 'search' = 'chat') => {
    if (value?.trim()) {
      if (type === 'chat') {
        sessionStorage.setItem('chat_search_query', value.trim());
        setQaModalOpen?.(true);
      } else {
        sessionStorage.setItem('chat_search_query', value.trim());
      }
    }
  };

  if (!mounted) {
    return (
      <Box
        sx={{
          position: 'sticky',
          top: 0,
          zIndex: 101,
          height: 64,
          flexShrink: 0,
          bgcolor: 'background.default',
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      />
    );
  }

  return (
    <>
      <CustomHeader
        isDocPage={isDocPage}
        mobile={mobile}
        docWidth={docWidth}
        catalogWidth={catalogWidth}
        logo={headerLogo}
        title={kbDetail?.settings?.title}
        placeholder={
          kbDetail?.settings?.web_app_custom_style?.header_search_placeholder
        }
        showSearch
        homePath={basePath || '/'}
        btns={headerButtons}
        onSearch={handleSearch}
        onQaClick={() => setQaModalOpen?.(true)}
      >
        <Stack sx={{ ml: 2 }} direction='row' alignItems='center' gap={1}>
          {!!authInfo && (
            <Box
              sx={theme => ({
                maxWidth: 180,
                color: theme.palette.text.secondary,
                fontSize: 14,
                fontWeight: 500,
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              })}
              title={authInfo.account || ''}
            >
              {authInfo.account}
            </Box>
          )}
          {!authInfo && (
            <LoginButton
              basePath={basePath}
              redirectPath={currentRedirectPath}
            />
          )}
          <ThemeSwitch />
          {!!authInfo && <LogoutButton />}
        </Stack>
      </CustomHeader>
      <QaModal />
    </>
  );
};

export const WelcomeHeader = ({
  showSearch = true,
}: {
  showSearch?: boolean;
}) => {
  const basePath = useBasePath();
  const {
    mobile = false,
    kbDetail,
    catalogWidth,
    setQaModalOpen,
    authInfo,
  } = useStore();
  const mounted = useMounted();
  const headerLogo = kbDetail?.settings?.icon
    ? getImagePath(kbDetail.settings.icon, basePath)
    : '';
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const currentRedirectPath = useMemo(() => {
    const search = searchParams?.toString();
    const normalizedPathname =
      pathname && basePath && pathname.startsWith(basePath)
        ? pathname.slice(basePath.length) || '/'
        : pathname || '/';

    return search ? `${normalizedPathname}?${search}` : normalizedPathname;
  }, [basePath, pathname, searchParams]);
  const headerButtons = getHeaderButtons(
    kbDetail?.settings?.btns || [],
    basePath,
    currentRedirectPath,
    !!authInfo,
  );
  const handleSearch = (value?: string, type: 'chat' | 'search' = 'chat') => {
    if (value?.trim()) {
      if (type === 'chat') {
        sessionStorage.setItem('chat_search_query', value.trim());
        setQaModalOpen?.(true);
      } else {
        sessionStorage.setItem('chat_search_query', value.trim());
      }
    }
  };

  if (!mounted) {
    return (
      <Box
        sx={{
          position: 'sticky',
          top: 0,
          zIndex: 101,
          height: 64,
          flexShrink: 0,
          bgcolor: 'background.default',
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      />
    );
  }

  return (
    <>
      <WelcomeHeaderComponent
        isDocPage={false}
        mobile={mobile}
        docWidth='full'
        catalogWidth={catalogWidth}
        logo={headerLogo}
        title={kbDetail?.settings?.title}
        placeholder={
          kbDetail?.settings?.web_app_custom_style?.header_search_placeholder
        }
        showSearch={showSearch}
        homePath={basePath || '/'}
        btns={headerButtons}
        onSearch={handleSearch}
        onQaClick={() => setQaModalOpen?.(true)}
      >
        {!authInfo && (
          <Box sx={{ ml: 2 }}>
            <LoginButton
              basePath={basePath}
              redirectPath={currentRedirectPath}
            />
          </Box>
        )}
        {!!authInfo && (
          <Box sx={{ ml: 2 }}>
            <LogoutButton />
          </Box>
        )}
      </WelcomeHeaderComponent>
      <QaModal />
    </>
  );
};

export default Header;
