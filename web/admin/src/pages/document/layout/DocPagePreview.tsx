import { getApiV1NodeDetail } from '@/request/Node';
import type { V1NodeDetailResp } from '@/request/types';
import { useAppSelector } from '@/store';
import FilePreview from '@/pages/document/editor/edit/FilePreview';
import MarkDown from '@/components/MarkDown';
import EmptyState from '@/components/EmptyState';
import Loading from '@/components/Loading';
import { Box, IconButton, Stack, Tooltip } from '@mui/material';
import { IconMuluzhankai } from '@panda-wiki/icons';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';
import FullscreenIcon from '@mui/icons-material/Fullscreen';
import FullscreenExitIcon from '@mui/icons-material/FullscreenExit';
import { useEffect, useState } from 'react';

interface DocPagePreviewProps {
  documentId: string | null;
  collapsed: boolean;
  onToggle: () => void;
  fullscreen: boolean;
  onToggleFullscreen: () => void;
}

const DocPagePreview = ({
  documentId,
  collapsed,
  onToggle,
  fullscreen,
  onToggleFullscreen,
}: DocPagePreviewProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [detail, setDetail] = useState<V1NodeDetailResp | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!documentId || !kb_id) {
      setDetail(null);
      return;
    }
    let active = true;
    setLoading(true);
    getApiV1NodeDetail({ id: documentId, kb_id })
      .then(value => {
        if (active) setDetail(value);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [documentId, kb_id]);

  const previewActions = (
    <Stack
      direction='row'
      justifyContent='flex-end'
      alignItems='center'
      spacing={0.5}
      sx={{
        px: 1,
        py: 0.5,
        borderBottom: 1,
        borderColor: 'divider',
        flexShrink: 0,
      }}
    >
      <Tooltip title='收起预览'>
        <IconButton size='small' onClick={onToggle}>
          <IconMuluzhankai />
        </IconButton>
      </Tooltip>
      {detail && !detail.meta?.file_readonly ? (
        <Tooltip title='全屏编辑'>
          <IconButton
            size='small'
            onClick={() => window.open(`/doc/editor/${detail.id}`, '_blank')}
          >
            <EditOutlinedIcon />
          </IconButton>
        </Tooltip>
      ) : (
        <Tooltip title={fullscreen ? '退出全屏预览' : '全屏预览'}>
          <IconButton size='small' onClick={onToggleFullscreen}>
            {fullscreen ? <FullscreenExitIcon /> : <FullscreenIcon />}
          </IconButton>
        </Tooltip>
      )}
    </Stack>
  );

  return (
    <Box
      sx={{
        position: 'relative',
        flex: collapsed ? '0 0 0' : 1,
        minWidth: 0,
        transition: 'flex-basis 0.15s ease-in-out',
        isolation: 'isolate',
      }}
    >
      {!collapsed && (
        <Stack
          sx={{
            height: fullscreen ? 'calc(100vh - 96px)' : 'calc(100vh - 183px)',
            overflow: 'hidden',
            bgcolor: 'background.paper',
            borderRadius: 2,
            border: '1px solid',
            borderColor: 'divider',
          }}
        >
          {loading ? (
            <Loading sx={{ flex: 1, justifyContent: 'center' }} />
          ) : !detail ? (
            <EmptyState text='请选择一个文档查看预览' sx={{ flex: 1 }} />
          ) : detail.meta?.file_readonly ? (
            <Stack sx={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
              {previewActions}
              <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
                <FilePreview detail={detail} />
              </Box>
            </Stack>
          ) : (
            <Stack sx={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
              {previewActions}
              <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto', p: 3 }}>
                <MarkDown content={detail.content || ''} />
              </Box>
            </Stack>
          )}
        </Stack>
      )}
    </Box>
  );
};

export default DocPagePreview;
