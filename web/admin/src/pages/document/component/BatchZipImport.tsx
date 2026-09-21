import {
  postApiV1FileImportZip,
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
  onClose: () => void;
  refresh?: () => void;
}

const BatchZipImport = ({
  open,
  parentId = null,
  onClose,
  refresh,
}: BatchZipImportProps) => {
  const { kb_id, nav_id } = useAppSelector(state => state.config);
  const [file, setFile] = useState<File | null>(null);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<BatchImportResult | null>(null);

  useEffect(() => {
    if (!open) {
      setFile(null);
      setLoading(false);
      setResult(null);
    }
  }, [open]);

  const close = () => {
    if (loading) return;
    onClose();
  };

  const submit = async () => {
    if (!file || !kb_id || !nav_id) {
      message.warning('请选择 ZIP 文件');
      return;
    }
    setLoading(true);
    try {
      const response = await postApiV1FileImportZip({
        file,
        kb_id,
        nav_id,
        ...(parentId ? { parent_id: parentId } : {}),
      });
      setResult(response);
      refresh?.();
      message.success(
        `导入完成：成功 ${response.imported} 个，失败 ${response.failed} 个`,
      );
    } catch (error) {
      console.error(error);
      message.error('批量导入失败');
    } finally {
      setLoading(false);
    }
  };

  return (
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
      cancelButtonProps={{ disabled: loading }}
      disableEscapeKeyDown
    >
      {!result ? (
        <Stack gap={2}>
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
          {loading && <LinearProgress />}
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
  );
};

export default BatchZipImport;
