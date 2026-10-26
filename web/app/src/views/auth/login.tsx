'use client';

import {
  getApiV1User,
  postApiV1UserLogin,
  logoutAccount,
  changeOwnPassword,
} from '@/request/User';
import { clearAuthStorage } from '@/utils/auth';
import { PasswordChangeDialog } from '@panda-wiki/ui/security';
import { clearCookie } from '@/utils/cookie';
import {
  alpha,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { message } from '@ctzhian/ui';
import {
  IconBukejian,
  IconKejian,
  IconMima,
  IconZhanghao,
} from '@panda-wiki/icons';
import Image from 'next/image';
import { useSearchParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';

import Logo from '@/assets/images/logo.png';
import { useBasePath } from '@/hooks';
import { useStore } from '@/provider';
import { getImagePath } from '@/utils/getImagePath';

const LEGACY_REMEMBER_KEY = 'panda_wiki_remember_login';
const REMEMBER_KEY = 'panda_wiki_remember_accounts';
const MAX_REMEMBERED_LOGINS = 8;

type RememberedLogin = {
  account: string;
  updatedAt: number;
};

const loadRememberedLogins = (): RememberedLogin[] => {
  try {
    const raw =
      localStorage.getItem(REMEMBER_KEY) ||
      localStorage.getItem(LEGACY_REMEMBER_KEY);
    // Remove legacy credentials even when the stored value cannot be parsed.
    localStorage.removeItem(LEGACY_REMEMBER_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const accounts = parsed
      .filter(item => item && typeof item.account === 'string')
      .map(item => ({
        account: item.account as string,
        updatedAt: Number(item.updatedAt) || 0,
      }))
      .sort((a, b) => b.updatedAt - a.updatedAt)
      .slice(0, MAX_REMEMBERED_LOGINS);
    localStorage.setItem(REMEMBER_KEY, JSON.stringify(accounts));
    return accounts;
  } catch {
    localStorage.removeItem(LEGACY_REMEMBER_KEY);
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
  const [changeRequired, setChangeRequired] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
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
      setRemember(true);
    }
  }, []);

  useEffect(() => {
    if (!authInfo?.id) {
      return;
    }
    if (authInfo.password_change_required) {
      setAccount(authInfo.account || '');
      setChangeRequired(true);
      return;
    }
    window.location.href = redirectUrl;
  }, [authInfo?.id, authInfo?.password_change_required, redirectUrl]);

  const handleAccountChange = (value: string) => {
    setAccount(value);
    const matched = rememberedLogins.find(item => item.account === value);
    if (matched) {
      setPassword('');
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
      try {
        localStorage.setItem('pw-reader-auth-change', String(Date.now()));
      } catch {}
      const userInfo = await getApiV1User({ isAlert: false }).catch(
        () => undefined,
      );
      setAuthInfo?.(userInfo);

      const trimmedAccount = account.trim();
      if (remember) {
        const nextLogins = [
          {
            account: trimmedAccount,
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

      if (res.password_change_required) {
        setChangeRequired(true);
        return;
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
                autoComplete='username'
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
                        <IconZhanghao
                          sx={{ fontSize: 20, width: 28, mr: 1.25 }}
                        />
                        {params.InputProps.startAdornment}
                      </>
                    ),
                  },
                }}
                sx={{
                  mb: 3,
                  borderRadius: '14px',
                  overflow: 'hidden',
                  '& input:-webkit-autofill': {
                    WebkitTextFillColor: 'inherit',
                    WebkitBoxShadow: '0 0 0 1000px transparent inset',
                    transition: 'background-color 99999s ease-out 0s',
                  },
                  '& .MuiInputBase-input': {
                    p: 0,
                    lineHeight: '26px',
                    height: '26px',
                    fontSize: 15,
                  },
                  '& .MuiOutlinedInput-root': {
                    minHeight: 54,
                    pr: '18px',
                    bgcolor: 'transparent',
                    boxShadow:
                      themeMode === 'dark'
                        ? '0 8px 24px rgba(0, 0, 0, 0.16)'
                        : '0 8px 24px rgba(15, 23, 42, 0.06)',
                    '& fieldset': {
                      borderRadius: '14px',
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
            type={showPassword ? 'text' : 'password'}
            value={password}
            onChange={e => setPassword(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder='密码'
            disabled={loading}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position='start' sx={{ mr: 1.25 }}>
                    <IconMima sx={{ fontSize: 20, width: 28 }} />
                  </InputAdornment>
                ),
                endAdornment: (
                  <InputAdornment position='end'>
                    <IconButton
                      edge='end'
                      size='small'
                      onClick={() => setShowPassword(prev => !prev)}
                      disabled={loading}
                      sx={{ color: 'text.secondary' }}
                    >
                      {showPassword ? (
                        <IconBukejian sx={{ fontSize: 18 }} />
                      ) : (
                        <IconKejian sx={{ fontSize: 18 }} />
                      )}
                    </IconButton>
                  </InputAdornment>
                ),
              },
            }}
            sx={{
              borderRadius: '14px',
              overflow: 'hidden',
              '& input:-webkit-autofill': {
                WebkitTextFillColor: 'inherit',
                WebkitBoxShadow: '0 0 0 1000px transparent inset',
                transition: 'background-color 99999s ease-out 0s',
              },
              '& .MuiInputBase-input': {
                p: 0,
                lineHeight: '26px',
                height: '26px',
                fontSize: 15,
              },
              '& .MuiOutlinedInput-root': {
                minHeight: 54,
                pr: '18px',
                bgcolor: 'transparent',
                boxShadow:
                  themeMode === 'dark'
                    ? '0 8px 24px rgba(0, 0, 0, 0.16)'
                    : '0 8px 24px rgba(15, 23, 42, 0.06)',
                '& fieldset': {
                  borderRadius: '14px',
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
              label='记住账号'
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
      <PasswordChangeDialog
        open={changeRequired}
        account={account}
        initialPassword={password}
        onSubmit={changeOwnPassword}
        onCancel={async () => {
          await logoutAccount();
          clearAuthStorage();
          setAuthInfo?.(undefined);
          setChangeRequired(false);
          setPassword('');
        }}
        onComplete={() => {
          clearAuthStorage();
          setAuthInfo?.(undefined);
          setChangeRequired(false);
          setPassword('');
          message.success('密码已更新，请重新登录');
        }}
      />
    </Box>
  );
}
