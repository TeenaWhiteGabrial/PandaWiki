import { V1NodeDetailResp } from '@/request/types';
import { Box, Button, Stack, Typography } from '@mui/material';

interface FilePreviewProps {
  detail: V1NodeDetailResp;
}

const FilePreview = ({ detail }: FilePreviewProps) => {
  const fileKey = detail.meta?.file_key;
  const previewKey = detail.meta?.preview_key;
  const fileType = (detail.meta?.file_type || '').toLowerCase();
  const fileName = detail.meta?.file_name || detail.name || '原始文件';
  const fileUrl = fileKey ? `/static-file/${fileKey}` : '';
  const previewUrl = previewKey ? `/static-file/${previewKey}` : '';
  const canPreview = !!previewUrl || fileType === 'pdf';

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
          src={previewUrl || fileUrl}
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
            可先下载原文件查看，或为 Office 文件生成 PDF 预览
          </Typography>
        </Stack>
      )}
    </Stack>
  );
};

export default FilePreview;
