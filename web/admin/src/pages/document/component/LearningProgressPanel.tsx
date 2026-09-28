import { Modal, message } from '@/components/ui';
import {
  getRagStatusMeta,
  hasAvailableRagIndex,
  isLearningTaskActive,
} from '@/pages/document/utils/ragStatus';
import {
  postApiV1NodeLearningCancel,
  postApiV1NodeRestudy,
} from '@/request/Node';
import {
  ConstsNodeLearningTaskStatus,
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

type FilterKey =
  | 'attention'
  | 'running'
  | 'failed'
  | 'unavailable'
  | 'answerable'
  | 'all';

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
  taskStatus?: ConstsNodeLearningTaskStatus;
  available: boolean;
};

const pageSize = 10;

const isRagRunningStatus = (status: LearningDoc['statusKey']) =>
  status === ConstsNodeRagInfoStatus.NodeRagStatusPending ||
  status === ConstsNodeRagInfoStatus.NodeRagStatusRunning ||
  status === ConstsNodeRagInfoStatus.NodeRagStatusReindexing;

const isDocRunning = (item: LearningDoc) =>
  isLearningTaskActive(item.taskStatus) ||
  (!item.available && isRagRunningStatus(item.statusKey));

const isDocFailed = (item: LearningDoc) =>
  item.taskStatus === ConstsNodeLearningTaskStatus.NodeLearningTaskFailed ||
  (!item.available &&
    item.statusKey === ConstsNodeRagInfoStatus.NodeRagStatusFailed);

