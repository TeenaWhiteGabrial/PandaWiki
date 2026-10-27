import { useDocumentActions } from './DocumentActionsProvider';
import type { ITreeItem } from '@/api';
import TreeMenu from '@/components/Drag/DragTree/TreeMenu';
import type {
  DomainNodeListItemResp,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
} from '@/request/types';
import { getDocumentPermissionTags } from '@/pages/document/utils/permissionTags';
import {
  Box,
  Breadcrumbs,
  Button,
  Card,
  Checkbox,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import DocumentTypeIcon from '@/components/DocumentTypeIcon';
import { getRagStatusMeta } from '@/pages/document/utils/ragStatus';
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined';
import HomeOutlinedIcon from '@mui/icons-material/HomeOutlined';
import NavigateNextIcon from '@mui/icons-material/NavigateNext';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import { IconGengduo, IconJiahao } from '@panda-wiki/icons';
import { useEffect, useMemo, useState } from 'react';

interface DocPageFileManagerProps {
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  loading?: boolean;
  loadError?: boolean;
  onRetry?: () => void;
  navId?: string;
  folderId: string;
  onFolderChange: (id: string) => void;
  onDocumentSelect: (item: DomainNodeListItemResp) => void;
  readOnlyUser?: boolean;
  fill?: boolean;
}

type ExplorerItem = DomainNodeListItemResp & {
  explorerKind?: 'nav';
  can_configure?: boolean;
  permission_tags?: ITreeItem['permission_tags'];
};

const NAV_PATH_PREFIX = 'nav:';

const DocPageFileManager = ({
  groups,
  loading = false,
  loadError = false,
  onRetry,
  navId,
  folderId,
  onFolderChange,
  onDocumentSelect,
  readOnlyUser = false,
  fill = false,
}: DocPageFileManagerProps) => {
  const {
    menu,
    mutationVersion,
    permissionCatalog,
    openCreate,
    openImport,
    openZipImport,
    openMove,
    openDelete,
    openAddNav,
  } = useDocumentActions();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedItemIds, setSelectedItemIds] = useState<string[]>([]);
  useEffect(() => {
    setSelectedId(null);
    setSelectedItemIds([]);
  }, [folderId, navId, mutationVersion]);

  const nodes = useMemo(
    () =>
      groups.flatMap(group => (group.list || []) as DomainNodeListItemResp[]),
    [groups],
  );

  const nodeMap = useMemo(
    () => new Map(nodes.map(item => [item.id || '', item])),
    [nodes],
  );

  const isNavPath = folderId.startsWith(NAV_PATH_PREFIX);
  const activeNavId = useMemo(() => {
    if (isNavPath) return folderId.slice(NAV_PATH_PREFIX.length);
    if (folderId) return nodeMap.get(folderId)?.nav_id || navId;
    return navId;
  }, [folderId, isNavPath, navId, nodeMap]);

  const activeNodes = useMemo(
    () => nodes.filter(item => item.nav_id === activeNavId),
    [activeNavId, nodes],
  );

  const rootName = useMemo(() => '根目录', []);

  const currentItems = useMemo((): ExplorerItem[] => {
    const items: ExplorerItem[] = !folderId
      ? groups
          .slice()
          .sort((a, b) => (a.position || 0) - (b.position || 0))
          .map(group => ({
            id: `${NAV_PATH_PREFIX}${group.nav_id || ''}`,
            name: group.nav_name || '未命名目录',
            nav_id: group.nav_id,
            parent_id: '',
            type: 1,
            explorerKind: 'nav' as const,
            permissions: group.permissions,
            effective_management_grants: group.permissions?.management_grants,
            can_manage: group.can_manage,
            can_create_document: group.can_create_document,
            can_create_folder: group.can_create_folder,
            can_configure: group.can_configure,
          }))
          .filter(item => item.nav_id)
      : activeNodes
          .filter(item =>
            isNavPath
              ? !(item.parent_id || '') || item.parent_id === activeNavId
              : (item.parent_id || '') === folderId,
          )
          .sort((a, b) => {
            return (a.position || 0) - (b.position || 0);
          });
    return items.map(item => {
      const permissions = item.effective_permissions || item.permissions;
      const managementGrants = item.effective_management_grants?.length
        ? item.effective_management_grants
        : item.permissions?.management_grants;
      return {
        ...item,
        permission_tags: getDocumentPermissionTags(
          permissions
            ? { ...permissions, management_grants: managementGrants }
            : undefined,
          permissionCatalog,
          item.effective_management_constraints,
        ),
      };
    });
  }, [
    activeNavId,
    activeNodes,
    folderId,
    groups,
    isNavPath,
    permissionCatalog,
  ]);

  const selectableItems = useMemo(
    () =>
      currentItems.filter(
        (item): item is ExplorerItem & { id: string } =>
          !readOnlyUser &&
          item.can_manage !== false &&
          !item.explorerKind &&
          !!item.id,
      ),
    [currentItems, readOnlyUser],
  );
  const selectedItems = useMemo(
    () => selectableItems.filter(item => selectedItemIds.includes(item.id)),
    [selectedItemIds, selectableItems],
  );
  const allVisibleItemsSelected =
    selectableItems.length > 0 &&
    selectableItems.every(item => selectedItemIds.includes(item.id));

  const breadcrumbs = useMemo(() => {
    const result: ExplorerItem[] = [];
    // 资源管理器根层展示所有目录，此时不能把当前选中的 nav_id
    // 自动追加到面包屑，否则会造成“根目录 > Local Wiki”的假层级。
    if (!folderId) return result;

    if (isNavPath) {
      const group = groups.find(item => item.nav_id === activeNavId);
      if (group?.nav_id) {
        result.push({
          id: `${NAV_PATH_PREFIX}${group.nav_id}`,
          name: group.nav_name || '未命名目录',
          nav_id: group.nav_id,
          type: 1,
          explorerKind: 'nav',
        });
      }
      return result;
    }

    let current = folderId ? nodeMap.get(folderId) : undefined;
    while (current) {
      result.unshift(current);
      current = current.parent_id ? nodeMap.get(current.parent_id) : undefined;
    }
    const breadcrumbNavId = result[0]?.nav_id || activeNavId;
    const group = groups.find(item => item.nav_id === breadcrumbNavId);
    if (group?.nav_id) {
      result.unshift({
        id: `${NAV_PATH_PREFIX}${group.nav_id}`,
        name: group.nav_name || '未命名目录',
        nav_id: group.nav_id,
        type: 1,
        explorerKind: 'nav',
      });
    }
    return result;
  }, [activeNavId, folderId, groups, isNavPath, nodeMap]);

  // 一级文件夹的 parent_id 可能记录为当前目录 nav_id；在资源管理器中，
  // 这个值应当视为虚拟根目录，而不是一个可继续进入的文件夹。
  const parentFolderId = useMemo(() => {
    if (isNavPath) return '';
    const current = nodeMap.get(folderId);
    if (!current) return '';
    if (current.parent_id && current.parent_id !== current.nav_id) {
      return current.parent_id;
    }
    return current.nav_id ? `${NAV_PATH_PREFIX}${current.nav_id}` : '';
  }, [folderId, isNavPath, nodeMap]);

  const canManageRoots =
    groups.length === 0 || groups.some(group => group.can_configure !== false);
  const activeGroup = groups.find(group => group.nav_id === activeNavId);
  const currentFolder = isNavPath ? undefined : nodeMap.get(folderId);
  const currentFolderManageable =
    !readOnlyUser &&
    (isNavPath
      ? !!activeGroup && activeGroup.can_manage !== false
      : !!currentFolder && currentFolder.can_manage !== false);
  const currentFolderCanCreate =
    currentFolderManageable &&
    (isNavPath
      ? activeGroup?.can_create_document !== false
      : currentFolder?.can_create_document !== false);
  const currentFolderCanCreateFolder =
    currentFolderManageable &&
    (isNavPath
      ? activeGroup?.can_create_folder !== false
      : currentFolder?.can_create_folder !== false);
  const currentFolderTarget: ITreeItem = {
    ...(currentFolder as ITreeItem | undefined),
    id: isNavPath ? activeNavId || '' : folderId,
    type: 1,
    level: 0,
    name:
      breadcrumbs
        .map(item => item.name)
        .filter(Boolean)
        .join(' > ') || rootName,
    nav_id: activeNavId,
    kind: isNavPath ? 'nav' : undefined,
    can_manage: currentFolderManageable,
    can_create_document: currentFolderCanCreate,
    can_create_folder: currentFolderCanCreateFolder,
  };

  return (
    <Card
      sx={{
        flex: fill ? 1 : '0 0 auto',
        width: fill ? '100%' : undefined,
        minWidth: 0,
        height: '100%',
        overflow: 'hidden',
      }}
    >
      <Stack sx={{ height: '100%' }}>
        <Stack
          direction='row'
          alignItems='center'
          justifyContent='space-between'
          sx={{ px: 2, py: 1.25 }}
        >
          <Stack direction='row' alignItems='center' gap={1} minWidth={0}>
            <Tooltip title='返回上一级'>
              <span>
                <IconButton
                  size='small'
                  disabled={!folderId}
                  onClick={() => onFolderChange(parentFolderId)}
                >
                  <ArrowBackIcon fontSize='small' />
                </IconButton>
              </span>
            </Tooltip>
            <Breadcrumbs
              separator={<NavigateNextIcon fontSize='small' />}
              sx={{ minWidth: 0 }}
            >
              <Typography
                variant='body2'
                noWrap
                sx={{ cursor: 'pointer', fontWeight: folderId ? 400 : 600 }}
                onClick={() => onFolderChange('')}
              >
                <HomeOutlinedIcon
                  sx={{ fontSize: 17, verticalAlign: 'middle', mr: 0.5 }}
                />
                {rootName}
              </Typography>
              {breadcrumbs.map(item => (
                <Typography
                  key={item.id}
                  variant='body2'
                  noWrap
                  sx={{
                    cursor: 'pointer',
                    fontWeight: item.id === folderId ? 600 : 400,
                    maxWidth: 180,
                  }}
                  onClick={() => onFolderChange(item.id || '')}
                >
                  {item.name}
                </Typography>
              ))}
            </Breadcrumbs>
            {loading && (
              <CircularProgress size={16} aria-label='正在加载当前目录' />
            )}
          </Stack>
          {!readOnlyUser &&
            (folderId
              ? currentFolderCanCreate || currentFolderCanCreateFolder
              : canManageRoots) && (
              <Stack direction='row' alignItems='center' gap={1} flexShrink={0}>
                {!folderId ? (
                  <Button
                    size='small'
                    variant='outlined'
                    startIcon={
                      <IconJiahao sx={{ fontSize: '10px !important' }} />
                    }
                    onClick={openAddNav}
                  >
                    新建目录
                  </Button>
                ) : (
                  <>
                    <Button
                      size='small'
                      disabled={!currentFolderCanCreateFolder}
                      variant='outlined'
                      onClick={() => openCreate(currentFolderTarget, 1)}
                    >
                      新建文件夹
                    </Button>
                    <Button
                      size='small'
                      variant='outlined'
                      disabled={!currentFolderCanCreate}
                      onClick={() => openCreate(currentFolderTarget, 2)}
                    >
                      新建文档
                    </Button>
                    <Button
                      size='small'
                      variant='outlined'
                      disabled={!currentFolderCanCreate}
                      onClick={() => openImport(currentFolderTarget)}
                    >
                      导入文档
                    </Button>
                    <Button
                      size='small'
                      variant='contained'
                      disabled={!currentFolderCanCreateFolder}
                      onClick={() => openZipImport(currentFolderTarget)}
                    >
                      批量导入 ZIP
                    </Button>
                  </>
                )}
              </Stack>
            )}
        </Stack>
        <Divider />
        <Stack
          direction='row'
          alignItems='center'
          sx={{ px: 1.5, py: 0.75, color: 'text.secondary', fontSize: 12 }}
        >
          <Stack
            direction='row'
            alignItems='center'
            sx={{ flex: 1, minWidth: 0 }}
          >
            <Box sx={{ width: 34, display: 'flex', justifyContent: 'center' }}>
              {selectableItems.length > 0 && (
                <Checkbox
                  size='small'
                  checked={allVisibleItemsSelected}
                  indeterminate={
                    selectedItems.length > 0 && !allVisibleItemsSelected
                  }
                  onChange={event =>
                    setSelectedItemIds(
                      event.target.checked
                        ? selectableItems.map(item => item.id)
                        : [],
                    )
                  }
                  inputProps={{ 'aria-label': '全选当前目录项目' }}
                  sx={{ p: 0.5 }}
                />
              )}
            </Box>
            <Box>名称</Box>
            {selectedItems.length > 0 && (
              <Box sx={{ ml: 1.5 }}>已选 {selectedItems.length} 项</Box>
            )}
          </Stack>
          <Stack direction='row' alignItems='center' gap={1}>
            {selectedItems.length > 0 && (
              <>
                <Button
                  size='small'
                  onClick={() => openMove(selectedItems)}
                  sx={{ minWidth: 0, py: 0.25 }}
                >
                  批量移动
                </Button>
                <Button
                  size='small'
                  color='error'
                  startIcon={<DeleteOutlineIcon fontSize='small' />}
                  onClick={() => openDelete(selectedItems)}
                  sx={{ minWidth: 0, py: 0.25 }}
                >
                  批量删除
                </Button>
                <Button
                  size='small'
                  onClick={() => setSelectedItemIds([])}
                  sx={{ minWidth: 0, py: 0.25 }}
                >
                  取消
                </Button>
              </>
            )}
            <Box sx={{ width: 168, textAlign: 'right' }}>状态</Box>
          </Stack>
        </Stack>
        <Box sx={{ flex: 1, overflow: 'auto', p: 1.5 }}>
          {currentItems.length === 0 ? (
            <Stack
              alignItems='center'
              justifyContent='center'
              sx={{ height: '100%', color: 'text.secondary' }}
            >
              {loading ? (
                <CircularProgress
                  size={32}
                  aria-label='正在加载目录'
                  sx={{ mb: 1 }}
                />
              ) : (
                <FolderOutlinedIcon sx={{ fontSize: 48, opacity: 0.35 }} />
              )}
              <Typography variant='body2'>
                {loadError ? (
                  <Button color='error' onClick={onRetry}>
                    加载失败，点击重试
                  </Button>
                ) : loading ? (
                  '正在加载目录…'
                ) : (
                  '此目录为空'
                )}
              </Typography>
            </Stack>
          ) : (
            <Stack gap={0.5}>
              {currentItems.map(item => {
                const isFolder = item.type === 1;
                const itemReadOnly = readOnlyUser || item.can_manage === false;
                const treeItem: ITreeItem = {
                  ...(item as unknown as ITreeItem),
                  id: item.explorerKind ? item.nav_id || '' : item.id || '',
                  level: 0,
                  name: item.name || '',
                  type: isFolder ? 1 : 2,
                  parentId: item.parent_id,
                  rag_status: item.rag_info?.status,
                  rag_message: item.rag_info?.message,
                  kind: item.explorerKind ? 'nav' : undefined,
                };
                const resolvedActionMenu = menu({
                  item: treeItem,
                  createItem: () => undefined,
                  renameItem: () => undefined,
                  isEditing: false,
                  removeItem: () => undefined,
                });
                const ragStatusMeta = getRagStatusMeta(item);
                const publishStatusMeta =
                  item.status === 2
                    ? {
                        label: '已发布',
                        color: 'success' as const,
                      }
                    : item.status === 1
                      ? {
                          label: '更新未发布',
                          color: 'warning' as const,
                        }
                      : {
                          label: '未发布',
                          color: 'default' as const,
                        };
                return (
                  <Stack
                    key={item.id}
                    direction='row'
                    alignItems='center'
                    gap={1.5}
                    sx={{
                      px: 1.5,
                      py: 1.25,
                      borderRadius: 1,
                      bgcolor:
                        selectedId === item.id
                          ? 'action.selected'
                          : 'transparent',
                      cursor: 'pointer',
                      '&:hover': { bgcolor: 'action.hover' },
                    }}
                    onClick={() => {
                      setSelectedId(item.id || null);
                      if (!isFolder) onDocumentSelect(item);
                    }}
                    onDoubleClick={() => {
                      setSelectedId(item.id || null);
                      if (isFolder) onFolderChange(item.id || '');
                    }}
                  >
                    <Box
                      sx={{
                        width: 34,
                        display: 'flex',
                        justifyContent: 'center',
                        flex: '0 0 34px',
                      }}
                      onClick={event => event.stopPropagation()}
                    >
                      {!itemReadOnly && !item.explorerKind && item.id && (
                        <Checkbox
                          size='small'
                          checked={selectedItemIds.includes(item.id)}
                          onChange={event =>
                            setSelectedItemIds(previous =>
                              event.target.checked
                                ? [...new Set([...previous, item.id!])]
                                : previous.filter(id => id !== item.id),
                            )
                          }
                          inputProps={{
                            'aria-label': `选择项目 ${item.name || ''}`,
                          }}
                          sx={{ p: 0.5 }}
                        />
                      )}
                    </Box>
                    {isFolder ? (
                      <FolderOutlinedIcon
                        sx={{ color: 'warning.main', fontSize: 28 }}
                      />
                    ) : (
                      <DocumentTypeIcon
                        contentType={item.content_type}
                        fileName={item.name}
                      />
                    )}
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      <Typography variant='body2' noWrap title={item.name}>
                        {item.name || '未命名'}
                      </Typography>
                    </Box>
                    {item.permission_tags &&
                      item.permission_tags.length > 0 && (
                        <Stack
                          direction='row'
                          alignItems='center'
                          gap={0.75}
                          sx={{
                            flexShrink: 1,
                            maxWidth: '48%',
                            flexWrap: 'wrap',
                          }}
                        >
                          {item.permission_tags.map((tag, index) => (
                            <Tooltip
                              key={`${tag.label}-${index}`}
                              title={tag.title || ''}
                            >
                              <Chip
                                size='small'
                                color={tag.color}
                                variant='outlined'
                                label={tag.label}
                                sx={{ height: 24, fontSize: 11 }}
                              />
                            </Tooltip>
                          ))}
                        </Stack>
                      )}
                    {isFolder &&
                      !itemReadOnly &&
                      resolvedActionMenu.some(action =>
                        ['folder', 'doc'].includes(action.key),
                      ) && (
                        <Box onClick={event => event.stopPropagation()}>
                          <TreeMenu
                            menu={resolvedActionMenu.filter(action =>
                              ['folder', 'doc'].includes(action.key),
                            )}
                            context={
                              <IconButton size='small' title='新建'>
                                <IconJiahao sx={{ fontSize: 18 }} />
                              </IconButton>
                            }
                          />
                        </Box>
                      )}
                    {resolvedActionMenu.length > 0 && (
                      <Box onClick={event => event.stopPropagation()}>
                        <TreeMenu
                          menu={resolvedActionMenu}
                          context={
                            <IconButton size='small' title='更多'>
                              <IconGengduo sx={{ fontSize: 18 }} />
                            </IconButton>
                          }
                        />
                      </Box>
                    )}
                    {!isFolder && (
                      <Stack
                        direction='row'
                        alignItems='center'
                        gap={0.75}
                        sx={{ flexShrink: 0 }}
                      >
                        <Chip
                          size='small'
                          variant='outlined'
                          label={publishStatusMeta.label}
                          color={publishStatusMeta.color}
                        />
                        {ragStatusMeta && (
                          <Tooltip
                            title={ragStatusMeta.message || ''}
                            disableHoverListener={!ragStatusMeta.message}
                          >
                            <Chip
                              size='small'
                              variant='outlined'
                              label={ragStatusMeta.label}
                              color={ragStatusMeta.color}
                            />
                          </Tooltip>
                        )}
                      </Stack>
                    )}
                  </Stack>
                );
              })}
            </Stack>
          )}
        </Box>
      </Stack>
    </Card>
  );
};

export default DocPageFileManager;
