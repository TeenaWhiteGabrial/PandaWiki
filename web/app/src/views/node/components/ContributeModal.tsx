'use client';

import { useBasePath } from '@/hooks';
import { useStore } from '@/provider';
import { postShareProV1ContributeSubmit } from '@/request/pro';
import { V1ShareNodeDetailResp } from '@/request/types';
import { Editor, useTiptap } from '@ctzhian/tiptap';
import { Modal, message } from '@ctzhian/ui';
import {
  Box,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  Stack,
  TextField,
} from '@mui/material';
import { useParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';

interface ContributeModalProps {
  open: boolean;
  onClose: () => void;
  info: V1ShareNodeDetailResp;
}

const ContributeModal = ({ open, onClose, info }: ContributeModalProps) => {
  const basePath = useBasePath();
  const { kbDetail } = useStore();
  const params = useParams() || {};
  const nodeId = params.id as string;
  const [loading, setLoading] = useState(false);
  const [type, setType] = useState<'edit' | 'add'>('edit');
  const [name, setName] = useState(info.name || '');
  const [reason, setReason] = useState('');
  const [editorReady, setEditorReady] = useState(false);
  const contributeEnabled =
    kbDetail?.settings?.contribute_settings?.is_enable ?? false;

  const isMarkdown = useMemo(() => {
    return info.meta?.content_type === 'md';
  }, [info.meta?.content_type]);

  const editorRef = useTiptap({
    content: type === 'edit' ? info.content || '' : '',
    editable: true,
    contentType: isMarkdown ? 'markdown' : 'html',
    immediatelyRender: false,
    baseUrl: basePath,
    onCreate: () => {
      setEditorReady(true);
    },
  });

  useEffect(() => {
    setEditorReady(false);
  }, [type]);

  useEffect(() => {
    if (!editorRef.editor) return;
    const nextContent = type === 'edit' ? info.content || '' : '';
    editorRef.setContent(nextContent, isMarkdown ? 'markdown' : 'html');
    setEditorReady(true);
  }, [editorRef.editor, editorRef, info.content, isMarkdown, type]);

  const handleSubmit = async () => {
    if (!contributeEnabled) {
      message.error('当前知识库未开启文档贡献');
      onClose();
      return;
    }
    if (!reason.trim()) {
      message.error('请填写贡献说明');
      return;
    }
    if (type === 'add' && !name.trim()) {
      message.error('请填写文档名称');
      return;
    }
    const contentValue =
      (isMarkdown ? editorRef.getMarkdown() : editorRef.getHTML()) || '';
    if (!contentValue.trim()) {
      message.error('请填写贡献内容');
      return;
    }

    setLoading(true);
    try {
      const Cap = (await import('@cap.js/widget')).default;
      const cap = new Cap({ apiEndpoint: `${basePath}/share/v1/captcha/` });
      const solution = await cap.solve();
      await postShareProV1ContributeSubmit(
        {
          captcha_token: solution.token,
          content: contentValue.trim(),
          content_type: isMarkdown ? 'md' : 'html',
          emoji: info.meta?.emoji || '',
          name: type === 'add' ? name.trim() : info.name,
          node_id: type === 'edit' ? nodeId : undefined,
          reason: reason.trim(),
          type,
        },
        {
          headers: {
            'X-KB-ID': info.kb_id || '',
          },
        },
      );
      message.success('贡献已提交，等待管理员审核');
      setReason('');
      setName(type === 'edit' ? info.name || '' : '');
      if (editorRef.editor) {
        editorRef.setContent(
          type === 'edit' ? info.content || '' : '',
          isMarkdown ? 'markdown' : 'html',
        );
      }
      onClose();
    } catch (error: any) {
      message.error(error?.message || '提交贡献失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title='提交文档贡献'
      open={open && contributeEnabled}
      onCancel={onClose}
      onOk={handleSubmit}
      okText='提交'
      okButtonProps={{ loading }}
      width={720}
    >
      <Stack gap={2} sx={{ pt: 1 }}>
        <FormControl size='small' fullWidth>
          <InputLabel>贡献类型</InputLabel>
          <Select
            label='贡献类型'
            value={type}
            onChange={event => {
              const nextType = event.target.value as 'edit' | 'add';
              setType(nextType);
              setName(nextType === 'edit' ? info.name || '' : '');
            }}
          >
            <MenuItem value='edit'>修改当前文档</MenuItem>
            <MenuItem value='add'>新增文档内容</MenuItem>
          </Select>
        </FormControl>
        {type === 'add' && (
          <TextField
            size='small'
            label='文档名称'
            value={name}
            onChange={event => setName(event.target.value)}
          />
        )}
        <TextField
          size='small'
          label='贡献说明'
          value={reason}
          onChange={event => setReason(event.target.value)}
          placeholder='说明你希望修改/新增什么，以及原因'
        />
        <Stack gap={1}>
          <Box sx={{ fontSize: 14, color: 'text.secondary' }}>贡献内容</Box>
          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1.5,
              minHeight: 360,
              overflow: 'hidden',
              '.tiptap.ProseMirror': {
                minHeight: 360,
                p: 2,
              },
            }}
          >
            {editorRef.editor && editorReady ? (
              <Editor editor={editorRef.editor} />
            ) : (
              <Stack sx={{ p: 2 }} gap={1}>
                <Skeleton height={24} width='30%' />
                <Skeleton height={22} width='100%' />
                <Skeleton height={22} width='95%' />
                <Skeleton height={22} width='88%' />
              </Stack>
            )}
          </Box>
        </Stack>
      </Stack>
    </Modal>
  );
};

export default ContributeModal;
