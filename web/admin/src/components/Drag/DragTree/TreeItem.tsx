import { ITreeItem } from '@/api';
import Emoji from '@/components/Emoji';
import DocumentTypeIcon from '@/components/DocumentTypeIcon';
import {
  TreeItemComponentProps,
  TreeItemWrapper,
} from '@/components/TreeDragSortable';
import { treeSx } from '@/constant/styles';
import { postApiV1Node, putApiV1NodeDetail } from '@/request/Node';
import { useAppSelector } from '@/store';
import { AppContext, updateTree } from '@/utils/drag';
import { isOrdinaryReadOnlyUser } from '@/utils/permission';
import { getRagStatusMeta } from '@/pages/document/utils/ragStatus';
import {
  handleMultiSelect,
  handleParentControlledSelect,
  hasSelectedDescendant,
  updateAllParentStatus,
} from '@/utils/tree';
import { message } from '@/components/ui';
import { Ellipsis } from '@/components/ui';
import {
  Box,
  Button,
  Checkbox,
  Skeleton,
  PaletteColor,
  Stack,
  TextField,
  Theme,
  Tooltip,
  alpha,
  styled,
} from '@mui/material';
import dayjs from 'dayjs';
import React, {
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import TreeMenu from './TreeMenu';

const StyledTag = styled('div')<{ color: keyof Theme['palette'] }>(
  ({ theme, color }) => ({
    color: (theme.palette[color] as PaletteColor).main,
    border: '1px solid',
    borderColor: (theme.palette[color] as PaletteColor).main,
    borderRadius: '10px',
    padding: theme.spacing(0, 1),
    maxWidth: 260,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    bgcolor: alpha((theme.palette[color] as PaletteColor).main, 0.1),
  }),
);

const TreeItem = React.forwardRef<
  HTMLDivElement,
  TreeItemComponentProps<ITreeItem>
>((props, ref) => {
  const {
    kb_id: id,
    nav_id,
    user,
    kbDetail,
  } = useAppSelector(state => state.config);
  const readOnlyUser = isOrdinaryReadOnlyUser(user, kbDetail);
  const { item, collapsed } = props;
  const context = useContext(AppContext);

  if (!context) throw new Error('TreeItem 必须在 AppContext.Provider 内部使用');

  const {
    data,
    ui = 'move',
    selected = [],
    onSelectChange,
    readOnly: contextReadOnly,
    supportSelect = false,
    menu,
    relativeSelect = true,
    selectionModel = 'cascade-parent-sync',
    updateData,
    refresh,
    disabled,
    scrollToItem,
    onItemClick,
    onExpand,
  } = context;

  const readOnly = contextReadOnly || item.can_manage === false;
  const [value, setValue] = useState(item.name);
  const [emoji, setEmoji] = useState(item.emoji);
  const isEditting = item.isEditting ?? false;
  const inputRef = useRef<HTMLInputElement>(null);
  const selectedSet = useMemo(() => new Set(selected), [selected]);

  const createItem = useCallback(
    async (type: 1 | 2, contentType?: string) => {
      await onExpand?.(item);
      const newItemId = new Date().getTime().toString();
      // A navigation root is a virtual grouping node, not a real node in
      // `nodes`. Its ID belongs in nav_id, never in parent_id. Real folders
      // remain the parent of second-level folders/documents.
      const parentId = item.kind === 'nav' ? undefined : item.id;
      const targetNavId =
        item.nav_id || (item.kind === 'nav' ? item.id : '') || nav_id || '';
      const temp = [...data];
      updateTree(temp, item.id, {
        ...item,
        collapsed: false, // 展开父节点
        children: [
          ...(item.children ?? []),
          {
            id: newItemId,
            name: '',
            level: item.level + 1,
            type,
            emoji: '',
            content_type: contentType,
            nav_id: targetNavId,
            status: 1,
            isEditting: true,
            parentId,
          },
        ],
      });
      updateData?.(temp);
      // 延迟滚动，等待 DOM 更新
      setTimeout(() => {
        scrollToItem?.(newItemId);
      }, 100);
    },
    [data, item, nav_id, updateData, scrollToItem],
  );

  const renameItem = useCallback(() => {
    const temp = [...data];
    updateTree(temp, item.id, {
      ...item,
      isEditting: true,
    });
    updateData?.(temp);
  }, [data, item, updateData]);

  const removeItem = useCallback(
    (id: string) => {
      const temp = [...data];
      const remove = (value: ITreeItem[]) => {
        return value.filter(item => {
          if (item.id === id) return false;
          if (item.children) {
            item.children = remove(item.children);
          }
          return true;
        });
      };
      const newItems = remove(temp);
      updateData?.(newItems);
    },
    [data, item, updateData],
  );

  const handleSelectChange = useCallback(
    (id: string) => {
      if (relativeSelect && selectionModel === 'cascade-parent-sync') {
        const newSelected = handleMultiSelect(data, id, selected);
        onSelectChange?.(newSelected || [], id);
      } else if (relativeSelect && selectionModel === 'parent-controls-child') {
        const newSelected = handleParentControlledSelect(data, id, selected);
        onSelectChange?.(newSelected || [], id);
      } else {
        const temp = [...selected];
        if (temp.includes(id)) {
          onSelectChange?.(
            temp.filter(item => item !== id),
            id,
          );
        } else {
          onSelectChange?.([...temp, id], id);
        }
      }
    },
    [onSelectChange, selected, data, relativeSelect, selectionModel],
  );

  useEffect(() => {
    if (
      relativeSelect &&
      selectionModel === 'cascade-parent-sync' &&
      selected.length > 0
    ) {
      const temp = [...data];
      const selectedSet = new Set(selected);
      updateAllParentStatus(temp, selectedSet);
      const newSelected = Array.from(selectedSet);
      if (newSelected.length !== selected.length) {
        onSelectChange?.(newSelected);
      }
    }
  }, [selected, data, relativeSelect, onSelectChange, selectionModel]);

  useEffect(() => {
    setValue(item.name);
    setEmoji(item.emoji);
  }, [item]);

  useEffect(() => {
    if (isEditting && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isEditting]);

  const menuList = useMemo(() => {
    if (readOnly || item.placeholder) return [];
    if (menu) {
      return (
        menu({
          item,
          createItem,
          renameItem,
          isEditing: isEditting,
          removeItem,
        })?.filter(
          option =>
            !(
              ['folder', 'batch-import-zip'].includes(option.key) &&
              item.can_create_folder === false
            ) && !(option.key === 'doc' && item.can_create_document === false),
        ) || []
      );
    }
    return [];
  }, [item, isEditting, createItem, renameItem, removeItem, readOnly]);
  const ragStatusMeta = useMemo(() => getRagStatusMeta(item), [item]);
  const enterprisePermissionTags = item.permission_tags || [];

  const isChecked = selectedSet.has(item.id);
  const isIndeterminate =
    selectionModel === 'parent-controls-child' &&
    !isChecked &&
    item.type === 1 &&
    hasSelectedDescendant(item, selectedSet);

  if (item.placeholder) {
    return (
      <TreeItemWrapper
        {...props}
        ref={ref}
        showDragHandle={false}
        handleProps={{
          ...props.handleProps,
          role: undefined,
          'aria-disabled': undefined,
          tabIndex: undefined,
        }}
      >
        {item.placeholder === 'loading' ? (
          <Skeleton width={180} height={28} aria-label='正在加载目录内容' />
        ) : (
          <Button
            size='small'
            color='error'
            onClick={e => {
              e.stopPropagation();
              item.retry?.();
            }}
          >
            加载失败，点击重试
          </Button>
        )}
      </TreeItemWrapper>
    );
  }

  return (
    <Box
      sx={[
        treeSx(readOnly ?? false),
        {
          '&:hover': {
            '.dnd-sortable-tree_simple_handle': {
              opacity: 1,
            },
          },
        },
      ]}
    >
      <Stack direction='row' alignItems={'center'} gap={supportSelect ? 0 : 0}>
        {!readOnly ? (
          <div
            className={'dnd-sortable-tree_simple_handle'}
            {...props.handleProps}
          />
        ) : (
          <Box />
        )}

        {supportSelect && (
          <Checkbox
            sx={{
              visibility: disabled?.(item) ? 'hidden' : 'visible',
              flexShrink: 0,
              color: 'text.disabled',
              width: '35px',
              height: '35px',
            }}
            checked={isChecked}
            indeterminate={isIndeterminate}
            onChange={e => {
              e.stopPropagation();
              handleSelectChange(item.id);
            }}
            disabled={disabled?.(item)}
          />
        )}

        <Box sx={{ flex: 1, pl: 3 }}>
          <TreeItemWrapper
            {...props}
            ref={ref}
            indentationWidth={23}
            showDragHandle={false}
          >
            <Stack
              direction='row'
              alignItems={'center'}
              justifyContent='space-between'
              gap={2}
              flex={1}
              sx={{ pr: 2 }}
              onClick={() => {
                onItemClick?.(item);
                if (readOnly) return;
                if (ui === 'select') {
                  handleSelectChange(item.id);
                  return;
                }
                if (item.type === 2)
                  window.open(`/doc/editor/${item.id}`, '_blank');
              }}
            >
              {item.isEditting ? (
                <Stack
                  direction='row'
                  alignItems={'center'}
                  gap={2}
                  onClick={e => e.stopPropagation()}
                >
                  {item.type === 1 ? (
                    <Emoji
                      type={item.type}
                      collapsed={collapsed}
                      value={emoji}
                      onChange={setEmoji}
                    />
                  ) : (
                    <DocumentTypeIcon
                      contentType={item.content_type}
                      fileName={item.name}
                      sx={{ fontSize: 22 }}
                    />
                  )}
                  <TextField
                    size='small'
                    value={value}
                    inputRef={inputRef}
                    placeholder='请输入文档名称'
                    sx={{
                      width: 300,
                      input: {
                        bgcolor: 'background.paper',
                      },
                    }}
                    onPointerDown={e => e.stopPropagation()}
                    onMouseDown={e => e.stopPropagation()}
                    onTouchStart={e => e.stopPropagation()}
                    onChange={e => setValue(e.target.value)}
                  />
                  <Button
                    variant='contained'
                    size='small'
                    onClick={e => {
                      e.stopPropagation();
                      if (readOnlyUser) return;
                      if (item.name) {
                        putApiV1NodeDetail({
                          id: item.id,
                          kb_id: id,
                          nav_id:
                            (item as { nav_id?: string }).nav_id ||
                            nav_id ||
                            '',
                          name: value,
                          emoji,
                        }).then(() => {
                          message.success('更新成功');
                          const temp = [...data];
                          updateTree(temp, item.id, {
                            ...item,
                            name: value,
                            emoji,
                            isEditting: false,
                            status: item.name === value ? item.status : 1,
                            updated_at: dayjs().toString(),
                          });
                          updateData?.(temp);
                          refresh?.();
                        });
                      } else {
                        if (value === '') {
                          message.error('文档名称不能为空');
                          return;
                        }
                        postApiV1Node({
                          name: value,
                          content: '',
                          kb_id: id,
                          nav_id: item.nav_id || nav_id || '',
                          parent_id: item.parentId,
                          type: item.type,
                          emoji,
                          content_type: item.content_type,
                        }).then(res => {
                          message.success('创建成功');
                          const temp = [...data];
                          const newItem = {
                            ...item,
                            id: res.id,
                            name: value,
                            emoji,
                            isEditting: false,
                            updated_at: dayjs().toString(),
                          };
                          if (item.type === 1) {
                            newItem.children = [];
                          }
                          updateTree(temp, item.id, newItem);
                          updateData?.(temp);
                          refresh?.();
                          // 滚动到保存后的项
                          setTimeout(() => {
                            scrollToItem?.(res.id);
                          }, 100);
                        });
                      }
                    }}
                  >
                    保存
                  </Button>
                  <Button
                    variant='outlined'
                    size='small'
                    onClick={e => {
                      e.stopPropagation();
                      if (!item.name) {
                        removeItem(item.id);
                      } else {
                        const temp = [...data];
                        updateTree(temp, item.id, {
                          ...item,
                          isEditting: false,
                        });
                        updateData?.(temp);
                      }
                    }}
                  >
                    取消
                  </Button>
                </Stack>
              ) : (
                <Stack
                  direction='row'
                  alignItems={'center'}
                  gap={1}
                  sx={{
                    fontSize: 14,
                    cursor: 'pointer',
                    ...(ui === 'select' && { width: '100%', flex: 1 }),
                  }}
                >
                  <Box
                    onClick={e => e.stopPropagation()}
                    sx={{ flexShrink: 0, cursor: 'pointer' }}
                  >
                    {item.type === 1 ? (
                      <Emoji
                        readOnly={readOnly}
                        sx={{ width: 24, height: 24, fontSize: 14 }}
                        type={item.type}
                        collapsed={collapsed}
                        value={item.emoji}
                        onChange={async value => {
                          if (readOnlyUser) return;
                          try {
                            await putApiV1NodeDetail({
                              id: item.id,
                              kb_id: id,
                              nav_id:
                                (item as { nav_id?: string }).nav_id ||
                                nav_id ||
                                '',
                              emoji: value,
                            });
                            message.success('更新成功');
                            const temp = [...data];
                            updateTree(temp, item.id, {
                              ...item,
                              updated_at: dayjs().toString(),
                              status: 1,
                              emoji: value,
                            });
                            updateData?.(temp);
                            refresh?.();
                          } catch (error) {
                            message.error('更新失败');
                          }
                        }}
                      />
                    ) : (
                      <DocumentTypeIcon
                        contentType={item.content_type}
                        fileName={item.name}
                        sx={{ fontSize: 22 }}
                      />
                    )}
                  </Box>
                  {ui === 'select' ? (
                    <Ellipsis sx={{ width: 0, flex: 1, overflow: 'hidden' }}>
                      {item.name}
                    </Ellipsis>
                  ) : (
                    <>
                      <Box
                        sx={{
                          minWidth: 0,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}
                        title={item.name}
                      >
                        {item.name}
                        {item.content_type === 'md' && (
                          <Box
                            component='span'
                            sx={{
                              fontSize: 10,
                              color: 'white',
                              background:
                                'linear-gradient(135deg, #a8edea 0%, #fed6e3 100%)',
                              px: 1,
                              ml: 1,
                              fontWeight: 500,
                              borderRadius: '4px',
                              height: '14px',
                              lineHeight: '14px',
                              display: 'inline-block',
                            }}
                          >
                            MD
                          </Box>
                        )}
                      </Box>
                    </>
                  )}
                </Stack>
              )}
              {(menuList.length > 0 ||
                item.kind !== 'nav' ||
                enterprisePermissionTags.length > 0) && (
                <>
                  <Box
                    sx={{
                      flex: 1,
                      alignSelf: 'center',
                      borderBottom: '1px dashed',
                      borderColor: 'divider',
                    }}
                  />
                  {item.kind !== 'nav' && (
                    <Stack
                      direction='row'
                      alignItems='center'
                      gap={1}
                      sx={{
                        flexShrink: 1,
                        maxWidth: '60%',
                        flexWrap: 'wrap',
                        fontSize: 12,
                      }}
                    >
                      {item.status === 0 && (
                        <StyledTag color='error'>未发布</StyledTag>
                      )}
                      {item.status === 1 && (
                        <StyledTag color='error'>更新未发布</StyledTag>
                      )}
                      {ragStatusMeta && (
                        <Tooltip
                          title={ragStatusMeta.message || ''}
                          disableHoverListener={!ragStatusMeta.message}
                        >
                          <StyledTag color={ragStatusMeta.tagColor}>
                            {ragStatusMeta.label}
                          </StyledTag>
                        </Tooltip>
                      )}
                    </Stack>
                  )}
                  <Stack
                    direction='row'
                    alignItems='center'
                    justifyContent='flex-end'
                    gap={1}
                    sx={{
                      flexShrink: 1,
                      maxWidth: '60%',
                      flexWrap: 'wrap',
                      fontSize: 12,
                    }}
                  >
                    {enterprisePermissionTags.map((tag, index) => (
                      <Tooltip
                        key={`${tag.label}-${index}`}
                        title={tag.title || ''}
                      >
                        <StyledTag color={tag.color}>{tag.label}</StyledTag>
                      </Tooltip>
                    ))}
                  </Stack>
                  {item.kind !== 'nav' && (
                    <Stack
                      sx={{
                        fontSize: 12,
                        fontFamily: 'monospace',
                        color: 'text.disabled',
                        width: 60,
                        textAlign: 'right',
                        flexShrink: 0,
                      }}
                    >
                      {dayjs(item.updated_at).fromNow()}
                    </Stack>
                  )}
                  {menuList.length > 0 && (
                    <Box onClick={e => e.stopPropagation()}>
                      <TreeMenu menu={menuList} />
                    </Box>
                  )}
                </>
              )}
            </Stack>
          </TreeItemWrapper>
        </Box>
      </Stack>
    </Box>
  );
});

export default TreeItem;
