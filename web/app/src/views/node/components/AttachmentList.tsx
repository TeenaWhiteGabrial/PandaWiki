'use client';

import type { DomainNodeAttachmentMeta } from '@/request/types';
import {
  Box,
  Button,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableRow,
  Typography,
} from '@mui/material';
import { useMemo } from 'react';

interface AttachmentListProps {
  attachments?: DomainNodeAttachmentMeta[];
}

const canPreviewInline = (attachment: DomainNodeAttachmentMeta) => {
  const fileType = (attachment.file_type || '').toLowerCase();
  return (
    fileType === 'pdf' ||
    (['doc', 'docx', 'ppt', 'pptx', 'xls', 'xlsx'].includes(fileType) &&
      !!attachment.preview_key)
  );
};

const AttachmentList = ({ attachments = [] }: AttachmentListProps) => {
  const normalizedAttachments = useMemo(() => {
    return attachments.filter(item => !!item?.file_key);
  }, [attachments]);

  if (!normalizedAttachments.length) {
    return null;
  }

  return (
    <Box
      sx={{
        mb: 6,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: '10px',
        bgcolor: 'background.paper3',
        p: '20px',
        backdropFilter: 'blur(5px)',
      }}
    >
      <Typography sx={{ fontWeight: 'bold', mb: 2, lineHeight: '22px' }}>
        附件
      </Typography>
      <Table
        size='small'
        sx={{
          tableLayout: 'fixed',
          '& th, & td': {
            px: 1.5,
            py: 1.25,
            borderColor: 'divider',
          },
        }}
      >
        <TableBody>
          {normalizedAttachments.map((attachment, index) => {
            const fileUrl = `/static-file/${attachment.file_key}`;
            const previewUrl = attachment.preview_key
              ? `/static-file/${attachment.preview_key}`
              : fileUrl;
            const previewable = canPreviewInline(attachment);

            return (
              <TableRow key={`${attachment.file_key}-${index}`}>
                <TableCell>
                  <Typography sx={{ fontWeight: 500, wordBreak: 'break-all' }}>
                    {attachment.file_name || '未命名附件'}
                  </Typography>
                </TableCell>
                <TableCell align='right'>
                  <Stack
                    direction='row'
                    gap={1}
                    justifyContent='flex-end'
                    flexWrap='wrap'
                  >
                    {previewable && (
                      <Button
                        component='a'
                        href={previewUrl}
                        target='_blank'
                        rel='noreferrer'
                        variant='outlined'
                      >
                        预览
                      </Button>
                    )}
                    <Button
                      component='a'
                      href={fileUrl}
                      download={attachment.file_name || 'attachment'}
                      target='_blank'
                      rel='noreferrer'
                      variant='contained'
                    >
                      下载
                    </Button>
                  </Stack>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </Box>
  );
};

export default AttachmentList;
