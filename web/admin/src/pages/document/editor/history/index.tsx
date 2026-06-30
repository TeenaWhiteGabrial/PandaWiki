import Card from '@/components/Card';
import {
  getNodeReleaseDetail,
  getNodeReleaseList,
  restoreNodeRelease,
} from '@/request';
import {
  DomainGetNodeReleaseDetailResp,
  DomainNodeReleaseListItem,
} from '@/request/pro/types';
import { useAppSelector } from '@/store';
import { isOrdinaryReadOnlyUser } from '@/utils/permission';
import { message } from '@/components/ui';
import { Ellipsis } from '@/components/ui';
import {
  Box,
  Button,
  Divider,
  Skeleton,
  Stack,
  Typography,
} from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

const History = () => {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const { kb_id = '', user, kbDetail } = useAppSelector(state => state.config);
  const readOnlyUser = isOrdinaryReadOnlyUser(user, kbDetail);

  const [loading, setLoading] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [list, setList] = useState<DomainNodeReleaseListItem[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [detail, setDetail] = useState<DomainGetNodeReleaseDetailResp | null>(
    null,
  );

  const currentItem = useMemo(
    () => list.find(item => item.id === selectedId) || null,
    [list, selectedId],
  );

  const loadList = () => {
    if (!kb_id || !id) return;
    setLoading(true);
    getNodeReleaseList({
      kb_id,
      node_id: id,
    })
      .then(res => {
        const historyList = res || [];
        setList(historyList);
        if (historyList.length > 0) {
          setSelectedId(prev => {
            const nextId =
              prev && historyList.some(item => item.id === prev)
                ? prev
                : historyList[0].id || '';
            return nextId;
          });
        } else {
          setSelectedId('');
          setDetail(null);
        }
      })
      .finally(() => {
        setLoading(false);
      });
  };

  const loadDetail = (releaseId: string) => {
    if (!kb_id || !releaseId) return;
    setLoading(true);
    getNodeReleaseDetail({
      kb_id,
      id: releaseId,
    })
      .then(res => {
        setDetail(res);
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    loadList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kb_id, id]);

  useEffect(() => {
    if (selectedId) {
      loadDetail(selectedId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId]);

  const handleRestore = () => {
    if (!selectedId || !kb_id) return;
    setRestoring(true);
    restoreNodeRelease({
      id: selectedId,
      kb_id,
    })
      .then(() => {
        message.success('已恢复为当前草稿');
        navigate(`/doc/editor/${id}`);
      })
      .finally(() => {
        setRestoring(false);
      });
  };

  return (
    <Stack
      direction='row'
      gap={2}
      sx={{
        p: 2,
        height: 'calc(100vh - 64px)',
        boxSizing: 'border-box',
      }}
    >
      <Card sx={{ width: 360, overflow: 'hidden', display: 'flex' }}>
        <Stack sx={{ width: '100%' }}>
          <Box
            sx={{
              px: 2,
              py: 1.5,
              fontSize: 16,
              fontWeight: 700,
              borderBottom: theme => `1px solid ${theme.palette.divider}`,
            }}
          >
            历史版本
          </Box>
          <Stack sx={{ overflowY: 'auto', minHeight: 0 }}>
            {loading && list.length === 0
              ? Array.from({ length: 5 }).map((_, index) => (
                  <Box key={index} sx={{ p: 2 }}>
                    <Skeleton height={20} width='80%' />
                    <Skeleton height={18} width='55%' />
                    <Skeleton height={18} width='65%' />
                  </Box>
                ))
              : list.map(item => {
                  const active = item.id === selectedId;
                  return (
                    <Box
                      key={item.id}
                      onClick={() => setSelectedId(item.id || '')}
                      sx={{
                        px: 2,
                        py: 1.5,
                        cursor: 'pointer',
                        bgcolor: active ? 'action.selected' : 'transparent',
                        borderBottom: theme =>
                          `1px solid ${theme.palette.divider}`,
                        '&:hover': {
                          bgcolor: active ? 'action.selected' : 'action.hover',
                        },
                      }}
                    >
                      <Stack gap={0.5}>
                        <Stack direction='row' alignItems='center' gap={1}>
                          <Box sx={{ fontSize: 18, lineHeight: 1 }}>
                            {item.meta?.emoji || '📝'}
                          </Box>
                          <Ellipsis sx={{ fontSize: 14, fontWeight: 600 }}>
                            {item.name || '未命名版本'}
                          </Ellipsis>
                        </Stack>
                        <Typography
                          variant='body2'
                          sx={{ color: 'text.secondary' }}
                        >
                          {dayjs(item.updated_at).format('YYYY-MM-DD HH:mm:ss')}
                        </Typography>
                        <Typography
                          variant='body2'
                          sx={{ color: 'text.tertiary' }}
                        >
                          发布人：{item.publisher_account || '-'}
                        </Typography>
                        <Typography
                          variant='body2'
                          sx={{ color: 'text.tertiary' }}
                        >
                          发布说明：
                          {item.release_message || item.release_name || '-'}
                        </Typography>
                      </Stack>
                    </Box>
                  );
                })}
            {!loading && list.length === 0 && (
              <Box sx={{ p: 4, color: 'text.secondary', fontSize: 14 }}>
                当前文档还没有历史发布版本。
              </Box>
            )}
          </Stack>
        </Stack>
      </Card>

      <Card sx={{ flex: 1, minWidth: 0, display: 'flex' }}>
        <Stack sx={{ width: '100%', minWidth: 0 }}>
          <Stack
            direction='row'
            alignItems='center'
            justifyContent='space-between'
            sx={{
              px: 2,
              py: 1.5,
              borderBottom: theme => `1px solid ${theme.palette.divider}`,
            }}
          >
            <Stack sx={{ minWidth: 0 }}>
              <Ellipsis sx={{ fontSize: 16, fontWeight: 700 }}>
                {detail?.name || currentItem?.name || '历史版本详情'}
              </Ellipsis>
              {currentItem && (
                <Typography variant='body2' sx={{ color: 'text.secondary' }}>
                  {dayjs(currentItem.updated_at).format('YYYY-MM-DD HH:mm:ss')}
                </Typography>
              )}
            </Stack>
            {!readOnlyUser && (
              <Button
                variant='contained'
                disabled={!selectedId}
                loading={restoring}
                onClick={handleRestore}
              >
                恢复为当前草稿
              </Button>
            )}
          </Stack>

          <Stack
            direction='row'
            gap={3}
            sx={{ px: 2, py: 1.5, color: 'text.secondary', fontSize: 13 }}
          >
            <Box>
              发布人：
              {detail?.publisher_account ||
                currentItem?.publisher_account ||
                '-'}
            </Box>
            <Box>
              编辑人：
              {detail?.editor_account || currentItem?.editor_account || '-'}
            </Box>
            <Box>
              创建人：
              {detail?.creator_account || currentItem?.creator_account || '-'}
            </Box>
          </Stack>
          <Divider />
          <Box
            sx={{
              p: 2,
              overflowY: 'auto',
              minHeight: 0,
              flex: 1,
              bgcolor: 'background.paper',
            }}
          >
            {loading && !detail ? (
              <Stack gap={1}>
                <Skeleton height={28} width='30%' />
                <Skeleton height={22} width='100%' />
                <Skeleton height={22} width='100%' />
                <Skeleton height={22} width='88%' />
              </Stack>
            ) : detail ? (
              <Box>
                <Typography
                  variant='body2'
                  sx={{ color: 'text.secondary', mb: 1 }}
                >
                  内容类型：{detail.meta?.content_type || 'md'}
                </Typography>
                <Box
                  component='pre'
                  sx={{
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                    fontSize: 14,
                    lineHeight: '24px',
                    m: 0,
                    fontFamily:
                      'ui-monospace, SFMono-Regular, SFMono-Regular, Consolas, monospace',
                  }}
                >
                  {detail.content || ''}
                </Box>
              </Box>
            ) : (
              <Box sx={{ color: 'text.secondary', fontSize: 14 }}>
                请选择一个历史版本查看内容。
              </Box>
            )}
          </Box>
        </Stack>
      </Card>
    </Stack>
  );
};

export default History;
