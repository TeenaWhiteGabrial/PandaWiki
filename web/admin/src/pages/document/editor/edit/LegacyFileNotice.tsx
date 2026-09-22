import { V1NodeDetailResp } from '@/request/types';
import { getStaticFileUrl } from '@/utils/staticFileUrl';
import { Box, Button, Stack, Typography } from '@mui/material';

interface LegacyFileNoticeProps {
  detail: V1NodeDetailResp;
}

const LegacyFileNotice = ({ detail }: LegacyFileNoticeProps) => {
  const fileKey = detail.meta?.file_key;
  const fileUrl = getStaticFileUrl(fileKey);

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
        spacing={1.5}
        alignItems='flex-start'
        sx={{
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 2,
          px: 3,
          py: 3,
          bgcolor: 'background.paper',
        }}
      >
        <Typography variant='h6'>该类型文档已不再支持页内预览</Typography>
        <Typography variant='body2' color='text.secondary'>
          当前系统只支持 Markdown 和富文本作为文档正文。Office/PDF
          文件请作为附件上传并在附件区预览。
        </Typography>
        {fileUrl && (
          <Box>
            <Button
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
    </Stack>
  );
};

export default LegacyFileNotice;
