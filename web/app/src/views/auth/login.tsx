'use client';

import { getApiV1User, postApiV1UserLogin } from '@/request/User';
import { clearCookie } from '@/utils/cookie';
import { alpha, Box, Stack, Typography } from '@mui/material';
import { message } from '@ctzhian/ui';
import { IconMima, IconZhanghao } from '@panda-wiki/icons';
import {
  AutoComplete,
  Button,
  Checkbox,
  ConfigProvider,
  Form,
  Input,
} from 'antd';
import Image from 'next/image';
import { useSearchParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';

import Logo from '@/assets/images/logo.png';
import { useBasePath } from '@/hooks';
import { useStore } from '@/provider';
import { getImagePath } from '@/utils/getImagePath';
import zhCN from 'antd/locale/zh_CN';

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
  const [form] = Form.useForm();

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
      form.setFieldsValue({
        account: parsed[0].account || '',
        password: parsed[0].password || '',
        remember: true,
      });
    }
  }, [form]);

  useEffect(() => {
    if (!authInfo?.id) {
      return;
    }
    window.location.href = redirectUrl;
  }, [authInfo?.id, redirectUrl]);

  const handleAccountChange = (value: string) => {
    setAccount(value);
    form.setFieldValue('account', value);
    const matched = rememberedLogins.find(item => item.account === value);
    if (matched) {
      setPassword(matched.password);
      setRemember(true);
      form.setFieldsValue({
        password: matched.password,
        remember: true,
      });
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
        <ConfigProvider
          locale={zhCN}
          theme={{
            token: {
              colorPrimary: themeMode === 'dark' ? '#7C9BFF' : '#1677ff',
              borderRadius: 12,
              colorBgContainer:
                themeMode === 'dark'
                  ? alpha('#0F172A', 0.72)
                  : 'rgba(255,255,255,0.96)',
              colorText:
                themeMode === 'dark' ? 'rgba(255,255,255,0.92)' : '#111827',
              colorTextPlaceholder:
                themeMode === 'dark' ? 'rgba(255,255,255,0.38)' : '#9CA3AF',
              colorBorder:
                themeMode === 'dark' ? 'rgba(255,255,255,0.14)' : '#D7DEEA',
            },
            components: {
              Input: {
                activeBorderColor: themeMode === 'dark' ? '#7C9BFF' : '#1677ff',
                hoverBorderColor: themeMode === 'dark' ? '#7C9BFF' : '#4096ff',
                colorBgContainer:
                  themeMode === 'dark'
                    ? 'rgba(15,23,42,0.72)'
                    : 'rgba(255,255,255,0.96)',
                paddingBlockLG: 13,
                paddingInlineLG: 14,
              },
              Button: {
                controlHeight: 52,
                fontWeight: 700,
              },
            },
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

            <Form
              form={form}
              layout='vertical'
              style={{ width: '100%' }}
              initialValues={{
                account: '',
                password: '',
                remember: false,
              }}
              onFinish={handleLogin}
            >
              <Form.Item
                name='account'
                help={
                  rememberedLogins.length
                    ? `已保存 ${rememberedLogins.length} 个账号，可下拉选择`
                    : ' '
                }
                style={{ marginBottom: 12 }}
              >
                <AutoComplete
                  options={rememberedLogins.map(item => ({
                    value: item.account,
                    label: item.account,
                  }))}
                  value={account}
                  onChange={value => handleAccountChange(value)}
                >
                  <Input
                    autoFocus
                    size='large'
                    placeholder='账号'
                    disabled={loading}
                    prefix={<IconZhanghao style={{ fontSize: 16 }} />}
                    onChange={e => handleAccountChange(e.target.value)}
                    onKeyDown={handleKeyDown}
                    styles={{
                      affixWrapper: {
                        minHeight: 54,
                        borderRadius: 14,
                        boxShadow:
                          themeMode === 'dark'
                            ? '0 8px 24px rgba(0, 0, 0, 0.16)'
                            : '0 8px 24px rgba(15, 23, 42, 0.06)',
                      },
                      input: {
                        boxShadow: 'none',
                        fontSize: 15,
                      },
                    }}
                  />
                </AutoComplete>
              </Form.Item>

              <Form.Item name='password' style={{ marginBottom: 12 }}>
                <Input.Password
                  size='large'
                  value={password}
                  placeholder='密码'
                  disabled={loading}
                  prefix={<IconMima style={{ fontSize: 16 }} />}
                  onChange={e => {
                    setPassword(e.target.value);
                    form.setFieldValue('password', e.target.value);
                  }}
                  onKeyDown={handleKeyDown}
                  styles={{
                    affixWrapper: {
                      minHeight: 54,
                      borderRadius: 14,
                      boxShadow:
                        themeMode === 'dark'
                          ? '0 8px 24px rgba(0, 0, 0, 0.16)'
                          : '0 8px 24px rgba(15, 23, 42, 0.06)',
                    },
                    input: {
                      fontSize: 15,
                    },
                  }}
                />
              </Form.Item>

              <Form.Item
                name='remember'
                valuePropName='checked'
                style={{ marginBottom: 24 }}
              >
                <Checkbox
                  checked={remember}
                  onChange={event => {
                    setRemember(event.target.checked);
                    form.setFieldValue('remember', event.target.checked);
                  }}
                >
                  记住密码并加入账号列表
                </Checkbox>
              </Form.Item>

              <Button
                block
                type='primary'
                size='large'
                htmlType='submit'
                loading={loading}
                disabled={!account.trim() || !password.trim()}
                style={{ fontSize: 16 }}
              >
                {loading ? '登录中...' : '登录'}
              </Button>
            </Form>
          </Stack>
        </ConfigProvider>
      </Box>
    </Box>
  );
}
