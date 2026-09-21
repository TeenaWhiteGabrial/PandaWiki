import Card from '@/components/Card';
import {
  deleteApiProV1TokenDelete,
  getApiProV1TokenList,
  postApiProV1TokenCreate,
} from '@/request/pro';
import {
  ConstsUserKBPermission,
  GithubComChaitinPandaWikiProApiTokenV1CreateAPITokenReq,
  GithubComChaitinPandaWikiProApiTokenV1APITokenListItem,
} from '@/request/pro/types';
import { useAppSelector } from '@/store';
import { isOrdinaryReadOnlyUser } from '@/utils/permission';
import { copyText } from '@/utils';
import { message } from '@/components/ui';
import { Select } from 'antd';
import { Box, Button, Stack, TextField, Typography } from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';

const MCPAccess = () => {
  const { kb_id, user, kbDetail } = useAppSelector(state => state.config);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [tokens, setTokens] = useState<
    GithubComChaitinPandaWikiProApiTokenV1APITokenListItem[]
  >([]);
  const [name, setName] = useState('knowledgeWiki MCP');
  const readOnlyMCP = isOrdinaryReadOnlyUser(user, kbDetail);
  const permission = readOnlyMCP ? 'not null' : 'doc_manage';

  const fetchTokens = () => {
    if (!kb_id) return;
    setLoading(true);
    getApiProV1TokenList({ kb_id })
      .then(res => {
        setTokens(res || []);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchTokens();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kb_id]);

  const baseUrl = useMemo(() => {
    if (typeof window === 'undefined') return 'http://127.0.0.1:8000';
    return `${window.location.protocol}//${window.location.hostname}:8000`;
  }, []);

  const buildMCPConfigSnippet = (token: string) =>
    JSON.stringify(
      {
        mcpServers: {
          knowledgeWiki: {
            type: 'http',
            url: `${baseUrl}/api/v1/mcp`,
            headers: {
              Authorization: `Bearer ${token}`,
            },
          },
        },
      },
      null,
      2,
    );

  const handleCreate = () => {
    if (!kb_id) return;
    if (!name.trim()) {
      message.error('名称不能为空');
      return;
    }
    setCreating(true);
    const payload = {
      kb_id,
      name: name.trim(),
      permission,
    } as GithubComChaitinPandaWikiProApiTokenV1CreateAPITokenReq & {
      permission: 'not null' | 'doc_manage';
    };
    postApiProV1TokenCreate(payload)
      .then(() => {
        message.success('创建成功');
        fetchTokens();
      })
      .finally(() => setCreating(false));
  };

  const handleDelete = (id?: string) => {
    if (!kb_id || !id) return;
    deleteApiProV1TokenDelete({ id, kb_id }).then(() => {
      message.success('删除成功');
      fetchTokens();
    });
  };

  return (
    <Stack spacing={2}>
      <Card sx={{ p: 3 }}>
        <Stack spacing={2}>
          <Typography variant='h6'>MCP 接入</Typography>
          <Typography color='text.secondary' fontSize={14}>
            为 MCP 客户端创建长期可用的专用 API Token。不要直接使用 24
            小时过期的登录 token。下方展示的是通用 HTTP MCP
            连接片段，具体客户端外层配置格式可能略有差异。
          </Typography>
          <Typography color='text.secondary' fontSize={13}>
            {readOnlyMCP
              ? '当前账号只能创建只读 MCP Token，可阅读知识库内容，不能上传或发布文档。'
              : '当前 MCP Token 可用于文档管理。'}
          </Typography>
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
            <TextField
              label='Token 名称'
              value={name}
              onChange={e => setName(e.target.value)}
              fullWidth
            />
            <Box sx={{ minWidth: 180 }}>
              <Typography fontSize={12} color='text.secondary' mb={0.5}>
                权限
              </Typography>
              <Select
                value={permission}
                disabled
                style={{ width: '100%' }}
                options={[
                  {
                    value: permission,
                    label: readOnlyMCP ? '只读' : '文档管理',
                  },
                ]}
              />
            </Box>
            <Button
              variant='contained'
              onClick={handleCreate}
              disabled={creating || !kb_id}
            >
              创建 MCP Token
            </Button>
          </Stack>
        </Stack>
      </Card>

      <Card sx={{ p: 3 }}>
        <Stack spacing={2}>
          <Typography variant='h6'>现有 Token</Typography>
          {tokens.length === 0 ? (
            <Typography color='text.secondary' fontSize={14}>
              {loading ? '加载中...' : '暂无 MCP Token'}
            </Typography>
          ) : (
            tokens.map(item => (
              <Box
                key={item.id}
                sx={{
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 2,
                  p: 2,
                }}
              >
                <Stack spacing={1.5}>
                  <Stack
                    direction={{ xs: 'column', md: 'row' }}
                    justifyContent='space-between'
                    spacing={1}
                  >
                    <Box>
                      <Typography fontWeight={600}>{item.name}</Typography>
                      <Typography color='text.secondary' fontSize={13}>
                        权限：
                        {item.permission ===
                        ConstsUserKBPermission.UserKBPermissionNotNull
                          ? '只读'
                          : item.permission ===
                              ConstsUserKBPermission.UserKBPermissionDocManage
                            ? '文档管理'
                            : item.permission}
                        {' · '}
                        创建时间：
                        {item.created_at
                          ? dayjs(item.created_at).format('YYYY-MM-DD HH:mm:ss')
                          : '-'}
                      </Typography>
                    </Box>
                    <Stack direction='row' spacing={1}>
                      <Button
                        size='small'
                        onClick={() =>
                          copyText(
                            item.token || '',
                            undefined,
                            undefined,
                            'Token',
                          )
                        }
                      >
                        复制 Token
                      </Button>
                      <Button
                        size='small'
                        onClick={() =>
                          copyText(
                            buildMCPConfigSnippet(item.token || ''),
                            undefined,
                            undefined,
                            'MCP 配置片段',
                          )
                        }
                      >
                        复制 MCP 配置片段
                      </Button>
                      <Button
                        size='small'
                        color='error'
                        onClick={() => handleDelete(item.id)}
                      >
                        删除
                      </Button>
                    </Stack>
                  </Stack>
                  <TextField
                    label='通用 MCP 配置片段'
                    value={buildMCPConfigSnippet(item.token || '')}
                    fullWidth
                    multiline
                    minRows={8}
                    InputProps={{ readOnly: true }}
                  />
                </Stack>
              </Box>
            ))
          )}
        </Stack>
      </Card>
    </Stack>
  );
};

export default MCPAccess;
