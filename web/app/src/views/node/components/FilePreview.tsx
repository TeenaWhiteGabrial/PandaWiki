'use client';

import { Box, Button, Stack, Typography } from '@mui/material';
import type { V1ShareNodeDetailResp } from '@/request/types';
import { openShareFile, resolveShareFileUrl } from '@/utils/shareFileUrl';
import { useEffect, useState } from 'react';

interface FilePreviewProps {
  info: V1ShareNodeDetailResp;
}

const FilePreview = ({ info }: FilePreviewProps) => {
  const fileKey = info.meta?.file_key || '';
  const previewKey = info.meta?.preview_key || '';
  const fileType = (info.meta?.file_type || '').toLowerCase();
  const fileName = info.meta?.file_name || info.name || '未命名文件';
  const [fileUrl, setFileUrl] = useState('');
  const [previewUrl, setPreviewUrl] = useState('');

  useEffect(() => {
    let disposed = false;
    setFileUrl('');
    setPreviewUrl('');

    if (!fileKey) return;

    resolveShareFileUrl(fileKey)
      .then(url => {
        if (!disposed) setFileUrl(url);
      })
      .catch(() => {
        if (!disposed) setFileUrl('');
      });

    if (previewKey) {
      resolveShareFileUrl(previewKey)
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
  }, [fileKey, previewKey]);

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
          <Typography variant='body2' color='text.secondary'>
            原始文件只读预览，不可编辑
          </Typography>
        </Box>
        <Button
          variant='outlined'
          disabled={!fileUrl}
          onClick={() => openShareFile(fileKey)}
        >
          下载原文件
        </Button>
      </Stack>

      {previewUrl || fileType === 'pdf' ? (
        <Box
          component='iframe'
          src={previewUrl || fileUrl || 'about:blank'}
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
            当前格式暂不支持内网页内预览
          </Typography>
          <Typography variant='body2' color='text.secondary'>
            当前文件还没有生成 PDF 预览，或预览生成失败。可以先下载原文件查看。
          </Typography>
        </Box>
      )}
    </Stack>
  );
};

export default FilePreview;
