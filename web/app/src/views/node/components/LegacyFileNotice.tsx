'use client';

import type { V1ShareNodeDetailResp } from '@/request/types';
import { Box, Button, Stack, Typography } from '@mui/material';

interface LegacyFileNoticeProps {
  info: V1ShareNodeDetailResp;
}

const LegacyFileNotice = ({ info }: LegacyFileNoticeProps) => {
  const fileKey = info.meta?.file_key || '';
  const fileUrl = fileKey ? `/static-file/${fileKey}` : '';

  return (
    <Stack
      spacing={2}
      sx={{
        mt: 4,
        p: 3,
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: 'background.paper',
      }}
    >
      <Box>
        <Typography sx={{ fontWeight: 600, mb: 1 }}>
          该类型文档已不再支持页内预览
        </Typography>
        <Typography variant='body2' color='text.secondary'>
          当前系统只支持 Markdown 和富文本作为文档正文。Office/PDF
          文件请作为附件上传并在附件区预览。
        </Typography>
      </Box>
      {fileUrl && (
        <Box>
          <Button
            component='a'
            href={fileUrl}
            target='_blank'
            rel='noreferrer'
            variant='outlined'
          >
            下载原文件
          </Button>
        </Box>
      )}
    </Stack>
  );
};

export default LegacyFileNotice;
