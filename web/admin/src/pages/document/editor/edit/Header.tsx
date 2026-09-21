import { ITreeItem } from '@/api';
import Cascader from '@/components/Cascader';
import VersionPublish from '@/pages/release/components/VersionPublish';
import { postApiV1Node } from '@/request';
import { V1NodeDetailResp } from '@/request/types';
import { useAppSelector } from '@/store';
import { addOpacityToColor, getShortcutKeyText } from '@/utils';
import { Ellipsis, message } from '@/components/ui';
import {
  Box,
  Button,
  IconButton,
  Skeleton,
  Stack,
  styled,
  Tooltip,
  useTheme,
} from '@mui/material';
import {
  IconBaocun,
  IconDaochu,
  IconGengduo,
  IconMuluzhankai,
} from '@panda-wiki/icons';
import dayjs from 'dayjs';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { useEditorContext } from '..';
import DocAddByCustomText from '../../component/DocAddByCustomText';
import DocDelete from '../../component/DocDelete';

interface HeaderProps {
  edit: boolean;
  detail: V1NodeDetailResp;
  updateDetail: (detail: V1NodeDetailResp) => void;
  handleSave: () => void;
  handleExport: (type: string) => void;
  handleConvertContentType?: (type: 'html' | 'md') => void;
  readOnlyUser?: boolean;
  embedded?: boolean;
  embeddedActions?: ReactNode;
}

