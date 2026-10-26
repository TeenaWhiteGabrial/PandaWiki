import { V1NodeDetailResp } from '@/request/types';
import { getApiV1FileDownloadUrl } from '@/request/File';
import { resolveStaticFileUrl } from '@/utils/staticFileUrl';
import { Box, Button, Stack, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import FileStorageInfo from './FileStorageInfo';

interface FilePreviewProps {
  detail: V1NodeDetailResp;
}

const DIRECT_PREVIEW_TYPES = [
  'pdf',
  'md',
  'markdown',
  'txt',
  'html',
  'htm',
  'png',
  'jpg',
  'jpeg',
  'gif',
  'webp',
  'svg',
  'bmp',
  'ico',
  'avif',
];
const IMAGE_PREVIEW_TYPES = new Set([
  'png',
  'jpg',
  'jpeg',
  'gif',
  'webp',
  'svg',
  'bmp',
  'ico',
  'avif',
]);
const VIDEO_PREVIEW_TYPES = new Set([
  'video',
  'mp4',
  'webm',
  'ogv',
  'mov',
  'm4v',
]);
const AUDIO_PREVIEW_TYPES = new Set([
  'audio',
  'mp3',
  'wav',
  'ogg',
  'oga',
  'm4a',
  'aac',
  'flac',
]);

const getFileType = (detail: V1NodeDetailResp) => {
  const explicitType = (detail.meta?.file_type || '').toLowerCase().trim();
  if (explicitType) {
    if (explicitType.startsWith('image/')) {
      return explicitType.replace(/^image\//, '').replace(/\+xml$/, '');
    }
    if (explicitType.startsWith('video/')) return 'video';
    if (explicitType.startsWith('audio/')) return 'audio';
    if (explicitType === 'application/pdf') return 'pdf';
    if (explicitType === 'text/plain') return 'txt';
    if (explicitType === 'text/html') return 'html';
    if (explicitType.includes('markdown')) return 'md';
    return explicitType.replace(/^\./, '');
  }

  const fileName =
    detail.meta?.file_name || detail.name || detail.meta?.file_key || '';
  const normalized = fileName.split('?')[0].split('#')[0];
  const ext = normalized.includes('.') ? normalized.split('.').pop() : '';
  return (ext || '').toLowerCase().trim();
};

const FilePreview = ({ detail }: FilePreviewProps) => {
  const fileKey = detail.meta?.file_key;
  const previewKey = detail.meta?.preview_key;
  const fileType = getFileType(detail);
  const fileName = detail.meta?.file_name || detail.name || '原始文件';
  const [fileUrl, setFileUrl] = useState('');
  const [downloadUrl, setDownloadUrl] = useState('');
  const [previewUrl, setPreviewUrl] = useState('');
  const effectivePreviewUrl = previewUrl || fileUrl;
  const canImagePreview = IMAGE_PREVIEW_TYPES.has(fileType);
  const canVideoPreview = VIDEO_PREVIEW_TYPES.has(fileType);
  const canAudioPreview = AUDIO_PREVIEW_TYPES.has(fileType);
  const canIframePreview =
    DIRECT_PREVIEW_TYPES.includes(fileType) || !!previewUrl;
  const canPreview =
    canImagePreview || canVideoPreview || canAudioPreview || canIframePreview;

  useEffect(() => {
    let cancelled = false;
    setFileUrl('');
    setDownloadUrl('');
    setPreviewUrl('');

    resolveStaticFileUrl(fileKey)
      .then(resolvedFileUrl => {
        if (!cancelled) {
          setFileUrl(resolvedFileUrl);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFileUrl('');
        }
      });

    if (fileKey && !/^(https?:|blob:|data:)/i.test(fileKey)) {
      getApiV1FileDownloadUrl({
        key: fileKey.replace(/^\/+/, ''),
        attachment: true,
        filename: fileName,
      })
        .then(({ url }) => {
          if (!cancelled) setDownloadUrl(url);
        })
        .catch(() => {
          if (!cancelled) setDownloadUrl('');
        });
    } else if (fileKey) {
      setDownloadUrl(fileKey);
    }

    if (previewKey) {
      resolveStaticFileUrl(previewKey)
        .then(resolvedPreviewUrl => {
          if (!cancelled) {
            setPreviewUrl(resolvedPreviewUrl);
          }
        })
        .catch(() => {
          if (!cancelled) {
            setPreviewUrl('');
          }
        });
    }

    return () => {
      cancelled = true;
    };
  }, [fileKey, fileName, previewKey]);

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
          {/* <Typography variant='body2' color='text.secondary'>
            当前文件仅支持预览和下载，不支持在线编辑
          </Typography> */}
        </Box>
        {(downloadUrl || fileUrl) && (
          <Button
            href={downloadUrl || fileUrl}
            target={downloadUrl ? undefined : '_blank'}
            rel='noreferrer'
          >
            下载原文件
          </Button>
        )}
      </Stack>

      <FileStorageInfo fileKey={fileKey} />

      {!effectivePreviewUrl ? (
        <Stack
          alignItems='center'
          justifyContent='center'
          sx={{
            flexGrow: 1,
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 2,
            bgcolor: 'background.paper',
          }}
        >
          <Typography variant='body2' color='text.secondary'>
            文件地址加载中...
          </Typography>
        </Stack>
      ) : canPreview ? (
        canImagePreview ? (
          <Box
            component='img'
            src={effectivePreviewUrl}
            alt={fileName}
            sx={{
              width: '100%',
              height: '100%',
              objectFit: 'contain',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 2,
              bgcolor: 'common.white',
            }}
          />
        ) : canVideoPreview ? (
          <Box
            component='video'
            src={effectivePreviewUrl}
            controls
            sx={{
              width: '100%',
              height: '100%',
              maxHeight: 'calc(100vh - 180px)',
              objectFit: 'contain',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 2,
              bgcolor: 'common.black',
            }}
          />
        ) : canAudioPreview ? (
          <Stack
            alignItems='center'
            justifyContent='center'
            sx={{
              flexGrow: 1,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 2,
              bgcolor: 'background.paper',
            }}
          >
            <Box
              component='audio'
              src={effectivePreviewUrl}
              controls
              sx={{ width: 'min(720px, 90%)' }}
            />
          </Stack>
        ) : (
          <Box
            component='iframe'
            src={effectivePreviewUrl}
            title={fileName}
            sandbox='allow-downloads'
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
        )
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
            支持 PDF、图片、视频、音频、Markdown、HTML、TXT
            预览；Office、压缩包等格式可下载原文件查看。
          </Typography>
        </Stack>
      )}
    </Stack>
  );
};

export default FilePreview;
