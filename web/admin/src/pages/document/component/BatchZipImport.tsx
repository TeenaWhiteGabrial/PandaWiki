import {
  getApiV1FileUploadZipTasks,
  postApiV1FileUploadZipPartComplete,
  postApiV1FileUploadZipTaskFail,
  postApiV1FileUploadZipTaskRetry,
  postApiV1FileUploadZipTaskComplete,
  postApiV1FileUploadZipTasks,
  putApiV1FileUploadZipPart,
  type BatchImportResult,
} from '@/request/BatchImport';
import { useAppSelector } from '@/store';
import { message, Modal } from '@/components/ui';
import {
  Alert,
  Box,
  Button,
  LinearProgress,
  Stack,
  Typography,
} from '@mui/material';
import { useEffect, useState } from 'react';

interface BatchZipImportProps {
  open: boolean;
  parentId?: string | null;
  parentName?: string;
  onClose: () => void;
  refresh?: () => void;
}

const BatchZipImport = ({
  open,
  parentId = null,
  parentName = '根目录',
  onClose,
  refresh,
}: BatchZipImportProps) => {
  const { kb_id, nav_id } = useAppSelector(state => state.config);
  const [file, setFile] = useState<File | null>(null);
  const [loading, setLoading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [result, setResult] = useState<BatchImportResult | null>(null);

  useEffect(() => {
    if (open) {
      setFile(null);
      setLoading(false);
      setProgress(0);
      setResult(null);
    }
  }, [open]);

  const close = () => {
    onClose();
  };

  const submit = async () => {
    if (!file || !kb_id || !nav_id) {
      message.warning('请选择 ZIP 文件');
      return;
    }
    setLoading(true);
    setProgress(0);
    let activeTaskId = '';
    try {
      const tasks = await getApiV1FileUploadZipTasks(kb_id);
      const savedTaskId = Object.keys(localStorage)
        .map(key =>
          key.startsWith('panda-wiki-zip-upload:')
            ? key.slice('panda-wiki-zip-upload:'.length)
            : '',
        )
        .find(Boolean);
      let task = (tasks || []).find(
        item =>
          item.id === savedTaskId &&
          item.filename === file.name &&
          item.size === file.size &&
          item.nav_id === nav_id &&
          (item.parent_id || '') === (parentId || '') &&
          item.status !== 'completed',
      );
      if (!task)
        task = (tasks || []).find(
          item =>
            item.filename === file.name &&
            item.size === file.size &&
            item.nav_id === nav_id &&
            (item.parent_id || '') === (parentId || '') &&
            item.status !== 'completed',
        );
      if (!task) {
        task = await postApiV1FileUploadZipTasks({
          kb_id,
          nav_id,
          ...(parentId ? { parent_id: parentId } : {}),
          filename: file.name,
          size: file.size,
        });
      } else if (task.status === 'failed') {
        task = await postApiV1FileUploadZipTaskRetry(task.id);
      }
      activeTaskId = task.id;
      localStorage.setItem(
        `panda-wiki-zip-upload:${task.id}`,
        JSON.stringify({ name: file.name, size: file.size }),
      );
      const uploadedParts: Record<string, string> =
        typeof task.uploaded_parts === 'string'
          ? JSON.parse(task.uploaded_parts || '{}')
          : task.uploaded_parts || {};
      for (let part = 1; part <= task.total_parts; part += 1) {
        const start = (part - 1) * task.part_size;
        const end = Math.min(file.size, start + task.part_size);
        if (uploadedParts[String(part)]) {
          setProgress(Math.round((end * 100) / file.size));
          continue;
        }
        let uploaded = false;
        let lastError: unknown;
        for (let attempt = 1; attempt <= 3 && !uploaded; attempt += 1) {
          try {
            const partResult = await putApiV1FileUploadZipPart(
              task.id,
              part,
              file.slice(start, end),
              (loaded, total) =>
                setProgress(
                  Math.round(
                    ((start + (loaded / total) * (end - start)) * 100) /
                      file.size,
                  ),
                ),
            );
            const etag = partResult.etag;
            await postApiV1FileUploadZipPartComplete(task.id, part, {
              etag,
              size: end - start,
            });
            uploaded = true;
          } catch (error) {
            lastError = error;
            if (attempt < 3)
              await new Promise(resolve =>
                window.setTimeout(resolve, attempt * 1000),
              );
          }
        }
        if (!uploaded)
          throw lastError instanceof Error
            ? lastError
            : new Error(`第 ${part} 个分片上传失败`);
      }
      const response = await postApiV1FileUploadZipTaskComplete(task.id);
      localStorage.removeItem(`panda-wiki-zip-upload:${task.id}`);
      setProgress(100);
      setResult({
        batch_id: response.node_id || '',
        total: 1,
        imported: 1,
        failed: 0,
        items: [
          {
            path: file.name,
            node_id: response.node_id,
            file_key: response.file_key,
            status: 'imported',
            content_ok: false,
            message:
              'ZIP 已上传，正在后台解压；目录和原始文件完成后会自动创建，发布时再进行问答内容解析',
          },
        ],
      });
      refresh?.();
      message.success('ZIP 已上传，后台正在解压；可在异步任务中查看进度');
    } catch (error) {
      console.error(error);
      if (activeTaskId)
        await postApiV1FileUploadZipTaskFail(
          activeTaskId,
          error instanceof Error ? error.message : 'ZIP 上传失败',
        ).catch(() => undefined);
      message.error(error instanceof Error ? error.message : 'ZIP 上传失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      {!open && loading && (
        <Box
          sx={{
            position: 'fixed',
            right: 24,
            bottom: 24,
            zIndex: 1400,
            width: 320,
            p: 2,
            borderRadius: 2,
            bgcolor: 'background.paper',
            boxShadow: 6,
          }}
        >
          <Typography variant='body2'>正在上传 {file?.name}</Typography>
          <LinearProgress
            variant='determinate'
            value={progress}
            sx={{ mt: 1 }}
          />
          <Typography variant='caption' color='text.secondary'>
            {progress}% · 上传完成后将自动登记为待发布文件
          </Typography>
        </Box>
      )}
      <Modal
        open={open}
        width={760}
        title='批量导入 ZIP'
        onCancel={close}
        onClose={close}
        footer={result ? null : ({ OkBtn, CancelBtn }) => [CancelBtn, OkBtn]}
        okText='开始导入'
        onOk={submit}
        okButtonProps={{ disabled: !file || loading, loading }}
        cancelButtonProps={{ disabled: false }}
      >
        {!result ? (
          <Stack gap={2}>
            <Box
              sx={{
                px: 1.5,
                py: 1,
                borderRadius: 1,
                bgcolor: 'action.hover',
              }}
            >
              <Typography variant='body2'>
                所属父目录：<strong>{parentName}</strong>
              </Typography>
            </Box>
            <Alert severity='info'>
              只支持
              ZIP。压缩包中的文件夹会同步创建为知识库目录，文件会保留原始文件并上传到对应
              OSS 路径。
            </Alert>
            <Button component='label' variant='outlined' disabled={loading}>
              {file ? '重新选择 ZIP' : '选择 ZIP 文件'}
              <input
                hidden
                type='file'
                accept='.zip,application/zip'
                onChange={event => setFile(event.target.files?.[0] || null)}
              />
            </Button>
            {file && (
              <Typography variant='body2' color='text.secondary'>
                {file.name}（{(file.size / 1024 / 1024).toFixed(2)} MB）
              </Typography>
            )}
            {loading && (
              <Stack gap={0.5}>
                <LinearProgress variant='determinate' value={progress} />
                <Typography variant='caption' color='text.secondary'>
                  {progress}% · 正在上传到
                  OSS，关闭导入窗口后仍会继续；关闭浏览器后重新选择相同文件即可续传
                </Typography>
              </Stack>
            )}
          </Stack>
        ) : (
          <Stack gap={2}>
            <Alert severity={result.failed > 0 ? 'warning' : 'success'}>
              共处理 {result.total} 个文件，成功 {result.imported} 个，失败{' '}
              {result.failed} 个。
            </Alert>
            <Box sx={{ maxHeight: 360, overflow: 'auto' }}>
              <Stack gap={1}>
                {result.items.map(item => (
                  <Box
                    key={`${item.path}-${item.node_id || item.status}`}
                    sx={{
                      px: 1.5,
                      py: 1,
                      borderRadius: 1,
                      bgcolor: 'background.default',
                    }}
                  >
                    <Typography variant='body2' noWrap title={item.path}>
                      {item.status === 'imported' ? '✓' : '✕'} {item.path}
                    </Typography>
                    {item.message && (
                      <Typography variant='caption' color='text.secondary'>
                        {item.message}
                      </Typography>
                    )}
                  </Box>
                ))}
              </Stack>
            </Box>
            <Button variant='contained' onClick={close}>
              完成
            </Button>
          </Stack>
        )}
      </Modal>
    </>
  );
};

export default BatchZipImport;
