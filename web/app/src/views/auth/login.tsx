'use client';

import { getApiV1User, postApiV1UserLogin } from '@/request/User';
import { clearCookie } from '@/utils/cookie';
import {
  alpha,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  Stack,
  TextField,
  Typography,
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
const MAX_REMEMBERED_LOGINS = 8;

type RememberedLogin = {
  account: string;
  password: string;
  updatedAt: number;
};

const loadRememberedLogins = (): RememberedLogin[] => {
  try {
    const raw = localStorage.getItem(REMEMBER_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as RememberedLogin[];
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(
        item =>
          item &&
          typeof item.account === 'string' &&
          typeof item.password === 'string',
      )
      .sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0));
  } catch {
    localStorage.removeItem(REMEMBER_KEY);
    return [];
  }
};

const saveRememberedLogins = (items: RememberedLogin[]) => {
  if (!items.length) {
    localStorage.removeItem(REMEMBER_KEY);
    return;
  }
  localStorage.setItem(
    REMEMBER_KEY,
    JSON.stringify(items.slice(0, MAX_REMEMBERED_LOGINS)),
  );
};

const useMounted = () => {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  return mounted;
};

export default function Login() {
  const searchParams = useSearchParams();
  const {
    kbDetail,
    mobile = false,
    themeMode,
    setAuthInfo,
    authInfo,
  } = useStore();
  const basePath = useBasePath();
  const mounted = useMounted();

  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [remember, setRemember] = useState(false);
  const [loading, setLoading] = useState(false);
  const [rememberedLogins, setRememberedLogins] = useState<RememberedLogin[]>(
    [],
  );

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
    const parsed = loadRememberedLogins();
    setRememberedLogins(parsed);
    if (parsed.length > 0) {
      setAccount(parsed[0].account || '');
      setPassword(parsed[0].password || '');
      setRemember(true);
    }
  }, []);

  useEffect(() => {
    if (!authInfo?.id) {
      return;
    }
    window.location.href = redirectUrl;
  }, [authInfo?.id, redirectUrl]);

  const handleAccountChange = (value: string) => {
    setAccount(value);
    const matched = rememberedLogins.find(item => item.account === value);
    if (matched) {
      setPassword(matched.password);
      setRemember(true);
    }
  };

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

      const trimmedAccount = account.trim();
      if (remember) {
        const nextLogins = [
          {
            account: trimmedAccount,
            password,
            updatedAt: Date.now(),
          },
          ...rememberedLogins.filter(item => item.account !== trimmedAccount),
        ];
        setRememberedLogins(nextLogins);
        saveRememberedLogins(nextLogins);
      } else {
        const nextLogins = rememberedLogins.filter(
          item => item.account !== trimmedAccount,
        );
        setRememberedLogins(nextLogins);
        saveRememberedLogins(nextLogins);
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
        bgcolor: themeMode === 'dark' ? 'background.default' : '#EEF3FB',
        position: 'relative',
        overflow: 'hidden',
        '&::before': {
          content: '""',
          position: 'absolute',
          inset: 0,
          background:
            themeMode === 'dark'
              ? 'radial-gradient(circle at top, rgba(71,120,255,0.18), transparent 36%), radial-gradient(circle at bottom right, rgba(20,167,141,0.14), transparent 28%)'
              : 'radial-gradient(circle at top left, rgba(71,120,255,0.18), transparent 28%), radial-gradient(circle at bottom right, rgba(20,167,141,0.12), transparent 24%)',
        },
      }}
    >
      <Box
        sx={{
          width: '100%',
          maxWidth: 420,
          p: 4.5,
          position: 'relative',
          zIndex: 1,
          backdropFilter: 'blur(14px)',
          bgcolor:
            themeMode === 'dark'
              ? alpha('#0F172A', 0.86)
              : alpha('#FFFFFF', 0.92),
          borderRadius: '24px',
          border: `1px solid ${
            themeMode === 'dark'
              ? alpha('#FFFFFF', 0.08)
              : alpha('#FFFFFF', 0.72)
          }`,
          boxShadow:
            themeMode === 'dark'
              ? '0px 20px 60px rgba(0,0,0,0.35)'
              : '0px 24px 80px rgba(15, 23, 42, 0.14)',
          ...(mounted &&
            mobile && {
              m: 3,
            }),
        }}
      >
        <Stack alignItems='center'>
          <Stack alignItems='center' gap={1} sx={{ mb: 5 }}>
            <Image src={logoSrc} width={44} height={44} alt='logo' />
            <Box sx={{ fontSize: 30, lineHeight: '38px', fontWeight: 700 }}>
              {title}
            </Box>
            <Typography
              sx={{
                fontSize: 14,
                color: 'text.secondary',
              }}
            >
              支持保存多个登录账号，选择后可直接带出密码
            </Typography>
          </Stack>

          <Autocomplete
            freeSolo
            fullWidth
            options={rememberedLogins.map(item => item.account)}
            value={account}
            onChange={(_, value) => handleAccountChange(value || '')}
            onInputChange={(_, value, reason) => {
              if (reason === 'input' || reason === 'clear') {
                handleAccountChange(value);
              }
            }}
            onKeyDown={handleKeyDown}
            slotProps={{
              paper: {
                sx: {
                  mt: 1,
                  borderRadius: '16px',
                  boxShadow:
                    themeMode === 'dark'
                      ? '0px 18px 48px rgba(0,0,0,0.32)'
                      : '0px 18px 48px rgba(15, 23, 42, 0.14)',
                  border: `1px solid ${
                    themeMode === 'dark'
                      ? alpha('#FFFFFF', 0.08)
                      : alpha('#D7DEEA', 0.9)
                  }`,
                },
              },
              listbox: {
                sx: {
                  py: 1,
                  '& .MuiAutocomplete-option': {
                    minHeight: 48,
                    px: 2,
                    fontSize: 14,
                  },
                },
              },
            }}
            renderInput={params => (
              <TextField
                {...params}
                fullWidth
                autoFocus
                placeholder='账号'
                disabled={loading}
                helperText={
                  rememberedLogins.length
                    ? `已保存 ${rememberedLogins.length} 个账号，可下拉选择`
                    : ' '
                }
                slotProps={{
                  input: {
                    ...params.InputProps,
                    startAdornment: (
                      <>
                        <IconZhanghao sx={{ fontSize: 16, width: 24, mr: 1 }} />
                        {params.InputProps.startAdornment}
                      </>
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
            )}
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
              label='记住密码并加入账号列表'
            />
          </Box>

          <Button
            fullWidth
            variant='contained'
            onClick={handleLogin}
            sx={{
              height: '52px',
              fontSize: 16,
              fontWeight: 700,
              borderRadius: '14px',
            }}
            disabled={loading || !account.trim() || !password.trim()}
          >
            {loading ? '登录中...' : '登录'}
          </Button>
        </Stack>
      </Box>
    </Box>
  );
}
