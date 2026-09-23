import {
  FolderDownloadTask,
  ZipUploadTask,
  getApiV1FileFolderDownload,
  getApiV1FileFolderDownloads,
  getApiV1FileUploadZipTasks,
  postApiV1FileFolderDownloadRetry,
  postApiV1FileUploadZipTaskCancel,
  postApiV1FileUploadZipTaskFail,
  postApiV1FileUploadZipTaskRetry,
} from '@/request/BatchImport';
import {
  canResumeZipUpload,
  getZipUploadedPartCount,
  uploadZipTaskParts,
} from '../utils/zipUploadResume';
import { message, Modal } from '@/components/ui';
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Pagination,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import DownloadOutlinedIcon from '@mui/icons-material/DownloadOutlined';
import RefreshIcon from '@mui/icons-material/Refresh';
import ReplayIcon from '@mui/icons-material/Replay';
import UploadFileOutlinedIcon from '@mui/icons-material/UploadFileOutlined';
import type { ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

interface Props {
  open: boolean;
  onClose: () => void;
  kbId?: string;
  onTaskSettled?: () => void;
}

type Tone = 'default' | 'info' | 'success' | 'warning' | 'error';

interface TaskView {
  phase: string;
  detail: string;
  percent: number;
  activeStep: number;
  indeterminate?: boolean;
  tone: Tone;
}

interface TaskListItem {
  id: string;
  kind: 'upload' | 'download';
  createdAt?: string;
  upload?: ZipUploadTask;
  download?: FolderDownloadTask;
}

const taskPageSize = 5;

const zipSteps = ['上传 ZIP', '解压建目录', '替换占位文件', '完成'];
const downloadSteps = ['创建任务', '压缩文件夹', '生成下载链接'];

const clampPercent = (value: number) =>
  Number.isFinite(value) ? Math.max(0, Math.min(100, Math.round(value))) : 0;

const formatSize = (size = 0) => {
  if (size >= 1024 ** 3) return `${(size / 1024 ** 3).toFixed(1)} GB`;
  if (size >= 1024 ** 2) return `${(size / 1024 ** 2).toFixed(1)} MB`;
  if (size >= 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${size} B`;
};

const statusChip = (tone: Tone, label: string) => (
  <Chip
    size='small'
    color={tone === 'default' ? 'default' : tone}
    variant={tone === 'success' ? 'filled' : 'outlined'}
    label={label}
    sx={{ height: 24, '& .MuiChip-label': { px: 1 } }}
  />
);

const buildZipTaskView = (
  task: ZipUploadTask,
  refreshingDirectory: boolean,
): TaskView => {
  const uploadPercent = task.size
    ? clampPercent(((task.uploaded_bytes || 0) * 100) / task.size)
    : 0;
  const totalFiles = task.total_files || 0;
  const processedFiles = task.processed_files || 0;
  const totalBytes = task.total_bytes || 0;
  const processedBytes = task.processed_bytes || 0;
  const uploadedPartCount = getZipUploadedPartCount(task);
  const partDetail = task.total_parts
    ? ` · 分片 ${uploadedPartCount}/${task.total_parts}`
    : '';
  const extractionPercent = totalFiles
    ? clampPercent((processedFiles * 100) / totalFiles)
    : totalBytes
      ? clampPercent((processedBytes * 100) / totalBytes)
      : 0;

  if (task.status === 'failed') {
    return {
      phase: '导入失败',
      detail:
        canResumeZipUpload(task) && uploadedPartCount < task.total_parts
          ? `上传未完成，已上传 ${formatSize(
              task.uploaded_bytes || 0,
            )} / ${formatSize(task.size)}${partDetail}，请选择同一个 ZIP 继续上传`
          : task.error_message || '任务执行失败，可点击重试',
      percent: Math.max(uploadPercent, extractionPercent),
      activeStep: Math.max(
        0,
        Math.min(
          2,
          totalFiles || totalBytes ? 1 : uploadPercent >= 100 ? 1 : 0,
        ),
      ),
      tone: 'error',
    };
  }

  if (task.status === 'interrupted') {
    return {
      phase: '上传中断',
      detail:
        task.error_message ||
        `长时间没有新的分片上传成功，已上传 ${formatSize(
          task.uploaded_bytes || 0,
        )} / ${formatSize(task.size)}${partDetail}，请选择同一个 ZIP 继续上传`,
      percent: uploadPercent,
      activeStep: 0,
      tone: 'warning',
    };
  }

  if (task.status === 'canceled') {
    return {
      phase: '已取消',
      detail: '任务已取消，未完成的 OSS 分片上传已终止',
      percent: uploadPercent,
      activeStep: 0,
      tone: 'default',
    };
  }

  if (task.status === 'completed') {
    return {
      phase: refreshingDirectory ? '正在刷新目录视图' : '导入完成',
      detail: refreshingDirectory
        ? '后台已完成解压和归档，正在刷新左侧目录结构'
        : totalFiles
          ? `已创建 ${totalFiles} 个文件，ZIP 占位文件已移除`
          : '已完成 ZIP 导入',
      percent: 100,
      activeStep: 3,
      tone: refreshingDirectory ? 'info' : 'success',
    };
  }

  if (task.status === 'processing') {
    if (!totalFiles) {
      return {
        phase: '正在扫描压缩包',
        detail: '上传已完成，正在读取 ZIP 文件结构并准备创建目录',
        percent: 100,
        activeStep: 1,
        indeterminate: true,
        tone: 'info',
      };
    }
    if (processedFiles >= totalFiles) {
      return {
        phase: '正在替换 ZIP 占位文件',
        detail: `已处理 ${processedFiles}/${totalFiles} 个文件，正在完成目录归档和页面刷新`,
        percent: 100,
        activeStep: 2,
        tone: 'info',
      };
    }
    return {
      phase: '正在解压并创建目录',
      detail: `已处理 ${processedFiles}/${totalFiles} 个文件 · ${formatSize(
        processedBytes,
      )} / ${formatSize(totalBytes)}`,
      percent: extractionPercent,
      activeStep: 1,
      tone: 'info',
    };
  }

  if (task.status === 'waiting') {
    return {
      phase: '等待上传',
      detail: '任务已创建，等待浏览器开始上传 ZIP 文件',
      percent: 0,
      activeStep: 0,
      tone: 'warning',
    };
  }

  return {
    phase: uploadPercent >= 100 ? '上传完成，等待服务端确认' : '正在上传到 OSS',
    detail: `${formatSize(task.uploaded_bytes || 0)} / ${formatSize(
      task.size,
    )} · ${uploadPercent}%${partDetail}`,
    percent: uploadPercent,
    activeStep: 0,
    tone: 'info',
  };
};

const buildDownloadTaskView = (task: FolderDownloadTask): TaskView => {
  const percent = task.total_bytes
    ? clampPercent(((task.processed_bytes || 0) * 100) / task.total_bytes)
    : task.total_files
      ? clampPercent(((task.processed_files || 0) * 100) / task.total_files)
      : 0;

  if (task.status === 'failed' || task.status === 'expired') {
    return {
      phase: task.status === 'expired' ? '下载链接已过期' : '压缩失败',
      detail: task.error_message || '任务执行失败，可点击重试',
      percent,
      activeStep: task.processed_files || task.processed_bytes ? 1 : 0,
      tone: 'error',
    };
  }

  if (task.status === 'completed') {
    return {
      phase: '压缩完成',
      detail: '下载链接已生成，可点击下载',
      percent: 100,
      activeStep: 2,
      tone: 'success',
    };
  }

  if (task.status === 'waiting') {
    return {
      phase: '等待压缩',
      detail: '任务已创建，等待后台开始处理',
      percent: 0,
      activeStep: 0,
      tone: 'warning',
    };
  }

  return {
    phase: '正在压缩文件夹',
    detail: `${task.processed_files || 0}/${task.total_files || 0} 个文件 · ${
      percent
    }%`,
    percent,
    activeStep: 1,
    tone: 'info',
  };
};

const StepBar = ({
  steps,
  activeStep,
  tone,
}: {
  steps: string[];
  activeStep: number;
  tone: Tone;
}) => (
  <Stack direction='row' alignItems='center' gap={0.75} sx={{ mt: 1 }}>
    {steps.map((step, index) => {
      const reached = index <= activeStep;
      const color =
        tone === 'error' && index === activeStep
          ? 'error.main'
          : reached
            ? tone === 'success'
              ? 'success.main'
              : 'primary.main'
            : 'text.disabled';

      return (
        <Stack
          key={step}
          direction='row'
          alignItems='center'
          gap={0.75}
          sx={{ minWidth: 0 }}
        >
          <Box
            sx={{
              width: 7,
              height: 7,
              borderRadius: '50%',
              bgcolor: color,
              flexShrink: 0,
            }}
          />
          <Typography
            variant='caption'
            noWrap
            sx={{ color, maxWidth: 96, fontWeight: reached ? 600 : 400 }}
          >
            {step}
          </Typography>
          {index < steps.length - 1 && (
            <Box
              sx={{
                width: 18,
                height: 1,
                bgcolor: reached ? color : 'divider',
                flexShrink: 0,
              }}
            />
          )}
        </Stack>
      );
    })}
  </Stack>
);

const TaskCard = ({
  title,
  view,
  steps,
  actions,
}: {
  title: string;
  view: TaskView;
  steps: string[];
  actions?: ReactNode;
}) => (
  <Paper
    variant='outlined'
    sx={{
      p: 1.75,
      borderRadius: 2,
      borderColor: view.tone === 'error' ? 'error.light' : 'divider',
      bgcolor: view.tone === 'error' ? 'error.lighter' : 'background.paper',
    }}
  >
    <Stack
      direction='row'
      alignItems='center'
      justifyContent='space-between'
      gap={1}
    >
      <Box sx={{ minWidth: 0 }}>
        <Typography variant='subtitle2' noWrap title={title}>
          {title}
        </Typography>
        <Typography
          variant='caption'
          color='text.secondary'
          sx={{ display: 'block', mt: 0.25 }}
        >
          {view.detail}
        </Typography>
      </Box>
      <Stack direction='row' alignItems='center' gap={1} flexShrink={0}>
        <Typography
          variant='subtitle2'
          sx={{ color: view.tone === 'error' ? 'error.main' : 'primary.main' }}
        >
          {view.indeterminate ? '--' : `${view.percent}%`}
        </Typography>
        {statusChip(view.tone, view.phase)}
        {actions}
      </Stack>
    </Stack>
    <StepBar steps={steps} activeStep={view.activeStep} tone={view.tone} />
  </Paper>
);

const EmptyState = () => (
  <Stack alignItems='center' justifyContent='center' sx={{ py: 6 }}>
    <Typography sx={{ fontWeight: 600 }}>暂无异步任务</Typography>
    <Typography variant='body2' color='text.secondary' sx={{ mt: 0.5 }}>
      批量导入 ZIP、下载文件夹等耗时任务会显示在这里。
    </Typography>
  </Stack>
);

const ZipUploadTaskPanel = ({ open, onClose, kbId, onTaskSettled }: Props) => {
  const [tasks, setTasks] = useState<ZipUploadTask[]>([]);
  const [downloadTasks, setDownloadTasks] = useState<FolderDownloadTask[]>([]);
  const [page, setPage] = useState(1);
  const [directoryRefreshingIds, setDirectoryRefreshingIds] = useState<
    Set<string>
  >(new Set());
  const [resumingTaskId, setResumingTaskId] = useState('');
  const [resumeProgress, setResumeProgress] = useState(0);
  const handledCompletedUploadsRef = useRef<Set<string>>(new Set());

  const load = useCallback(async () => {
    if (!kbId) return;
    const [nextTasks, nextDownloadTasks] = await Promise.all([
      getApiV1FileUploadZipTasks(kbId),
      getApiV1FileFolderDownloads(kbId),
    ]);
    const uploadTasks = nextTasks || [];
    setTasks(uploadTasks);
    setDownloadTasks(nextDownloadTasks || []);

    const newlyCompleted = uploadTasks.filter(
      task =>
        task.status === 'completed' &&
        !handledCompletedUploadsRef.current.has(task.id),
    );
    if (newlyCompleted.length > 0) {
      newlyCompleted.forEach(task =>
        handledCompletedUploadsRef.current.add(task.id),
      );
      setDirectoryRefreshingIds(previous => {
        const next = new Set(previous);
        newlyCompleted.forEach(task => next.add(task.id));
        return next;
      });
      onTaskSettled?.();
      window.setTimeout(() => {
        setDirectoryRefreshingIds(previous => {
          const next = new Set(previous);
          newlyCompleted.forEach(task => next.delete(task.id));
          return next;
        });
      }, 5000);
    }
  }, [kbId, onTaskSettled]);

  useEffect(() => {
    if (!open) return undefined;
    load();
    const timer = window.setInterval(load, 3000);
    return () => window.clearInterval(timer);
  }, [open, load]);

  const retry = async (id: string, kind: 'upload' | 'download') => {
    if (kind === 'upload') await postApiV1FileUploadZipTaskRetry(id);
    else await postApiV1FileFolderDownloadRetry(id);
    load();
  };

  const cancelUpload = (task: ZipUploadTask) => {
    Modal.confirm({
      title: '取消上传任务',
      content:
        '取消后会终止 OSS 分片上传，当前未完成的 ZIP 需要重新创建任务才能导入。是否确认取消？',
      okText: '确认取消',
      cancelText: '再想想',
      okButtonProps: { color: 'error' },
      onOk: async () => {
        await postApiV1FileUploadZipTaskCancel(task.id);
        message.success('已取消上传任务');
        load();
      },
    });
  };

  const resumeUpload = async (task: ZipUploadTask, file?: File) => {
    if (!file) return;
    try {
      setResumingTaskId(task.id);
      setResumeProgress(0);
      await uploadZipTaskParts({
        file,
        task,
        onProgress: progress => setResumeProgress(progress),
        onTaskUpdate: nextTask => {
          setTasks(previous =>
            previous.map(item => (item.id === nextTask.id ? nextTask : item)),
          );
        },
      });
      message.success('ZIP 已上传完成，正在后台解压');
      onTaskSettled?.();
      load();
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : 'ZIP 续传失败';
      await postApiV1FileUploadZipTaskFail(task.id, errorMessage).catch(
        () => undefined,
      );
      message.error(errorMessage);
      load();
    } finally {
      setResumingTaskId('');
      setResumeProgress(0);
    }
  };

  const downloadFolder = async (id: string) => {
    const popup = window.open('about:blank', '_blank');
    const result = await getApiV1FileFolderDownload(id);
    if (result.download_url) {
      if (popup) popup.location.href = result.download_url;
      else window.open(result.download_url, '_blank', 'noopener,noreferrer');
    } else {
      popup?.close();
    }
  };

  const hasTasks = tasks.length > 0 || downloadTasks.length > 0;
  const allTasks = useMemo<TaskListItem[]>(() => {
    return [
      ...tasks.map(task => ({
        id: task.id,
        kind: 'upload' as const,
        createdAt: task.created_at || task.updated_at,
        upload: task,
      })),
      ...downloadTasks.map(task => ({
        id: task.id,
        kind: 'download' as const,
        createdAt: task.created_at || task.updated_at,
        download: task,
      })),
    ].sort((a, b) => {
      const left = a.createdAt ? new Date(a.createdAt).getTime() : 0;
      const right = b.createdAt ? new Date(b.createdAt).getTime() : 0;
      return right - left;
    });
  }, [downloadTasks, tasks]);
  const summary = useMemo(() => {
    return {
      total: allTasks.length,
      running: allTasks.filter(item => {
        const status =
          item.kind === 'upload' ? item.upload?.status : item.download?.status;
        return (
          status && ['waiting', 'uploading', 'processing'].includes(status)
        );
      }).length,
      failed: allTasks.filter(item => {
        const status =
          item.kind === 'upload' ? item.upload?.status : item.download?.status;
        return status && ['failed', 'expired', 'interrupted'].includes(status);
      }).length,
      completed: allTasks.filter(item => {
        const status =
          item.kind === 'upload' ? item.upload?.status : item.download?.status;
        return status === 'completed';
      }).length,
    };
  }, [allTasks]);
  const totalPages = Math.max(1, Math.ceil(allTasks.length / taskPageSize));
  const pageTasks = useMemo(
    () => allTasks.slice((page - 1) * taskPageSize, page * taskPageSize),
    [allTasks, page],
  );

  useEffect(() => {
    if (open) setPage(1);
  }, [open]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth='md'>
      <DialogTitle
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          pb: 1,
        }}
      >
        <Box>
          <Typography variant='h6' sx={{ fontWeight: 700 }}>
            异步任务
          </Typography>
          <Typography variant='body2' color='text.secondary'>
            跟踪 ZIP 上传、解压建目录、替换占位文件和文件夹下载进度。
          </Typography>
        </Box>
        <Stack direction='row'>
          <IconButton onClick={load} aria-label='刷新'>
            <RefreshIcon />
          </IconButton>
          <IconButton onClick={onClose} aria-label='关闭'>
            <CloseIcon />
          </IconButton>
        </Stack>
      </DialogTitle>
      <DialogContent dividers sx={{ bgcolor: 'background.default' }}>
        {!hasTasks ? (
          <EmptyState />
        ) : (
          <Stack gap={1.5}>
            <Stack direction='row' gap={1} flexWrap='wrap' useFlexGap>
              {statusChip('default', `全部 ${summary.total}`)}
              {statusChip('info', `进行中 ${summary.running}`)}
              {statusChip('success', `已完成 ${summary.completed}`)}
              {summary.failed > 0 &&
                statusChip('error', `失败 ${summary.failed}`)}
            </Stack>

            <Stack gap={1.25}>
              {pageTasks.map(item => {
                if (item.kind === 'upload' && item.upload) {
                  const task = item.upload;
                  const view = buildZipTaskView(
                    task,
                    directoryRefreshingIds.has(task.id),
                  );
                  const canResume = canResumeZipUpload(task);
                  const canCancel =
                    !task.node_id &&
                    ['waiting', 'uploading', 'interrupted', 'failed'].includes(
                      task.status,
                    );
                  return (
                    <TaskCard
                      key={`upload-${task.id}`}
                      title={`ZIP 上传：${task.filename}`}
                      view={view}
                      steps={zipSteps}
                      actions={
                        <Stack direction='row' alignItems='center' gap={0.5}>
                          {canResume && (
                            <Button
                              size='small'
                              component='label'
                              disabled={Boolean(resumingTaskId)}
                              startIcon={<UploadFileOutlinedIcon />}
                            >
                              {resumingTaskId === task.id
                                ? `续传中 ${resumeProgress}%`
                                : '继续上传'}
                              <input
                                hidden
                                type='file'
                                accept='.zip,application/zip'
                                onChange={event => {
                                  const file = event.target.files?.[0];
                                  event.currentTarget.value = '';
                                  resumeUpload(task, file);
                                }}
                              />
                            </Button>
                          )}
                          {task.status === 'failed' && !canResume && (
                            <Button
                              size='small'
                              startIcon={<ReplayIcon />}
                              onClick={() => retry(task.id, 'upload')}
                            >
                              重试
                            </Button>
                          )}
                          {canCancel && (
                            <Button
                              size='small'
                              color='error'
                              disabled={Boolean(resumingTaskId)}
                              onClick={() => cancelUpload(task)}
                            >
                              取消
                            </Button>
                          )}
                        </Stack>
                      }
                    />
                  );
                }

                if (item.kind === 'download' && item.download) {
                  const task = item.download;
                  const view = buildDownloadTaskView(task);
                  return (
                    <TaskCard
                      key={`download-${task.id}`}
                      title='文件夹压缩下载'
                      view={view}
                      steps={downloadSteps}
                      actions={
                        task.status === 'failed' ? (
                          <Button
                            size='small'
                            startIcon={<ReplayIcon />}
                            onClick={() => retry(task.id, 'download')}
                          >
                            重试
                          </Button>
                        ) : task.status === 'completed' ? (
                          <Button
                            size='small'
                            startIcon={<DownloadOutlinedIcon />}
                            onClick={() => downloadFolder(task.id)}
                          >
                            下载
                          </Button>
                        ) : undefined
                      }
                    />
                  );
                }

                return null;
              })}
            </Stack>

            {allTasks.length > taskPageSize && (
              <Stack
                direction='row'
                alignItems='center'
                justifyContent='space-between'
                sx={{ pt: 0.5 }}
              >
                <Typography variant='caption' color='text.secondary'>
                  {(page - 1) * taskPageSize + 1}-
                  {Math.min(page * taskPageSize, allTasks.length)} / 共{' '}
                  {allTasks.length} 条
                </Typography>
                <Pagination
                  size='small'
                  shape='rounded'
                  page={page}
                  count={totalPages}
                  onChange={(_, nextPage) => setPage(nextPage)}
                />
              </Stack>
            )}
          </Stack>
        )}
      </DialogContent>
    </Dialog>
  );
};

export default ZipUploadTaskPanel;