const needsAttention = (item: LearningDoc) =>
  !item.available ||
  item.taskStatus === ConstsNodeLearningTaskStatus.NodeLearningTaskFailed ||
  item.taskStatus === ConstsNodeLearningTaskStatus.NodeLearningTaskCanceled;

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
          taskStatus: item.rag_info?.task_status,
          available: hasAvailableRagIndex(item),
        });
      });
    });

    return result.sort((a, b) => {
      const getOrder = (item: LearningDoc) => {
        if (isDocFailed(item)) return 0;
        if (isDocRunning(item)) return 1;
        if (!item.available) return 2;
        return 3;
      };
      const statusDiff = getOrder(a) - getOrder(b);
      if (statusDiff !== 0) return statusDiff;
      return dayjs(b.updated_at).valueOf() - dayjs(a.updated_at).valueOf();
    });
  }, [groups]);

  const counts = useMemo(() => {
    const answerable = docs.filter(item => item.available).length;
    const unavailable = docs.filter(item => !item.available).length;
    const failed = docs.filter(item => isDocFailed(item)).length;
    const running = docs.filter(item => isDocRunning(item)).length;
    return {
      total: docs.length,
      attention: docs.filter(item => needsAttention(item)).length,
      running,
      failed,
      unavailable,
      answerable,
    };
  }, [docs]);

  const visibleDocs = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return docs.filter(item => {
      if (filter === 'attention') {
        if (!needsAttention(item)) {
          return false;
        }
      } else if (filter === 'running') {
        if (!isDocRunning(item)) return false;
      } else if (filter === 'failed') {
        if (!isDocFailed(item)) return false;
      } else if (filter === 'unavailable') {
        if (item.available) return false;
      } else if (filter === 'answerable') {
        if (!item.available) return false;
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
  const shouldAutoRefresh = docs.some(item => isDocRunning(item));
  const pendingDocs = docs.filter(
    item =>
      !isDocRunning(item) &&
      (!item.available ||
        item.taskStatus ===
          ConstsNodeLearningTaskStatus.NodeLearningTaskFailed ||
        item.taskStatus ===
          ConstsNodeLearningTaskStatus.NodeLearningTaskCanceled),
  );
  const runningVisibleDocs = visibleDocs.filter(item => isDocRunning(item));

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

  const cancelLearning = (items: LearningDoc[]) => {
    if (!kb_id) return;
    const ids = Array.from(
      new Set(items.map(item => item.id).filter(Boolean) as string[]),
    );
    if (ids.length === 0) {
      message.warning('当前筛选结果没有进行中的学习任务');
      return;
    }
    setSubmitting(true);
    postApiV1NodeLearningCancel({
      kb_id,
      node_ids: ids,
    })
      .then(res => {
        const count = res.count ?? ids.length;
        message.success(`已停止 ${count} 个学习/重建任务`);
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
    { key: 'attention', label: '需处理', count: counts.attention },
    { key: 'running', label: '进行中', count: counts.running },
    { key: 'failed', label: '本次异常', count: counts.failed },
    { key: 'unavailable', label: '不可问答', count: counts.unavailable },
    { key: 'answerable', label: '可问答', count: counts.answerable },
    { key: 'all', label: '全部', count: counts.total },
  ];

  const confirmRebuildCurrentResult = () => {
    const currentFilterLabel =
      filterOptions.find(item => item.key === filter)?.label || '当前';
    Modal.confirm({
      title: '确认重建当前结果？',
      content: `将对“${currentFilterLabel}”筛选结果中的 ${visibleDocs.length} 个文档提交后台重建任务。已有可用索引会保留，重建期间仍可继续问答。`,
      okText: '确认重建',
      cancelText: '取消',
      onOk: () => {
        submitRestudy(visibleDocs, '当前筛选结果没有可重建的文档');
      },
    });
  };

  return (
    <Modal
      title='索引状态'
      open={open}
      width={920}
      footer={null}
      onCancel={onClose}
    >
      <Stack gap={2}>
        <Stack direction='row' gap={1.5}>
          {[
            {
              label: '可问答',
              value: counts.answerable,
              color: 'success.main',
            },
            {
              label: '不可问答',
              value: counts.unavailable,
              color: 'error.main',
            },
            { label: '进行中', value: counts.running, color: 'info.main' },
            { label: '本次异常', value: counts.failed, color: 'warning.main' },
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

        <Typography fontSize={12} color='text.secondary'>
          可问答表示当前已有可用索引；本次异常表示最近一次学习/重建任务失败，通常不影响已有旧索引继续问答。
        </Typography>

        <Stack direction='row' alignItems='center' gap={1} flexWrap='wrap'>
          <Stack
            direction='row'
            alignItems='center'
            gap={0.75}
            flexWrap='wrap'
            sx={{ flex: 1, minWidth: 300 }}
          >
            {filterOptions.map(item => (
              <Button
                key={item.key}
                size='small'
                variant={filter === item.key ? 'contained' : 'outlined'}
                sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 72 }}
                onClick={() => {
                  setFilter(item.key);
                  setPage(1);
                }}
              >
                {item.label} {item.count}
              </Button>
            ))}
          </Stack>

          <Stack
            direction='row'
            alignItems='center'
            justifyContent='flex-end'
            gap={1}
            sx={{ flexShrink: 0, ml: 'auto' }}
          >
            <TextField
              size='small'
              placeholder='搜索文档 / 目录 / 原因'
              value={keyword}
              onChange={event => {
                setKeyword(event.target.value);
                setPage(1);
              }}
              sx={{ width: 220, flexShrink: 0 }}
            />
            <Button
              size='small'
              sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 64 }}
              onClick={refresh}
            >
              刷新
            </Button>
            <Button
              size='small'
              color='warning'
              variant='outlined'
              disabled={submitting || runningVisibleDocs.length === 0}
              sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 112 }}
              onClick={() => cancelLearning(runningVisibleDocs)}
            >
              停止进行中 {runningVisibleDocs.length}
            </Button>
            <Button
              size='small'
              variant='outlined'
              disabled={submitting || pendingDocs.length === 0}
              sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 132 }}
              onClick={() => submitRestudy(pendingDocs, '暂无需要学习的文档')}
            >
              学习需处理 {pendingDocs.length}
            </Button>
            <Button
              size='small'
              color='primary'
              variant='contained'
              disabled={submitting || visibleDocs.length === 0}
              sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 120 }}
              onClick={confirmRebuildCurrentResult}
            >
              重建当前结果
            </Button>
          </Stack>
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
                const message =
                  item.rag_info?.task_message ||
                  item.rag_info?.message ||
                  statusMeta?.message;
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
                          color: isDocFailed(item)
                            ? 'error.main'
                            : 'text.secondary',
                          fontSize: 12,
                        }}
                      >
                        {message ||
                          (isDocRunning(item) ? '等待后台更新状态' : '-')}
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
