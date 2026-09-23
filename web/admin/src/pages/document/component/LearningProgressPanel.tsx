import { Modal, message } from '@/components/ui';
import { getRagStatusMeta } from '@/pages/document/utils/ragStatus';
import { postApiV1NodeRestudy } from '@/request/Node';
import {
  ConstsNodeRagInfoStatus,
  DomainNodeListItemResp,
  DomainNodeStatus,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
} from '@/request/types';
import { useAppSelector } from '@/store';
import {
  Box,
  Button,
  Chip,
  Divider,
  Pagination,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';

type FilterKey = 'attention' | 'running' | 'failed' | 'succeeded' | 'all';

interface LearningProgressPanelProps {
  open: boolean;
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  onClose: () => void;
  refresh: () => void;
}

type LearningDoc = DomainNodeListItemResp & {
  navName: string;
  folderPath: string;
  statusKey: ConstsNodeRagInfoStatus | 'NONE';
};

const pageSize = 10;

const isRunningStatus = (status: LearningDoc['statusKey']) =>
  status === ConstsNodeRagInfoStatus.NodeRagStatusPending ||
  status === ConstsNodeRagInfoStatus.NodeRagStatusRunning ||
  status === ConstsNodeRagInfoStatus.NodeRagStatusReindexing ||
  status === 'NONE';

const LearningProgressPanel = ({
  open,
  groups,
  onClose,
  refresh,
}: LearningProgressPanelProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [filter, setFilter] = useState<FilterKey>('attention');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [submitting, setSubmitting] = useState(false);

  const docs = useMemo<LearningDoc[]>(() => {
    const result: LearningDoc[] = [];
    groups.forEach(group => {
      const list = (group.list || []) as DomainNodeListItemResp[];
      const nodeMap = new Map(list.map(item => [item.id || '', item]));
      const navName = group.nav_name || '未命名目录';

      const getFolderPath = (item: DomainNodeListItemResp) => {
        const folders: string[] = [];
        let current = item.parent_id ? nodeMap.get(item.parent_id) : undefined;
        const visited = new Set<string>();
        while (current?.id && !visited.has(current.id)) {
          visited.add(current.id);
          if (current.name) folders.unshift(current.name);
          current = current.parent_id
            ? nodeMap.get(current.parent_id)
            : undefined;
        }
        return [navName, ...folders].filter(Boolean).join(' / ');
      };

      list.forEach(item => {
        if (
          item.type !== 2 ||
          item.status !== DomainNodeStatus.NodeStatusPublished
        ) {
          return;
        }
        result.push({
          ...item,
          navName,
          folderPath: getFolderPath(item),
          statusKey: item.rag_info?.status || 'NONE',
        });
      });
    });

    const order: Record<LearningDoc['statusKey'], number> = {
      [ConstsNodeRagInfoStatus.NodeRagStatusFailed]: 0,
      [ConstsNodeRagInfoStatus.NodeRagStatusRunning]: 1,
      [ConstsNodeRagInfoStatus.NodeRagStatusReindexing]: 2,
      [ConstsNodeRagInfoStatus.NodeRagStatusPending]: 3,
      NONE: 4,
      [ConstsNodeRagInfoStatus.NodeRagStatusSucceeded]: 5,
    };

    return result.sort((a, b) => {
      const statusDiff =
        (order[a.statusKey] ?? 99) - (order[b.statusKey] ?? 99);
      if (statusDiff !== 0) return statusDiff;
      return dayjs(b.updated_at).valueOf() - dayjs(a.updated_at).valueOf();
    });
  }, [groups]);

  const counts = useMemo(() => {
    const succeeded = docs.filter(
      item => item.statusKey === ConstsNodeRagInfoStatus.NodeRagStatusSucceeded,
    ).length;
    const failed = docs.filter(
      item => item.statusKey === ConstsNodeRagInfoStatus.NodeRagStatusFailed,
    ).length;
    const running = docs.filter(item => isRunningStatus(item.statusKey)).length;
    return {
      total: docs.length,
      attention: docs.length - succeeded,
      running,
      failed,
      succeeded,
    };
  }, [docs]);

  const visibleDocs = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return docs.filter(item => {
      if (filter === 'attention') {
        if (item.statusKey === ConstsNodeRagInfoStatus.NodeRagStatusSucceeded) {
          return false;
        }
      } else if (filter === 'running') {
        if (!isRunningStatus(item.statusKey)) return false;
      } else if (filter === 'failed') {
        if (item.statusKey !== ConstsNodeRagInfoStatus.NodeRagStatusFailed) {
          return false;
        }
      } else if (filter === 'succeeded') {
        if (item.statusKey !== ConstsNodeRagInfoStatus.NodeRagStatusSucceeded) {
          return false;
        }
      }

      if (!normalizedKeyword) return true;
      return `${item.name || ''} ${item.folderPath || ''} ${
        item.rag_info?.message || ''
      }`
        .toLowerCase()
        .includes(normalizedKeyword);
    });
  }, [docs, filter, keyword]);

  const totalPages = Math.max(1, Math.ceil(visibleDocs.length / pageSize));
  const pagedDocs = visibleDocs.slice((page - 1) * pageSize, page * pageSize);
  const shouldAutoRefresh = docs.some(item => isRunningStatus(item.statusKey));
  const pendingDocs = docs.filter(
    item =>
      item.statusKey === 'NONE' ||
      item.statusKey === ConstsNodeRagInfoStatus.NodeRagStatusPending ||
      item.statusKey === ConstsNodeRagInfoStatus.NodeRagStatusFailed,
  );

  const submitRestudy = (items: LearningDoc[], emptyMessage: string) => {
    if (!kb_id) return;
    const ids = Array.from(
      new Set(items.map(item => item.id).filter(Boolean) as string[]),
    );
    if (ids.length === 0) {
      message.warning(emptyMessage);
      return;
    }
    setSubmitting(true);
    postApiV1NodeRestudy({
      kb_id,
      node_ids: ids,
    })
      .then(() => {
        message.success(`已提交 ${ids.length} 个文档，后台正在学习/重建索引`);
        refresh();
      })
      .finally(() => setSubmitting(false));
  };

  useEffect(() => {
    if (!open) return;
    setPage(1);
    refresh();
  }, [open, refresh]);

  useEffect(() => {
    if (!open || !shouldAutoRefresh) return;
    const timer = window.setInterval(() => {
      refresh();
    }, 5000);
    return () => window.clearInterval(timer);
  }, [open, refresh, shouldAutoRefresh]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  const filterOptions: Array<{
    key: FilterKey;
    label: string;
    count: number;
  }> = [
    { key: 'attention', label: '待处理', count: counts.attention },
    { key: 'running', label: '进行中', count: counts.running },
    { key: 'failed', label: '失败', count: counts.failed },
    { key: 'succeeded', label: '成功', count: counts.succeeded },
    { key: 'all', label: '全部', count: counts.total },
  ];

  return (
    <Modal
      title='学习进度'
      open={open}
      width={920}
      footer={null}
      onCancel={onClose}
    >
      <Stack gap={2}>
        <Stack direction='row' gap={1.5}>
          {[
            { label: '待处理', value: counts.attention, color: 'warning.main' },
            { label: '进行中', value: counts.running, color: 'info.main' },
            { label: '失败', value: counts.failed, color: 'error.main' },
            { label: '成功', value: counts.succeeded, color: 'success.main' },
          ].map(item => (
            <Box
              key={item.label}
              sx={{
                flex: 1,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 2,
                px: 1.5,
                py: 1.25,
                bgcolor: 'background.paper',
              }}
            >
              <Typography fontSize={12} color='text.secondary'>
                {item.label}
              </Typography>
              <Typography fontSize={22} fontWeight={700} color={item.color}>
                {item.value}
              </Typography>
            </Box>
          ))}
        </Stack>

        <Stack direction='row' alignItems='center' gap={1}>
          {filterOptions.map(item => (
            <Button
              key={item.key}
              size='small'
              variant={filter === item.key ? 'contained' : 'outlined'}
              onClick={() => {
                setFilter(item.key);
                setPage(1);
              }}
            >
              {item.label} {item.count}
            </Button>
          ))}
          <Box sx={{ flex: 1 }} />
          <TextField
            size='small'
            placeholder='搜索文档 / 目录 / 原因'
            value={keyword}
            onChange={event => {
              setKeyword(event.target.value);
              setPage(1);
            }}
            sx={{ width: 240 }}
          />
          <Button size='small' onClick={refresh}>
            刷新
          </Button>
          <Button
            size='small'
            variant='outlined'
            disabled={submitting || pendingDocs.length === 0}
            onClick={() =>
              submitRestudy(pendingDocs, '暂无需要学习的待处理文档')
            }
          >
            学习待处理 {pendingDocs.length}
          </Button>
          <Button
            size='small'
            color='primary'
            variant='contained'
            disabled={submitting || visibleDocs.length === 0}
            onClick={() =>
              submitRestudy(visibleDocs, '当前筛选结果没有可重建的文档')
            }
          >
            重建当前结果
          </Button>
        </Stack>

        <Stack
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 2,
            overflow: 'hidden',
          }}
        >
          <Stack
            direction='row'
            alignItems='center'
            sx={{
              px: 1.5,
              py: 1,
              bgcolor: 'background.paper3',
              color: 'text.secondary',
              fontSize: 12,
            }}
          >
            <Box sx={{ flex: 1.2 }}>文档</Box>
            <Box sx={{ flex: 1 }}>所在目录</Box>
            <Box sx={{ width: 96 }}>状态</Box>
            <Box sx={{ width: 132 }}>更新时间</Box>
            <Box sx={{ flex: 1.4 }}>进展 / 失败原因</Box>
          </Stack>
          <Divider />
          <Stack sx={{ minHeight: 360, maxHeight: 420, overflowY: 'auto' }}>
            {pagedDocs.length === 0 ? (
              <Stack
                alignItems='center'
                justifyContent='center'
                sx={{ height: 220, color: 'text.secondary' }}
              >
                暂无符合条件的文档
              </Stack>
            ) : (
              pagedDocs.map(item => {
                const statusMeta = getRagStatusMeta(item);
                const message = item.rag_info?.message || statusMeta?.message;
                return (
                  <Stack
                    key={item.id}
                    direction='row'
                    alignItems='center'
                    gap={1}
                    sx={{
                      px: 1.5,
                      py: 1.15,
                      borderBottom: '1px solid',
                      borderColor: 'divider',
                      '&:last-of-type': { borderBottom: 0 },
                    }}
                  >
                    <Tooltip title={item.name || ''}>
                      <Box
                        sx={{
                          flex: 1.2,
                          minWidth: 0,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                          fontSize: 13,
                        }}
                      >
                        {item.name || '未命名'}
                      </Box>
                    </Tooltip>
                    <Tooltip title={item.folderPath || ''}>
                      <Box
                        sx={{
                          flex: 1,
                          minWidth: 0,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                          color: 'text.secondary',
                          fontSize: 12,
                        }}
                      >
                        {item.folderPath || '-'}
                      </Box>
                    </Tooltip>
                    <Box sx={{ width: 96 }}>
                      {statusMeta && (
                        <Chip
                          size='small'
                          variant='outlined'
                          label={statusMeta.label}
                          color={statusMeta.color}
                        />
                      )}
                    </Box>
                    <Box
                      sx={{ width: 132, color: 'text.secondary', fontSize: 12 }}
                    >
                      {item.updated_at ? dayjs(item.updated_at).fromNow() : '-'}
                    </Box>
                    <Tooltip title={message || ''}>
                      <Box
                        sx={{
                          flex: 1.4,
                          minWidth: 0,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                          color:
                            item.statusKey ===
                            ConstsNodeRagInfoStatus.NodeRagStatusFailed
                              ? 'error.main'
                              : 'text.secondary',
                          fontSize: 12,
                        }}
                      >
                        {message ||
                          (isRunningStatus(item.statusKey)
                            ? '等待后台更新状态'
                            : '-')}
                      </Box>
                    </Tooltip>
                  </Stack>
                );
              })
            )}
          </Stack>
        </Stack>

        <Stack
          direction='row'
          alignItems='center'
          justifyContent='space-between'
        >
          <Typography fontSize={12} color='text.secondary'>
            共 {visibleDocs.length} 个文档；进行中的任务会每 5 秒自动刷新
          </Typography>
          <Pagination
            size='small'
            page={page}
            count={totalPages}
            onChange={(_, value) => setPage(value)}
          />
        </Stack>
      </Stack>
    </Modal>
  );
};

export default LearningProgressPanel;
