import { getApiV1NodeDetail, putApiV1NodeDetail } from '@/request/Node';
import type { V1NodeDetailResp } from '@/request/types';
import { useAppSelector } from '@/store';
import FilePreview from '@/pages/document/editor/edit/FilePreview';
import EditorWrap from '@/pages/document/editor/edit/Wrap';
import type { WrapContext } from '@/pages/document/editor';
import EmptyState from '@/components/EmptyState';
import Loading from '@/components/Loading';
import { Box, IconButton, Stack, Tooltip } from '@mui/material';
import { IconMuluzhankai } from '@panda-wiki/icons';
import FullscreenIcon from '@mui/icons-material/Fullscreen';
import FullscreenExitIcon from '@mui/icons-material/FullscreenExit';
import { useEffect, useRef, useState } from 'react';

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
  const saveCurrentDocRef = useRef<(() => Promise<void>) | null>(null);

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
            <FilePreview detail={detail} />
          ) : (
            <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
              <EditorWrap
                detail={detail}
                embedded
                embeddedActions={
                  <Stack
                    direction='row'
                    spacing={0.25}
                    alignItems='center'
                    sx={{ flexWrap: 'nowrap', whiteSpace: 'nowrap' }}
                  >
                    <Tooltip title='收起预览' placement='bottom'>
                      <IconButton
                        size='small'
                        aria-label='收起预览'
                        onClick={onToggle}
                        sx={{
                          width: 28,
                          height: 28,
                          flexShrink: 0,
                        }}
                      >
                        <IconMuluzhankai sx={{ fontSize: 20 }} />
                      </IconButton>
                    </Tooltip>
                    <Tooltip
                      title={fullscreen ? '退出全屏编辑' : '全屏编辑'}
                      placement='bottom'
                    >
                      <IconButton
                        size='small'
                        aria-label={fullscreen ? '退出全屏编辑' : '全屏编辑'}
                        onClick={onToggleFullscreen}
                        sx={{
                          width: 28,
                          height: 28,
                          flexShrink: 0,
                        }}
                      >
                        {fullscreen ? (
                          <FullscreenExitIcon fontSize='small' />
                        ) : (
                          <FullscreenIcon fontSize='small' />
                        )}
                      </IconButton>
                    </Tooltip>
                  </Stack>
                }
                context={
                  {
                    catalogOpen: false,
                    setCatalogOpen: () => {},
                    nodeDetail: detail,
                    setNodeDetail: value =>
                      setDetail(previous => ({ ...previous, ...value })),
                    onSave: async (content, editorState) => {
                      if (!kb_id || !detail.id) return;
                      await putApiV1NodeDetail({
                        kb_id,
                        id: detail.id,
                        nav_id: detail.nav_id || '',
                        name: detail.name || '',
                        content,
                        editor_format:
                          editorState?.editor_format || 'legacy-string',
                        editor_version: editorState?.editor_version || 1,
                        ...(editorState?.editor_content
                          ? { editor_content: editorState.editor_content }
                          : {}),
                        ...(editorState?.editor_blocks
                          ? { editor_blocks: editorState.editor_blocks }
                          : {}),
                      });
                      setDetail(previous => ({
                        ...previous,
                        content,
                        ...(editorState?.editor_content
                          ? { editor_content: editorState.editor_content }
                          : {}),
                        ...(editorState?.editor_blocks
                          ? { editor_blocks: editorState.editor_blocks }
                          : {}),
                      }));
                    },
                    docWidth: 'full',
                    catalogData: [],
                    groups: [],
                    nav_id: detail.nav_id || '',
                    refreshCatalog: async () => [],
                    saveCurrentDocRef,
                  } satisfies WrapContext
                }
              />
            </Box>
          )}
        </Stack>
      )}
    </Box>
  );
};

export default DocPagePreview;
