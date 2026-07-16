import { uploadAttachment } from '@/api';
import { putApiV1NodeDetail } from '@/request';
import { postApiV1FilePreviewGenerate } from '@/request/FilePreview';
import { DomainNodeAttachmentMeta, V1NodeDetailResp } from '@/request/types';
import { message } from '@/components/ui';
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material';
import { IconShanchu, IconWenjian } from '@panda-wiki/icons';
import { useCallback, useMemo, useRef, useState } from 'react';

interface AttachmentListProps {
  detail: V1NodeDetailResp;
  readOnlyUser?: boolean;
  updateDetail: (detail: V1NodeDetailResp) => void;
}

const AttachmentList = ({
  detail,
  readOnlyUser = false,
  updateDetail,
}: AttachmentListProps) => {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [uploading, setUploading] = useState(false);
  const [removingKey, setRemovingKey] = useState<string | null>(null);

  const attachments = useMemo(
    () => detail.meta?.attachments || [],
    [detail.meta?.attachments],
  );

  const getPreviewUrl = useCallback((attachment: DomainNodeAttachmentMeta) => {
    const fileType = (attachment.file_type || '').toLowerCase();
    if (fileType === 'pdf' && attachment.file_key) {
      return `/static-file/${attachment.file_key}`;
    }
    if (
      ['doc', 'docx', 'ppt', 'pptx', 'xls', 'xlsx'].includes(fileType) &&
      attachment.preview_key
    ) {
      return `/static-file/${attachment.preview_key}`;
    }
    return '';
  }, []);

  const buildAttachmentMeta = useCallback(
    async (file: File): Promise<DomainNodeAttachmentMeta> => {
      const formData = new FormData();
      formData.append('file', file);
      const uploaded = await uploadAttachment(formData);
      const fileType = (uploaded.filename?.split('.').pop() || '')
        .trim()
        .toLowerCase();
      const previewKey =
        fileType === 'pdf'
          ? uploaded.key
          : ['doc', 'docx', 'ppt', 'pptx', 'xls', 'xlsx'].includes(fileType)
            ? (
                await postApiV1FilePreviewGenerate({
                  kb_id: detail.kb_id || '',
                  file_key: uploaded.key,
                  file_name: uploaded.filename || file.name,
                })
              )?.preview_key
            : undefined;
      return {
        file_key: uploaded.key,
        file_name: uploaded.filename || file.name,
        file_type: fileType,
        preview_key: previewKey,
      };
    },
    [detail.kb_id],
  );

  const persistAttachments = useCallback(
    async (nextAttachments: DomainNodeAttachmentMeta[]) => {
      const nextMeta = {
        ...detail.meta,
        attachments: nextAttachments,
      };
      await putApiV1NodeDetail({
        id: detail.id || '',
        kb_id: detail.kb_id || '',
        nav_id: detail.nav_id || '',
        meta: nextMeta,
      });
      updateDetail({
        meta: nextMeta,
        updated_at: new Date().toISOString(),
      });
    },
    [detail.id, detail.kb_id, detail.meta, detail.nav_id, updateDetail],
  );

  const handleUploadFiles = useCallback(
    async (files: FileList | null) => {
      if (!files || files.length === 0 || readOnlyUser || uploading) {
        return;
      }
      setUploading(true);
      try {
        const nextItems: DomainNodeAttachmentMeta[] = [];
        for (const file of Array.from(files)) {
          nextItems.push(await buildAttachmentMeta(file));
        }
        await persistAttachments([...attachments, ...nextItems]);
        message.success('附件已上传');
      } catch (error) {
        console.error(error);
        message.error('附件上传失败');
      } finally {
        setUploading(false);
        if (inputRef.current) {
          inputRef.current.value = '';
        }
      }
    },
    [
      attachments,
      buildAttachmentMeta,
      persistAttachments,
      readOnlyUser,
      uploading,
    ],
  );

  const handleRemove = useCallback(
    async (attachment: DomainNodeAttachmentMeta, index: number) => {
      if (readOnlyUser || !attachment.file_key) {
        return;
      }
      setRemovingKey(attachment.file_key);
      try {
        const nextAttachments = attachments.filter((item, currentIndex) => {
          if (currentIndex !== index) {
            return true;
          }
          return item.file_key !== attachment.file_key;
        });
        await persistAttachments(nextAttachments);
        message.success('附件已移除');
      } catch (error) {
        console.error(error);
        message.error('移除附件失败');
      } finally {
        setRemovingKey(null);
      }
    },
    [attachments, persistAttachments, readOnlyUser],
  );

  if (attachments.length === 0 && readOnlyUser) {
    return null;
  }

  return (
    <Stack
      spacing={1.5}
      sx={{
        mt: 3,
        p: 2,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        bgcolor: 'background.paper',
      }}
    >
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        gap={2}
      >
        <Stack direction='row' alignItems='center' gap={1}>
          <Typography variant='body1' fontWeight={600}>
            附件
          </Typography>
          <Chip
            size='small'
            label={`${attachments.length} 个文件`}
            sx={{ height: 22 }}
          />
        </Stack>
        {!readOnlyUser && (
          <>
            <input
              ref={inputRef}
              hidden
              type='file'
              multiple
              onChange={event => handleUploadFiles(event.target.files)}
            />
            <Button
              size='small'
              variant='outlined'
              disabled={uploading}
              onClick={() => inputRef.current?.click()}
            >
              {uploading ? '上传中...' : '添加附件'}
            </Button>
          </>
        )}
      </Stack>

      {attachments.length === 0 ? (
        <Box sx={{ fontSize: 13, color: 'text.secondary' }}>暂无附件</Box>
      ) : (
        attachments.map((attachment, index) => {
          const fileUrl = attachment.file_key
            ? `/static-file/${attachment.file_key}`
            : '';
          const previewUrl = getPreviewUrl(attachment);
          const removable =
            !readOnlyUser &&
            !!attachment.file_key &&
            removingKey !== attachment.file_key;
          return (
            <Stack
              key={`${attachment.file_key || attachment.file_name || 'attachment'}-${index}`}
              direction='row'
              alignItems='center'
              justifyContent='space-between'
              gap={2}
              sx={{
                p: 1.5,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1.5,
              }}
            >
              <Stack
                direction='row'
                alignItems='center'
                gap={1.5}
                sx={{ minWidth: 0 }}
              >
                <IconWenjian sx={{ fontSize: 18, color: 'text.secondary' }} />
                <Box sx={{ minWidth: 0 }}>
                  <Typography
                    variant='body2'
                    fontWeight={500}
                    noWrap
                    sx={{ maxWidth: 360 }}
                  >
                    {attachment.file_name || `附件 ${index + 1}`}
                  </Typography>
                  <Stack
                    direction='row'
                    alignItems='center'
                    gap={1}
                    sx={{ mt: 0.25 }}
                  >
                    <Typography variant='caption' color='text.secondary'>
                      {(attachment.file_type || '').toUpperCase() || 'FILE'}
                    </Typography>
                    <Chip
                      size='small'
                      label={previewUrl ? '可预览' : '仅下载'}
                      sx={{
                        height: 18,
                        fontSize: 10,
                        bgcolor: previewUrl
                          ? 'success.light'
                          : 'background.paper2',
                        color: previewUrl
                          ? 'success.contrastText'
                          : 'text.secondary',
                      }}
                    />
                  </Stack>
                </Box>
              </Stack>
              <Stack direction='row' spacing={1} alignItems='center'>
                {previewUrl && (
                  <Button
                    size='small'
                    variant='outlined'
                    href={previewUrl}
                    target='_blank'
                    rel='noreferrer'
                  >
                    预览
                  </Button>
                )}
                {fileUrl && (
                  <Button
                    size='small'
                    variant='text'
                    href={fileUrl}
                    download={attachment.file_name || `attachment-${index + 1}`}
                    target='_blank'
                    rel='noreferrer'
                  >
                    下载
                  </Button>
                )}
                {!readOnlyUser && (
                  <Tooltip title='删除附件'>
                    <span>
                      <IconButton
                        size='small'
                        disabled={!removable}
                        onClick={() => handleRemove(attachment, index)}
                      >
                        {removingKey === attachment.file_key ? (
                          <CircularProgress size={16} />
                        ) : (
                          <IconShanchu sx={{ fontSize: 16 }} />
                        )}
                      </IconButton>
                    </span>
                  </Tooltip>
                )}
              </Stack>
            </Stack>
          );
        })
      )}
    </Stack>
  );
};

export default AttachmentList;