const Header = ({
  edit,
  detail,
  updateDetail,
  handleSave,
  handleExport,
  handleConvertContentType,
  readOnlyUser = false,
  embedded = false,
  embeddedActions,
}: HeaderProps) => {
  const theme = useTheme();
  const navigate = useNavigate();
  const firstLoad = useRef(true);
  const saveTipTimerRef = useRef<number | null>(null);

  const { kb_id, nav_id, kbDetail } = useAppSelector(state => state.config);

  const {
    catalogOpen,
    nodeDetail,
    setCatalogOpen,
    refreshCatalog,
    catalogData,
  } = useEditorContext()!;

  const [renameOpen, setRenameOpen] = useState(false);
  const [delOpen, setDelOpen] = useState(false);
  const [publishOpen, setPublishOpen] = useState(false);

  const [showSaveTip, setShowSaveTip] = useState(false);
  const isReadonlyFilePreview =
    !!detail.meta?.file_key && !!detail.meta?.file_readonly;

  const handlePublish = useCallback(() => {
    if (readOnlyUser) return;
    if (nodeDetail?.status === 2 && !edit) {
      message.info('当前已是最新版本！');
    } else {
      handleSave();
      setTimeout(() => {
        setPublishOpen(true);
      }, 200);
    }
  }, [nodeDetail, edit, handleSave, readOnlyUser]);

  const handleDeleteAndNavigate = async () => {
    // 深度优先遍历树，按照目录顺序收集所有文档
    const collectDocs = (items: ITreeItem[]): ITreeItem[] => {
      const docs: ITreeItem[] = [];
      // 先对 items 按 order 排序，与 Catalog 渲染顺序保持一致
      const sortedItems = [...items].sort(
        (a, b) => (a.order ?? 0) - (b.order ?? 0),
      );
      sortedItems.forEach(item => {
        if (item.type === 2) {
          // 是文档，添加到列表
          docs.push(item);
        }
        if (item.children && item.children.length > 0) {
          // 递归处理子节点
          docs.push(...collectDocs(item.children));
        }
      });
      return docs;
    };

    // 使用删除前的原始数据查找下一个文档
    const allDocs = collectDocs(catalogData);

    // 找到下一个文档
    let nextDoc = null;
    if (allDocs.length > 0) {
      // 找到当前文档在列表中的索引
      const currentIndex = allDocs.findIndex(
        (doc: ITreeItem) => doc.id === detail.id,
      );

      if (currentIndex !== -1) {
        // 如果当前文档不是最后一个，选择下一个文档
        if (currentIndex < allDocs.length - 1) {
          nextDoc = allDocs[currentIndex + 1];
        }
        // 如果当前文档是最后一个但不是唯一的，选择前一个文档
        else if (allDocs.length > 1) {
          nextDoc = allDocs[currentIndex - 1];
        }
      }
    }

    // 刷新目录数据（删除后更新目录显示）
    await refreshCatalog();

    // 导航到下一个文档或首页
    if (nextDoc) {
      // 有其他文档，导航到下一个文档
      navigate(`/doc/editor/${nextDoc.id}`);
    } else {
      // 没有其他文档，回到首页
      navigate('/');
    }
  };

  useEffect(() => {
    const updatedAt = detail.updated_at || nodeDetail?.updated_at;
    if (updatedAt && !firstLoad.current && !edit) {
      setShowSaveTip(true);
      if (saveTipTimerRef.current) {
        window.clearTimeout(saveTipTimerRef.current);
      }
      saveTipTimerRef.current = window.setTimeout(() => {
        setShowSaveTip(false);
        saveTipTimerRef.current = null;
      }, 1500);
    }
    firstLoad.current = false;
    return () => {
      if (saveTipTimerRef.current) {
        window.clearTimeout(saveTipTimerRef.current);
        saveTipTimerRef.current = null;
      }
    };
  }, [detail.updated_at, edit, nodeDetail?.updated_at]);

  return (
    <Box sx={{ p: 1 }}>
      <Stack
        direction={'row'}
        alignItems={'center'}
        gap={1}
        justifyContent={'space-between'}
        sx={{ height: '40px' }}
      >
        <Stack
          direction='row'
          alignItems='center'
          gap={1}
          sx={{ width: 0, flex: 1, minWidth: 0 }}
        >
          {!embedded && !catalogOpen ? (
            <Stack
              alignItems='center'
              justifyContent='space-between'
              onClick={() => setCatalogOpen(true)}
              sx={{
                cursor: 'pointer',
                color: 'text.tertiary',
                ':hover': {
                  color: 'text.primary',
                },
              }}
            >
              <IconMuluzhankai sx={{ fontSize: 24 }} />
            </Stack>
          ) : null}
          {detail.meta?.content_type === 'md' && (
            <Box
              component={'span'}
              sx={{
                fontSize: 10,
                color: 'white',
                background: 'linear-gradient(135deg, #a8edea 0%, #fed6e3 100%)',
                px: 1,
                flexShrink: 0,
                fontWeight: '500',
                borderRadius: '4px',
                height: '20px',
                lineHeight: '20px',
                display: 'inline-block',
              }}
            >
              MD
            </Box>
          )}
          <Stack sx={{ width: 0, flex: 1, minWidth: 0 }}>
            <Stack
              direction='row'
              alignItems='center'
              gap={0.5}
              sx={{ minWidth: 0 }}
            >
              {embedded && embeddedActions}
              {detail?.name ? (
                <Ellipsis
                  sx={{
                    flex: 1,
                    minWidth: 0,
                    fontSize: 14,
                    fontWeight: 'bold',
                  }}
                >
                  <Box
                    component='span'
                    sx={{ cursor: 'pointer' }}
                    // onClick={() => setRenameOpen(true)}
                  >
                    {detail.name}
                  </Box>
                </Ellipsis>
              ) : (
                <Skeleton variant='text' width={300} height={24} />
              )}
            </Stack>
            <Stack
              direction={'row'}
              alignItems={'center'}
              gap={0.5}
              sx={{ fontSize: 12, color: 'text.tertiary' }}
            >
              <IconBaocun sx={{ fontSize: 12 }} />
              {showSaveTip ? (
                '已保存'
              ) : detail.updated_at || nodeDetail?.updated_at ? (
                dayjs(detail.updated_at || nodeDetail?.updated_at).format(
                  'YYYY-MM-DD HH:mm:ss',
                )
              ) : (
                <Skeleton variant='text' width={100} height={24} />
              )}
            </Stack>
          </Stack>
        </Stack>
        <Stack direction={'row'} gap={1}>
          {!readOnlyUser && (
            <Cascader
              list={[
                {
                  key: 'copy',
                  textSx: { flex: 1 },
                  label: <StyledMenuSelect>创建副本</StyledMenuSelect>,
                  onClick: () => {
                    if (readOnlyUser) return;
                    if (kb_id) {
                      postApiV1Node({
                        name: detail.name + ' [副本]',
                        content: detail.content,
                        kb_id,
                        nav_id: nav_id || detail.nav_id || '',
                        parent_id: detail.parent_id || undefined,
                        type: 2,
                        emoji: detail.meta?.emoji,
                      }).then(res => {
                        message.success('复制成功');
                        window.open(`/doc/editor/${res.id}`, '_blank');
                      });
                    }
                  },
                },
                ...(!isReadonlyFilePreview
                  ? [
                      {
                        key: 'convert_content_type',
                        textSx: { flex: 1 },
                        label: (
                          <StyledMenuSelect>
                            转为
                            {detail.meta?.content_type === 'md'
                              ? '富文本'
                              : 'Markdown'}
                          </StyledMenuSelect>
                        ),
                        onClick: () => {
                          if (readOnlyUser) return;
                          handleConvertContentType?.(
                            detail.meta?.content_type === 'md' ? 'html' : 'md',
                          );
                        },
                      },
                    ]
                  : []),
                {
                  key: 'version',
                  textSx: { flex: 1 },
                  label: <StyledMenuSelect>历史版本</StyledMenuSelect>,
                  onClick: () => {
                    navigate(`/doc/editor/history/${detail.id}`);
                  },
                },
                {
                  key: 'rename',
                  textSx: { flex: 1 },
                  label: <StyledMenuSelect>重命名</StyledMenuSelect>,
                  onClick: () => {
                    if (readOnlyUser) return;
                    setRenameOpen(true);
                  },
                },
                {
                  key: 'delete',
                  textSx: { flex: 1 },
                  label: <StyledMenuSelect type='error'>删除</StyledMenuSelect>,
                  onClick: () => {
                    if (readOnlyUser) return;
                    setDelOpen(true);
                  },
                },
              ]}
              context={
                <IconButton
                  size='small'
                  disabled={!detail.name}
                  sx={{ flexShrink: 0 }}
                >
                  <IconGengduo sx={{ fontSize: 14 }} />
                </IconButton>
              }
            />
          )}
          <Cascader
            list={[
              {
                key: 'html',
                label: (
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{
                      fontSize: 14,
                      px: 2,
                      lineHeight: '40px',
                      height: 40,
                      width: 140,
                      borderRadius: '5px',
                      cursor: 'pointer',
                      ':hover': {
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                      },
                    }}
                  >
                    导出 HTML
                  </Stack>
                ),
                onClick: () => handleExport('html'),
              },
              {
                key: 'md',
                label: (
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{
                      fontSize: 14,
                      px: 2,
                      lineHeight: '40px',
                      height: 40,
                      width: 140,
                      borderRadius: '5px',
                      cursor: 'pointer',
                      ':hover': {
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                      },
                    }}
                  >
                    导出 Markdown
                  </Stack>
                ),
                onClick: () => handleExport('md'),
              },
            ]}
            context={
              <Button
                size='small'
                variant='outlined'
                disabled={!detail.name}
                startIcon={<IconDaochu sx={{ fontSize: 14 }} />}
              >
                导出
              </Button>
            }
          />
          {!readOnlyUser && !isReadonlyFilePreview && (
            <Cascader
              list={[
                {
                  key: 'save',
                  label: (
                    <Tooltip
                      title={<Box>{getShortcutKeyText(['ctrl', 's'])}</Box>}
                      placement='right'
                      arrow
                    >
                      <Stack
                        direction={'row'}
                        alignItems={'center'}
                        gap={1}
                        sx={{
                          fontSize: 14,
                          px: 2,
                          lineHeight: '40px',
                          height: 40,
                          width: 140,
                          borderRadius: '5px',
                          cursor: 'pointer',
                          ':hover': {
                            bgcolor: addOpacityToColor(
                              theme.palette.primary.main,
                              0.1,
                            ),
                          },
                        }}
                      >
                        保存
                      </Stack>
                    </Tooltip>
                  ),
                  onClick: handleSave,
                },
                {
                  key: 'save_publish',
                  label: (
                    <Stack
                      direction={'row'}
                      alignItems={'center'}
                      gap={1}
                      sx={{
                        fontSize: 14,
                        px: 2,
                        lineHeight: '40px',
                        height: 40,
                        width: 140,
                        borderRadius: '5px',
                        cursor: 'pointer',
                        ':hover': {
                          bgcolor: addOpacityToColor(
                            theme.palette.primary.main,
                            0.1,
                          ),
                        },
                      }}
                    >
                      保存并发布
                    </Stack>
                  ),
                  onClick: handlePublish,
                },
              ]}
              context={
                <Button
                  size='small'
                  variant='contained'
                  disabled={!detail.name}
                  startIcon={<IconBaocun sx={{ fontSize: 14 }} />}
                >
                  保存
                </Button>
              }
            />
          )}
        </Stack>
      </Stack>
      {!readOnlyUser && (
        <DocAddByCustomText
          type={detail.type}
          open={renameOpen}
          onClose={() => {
            setRenameOpen(false);
          }}
          data={detail}
          setDetail={updateDetail}
        />
      )}
      {!readOnlyUser && (
        <VersionPublish
          open={publishOpen}
          defaultSelected={[detail.id!]}
          onClose={() => setPublishOpen(false)}
          refresh={() =>
            updateDetail({
              status: 2,
            })
          }
        />
      )}
      {!readOnlyUser && (
        <DocDelete
          open={delOpen}
          onClose={() => {
            setDelOpen(false);
          }}
          onDeleted={handleDeleteAndNavigate}
          data={[
            {
              ...detail,
              emoji: detail.meta?.emoji || '',
              parent_id: '',
              summary: detail.meta?.summary || '',
              position: 0,
              status: 1,
            },
          ]}
        />
      )}
    </Box>
  );
};

const StyledMenuSelect = styled('div')<{
  disabled?: boolean;
  type?: 'default' | 'error';
}>(({ theme, disabled = false, type = 'default' }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between ',
  fontSize: 14,
  padding: theme.spacing(0, 2),
  lineHeight: '40px',
  height: 40,
  minWidth: 106,
  borderRadius: '5px',
  color: disabled
    ? theme.palette.text.secondary
    : type === 'error'
      ? theme.palette.error.main
      : theme.palette.text.primary,
  cursor: disabled ? 'not-allowed' : 'pointer',
  ':hover': {
    backgroundColor: disabled
      ? 'transparent'
      : type === 'error'
        ? addOpacityToColor(theme.palette.error.main, 0.1)
        : addOpacityToColor(theme.palette.primary.main, 0.1),
  },
}));

export default Header;
