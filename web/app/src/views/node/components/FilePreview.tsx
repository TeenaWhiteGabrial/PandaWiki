'use client';

import { Box, Button, Stack, Typography } from '@mui/material';
import type { V1ShareNodeDetailResp } from '@/request/types';
import { openShareFile, resolveShareFileUrl } from '@/utils/shareFileUrl';
import { useEffect, useState } from 'react';

interface FilePreviewProps {
  info: V1ShareNodeDetailResp;
}

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

const IFRAME_PREVIEW_TYPES = new Set([
  'pdf',
  'md',
  'markdown',
  'txt',
  'html',
  'htm',
]);

const getFileType = (info: V1ShareNodeDetailResp) => {
  const explicitType = (info.meta?.file_type || '').toLowerCase().trim();
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
    info.meta?.file_name || info.name || info.meta?.file_key || '';
  const normalized = fileName.split('?')[0].split('#')[0];
  const ext = normalized.includes('.') ? normalized.split('.').pop() : '';
  return (ext || '').toLowerCase().trim();
};

const FilePreview = ({ info }: FilePreviewProps) => {
  const fileKey = info.meta?.file_key || '';
  const previewKey = info.meta?.preview_key || '';
  const nodeId = info.id || '';
  const kbId = info.kb_id || '';
  const fileType = getFileType(info);
  const fileName = info.meta?.file_name || info.name || '未命名文件';
  const [fileUrl, setFileUrl] = useState('');
  const [previewUrl, setPreviewUrl] = useState('');
  const effectivePreviewUrl = previewUrl || fileUrl;
  const canIframePreview = IFRAME_PREVIEW_TYPES.has(fileType) || !!previewUrl;
  const canImagePreview = IMAGE_PREVIEW_TYPES.has(fileType);
  const canVideoPreview = VIDEO_PREVIEW_TYPES.has(fileType);
  const canAudioPreview = AUDIO_PREVIEW_TYPES.has(fileType);

  useEffect(() => {
    let disposed = false;
    setFileUrl('');
    setPreviewUrl('');

    if (!fileKey) return;

    resolveShareFileUrl(fileKey, nodeId, kbId)
      .then(url => {
        if (!disposed) setFileUrl(url);
      })
      .catch(() => {
        if (!disposed) setFileUrl('');
      });

    if (previewKey) {
      resolveShareFileUrl(previewKey, nodeId, kbId)
        .then(url => {
          if (!disposed) setPreviewUrl(url);
        })
        .catch(() => {
          if (!disposed) setPreviewUrl('');
        });
    }

    return () => {
      disposed = true;
    };
  }, [fileKey, kbId, nodeId, previewKey]);

  if (!fileKey) {
    return (
      <Box
        sx={{
          mt: 4,
          p: 4,
          borderRadius: 2,
          border: '1px solid',
          borderColor: 'divider',
          bgcolor: 'background.paper',
        }}
      >
        <Typography color='text.secondary'>
          原始文件不存在，当前只能查看解析后的内容。
        </Typography>
      </Box>
    );
  }

  return (
    <Stack gap={3} sx={{ mt: 4 }}>
      <Stack
        direction='row'
        justifyContent='space-between'
        alignItems='center'
        gap={2}
        sx={{
          p: 2,
          borderRadius: 2,
          border: '1px solid',
          borderColor: 'divider',
          bgcolor: 'background.paper',
        }}
      >
        <Box>
          <Typography sx={{ fontWeight: 600 }}>{fileName}</Typography>
          {/* <Typography variant='body2' color='text.secondary'>
            原始文件只读预览，不可编辑
          </Typography> */}
        </Box>
        <Button
          variant='outlined'
          disabled={!fileUrl}
          onClick={() => openShareFile(fileKey, nodeId, kbId)}
        >
          下载原文件
        </Button>
      </Stack>

      {!effectivePreviewUrl ? (
        <Box
          sx={{
            p: 4,
            borderRadius: 2,
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'background.paper',
          }}
        >
          <Typography color='text.secondary'>文件地址加载中...</Typography>
        </Box>
      ) : canImagePreview ? (
        <Box
          component='img'
          src={effectivePreviewUrl}
          alt={fileName}
          sx={{
            width: '100%',
            maxHeight: '80vh',
            objectFit: 'contain',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 2,
            bgcolor: '#fff',
          }}
        />
      ) : canVideoPreview ? (
        <Box
          component='video'
          src={effectivePreviewUrl}
          controls
          sx={{
            width: '100%',
            maxHeight: '80vh',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 2,
            bgcolor: '#000',
          }}
        />
      ) : canAudioPreview ? (
        <Stack
          alignItems='center'
          justifyContent='center'
          sx={{
            minHeight: 240,
            p: 4,
            borderRadius: 2,
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'background.paper',
          }}
        >
          <Box
            component='audio'
            src={effectivePreviewUrl}
            controls
            sx={{ width: 'min(720px, 100%)' }}
          />
        </Stack>
      ) : canIframePreview ? (
        <Box
          component='iframe'
          src={effectivePreviewUrl}
          title={fileName}
          sandbox={fileType === 'html' || fileType === 'htm' ? '' : undefined}
          sx={{
            width: '100%',
            minHeight: '80vh',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 2,
            bgcolor: '#fff',
          }}
        />
      ) : (
        <Box
          sx={{
            p: 4,
            borderRadius: 2,
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'background.paper',
          }}
        >
          <Typography sx={{ mb: 1, fontWeight: 600 }}>
            当前文件暂不支持站内预览
          </Typography>
          <Typography variant='body2' color='text.secondary'>
            当前系统支持 PDF、图片、视频、音频、Markdown、HTML、TXT
            的站内预览；Office、压缩包等格式请下载原文件查看。
          </Typography>
        </Box>
      )}
    </Stack>
  );
};

export default FilePreview;
