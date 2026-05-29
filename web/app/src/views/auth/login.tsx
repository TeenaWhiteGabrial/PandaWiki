'use client';

import {
  getShareV1AuthGet,
  postShareV1AuthLoginSimple,
} from '@/request/ShareAuth';
import { getShareV1NodeList } from '@/request/ShareNode';
import { parseNodeListResponse } from '@/utils/tree';
import { clearCookie } from '@/utils/cookie';

import {
  IconKoulingrenzheng,
  IconLDAP,
  IconMima,
  IconZhanghao,
} from '@panda-wiki/icons';

import Logo from '@/assets/images/logo.png';
import { FooterProvider } from '@/components/footer';
import { useStore } from '@/provider';
import { ConstsSourceType, ConstsAuthType } from '@/request/types';
import { Box, Button, InputAdornment, Stack, TextField } from '@mui/material';
import { message } from '@ctzhian/ui';
import Image from 'next/image';
import { useSearchParams } from 'next/navigation';
import { useEffect, useState } from 'react';
import { useBasePath } from '@/hooks';
import { getImagePath } from '@/utils/getImagePath';
export default function Login() {
  const searchParams = useSearchParams();
  const [password, setPassword] = useState('');
  const [username, setUsername] = useState('');
  const [loading, setLoading] = useState(false);
  const [authType, setAuthType] = useState<ConstsAuthType>();
  const [sourceType, setSourceType] = useState<ConstsSourceType>();
  const { kbDetail, themeMode, mobile = false, setNodeList } = useStore();
  const basePath = useBasePath();

  const redirectUrl =
    typeof window !== 'undefined'
      ? `${window.location.origin}${basePath}${decodeURIComponent(searchParams.get('redirect') || '')}`
      : '';

  const handleLogin = async () => {
    if (!password.trim()) {
      message.error('请输入访问口令');
      return;
    }
    setLoading(true);
    try {
      clearCookie();
      postShareV1AuthLoginSimple({
        password,
      }).then(() => {
        getShareV1NodeList().then(res => {
          const raw = (res as any) ?? [];
          const { isGrouped, navDataMap, defaultNavId } =
            parseNodeListResponse(raw);
          const nodeList = isGrouped
            ? (navDataMap[defaultNavId || ''] ??
              navDataMap[Object.keys(navDataMap)[0]] ??
              [])
            : raw;
          setNodeList?.(Array.isArray(nodeList) ? nodeList : []);
          message.success('认证成功');
          window.open(redirectUrl, '_self');
        });
      });
    } catch (error) {
      message.error('认证失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      if (authType === ConstsAuthType.AuthTypeEnterprise) {
        if (username.trim() && password.trim()) {
          handleLDAPLogin();
        }
      } else {
        handleLogin();
      }
    }
  };

  const handleLDAPLogin = () => {
    setLoading(true);
    try {
      message.error('当前页面未启用该认证方式');
    } catch (error) {
      message.error('认证失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    getShareV1AuthGet({}).then(res => {
      setAuthType(res?.auth_type);
      setSourceType(res?.source_type);
      if (res?.auth_type === ConstsAuthType.AuthTypeNull) {
        window.open(redirectUrl, '_self');
      }
    });
  }, []);

  return (
    <>
      <Box
        sx={{
          minHeight: mobile ? `calc(100vh - 120px)` : `calc(100vh - 40px)`,
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
            maxWidth: 400,
            p: 4,
            backdropFilter: 'blur(2px)',
            bgcolor:
              themeMode === 'dark' ? 'background.paper' : 'background.default',
            borderRadius: '10px',
            ...(mobile && {
              m: 3,
            }),
          }}
        >
          <Stack alignItems='center'>
            <Stack alignItems='center' gap={1} sx={{ mb: 5 }}>
              {kbDetail?.settings?.icon ? (
                <img
                  src={getImagePath(kbDetail?.settings?.icon, basePath)}
                  alt='logo'
                  width={40}
                  height={40}
                />
              ) : (
                <Image src={Logo.src} width={40} height={40} alt='logo' />
              )}
              <Box
                sx={{ fontSize: 28, lineHeight: '36px', fontWeight: 'bold' }}
              >
                {kbDetail?.settings?.title}
              </Box>
            </Stack>
            {authType === ConstsAuthType.AuthTypeSimple && (
              <>
                <TextField
                  fullWidth
                  type='password'
                  value={password}
                  autoFocus
                  onChange={e => setPassword(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder='请输入访问口令'
                  disabled={loading}
                  slotProps={{
                    input: {
                      startAdornment: (
                        <InputAdornment position='start'>
                          <IconKoulingrenzheng
                            sx={{ fontSize: 16, width: 24, height: 16 }}
                          />
                        </InputAdornment>
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
                      fontFamily: 'Mono',
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
                <Button
                  fullWidth
                  variant='contained'
                  onClick={handleLogin}
                  sx={{ mt: 5, height: '50px', fontSize: 16 }}
                  disabled={loading || !password.trim()}
                >
                  {loading ? '验证中...' : '认证访问'}
                </Button>
              </>
            )}

            {authType === ConstsAuthType.AuthTypeEnterprise && (
              <>
                {sourceType === ConstsSourceType.SourceTypeDingTalk && (
                  <Button
                    fullWidth
                    variant='contained'
                    disabled
                    sx={{ height: '50px', fontSize: 16 }}
                  >
                    当前未启用
                  </Button>
                )}
                {sourceType === ConstsSourceType.SourceTypeFeishu && (
                  <Button
                    fullWidth
                    variant='contained'
                    disabled
                    sx={{ height: '50px', fontSize: 16 }}
                  >
                    当前未启用
                  </Button>
                )}
                {sourceType === ConstsSourceType.SourceTypeWeCom && (
                  <Button
                    fullWidth
                    variant='contained'
                    disabled
                    sx={{ height: '50px', fontSize: 16 }}
                  >
                    当前未启用
                  </Button>
                )}
                {sourceType === ConstsSourceType.SourceTypeOAuth && (
                  <Button
                    fullWidth
                    variant='contained'
                    disabled
                    sx={{ height: '50px', fontSize: 16 }}
                  >
                    当前未启用
                  </Button>
                )}
                {sourceType === ConstsSourceType.SourceTypeGitHub && (
                  <Button
                    fullWidth
                    variant='contained'
                    disabled
                    sx={{ height: '50px', fontSize: 16 }}
                  >
                    当前未启用
                  </Button>
                )}

                {sourceType === ConstsSourceType.SourceTypeCAS && (
                  <Button
                    fullWidth
                    variant='contained'
                    disabled
                    sx={{ height: '50px', fontSize: 16 }}
                  >
                    当前未启用
                  </Button>
                )}

                {sourceType === ConstsSourceType.SourceTypeLDAP && (
                  <Stack spacing={2} width='100%'>
                    {(() => {
                      const textFieldSx = {
                        borderRadius: '10px',
                        overflow: 'hidden',
                        '& .MuiInputBase-input': {
                          p: 2,
                          lineHeight: '24px',
                          height: '24px',
                          fontFamily: 'Mono',
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
                      };

                      return (
                        <>
                          <TextField
                            fullWidth
                            type='text'
                            value={username}
                            autoFocus
                            onChange={e => setUsername(e.target.value)}
                            placeholder='用户名'
                            disabled={loading}
                            slotProps={{
                              input: {
                                startAdornment: (
                                  <InputAdornment position='start'>
                                    <IconZhanghao
                                      sx={{
                                        fontSize: 16,
                                        width: 24,
                                        height: 16,
                                      }}
                                    />
                                  </InputAdornment>
                                ),
                              },
                            }}
                            sx={textFieldSx}
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
                                  <InputAdornment position='start'>
                                    <IconMima
                                      sx={{
                                        fontSize: 16,
                                        width: 24,
                                        height: 16,
                                      }}
                                    />
                                  </InputAdornment>
                                ),
                              },
                            }}
                            sx={textFieldSx}
                          />
                          <Button
                            fullWidth
                            variant='contained'
                            onClick={handleLDAPLogin}
                            sx={{
                              mt: 2,
                              height: '50px',
                              fontSize: 16,
                              borderRadius: '10px',
                              boxShadow: '0px 4px 12px rgba(0, 0, 0, 0.1)',
                            }}
                            startIcon={
                              <IconLDAP
                                sx={{
                                  fontSize: 16,
                                  width: 24,
                                  height: 16,
                                  color:
                                    loading ||
                                    !username.trim() ||
                                    !password.trim()
                                      ? ''
                                      : '#e73f3f',
                                }}
                              />
                            }
                            disabled={
                              loading || !username.trim() || !password.trim()
                            }
                          >
                            {loading ? '验证中...' : '登录'}
                          </Button>
                        </>
                      );
                    })()}
                  </Stack>
                )}
              </>
            )}

            <Box
              sx={{
                textAlign: 'center',
                color: 'text.disabled',
                fontSize: 14,
                lineHeight: '20px',
                mt: 2,
              }}
            >
              需要认证以后才能访问
            </Box>
          </Stack>
        </Box>
      </Box>
      <FooterProvider showBrand={false} />
    </>
  );
}
