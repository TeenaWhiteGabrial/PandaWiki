import { V1NodeDetailResp } from '@/request/types';
import { resolveStaticFileUrl } from '@/utils/staticFileUrl';
import { Box, Button, Stack, Typography } from '@mui/material';
import { useEffect, useState } from 'react';

interface FilePreviewProps {
  detail: V1NodeDetailResp;
}

const DIRECT_PREVIEW_TYPES = ['pdf', 'md', 'markdown', 'html', 'htm'];

const FilePreview = ({ detail }: FilePreviewProps) => {
  const fileKey = detail.meta?.file_key;
  const previewKey = detail.meta?.preview_key;
  const fileType = (detail.meta?.file_type || '').toLowerCase();
  const fileName = detail.meta?.file_name || detail.name || '原始文件';
  const [fileUrl, setFileUrl] = useState('');
  const [previewUrl, setPreviewUrl] = useState('');
  const canPreview = DIRECT_PREVIEW_TYPES.includes(fileType);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      resolveStaticFileUrl(fileKey),
      resolveStaticFileUrl(previewKey),
    ])
      .then(([resolvedFileUrl, resolvedPreviewUrl]) => {
        if (!cancelled) {
          setFileUrl(resolvedFileUrl);
          setPreviewUrl(resolvedPreviewUrl);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFileUrl('');
          setPreviewUrl('');
        }
      });
    return () => {
      cancelled = true;
    };
  }, [fileKey, previewKey]);

  return (
    <Stack
      spacing={2}
      sx={{
        height: 'calc(100vh - 88px)',
        px: 3,
        pb: 3,
      }}
    >
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        sx={{
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 2,
          px: 2,
          py: 1.5,
          bgcolor: 'background.paper',
        }}
      >
        <Box>
          <Typography variant='body1' fontWeight={600}>
            {fileName}
          </Typography>
          <Typography variant='body2' color='text.secondary'>
            当前文件仅支持预览和下载，不支持在线编辑
          </Typography>
        </Box>
        {fileUrl && (
          <Button href={fileUrl} target='_blank' rel='noreferrer'>
            下载原文件
          </Button>
        )}
      </Stack>

      {canPreview ? (
        <Box
          component='iframe'
          src={fileType === 'pdf' && previewUrl ? previewUrl : fileUrl}
          title={fileName}
          sandbox={fileType === 'html' || fileType === 'htm' ? '' : undefined}
          sx={{
            width: '100%',
            flexGrow: 1,
            minHeight: 0,
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 2,
            bgcolor: 'common.white',
          }}
        />
      ) : (
        <Stack
          alignItems='center'
          justifyContent='center'
          spacing={1}
          sx={{
            flexGrow: 1,
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 2,
            bgcolor: 'background.paper',
          }}
        >
          <Typography variant='body1'>当前文件暂不支持站内预览</Typography>
          <Typography variant='body2' color='text.secondary'>
            当前阶段支持 Markdown、富文本、PDF 和 HTML 预览，可下载原文件查看。
          </Typography>
        </Stack>
      )}
    </Stack>
  );
};

export default FilePreview;
