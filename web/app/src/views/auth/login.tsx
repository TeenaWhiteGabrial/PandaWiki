'use client';

import { getApiV1User, postApiV1UserLogin } from '@/request/User';
import { clearCookie } from '@/utils/cookie';
import {
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  Stack,
  TextField,
} from '@mui/material';
import { message } from '@ctzhian/ui';
import { IconMima, IconZhanghao } from '@panda-wiki/icons';
import Image from 'next/image';
import { useSearchParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';

import Logo from '@/assets/images/logo.png';
import { useBasePath } from '@/hooks';
import { useStore } from '@/provider';
import { getImagePath } from '@/utils/getImagePath';

const REMEMBER_KEY = 'panda_wiki_remember_login';

const useMounted = () => {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  return mounted;
};

export default function Login() {
  const searchParams = useSearchParams();
  const { kbDetail, mobile = false, themeMode, setAuthInfo } = useStore();
  const basePath = useBasePath();
  const mounted = useMounted();

  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [remember, setRemember] = useState(false);
  const [loading, setLoading] = useState(false);

  const redirectPath = searchParams.get('redirect') || '/';
  const redirectUrl = useMemo(() => {
    if (typeof window === 'undefined') return redirectPath;
    return `${window.location.origin}${basePath}${decodeURIComponent(redirectPath)}`;
  }, [basePath, redirectPath]);
  const logoSrc =
    mounted && kbDetail?.settings?.icon
      ? getImagePath(kbDetail.settings.icon, basePath)
      : Logo.src;
  const title = kbDetail?.settings?.title || '知识库';

  useEffect(() => {
    const remembered = localStorage.getItem(REMEMBER_KEY);
    if (!remembered) return;
    try {
      const parsed = JSON.parse(remembered) as {
        account?: string;
        password?: string;
      };
      setAccount(parsed.account || '');
      setPassword(parsed.password || '');
      setRemember(true);
    } catch {
      localStorage.removeItem(REMEMBER_KEY);
    }
  }, []);

  const handleLogin = async () => {
    if (!account.trim()) {
      message.error('请输入账号');
      return;
    }
    if (!password.trim()) {
      message.error('请输入密码');
      return;
    }

    setLoading(true);
    try {
      clearCookie();
      const res = await postApiV1UserLogin({
        account: account.trim(),
        password,
      });
      const token = res?.token;
      if (!token) {
        message.error('登录失败，请重试');
        return;
      }

      localStorage.setItem('panda_wiki_token', token);
      document.cookie = `panda_wiki_token=${encodeURIComponent(token)}; path=/; SameSite=Lax`;
      const userInfo = await getApiV1User({ isAlert: false }).catch(
        () => undefined,
      );
      setAuthInfo?.(userInfo);

      if (remember) {
        localStorage.setItem(
          REMEMBER_KEY,
          JSON.stringify({
            account: account.trim(),
            password,
          }),
        );
      } else {
        localStorage.removeItem(REMEMBER_KEY);
      }

      message.success('登录成功');
      window.location.href = redirectUrl;
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && account.trim() && password.trim()) {
      handleLogin();
    }
  };

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor:
          themeMode === 'dark' ? 'background.default' : 'background.paper',
      }}
    >
      <Box
        sx={{
          width: '100%',
          maxWidth: 420,
          p: 4,
          backdropFilter: 'blur(2px)',
          bgcolor:
            themeMode === 'dark' ? 'background.paper' : 'background.default',
          borderRadius: '10px',
          ...(mounted &&
            mobile && {
              m: 3,
            }),
        }}
      >
        <Stack alignItems='center'>
          <Stack alignItems='center' gap={1} sx={{ mb: 5 }}>
            <Image src={logoSrc} width={40} height={40} alt='logo' />
            <Box sx={{ fontSize: 28, lineHeight: '36px', fontWeight: 'bold' }}>
              {title}
            </Box>
          </Stack>

          <TextField
            fullWidth
            value={account}
            autoFocus
            onChange={e => setAccount(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder='账号'
            disabled={loading}
            slotProps={{
              input: {
                startAdornment: (
                  <IconZhanghao sx={{ fontSize: 16, width: 24, mr: 1 }} />
                ),
              },
            }}
            sx={{
              mb: 3,
              borderRadius: '10px',
              overflow: 'hidden',
              '& .MuiInputBase-input': {
                p: 2,
                lineHeight: '24px',
                height: '24px',
              },
              '& .MuiOutlinedInput-root': {
                pr: '18px',
                bgcolor: 'background.paper',
                '& fieldset': {
                  borderRadius: '10px',
                  borderColor: 'divider',
                  px: 2,
                },
              },
            }}
          />

          <TextField
            fullWidth
            type='password'
            value={password}
            onChange={e => setPassword(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder='密码'
            disabled={loading}
            slotProps={{
              input: {
                startAdornment: (
                  <IconMima sx={{ fontSize: 16, width: 24, mr: 1 }} />
                ),
              },
            }}
            sx={{
              borderRadius: '10px',
              overflow: 'hidden',
              '& .MuiInputBase-input': {
                p: 2,
                lineHeight: '24px',
                height: '24px',
              },
              '& .MuiOutlinedInput-root': {
                pr: '18px',
                bgcolor: 'background.paper',
                '& fieldset': {
                  borderRadius: '10px',
                  borderColor: 'divider',
                  px: 2,
                },
              },
            }}
          />

          <Box sx={{ width: '100%', mt: 1, mb: 3 }}>
            <FormControlLabel
              control={
                <Checkbox
                  checked={remember}
                  onChange={event => setRemember(event.target.checked)}
                />
              }
              label='记住密码'
            />
          </Box>

          <Button
            fullWidth
            variant='contained'
            onClick={handleLogin}
            sx={{ height: '50px', fontSize: 16 }}
            disabled={loading || !account.trim() || !password.trim()}
          >
            {loading ? '登录中...' : '登录'}
          </Button>
        </Stack>
      </Box>
    </Box>
  );
}
