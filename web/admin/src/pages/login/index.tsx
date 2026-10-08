import { postApiV1UserLogin } from '@/request/User';
import Bgi from '@/assets/images/login-bgi.png';
import Logo from '@/assets/images/logo.png';
import Avatar from '@/components/Avatar';
import Card from '@/components/Card';
import { useURLSearchParams } from '@/hooks';
import { AutoComplete, Input } from 'antd';
import {
  alpha,
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  IconButton,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { message } from '@/components/ui';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAppSelector } from '@/store';
import {
  IconZhanghao,
  IconIcon_tool_close,
  IconMima,
  IconKejian,
  IconBukejian,
} from '@panda-wiki/icons';

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

const Login = () => {
  const navigate = useNavigate();
  const { appBrand } = useAppSelector(state => state.config);
  const [searchParams] = useURLSearchParams();
  const redirect = searchParams.get('redirect') || '/';
  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [remember, setRemember] = useState(false);
  const [see, setSee] = useState(false);
  const [loading, setLoading] = useState(false);
  const [rememberedLogins, setRememberedLogins] = useState<RememberedLogin[]>(
    [],
  );

  useEffect(() => {
    const parsed = loadRememberedLogins();
    setRememberedLogins(parsed);
    if (parsed.length > 0) {
      setAccount(parsed[0].account || '');
      setPassword(parsed[0].password || '');
      setRemember(true);
    }
  }, []);

  const handleAccountChange = (value: string) => {
    setAccount(value);
    const matched = rememberedLogins.find(item => item.account === value);
    if (matched) {
      setPassword(matched.password);
      setRemember(true);
    }
  };

  const submit = () => {
    postApiV1UserLogin({ account, password })
      .then(res => {
        localStorage.setItem('panda_wiki_token', res.token!);
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
        navigate(redirect);
        message.success('登录成功');
      })
      .catch(() => undefined)
      .finally(() => {
        setLoading(false);
      });
  };

  return (
    <Box
      sx={{
        width: '100vw',
        height: '100vh',
        backgroundImage: `url(${Bgi})`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat',
        position: 'relative',
        '&::after': {
          content: '""',
          position: 'absolute',
          inset: 0,
          background:
            'linear-gradient(135deg, rgba(11,18,36,0.62) 0%, rgba(26,45,88,0.38) 45%, rgba(255,255,255,0.12) 100%)',
        },
      }}
    >
      <Stack
        direction='row'
        alignItems={'center'}
        justifyContent={'center'}
        sx={{
          width: '100%',
          height: '100%',
          position: 'relative',
          zIndex: 1,
        }}
      >
        <Card
          sx={{
            mt: -3,
            p: 4.5,
            width: 458,
            boxShadow:
              '0px 24px 80px rgba(6, 16, 38, 0.24), 0px 6px 18px rgba(6, 16, 38, 0.12)',
            position: 'absolute',
            top: '50%',
            left: '50%',
            bgcolor: alpha('#FFFFFF', 0.94),
            border: `1px solid ${alpha('#FFFFFF', 0.55)}`,
            borderRadius: '24px',
            backdropFilter: 'blur(14px)',
            transform: 'translate(-50%, -50%)',
          }}
        >
          <Stack alignItems={'center'}>
            <Avatar
              src={appBrand.logo || Logo}
              sx={{ width: 68, height: 68, mb: 1.5 }}
            />
            <Stack
              direction={'row'}
              alignItems={'center'}
              gap={1}
              sx={{
                fontSize: 30,
                fontWeight: 700,
                color: 'text.primary',
                mb: 1,
              }}
            >
              {appBrand.title || '知识库管理台'}
            </Stack>
            <Typography
              sx={{
                mb: 4,
                fontSize: 14,
                color: 'text.secondary',
              }}
            >
              仅管理员和超级管理员可登录
              <br />
              普通用户请使用用户端查阅文档
            </Typography>
            <AutoComplete
              style={{ width: '100%', marginBottom: 24 }}
              options={rememberedLogins.map(item => ({
                value: item.account,
                label: item.account,
              }))}
              value={account}
              onSelect={value => handleAccountChange(value)}
              onChange={value => handleAccountChange(value)}
              onSearch={value => handleAccountChange(value)}
              allowClear
              popupMatchSelectWidth
            >
              <Input
                size='large'
                placeholder='账号'
                autoFocus
                tabIndex={1}
                prefix={<IconZhanghao sx={{ fontSize: 16, mr: 1 }} />}
                suffix={
                  account ? (
                    <IconButton
                      onClick={() => handleAccountChange('')}
                      size='small'
                      tabIndex={-1}
                    >
                      <IconIcon_tool_close
                        sx={{
                          fontSize: 14,
                          color: 'text.tertiary',
                        }}
                      />
                    </IconButton>
                  ) : null
                }
                styles={{
                  input: {
                    fontSize: 14,
                  },
                }}
              />
            </AutoComplete>
            <Typography
              sx={{
                width: '100%',
                mt: -2,
                mb: 3,
                minHeight: 20,
                fontSize: 12,
                color: 'text.secondary',
              }}
            >
              {rememberedLogins.length
                ? `已保存 ${rememberedLogins.length} 个账号，可下拉选择`
                : ' '}
            </Typography>
            <TextField
              value={password}
              fullWidth
              sx={{ mb: 4 }}
              onChange={e => setPassword(e.target.value)}
              tabIndex={2}
              onKeyDown={e => {
                if (e.key === 'Enter') {
                  if (!account || !password) return;
                  setLoading(true);
                  submit();
                }
              }}
              placeholder='密码'
              type={see ? 'text' : 'password'}
              slotProps={{
                input: {
                  startAdornment: (
                    <IconMima sx={{ fontSize: 16, mr: 2, flexShrink: 0 }} />
                  ),
                  endAdornment: password ? (
                    <Stack direction={'row'} alignItems={'center'}>
                      <IconButton
                        onClick={() => setSee(!see)}
                        size='small'
                        tabIndex={-1}
                      >
                        {see ? (
                          <IconKejian
                            sx={{ fontSize: 18, color: 'text.tertiary' }}
                          />
                        ) : (
                          <IconBukejian
                            sx={{ fontSize: 18, color: 'text.tertiary' }}
                          />
                        )}
                      </IconButton>
                      <IconButton
                        onClick={() => setPassword('')}
                        size='small'
                        tabIndex={-1}
                      >
                        <IconIcon_tool_close
                          sx={{ fontSize: 14, color: 'text.tertiary' }}
                        />
                      </IconButton>
                    </Stack>
                  ) : null,
                },
              }}
            />
            <Box sx={{ width: '100%', mb: 3, mt: -2 }}>
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
              sx={{
                height: 50,
                borderRadius: '14px',
                fontSize: 16,
                fontWeight: 700,
              }}
              onClick={() => {
                if (!account || !password) return;
                setLoading(true);
                submit();
              }}
              loading={loading}
            >
              登录
            </Button>
          </Stack>
        </Card>
      </Stack>
    </Box>
  );
};

export default Login;
