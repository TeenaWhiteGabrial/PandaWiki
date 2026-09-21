import Upload from '@/components/UploadFile/Drag';
import Emoji from '@/components/Emoji';
import { DomainCreateNodeReq, V1NodeDetailResp } from '@/request';
import { postApiV1Node, putApiV1NodeDetail } from '@/request/Node';
import { postApiV1FilePreviewGenerate } from '@/request/FilePreview';
import { useAppSelector } from '@/store';
import { message, Modal } from '@/components/ui';
import {
  Box,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  TextField,
} from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { uploadAttachment } from '@/api';
import { DomainNodeAttachmentMeta } from '@/request/types';

type FormValues = { name: string; emoji: string; content_type: string };

interface DocAddByCustomTextProps {
  open: boolean;
  data?: V1NodeDetailResp;
  autoJump?: boolean;
  onClose: () => void;
  parentId?: string;
  setDetail?: (data: V1NodeDetailResp) => void;
  refresh?: () => void;
  type?: 1 | 2;
  onCreated?: (node: {
    id: string;
    name: string;
    type: 1 | 2;
    content_type?: string;
    emoji?: string;
  }) => void;
}

const DocAddByCustomText = ({
  open,
  data,
  autoJump = true,
  parentId,
  onClose,
  refresh,
  setDetail,
  type = 2,
  onCreated,
}: DocAddByCustomTextProps) => {
  const { kb_id: id, nav_id } = useAppSelector(state => state.config);
  const text = type === 1 ? '文件夹' : '文档';
  const [attachments, setAttachments] = useState<File[]>([]);
  const [submitting, setSubmitting] = useState(false);

  const {
    control,
    handleSubmit,
    reset,
    setValue,
    formState: { errors },
  } = useForm<FormValues>({
    defaultValues: {
      name: '',
      emoji: '',
      content_type: '',
    },
  });

  const handleClose = () => {
    reset();
    setAttachments([]);
    onClose();
  };

  const getFileExt = useCallback((name?: string) => {
    return (name?.split('.').pop() || '').trim().toLowerCase();
  }, []);

  const buildAttachments = useCallback(async () => {
    const nextAttachments: DomainNodeAttachmentMeta[] = [];
    for (const file of attachments) {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('kb_id', id);
      const uploaded = await uploadAttachment(formData);
      const fileType = getFileExt(uploaded.filename || file.name);
      const previewKey =
        fileType === 'pdf'
          ? uploaded.key
          : ['doc', 'docx', 'ppt', 'pptx', 'xls', 'xlsx'].includes(fileType)
            ? (
                await postApiV1FilePreviewGenerate({
                  kb_id: id,
                  file_key: uploaded.key,
                  file_name: uploaded.filename || file.name,
                })
              )?.preview_key
            : undefined;
      nextAttachments.push({
        file_key: uploaded.key,
        file_name: uploaded.filename || file.name,
        file_type: fileType,
        preview_key: previewKey,
      });
    }
    return nextAttachments;
  }, [attachments, getFileExt, id]);

  const submit = async (value: FormValues) => {
    setSubmitting(true);
    if (data) {
      putApiV1NodeDetail({
        id: data.id || '',
        kb_id: id,
        nav_id: data.nav_id || nav_id || '',
        name: value.name,
        emoji: value.emoji,
      })
        .then(() => {
          message.success('修改成功');
          reset();
          handleClose();
          refresh?.();
          setDetail?.({
            name: value.name,
            meta: { ...data.meta, emoji: value.emoji },
            status: 1,
          });
        })
        .finally(() => {
          setSubmitting(false);
        });
    } else {
      if (!id) {
        setSubmitting(false);
        return;
      }
      try {
        const nextAttachments = type === 2 ? await buildAttachments() : [];
        const params: DomainCreateNodeReq = {
          name: value.name,
          content: '',
          kb_id: id,
          nav_id: nav_id || '',
          type,
          emoji: value.emoji,
          content_type: value.content_type,
          meta:
            type === 2 && nextAttachments.length > 0
              ? { attachments: nextAttachments }
              : undefined,
        };
        if (parentId) {
          params.parent_id = parentId;
        }
        const { id: nodeId } = await postApiV1Node(params);
        message.success('创建成功');
        reset();
        handleClose();
        onCreated?.({
          id: nodeId,
          name: value.name,
          type,
          content_type: value.content_type,
          emoji: value.emoji,
        });
        refresh?.();
        if (type === 2 && autoJump) {
          window.open(`/doc/editor/${nodeId}`, '_blank');
        }
      } finally {
        setSubmitting(false);
      }
    }
  };

  useEffect(() => {
    if (!open) return;
    if (data) {
      reset({
        name: data.name || '',
        emoji: data.meta?.emoji || '',
        content_type: type === 1 ? '' : data.meta?.content_type || 'html',
      });
    } else {
      setValue('content_type', type === 1 ? '' : 'html');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, type, open]);

  return (
    <Modal
      title={data ? `编辑${text}` : `创建${text}`}
      open={open}
      width={600}
      okText={data ? '保存' : '创建'}
      okButtonProps={{ loading: submitting }}
      onCancel={handleClose}
      onOk={handleSubmit(submit)}
    >
      <Box sx={{ fontSize: 14, lineHeight: '36px' }}>{text}名称</Box>
      <Controller
        control={control}
        name='name'
        rules={{ required: `请输入${text}名称` }}
        render={({ field }) => (
          <TextField
            {...field}
            fullWidth
            autoFocus
            size='small'
            placeholder={`请输入${text}名称`}
            error={!!errors.name}
            helperText={errors.name?.message}
          />
        )}
      />
      <Box sx={{ fontSize: 14, lineHeight: '36px', mt: 1 }}>{text}图标</Box>
      <Controller
        control={control}
        name='emoji'
        render={({ field }) => <Emoji {...field} type={type} />}
      />
      {type === 2 && !data && (
        <>
          <Box sx={{ fontSize: 14, lineHeight: '36px', mt: 1 }}>文档类型</Box>
          <Controller
            control={control}
            name='content_type'
            render={({ field }) => (
              <RadioGroup {...field} row>
                <FormControlLabel
                  value='html'
                  control={<Radio size='small' />}
                  label='富文本'
                />
                <FormControlLabel
                  value='md'
                  control={<Radio size='small' />}
                  label='Markdown'
                />
              </RadioGroup>
            )}
          />
          <Box sx={{ fontSize: 14, lineHeight: '36px', mt: 1 }}>附件</Box>
          <Stack gap={1}>
            <Upload
              multiple
              type='drag'
              onChange={acceptedFiles => setAttachments(acceptedFiles)}
            />
            {attachments.length > 0 && (
              <Box sx={{ fontSize: 12, color: 'text.secondary' }}>
                已选择 {attachments.length} 个附件
              </Box>
            )}
          </Stack>
        </>
      )}
    </Modal>
  );
};

export default DocAddByCustomText;
