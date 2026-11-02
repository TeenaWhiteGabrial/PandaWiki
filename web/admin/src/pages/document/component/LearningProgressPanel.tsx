import LearningProgressDetail from './LearningProgressDetail';
import {
  learningStageLabels,
  formatLearningDuration,
  stageElapsedMs,
} from '../utils/learningProgress';
import { Modal, message } from '@/components/ui';
import {
  cancelPendingLearning,
  getLearningTaskTargets,
  getLearningTaskDetail,
  type LearningFilter,
  type LearningTaskItem,
  type LearningTaskQuery,
} from '@/request/nodeLearning';
import { useLearningTasks } from '../hooks/useLearningTasks';
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
} from '@/request/types';
import { useAppSelector } from '@/store';
import {
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Divider,
  Pagination,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';

type FilterKey = Exclude<LearningFilter, 'needs_learning'>;
interface LearningProgressPanelProps {
  open: boolean;
  onClose: () => void;
  refresh: () => void;
}

type LearningDoc = DomainNodeListItemResp & {
  folderPath: string;
  statusKey: ConstsNodeRagInfoStatus | 'NONE';
  taskStatus?: ConstsNodeLearningTaskStatus;
  available: boolean;
};
const pageSize = 10;
const asLearningDoc = (item: LearningTaskItem): LearningDoc => ({
  ...item,
  folderPath: item.folder_path,
  statusKey: item.rag_info?.status || 'NONE',
  taskStatus: item.rag_info?.task_status,
  available: hasAvailableRagIndex(item),
});
const isDocRunning = (item: LearningDoc) =>
  item.taskStatus
    ? isLearningTaskActive(item.taskStatus)
    : !item.available &&
      [
        ConstsNodeRagInfoStatus.NodeRagStatusPending,
        ConstsNodeRagInfoStatus.NodeRagStatusRunning,
        ConstsNodeRagInfoStatus.NodeRagStatusReindexing,
      ].includes(item.statusKey as ConstsNodeRagInfoStatus);
const isDocFailed = (item: LearningDoc) =>
  item.taskStatus === ConstsNodeLearningTaskStatus.NodeLearningTaskFailed ||
  (!item.available &&
    item.statusKey === ConstsNodeRagInfoStatus.NodeRagStatusFailed);

const LearningProgressPanel = ({
  open,
  onClose,
  refresh,
}: LearningProgressPanelProps) => {
  const { kb_id, user } = useAppSelector(state => state.config);
  const identity = `${user?.id || ''}:${kb_id || ''}`;
  const [detailId, setDetailId] = useState<string>();
  const [filter, setFilter] = useState<FilterKey>('attention');
  const [keyword, setKeyword] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [submitting, setSubmitting] = useState(false);
  const [selectionMode, setSelectionMode] = useState(false);
  const [selected, setSelected] = useState<Map<string, LearningDoc>>(new Map());
  const [pendingCancelMessage, setPendingCancelMessage] = useState('');
  const [detail, setDetail] = useState<{
    identity: string;
    id: string;
    doc: LearningTaskItem;
  }>();
  const [detailError, setDetailError] = useState(false);
  const [detailReload, setDetailReload] = useState(0);
  useEffect(() => {
    const timer = setTimeout(() => setSearch(keyword.trim()), 300);
    return () => clearTimeout(timer);
  }, [keyword]);
  const query = {
    kb_id: kb_id || '',
    filter,
    keyword: search,
    page,
    page_size: pageSize,
  };
  const {
    data,
    counts: loadedCounts,
    initialLoading,
    loadError,
    summaryError,
    fetching,
    reload,
    invalidate,
  } = useLearningTasks(open, user?.id || '', query);
  const counts = loadedCounts || {
    total: 0,
    attention: 0,
    running: 0,
    failed: 0,
    unavailable: 0,
    answerable: 0,
    needs_learning: 0,
  };
  const dataUnavailable = !data;
  const searchPending = keyword.trim() !== search;
  const actionUnavailable = dataUnavailable || submitting || searchPending;
  const docs = useMemo(() => (data?.list || []).map(asLearningDoc), [data]);
  const pagedDocs = docs;
  const total = data?.total || 0;
  const runningVisibleCount = data?.running || 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const selectedIdSet = new Set(selected.keys());
  const selectedDocs = Array.from(selected.values());
  const selectedRunningDocs = selectedDocs.filter(isDocRunning);
  const pagedSelectableIds = docs
    .map(item => item.id)
    .filter(Boolean) as string[];
  const selectedPagedCount = pagedSelectableIds.filter(id =>
    selected.has(id),
  ).length;
  const isPagedAllSelected =
    pagedSelectableIds.length > 0 &&
    selectedPagedCount === pagedSelectableIds.length;
  const isPagedIndeterminate =
    selectedPagedCount > 0 && selectedPagedCount < pagedSelectableIds.length;

  useEffect(() => {
    setSelected(new Map());
    setDetailId(undefined);
    setSelectionMode(false);
    setPendingCancelMessage('');
    setPage(1);
  }, [identity]);
  useEffect(() => {
    if (data && page !== data.page) setPage(data.page);
  }, [data, page]);
  useEffect(() => {
    if (open) return;
    setDetailId(undefined);
    setSelected(new Map());
    setSelectionMode(false);
  }, [open]);
  // Keep selections across pages, but refresh their status when their page is read.
  useEffect(() => {
    setSelected(previous => {
      const next = new Map(previous);
      for (const item of docs)
        if (item.id && next.has(item.id)) next.set(item.id, item);
      return next;
    });
  }, [docs]);

  useEffect(() => {
    if (!open || !detailId || !kb_id) return;
    const controller = new AbortController();
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    setDetailError(false);
    const load = async () => {
      try {
        const doc = await getLearningTaskDetail(
          kb_id,
          detailId,
          controller.signal,
        );
        if (stopped) return;
        setDetail({ identity, id: detailId, doc });
        setDetailError(false);
        if (isDocRunning(asLearningDoc(doc))) timer = setTimeout(load, 5000);
      } catch (error) {
        if (!stopped) {
          if ([401, 403].includes((error as { status?: number })?.status || 0))
            setDetail(undefined);
          setDetailError(true);
        }
      }
    };
    void load();
    return () => {
      stopped = true;
      controller.abort();
      clearTimeout(timer);
    };
  }, [open, detailId, kb_id, identity, detailReload]);
  const detailDoc =
    detail?.identity === identity && detail.id === detailId
      ? detail.doc
      : undefined;

  const toggleSelected = (id?: string) => {
    const doc = docs.find(item => item.id === id);
    if (!id || !doc) return;
    setSelected(previous => {
      const next = new Map(previous);
      if (next.has(id)) next.delete(id);
      else next.set(id, doc);
      return next;
    });
  };
  const toggleCurrentPageSelected = () =>
    setSelected(previous => {
      const next = new Map(previous);
      docs.forEach(doc => {
        if (!doc.id) return;
        if (isPagedAllSelected) next.delete(doc.id);
        else next.set(doc.id, doc);
      });
      return next;
    });
  const exitSelectionMode = () => {
    setSelectionMode(false);
    setSelected(new Map());
  };
  const afterMutation = () => {
    invalidate();
    refresh();
  };
  const targets = async (override: Partial<LearningTaskQuery> = {}) => {
    const result = await getLearningTaskTargets({ ...query, ...override });
    return result.ids || [];
  };
  const submitRestudy = async (
    resolve: () => Promise<string[]>,
    emptyMessage: string,
    priority = 10,
  ) => {
    if (!kb_id) return;
    setSubmitting(true);
    try {
      const ids = [...new Set(await resolve())];
      if (!ids.length) {
        message.warning(emptyMessage);
        return;
      }
      await postApiV1NodeRestudy({ kb_id, node_ids: ids, priority });
      message.success(`已提交 ${ids.length} 个文档，后台正在学习/重建索引`);
      afterMutation();
    } catch {
      // The shared request client displays the server error.
    } finally {
      setSubmitting(false);
    }
  };
  const cancelLearning = async (resolve: () => Promise<string[]>) => {
    if (!kb_id) return;
    setSubmitting(true);
    try {
      const ids = [...new Set(await resolve())];
      if (!ids.length) {
        message.warning('当前筛选结果没有进行中的学习任务');
        return;
      }
      const result = await postApiV1NodeLearningCancel({
        kb_id,
        node_ids: ids,
      });
      message.success(
        `已取消 ${result.count ?? 0} 个未提交任务；已提交或正在提交的任务继续处理`,
      );
      afterMutation();
    } catch {
      // The shared request client displays the server error.
    } finally {
      setSubmitting(false);
    }
  };
  const confirmCancelPending = () => {
    if (!kb_id) return;
    const requestedKB = kb_id;
    Modal.confirm({
      title: '取消未提交任务？',
      content:
        '将取消当前知识库所有尚未提交的学习和重建任务（包含旧版静默全量重建，不受当前筛选条件限制）。已提交或已开始提交给 RAGLite 的任务继续处理，不删除原文件和已有索引。取消标记会保留，重启 Consumer 后也会跳过这些旧任务。',
      okText: '取消未提交任务',
      cancelText: '返回',
      onOk: async () => {
        setSubmitting(true);
        try {
          const result = await cancelPendingLearning(requestedKB);
          setPendingCancelMessage(
            `已取消 ${result.count ?? 0} 个已登记的未提交任务；旧版队列也已设置取消标记，恢复 Consumer 后将跳过旧任务。已提交或正在提交的任务继续处理。`,
          );
          message.success('取消标记已保存');
          afterMutation();
        } finally {
          setSubmitting(false);
        }
      },
    });
  };
  const confirmRebuildSelected = () => {
    const ids = selectedDocs.map(item => item.id!).filter(Boolean);
    if (!ids.length) {
      message.warning('请先勾选需要重建索引的文档');
      return;
    }
    Modal.confirm({
      title: '确认重建所选文档？',
      content: `将对已选中的 ${ids.length} 个文档提交后台重建任务。已有可用索引会保留，重建期间仍可继续问答。`,
      okText: '确认重建',
      cancelText: '取消',
      onOk: () =>
        submitRestudy(async () => ids, '请先勾选需要重建索引的文档', 100),
    });
  };

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
      content: `将对“${currentFilterLabel}”筛选结果中的 ${total} 个文档提交后台重建任务。已有可用索引会保留，重建期间仍可继续问答。`,
      okText: '确认重建',
      cancelText: '取消',
      onOk: () =>
        submitRestudy(() => targets(), '当前筛选结果没有可重建的文档'),
    });
  };

  return (
    <Modal
      title='后台任务'
      open={open}
      width={1440}
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
            {
              label: '本次异常',
              value: counts.failed,
              color: 'warning.main',
            },
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
                {loadedCounts ? item.value : '—'}
              </Typography>
            </Box>
          ))}
        </Stack>

        <Typography fontSize={12} color='text.secondary'>
          可问答表示当前已有可用索引；本次异常表示最近一次学习/重建任务失败，通常不影响已有旧索引继续问答。
        </Typography>

        {summaryError && (
          <Typography role='status' fontSize={12} color='warning.main'>
            分类统计刷新失败，列表仍可使用。
            <Button size='small' onClick={reload}>
              重试
            </Button>
          </Typography>
        )}
        {fetching && !initialLoading && (
          <Stack direction='row' alignItems='center' gap={1} role='status'>
            <CircularProgress size={14} />
            <Typography fontSize={12} color='text.secondary'>
              正在更新任务数据…
            </Typography>
          </Stack>
        )}
        {loadError === 'refresh' && (
          <Typography role='status' fontSize={12} color='warning.main'>
            刷新失败，当前保留上次加载的数据，稍后将自动重试。
          </Typography>
        )}

        {pendingCancelMessage && (
          <Typography role='status' fontSize={12} color='success.main'>
            {pendingCancelMessage}
          </Typography>
        )}

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
                disabled={actionUnavailable}
                variant={filter === item.key ? 'contained' : 'outlined'}
                sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 72 }}
                onClick={() => {
                  setFilter(item.key);
                  setPage(1);
                }}
              >
                {item.label} {loadedCounts ? item.count : '—'}
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
              disabled={dataUnavailable || submitting}
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
              disabled={fetching}
              onClick={() => {
                reload();
              }}
            >
              刷新
            </Button>
            <Button
              size='small'
              color='warning'
              variant='outlined'
              disabled={actionUnavailable || runningVisibleCount === 0}
              sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 112 }}
              onClick={() =>
                cancelLearning(() => targets({ only_running: true }))
              }
            >
              取消筛选内未提交
            </Button>
            {user.role === 'admin' && (
              <Button
                size='small'
                color='warning'
                variant='outlined'
                disabled={submitting}
                sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
                onClick={confirmCancelPending}
              >
                取消未提交任务
              </Button>
            )}
            <Button
              size='small'
              variant='outlined'
              disabled={
                actionUnavailable ||
                !loadedCounts ||
                counts.needs_learning === 0
              }
              sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 132 }}
              onClick={() =>
                submitRestudy(
                  () => targets({ filter: 'needs_learning', keyword: '' }),
                  '暂无需要学习的文档',
                )
              }
            >
              学习需处理 {loadedCounts ? counts.needs_learning : '—'}
            </Button>
            {!selectionMode && (
              <Button
                size='small'
                variant='outlined'
                disabled={actionUnavailable || total === 0}
                sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 132 }}
                onClick={() => {
                  setSelectionMode(true);
                  setSelected(new Map());
                }}
              >
                重建指定文档
              </Button>
            )}
            <Button
              size='small'
              color='primary'
              variant='contained'
              disabled={actionUnavailable || total === 0}
              sx={{ flexShrink: 0, whiteSpace: 'nowrap', minWidth: 120 }}
              onClick={confirmRebuildCurrentResult}
            >
              重建当前结果
            </Button>
          </Stack>
        </Stack>

        {selectionMode && !dataUnavailable && (
          <Stack
            direction='row'
            alignItems='center'
            justifyContent='space-between'
            gap={1}
            sx={{
              px: 1.25,
              py: 1,
              borderRadius: 2,
              bgcolor: 'action.hover',
              border: '1px solid',
              borderColor: 'divider',
            }}
          >
            <Typography fontSize={13} color='primary.main'>
              请选择需要重建索引的文档，已选择 {selectedDocs.length} 个
              {selectedRunningDocs.length > 0
                ? `，其中 ${selectedRunningDocs.length} 个正在进行`
                : ''}
            </Typography>
            <Stack direction='row' alignItems='center' gap={1}>
              <Button
                size='small'
                variant='contained'
                disabled={actionUnavailable || selectedDocs.length === 0}
                sx={{ whiteSpace: 'nowrap', minWidth: 104 }}
                onClick={confirmRebuildSelected}
              >
                重建所选
              </Button>
              <Button
                size='small'
                color='warning'
                variant='outlined'
                disabled={actionUnavailable || selectedRunningDocs.length === 0}
                sx={{ whiteSpace: 'nowrap', minWidth: 104 }}
                onClick={() =>
                  cancelLearning(async () =>
                    selectedDocs.map(item => item.id!).filter(Boolean),
                  )
                }
              >
                取消所选未提交
              </Button>
              <Button
                size='small'
                disabled={submitting}
                sx={{ whiteSpace: 'nowrap', minWidth: 72 }}
                onClick={() => setSelected(new Map())}
              >
                清空
              </Button>
              <Button
                size='small'
                disabled={submitting}
                sx={{ whiteSpace: 'nowrap', minWidth: 72 }}
                onClick={exitSelectionMode}
              >
                退出
              </Button>
            </Stack>
          </Stack>
        )}

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
            {selectionMode && (
              <Box sx={{ width: 42 }}>
                <Checkbox
                  size='small'
                  checked={isPagedAllSelected}
                  indeterminate={isPagedIndeterminate}
                  disabled={dataUnavailable || pagedSelectableIds.length === 0}
                  sx={{ p: 0.25 }}
                  onChange={toggleCurrentPageSelected}
                />
              </Box>
            )}
            <Box sx={{ flex: 1, minWidth: 0 }}>文档</Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>所在目录</Box>
            <Box sx={{ width: 96 }}>状态</Box>
            <Box sx={{ width: 132 }}>进度更新时间</Box>
            <Box sx={{ flex: 2, minWidth: 0 }}>进展 / 失败原因</Box>
          </Stack>
          <Divider />
          <Stack
            aria-busy={initialLoading}
            sx={{ minHeight: 360, maxHeight: 420, overflowY: 'auto' }}
          >
            {initialLoading ? (
              <Stack
                role='status'
                alignItems='center'
                justifyContent='center'
                gap={1.5}
                sx={{ minHeight: 360, color: 'text.secondary' }}
              >
                <CircularProgress size={32} aria-label='正在加载任务数据' />
                <Typography fontSize={13}>正在加载任务数据…</Typography>
              </Stack>
            ) : loadError === 'initial' ? (
              <Stack
                role='alert'
                alignItems='center'
                justifyContent='center'
                gap={1.5}
                sx={{ minHeight: 360, color: 'text.secondary' }}
              >
                <Typography fontSize={13}>
                  任务数据加载失败，请重试。
                </Typography>
                <Button size='small' onClick={reload}>
                  重新加载
                </Button>
              </Stack>
            ) : pagedDocs.length === 0 ? (
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
                    alignItems='flex-start'
                    gap={1}
                    sx={{
                      px: 1.5,
                      py: 1.15,
                      bgcolor:
                        selectionMode && item.id && selectedIdSet.has(item.id)
                          ? 'action.hover'
                          : 'transparent',
                      borderBottom: '1px solid',
                      borderColor: 'divider',
                      '&:last-of-type': { borderBottom: 0 },
                    }}
                  >
                    {selectionMode && (
                      <Box sx={{ width: 42 }}>
                        <Checkbox
                          size='small'
                          checked={!!item.id && selectedIdSet.has(item.id)}
                          sx={{ p: 0.25 }}
                          onChange={() => toggleSelected(item.id)}
                        />
                      </Box>
                    )}
                    <Tooltip title={item.name || ''}>
                      <Box
                        sx={{
                          flex: 1,
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
                      sx={{
                        width: 132,
                        color: 'text.secondary',
                        fontSize: 12,
                      }}
                    >
                      {item.rag_info?.learning_progress?.stages?.length
                        ? dayjs(
                            item.rag_info.learning_progress.last_progress_at,
                          ).fromNow()
                        : '-'}
                    </Box>
                    <Stack sx={{ flex: 2, minWidth: 0 }} gap={0.5}>
                      {item.rag_info?.learning_progress?.extraction && (
                        <Typography
                          fontSize={12}
                          color={
                            item.rag_info.learning_progress.extraction.scope ===
                            'full'
                              ? 'text.secondary'
                              : 'warning.main'
                          }
                        >
                          索引内容：
                          {
                            {
                              full: '完整正文',
                              partial: '部分正文',
                              metadata: '仅名称和元信息',
                              unknown: '正文覆盖待核验',
                            }[item.rag_info.learning_progress.extraction.scope]
                          }
                        </Typography>
                      )}
                      {item.rag_info?.learning_progress?.stages?.at(-1) &&
                        (() => {
                          const stage =
                            item.rag_info.learning_progress.stages.at(-1)!;
                          return (
                            <Typography fontSize={12} fontWeight={600}>
                              {learningStageLabels[stage.key] || stage.key} ·{' '}
                              {formatLearningDuration(stageElapsedMs(stage))}
                            </Typography>
                          );
                        })()}
                      <Typography
                        fontSize={12}
                        color={
                          isDocFailed(item) ? 'error.main' : 'text.secondary'
                        }
                        sx={{ overflowWrap: 'anywhere' }}
                      >
                        {item.rag_info?.learning_progress?.check_error ||
                          item.rag_info?.learning_progress?.stages?.at(-1)
                            ?.message ||
                          message ||
                          (isDocRunning(item) ? '暂无步骤记录' : '-')}
                      </Typography>
                      <Button
                        size='small'
                        sx={{ alignSelf: 'flex-start', p: 0, minWidth: 0 }}
                        onClick={() => {
                          setDetail(undefined);
                          setDetailError(false);
                          setDetailId(item.id);
                        }}
                      >
                        步骤与耗时
                      </Button>
                    </Stack>
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
            共 {dataUnavailable ? '—' : total} 个文档
            {selectionMode ? `，已选择 ${selectedDocs.length} 个` : ''}
            ；有进行中任务时每 5 秒刷新，无进行中任务时每 30 秒刷新
          </Typography>
          <Pagination
            size='small'
            disabled={actionUnavailable}
            page={page}
            count={totalPages}
            onChange={(_, value) => setPage(value)}
          />
        </Stack>
      </Stack>
      <LearningProgressDetail
        doc={open ? detailDoc : undefined}
        open={open && !!detailId}
        loading={!detailDoc && !detailError}
        loadError={detailError}
        onRetry={() => setDetailReload(value => value + 1)}
        onClose={() => setDetailId(undefined)}
      />
    </Modal>
  );
};

export default LearningProgressPanel;
