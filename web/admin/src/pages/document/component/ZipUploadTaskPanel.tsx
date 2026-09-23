import {
  FolderDownloadTask,
  ZipUploadTask,
  getApiV1FileFolderDownloads,
  getApiV1FileFolderDownload,
  getApiV1FileUploadZipTasks,
  postApiV1FileFolderDownloadRetry,
  postApiV1FileUploadZipTaskRetry,
} from '@/request/BatchImport';
import {
  Button,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  List,
  ListItem,
  ListItemText,
  Stack,
  Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import RefreshIcon from '@mui/icons-material/Refresh';
import { useCallback, useEffect, useState } from 'react';

interface Props {
  open: boolean;
  onClose: () => void;
  kbId?: string;
}

const formatSize = (size: number) =>
  size >= 1024 ** 3
    ? `${(size / 1024 ** 3).toFixed(1)} GB`
    : `${(size / 1024 ** 2).toFixed(0)} MB`;
const statusText = (status: string) =>
  ({
    waiting: '等待处理',
    uploading: '上传中',
    processing: '处理中',
    completed: '已完成',
    failed: '失败',
    expired: '已过期',
  })[status] || status;

const ZipUploadTaskPanel = ({ open, onClose, kbId }: Props) => {
  const [tasks, setTasks] = useState<ZipUploadTask[]>([]);
  const [downloadTasks, setDownloadTasks] = useState<FolderDownloadTask[]>([]);
  const load = useCallback(() => {
    if (!kbId) return;
    getApiV1FileUploadZipTasks(kbId).then(value => setTasks(value || []));
    getApiV1FileFolderDownloads(kbId).then(value =>
      setDownloadTasks(value || []),
    );
  }, [kbId]);
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
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth='sm'>
      <DialogTitle
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        异步任务
        <Stack direction='row'>
          <IconButton onClick={load} aria-label='刷新'>
            <RefreshIcon />
          </IconButton>
          <IconButton onClick={onClose} aria-label='关闭'>
            <CloseIcon />
          </IconButton>
        </Stack>
      </DialogTitle>
      <DialogContent dividers>
        {tasks.length === 0 && downloadTasks.length === 0 ? (
          <Typography
            color='text.secondary'
            sx={{ py: 4, textAlign: 'center' }}
          >
            暂无异步任务
          </Typography>
        ) : (
          <List>
            {tasks.map(task => {
              const uploadPercent = task.size
                ? Math.min(
                    100,
                    Math.round((task.uploaded_bytes / task.size) * 100),
                  )
                : 0;
              const extractionPercent = task.total_files
                ? Math.min(
                    100,
                    Math.round(
                      ((task.processed_files || 0) * 100) / task.total_files,
                    ),
                  )
                : task.total_bytes
                  ? Math.min(
                      100,
                      Math.round(
                        ((task.processed_bytes || 0) * 100) / task.total_bytes,
                      ),
                    )
                  : 0;
              const percent =
                task.status === 'processing'
                  ? extractionPercent
                  : uploadPercent;
              const detail =
                task.status === 'failed'
                  ? `失败：${task.error_message || '未知错误'}；可点击重试`
                  : task.status === 'processing'
                    ? `解压 ${task.processed_files || 0}/${task.total_files || 0} 个文件 · ${percent}%`
                    : task.status === 'completed'
                      ? '已完成'
                      : `${formatSize(task.uploaded_bytes)} / ${formatSize(task.size)} · ${percent}%`;
              return (
                <ListItem
                  key={`upload-${task.id}`}
                  divider
                  secondaryAction={
                    task.status === 'failed' ? (
                      <Button onClick={() => retry(task.id, 'upload')}>
                        重试
                      </Button>
                    ) : undefined
                  }
                >
                  <ListItemText
                    primary={`ZIP 上传：${task.filename}`}
                    secondaryTypographyProps={{ component: 'div' }}
                    secondary={
                      <Stack spacing={0.5} sx={{ mt: 0.5 }}>
                        <Typography variant='caption'>
                          {statusText(task.status)} · {detail}
                        </Typography>
                        {task.status !== 'completed' && (
                          <LinearProgress
                            variant='determinate'
                            value={percent}
                          />
                        )}
                      </Stack>
                    }
                  />
                </ListItem>
              );
            })}
            {downloadTasks.map(task => {
              const percent = task.total_bytes
                ? Math.min(
                    100,
                    Math.round((task.processed_bytes / task.total_bytes) * 100),
                  )
                : 0;
              const detail =
                task.status === 'failed'
                  ? `失败：${task.error_message || '未知错误'}`
                  : task.status === 'completed'
                    ? '已完成'
                    : `${task.processed_files}/${task.total_files} 个文件 · ${percent}%`;
              return (
                <ListItem
                  key={`download-${task.id}`}
                  divider
                  secondaryAction={
                    task.status === 'failed' ? (
                      <Button onClick={() => retry(task.id, 'download')}>
                        重试
                      </Button>
                    ) : task.status === 'completed' ? (
                      <Button onClick={() => downloadFolder(task.id)}>
                        下载
                      </Button>
                    ) : undefined
                  }
                >
                  <ListItemText
                    primary='文件夹压缩下载'
                    secondaryTypographyProps={{ component: 'div' }}
                    secondary={
                      <Stack spacing={0.5} sx={{ mt: 0.5 }}>
                        <Typography variant='caption'>
                          {statusText(task.status)} · {detail}
                        </Typography>
                        {task.status !== 'completed' && (
                          <LinearProgress
                            variant='determinate'
                            value={percent}
                          />
                        )}
                      </Stack>
                    }
                  />
                </ListItem>
              );
            })}
          </List>
        )}
      </DialogContent>
    </Dialog>
  );
};

export default ZipUploadTaskPanel;
