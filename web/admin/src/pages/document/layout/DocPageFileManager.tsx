import AddDocBtn from '@/pages/document/component/AddDocBtn';
import AddDocByType from '@/pages/document/component/AddDocByType';
import BatchZipImport from '@/pages/document/component/BatchZipImport';
import DocAddByCustomText from '@/pages/document/component/DocAddByCustomText';
import DocDelete from '@/pages/document/component/DocDelete';
import MoveDocs from '@/pages/document/component/MoveDocs';
import NavEditModal from '@/pages/document/layout/DocPageNavs/NavEditModal';
import NavPermissionModal from '@/pages/document/layout/DocPageNavs/NavPermissionModal';
import { message, Modal } from '@/components/ui';
import { ConstsCrawlerSource } from '@/request';
import { deleteApiV1NavDelete } from '@/request/Nav';
import { getNodePermissionOptions } from '@/request/NodeManagement';
import type { ITreeItem } from '@/api';
import TreeMenu, {
  type TreeMenuItem,
} from '@/components/Drag/DragTree/TreeMenu';
import type { DocTreeMenuFn } from './DocPageList/types';
import { putApiV1NodeDetail } from '@/request/Node';
import { useAppSelector } from '@/store';
import type {
  DomainNodeListItemResp,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
  V1NavListResp,
} from '@/request/types';
import {
  getDocumentPermissionTags,
  type PermissionAudienceCatalog,
} from '@/pages/document/utils/permissionTags';
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
  TextField,
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
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import { IconGengduo, IconJiahao } from '@panda-wiki/icons';
import { useEffect, useMemo, useState } from 'react';

interface DocPageFileManagerProps {
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  loading?: boolean;
  navId?: string;
  folderId: string;
  onFolderChange: (id: string) => void;
  onDocumentSelect: (item: DomainNodeListItemResp) => void;
  refresh: () => void;
  readOnlyUser?: boolean;
  fill?: boolean;
  menu?: DocTreeMenuFn;
  onNavDeleted?: (navId: string) => void;
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
  navId,
  folderId,
  onFolderChange,
  onDocumentSelect,
  refresh,
  readOnlyUser = false,
  fill = false,
  menu,
  onNavDeleted,
}: DocPageFileManagerProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [permissionCatalog, setPermissionCatalog] =
    useState<PermissionAudienceCatalog>({
      departments: {},
      levels: {},
      users: {},
    });
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedItemIds, setSelectedItemIds] = useState<string[]>([]);
  const [deleteSelectedDocsOpen, setDeleteSelectedDocsOpen] = useState(false);
  const [moveOpen, setMoveOpen] = useState(false);
  const [moveSelectedItems, setMoveSelectedItems] = useState<
    DomainNodeListItemResp[]
  >([]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  const [batchImportParentId, setBatchImportParentId] = useState<string | null>(
    null,
  );
  const [batchImportParentName, setBatchImportParentName] = useState('根目录');
  const [batchImportNavId, setBatchImportNavId] = useState<string | null>(null);
  const [batchImportOpen, setBatchImportOpen] = useState(false);
  const [docImportOpen, setDocImportOpen] = useState(false);
  const [docImportTarget, setDocImportTarget] = useState<{
    parentId: string | null;
    navId: string | null;
  } | null>(null);
  const [createType, setCreateType] = useState<1 | 2 | null>(null);
  const [createParentId, setCreateParentId] = useState<string | null>(null);
  const [createNavId, setCreateNavId] = useState<string | null>(null);
  const [addNavOpen, setAddNavOpen] = useState(false);
  const [editingNav, setEditingNav] = useState<V1NavListResp | null>(null);
  const [permissionNav, setPermissionNav] = useState<V1NavListResp | null>(
    null,
  );
  const [permissionOpen, setPermissionOpen] = useState(false);
  const [deletingNav, setDeletingNav] = useState<V1NavListResp | null>(null);
  const [deleteConfirmInput, setDeleteConfirmInput] = useState('');

  useEffect(() => {
    if (!kb_id) return;
    let active = true;
    getNodePermissionOptions(kb_id)
      .then(options => {
        if (!active) return;
        setPermissionCatalog({
          departments: Object.fromEntries(
            [
              ...(options.departments || []),
              ...(options.management_departments || []),
            ].map(item => [item.id, item.name] as const),
          ),
          levels: Object.fromEntries(
            (options.levels || []).map(item => [item.id, item.name] as const),
          ),
          users: Object.fromEntries(
            [...(options.users || []), ...(options.management_users || [])].map(
              item => [item.id, item.name || item.account] as const,
            ),
          ),
        });
      })
      .catch(() => {
        // Generic permission labels remain useful when the audience catalog is unavailable.
      });
    return () => {
      active = false;
    };
  }, [kb_id]);

  useEffect(() => {
    setSelectedId(null);
    setSelectedItemIds([]);
  }, [folderId, navId]);

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
            if (a.type !== b.type) return a.type === 1 ? -1 : 1;
            return (a.name || '').localeCompare(b.name || '', 'zh-CN');
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
          !item.explorerKind && !!item.id,
      ),
    [currentItems],
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
  const currentFolderCanCreate = isNavPath
    ? activeGroup?.can_create_document !== false
    : nodeMap.get(folderId)?.can_create_document !== false;
  const currentFolderCanCreateFolder = isNavPath
    ? activeGroup?.can_create_folder !== false
    : nodeMap.get(folderId)?.can_create_folder !== false;
  const currentFolderParentId = isNavPath ? null : folderId || null;
  const currentFolderName =
    breadcrumbs
      .map(item => item.name)
      .filter(Boolean)
      .join(' > ') || rootName;

  const toNavResp = (item: ExplorerItem): V1NavListResp => ({
    id: item.nav_id || (item.id || '').slice(NAV_PATH_PREFIX.length),
    name: item.name || '未命名目录',
  });

  const closeDeleteNavConfirm = () => {
    setDeletingNav(null);
    setDeleteConfirmInput('');
  };

  const handleDeleteNavConfirm = () => {
    if (!deletingNav?.id || !kb_id) return;
    const navId = deletingNav.id;
    deleteApiV1NavDelete({ id: navId, kb_id }).then(() => {
      message.success('删除成功');
      closeDeleteNavConfirm();
      if (folderId === `${NAV_PATH_PREFIX}${navId}`) {
        onFolderChange('');
      }
      onNavDeleted?.(navId);
      refresh();
    });
  };

  const openMove = (items: DomainNodeListItemResp[]) => {
    setMoveSelectedItems(items);
    setMoveOpen(true);
  };

  const withMoveAction = (
    actions: TreeMenuItem[],
    item: DomainNodeListItemResp,
  ): TreeMenuItem[] => {
    const moveAction: TreeMenuItem = {
      key: 'move',
      label: '移动',
      onClick: () => openMove([item]),
    };
    const deleteIndex = actions.findIndex(action => action.key === 'delete');
    if (deleteIndex < 0) return [...actions, moveAction];
    return [
      ...actions.slice(0, deleteIndex),
      moveAction,
      ...actions.slice(deleteIndex),
    ];
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
            (folderId ? currentFolderCanCreate : canManageRoots) && (
              <Stack direction='row' alignItems='center' gap={1} flexShrink={0}>
                {!folderId ? (
                  <Button
                    size='small'
                    variant='outlined'
                    startIcon={
                      <IconJiahao sx={{ fontSize: '10px !important' }} />
                    }
                    onClick={() => setAddNavOpen(true)}
                  >
                    新建目录
                  </Button>
                ) : (
                  <>
                    <Button
                      size='small'
                      disabled={!currentFolderCanCreateFolder}
                      variant='outlined'
                      onClick={() => {
                        setCreateParentId(currentFolderParentId);
                        setCreateNavId(activeNavId || null);
                        setCreateType(1);
                      }}
                    >
                      新建文件夹
                    </Button>
                    <Button
                      size='small'
                      variant='outlined'
                      onClick={() => {
                        setCreateParentId(currentFolderParentId);
                        setCreateNavId(activeNavId || null);
                        setCreateType(2);
                      }}
                    >
                      新建文档
                    </Button>
                    <Button
                      size='small'
                      variant='outlined'
                      onClick={() => {
                        setDocImportTarget({
                          parentId: currentFolderParentId,
                          navId: activeNavId || null,
                        });
                        setDocImportOpen(true);
                      }}
                    >
                      导入文档
                    </Button>
                    <Button
                      size='small'
                      variant='contained'
                      disabled={!currentFolderCanCreateFolder}
                      onClick={() => {
                        setBatchImportParentId(currentFolderParentId);
                        setBatchImportParentName(currentFolderName);
                        setBatchImportNavId(activeNavId || null);
                        setBatchImportOpen(true);
                      }}
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
                  onClick={() => setDeleteSelectedDocsOpen(true)}
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
                {loading ? '正在加载目录…' : '此目录为空'}
              </Typography>
            </Stack>
          ) : (
            <Stack gap={0.5}>
              {currentItems.map(item => {
                const isFolder = item.type === 1;
                const itemReadOnly = readOnlyUser || item.can_manage === false;
                const treeItem: ITreeItem = {
                  ...(item as unknown as ITreeItem),
                  id: item.id || '',
                  name: item.name || '',
                  type: isFolder ? 1 : 2,
                  parentId: item.parent_id,
                  rag_status: item.rag_info?.status,
                  rag_message: item.rag_info?.message,
                  kind: item.explorerKind ? 'nav' : undefined,
                };
                const navDownloadAction =
                  item.explorerKind && !itemReadOnly && menu
                    ? menu({
                        item: {
                          ...treeItem,
                          id: item.nav_id || treeItem.id,
                          kind: 'nav',
                        },
                        createItem: () => undefined,
                        renameItem: () => undefined,
                        isEditing: false,
                        removeItem: () => undefined,
                      }).find(action => action.key === 'download-folder')
                    : undefined;
                const actionMenu: TreeMenuItem[] = item.explorerKind
                  ? !itemReadOnly
                    ? [
                        {
                          key: 'folder',
                          label: '创建文件夹',
                          onClick: () => {
                            setCreateParentId(null);
                            setCreateNavId(item.nav_id || null);
                            setCreateType(1);
                          },
                        },
                        {
                          key: 'doc',
                          label: '创建文档',
                          children: [
                            {
                              key: 'rich_text',
                              label: '创建富文本',
                              onClick: () => {
                                setCreateParentId(null);
                                setCreateNavId(item.nav_id || null);
                                setCreateType(2);
                              },
                            },
                            {
                              key: 'md',
                              label: '创建 Markdown',
                              onClick: () => {
                                setCreateParentId(null);
                                setCreateNavId(item.nav_id || null);
                                setCreateType(2);
                              },
                            },
                          ],
                        },
                        {
                          key: ConstsCrawlerSource.CrawlerSourceFile,
                          label: '导入文档',
                          onClick: () => {
                            setDocImportTarget({
                              parentId: null,
                              navId: item.nav_id || null,
                            });
                            setDocImportOpen(true);
                          },
                        },
                        {
                          key: 'batch-import-zip',
                          label: '批量导入 ZIP',
                          onClick: () => {
                            setBatchImportParentId(null);
                            setBatchImportParentName(item.name || '根目录');
                            setBatchImportNavId(item.nav_id || null);
                            setBatchImportOpen(true);
                          },
                        },
                        ...(navDownloadAction ? [navDownloadAction] : []),
                        {
                          key: 'edit-nav',
                          label: '修改目录',
                          onClick: () => {
                            setEditingNav(toNavResp(item));
                            setAddNavOpen(true);
                          },
                        },
                        {
                          key: 'permission',
                          label: '目录权限',
                          onClick: () => {
                            setPermissionNav(toNavResp(item));
                            setPermissionOpen(true);
                          },
                        },
                        ...(groups.length > 1
                          ? [
                              {
                                key: 'delete-nav',
                                label: '删除目录',
                                color: 'error' as const,
                                onClick: () => setDeletingNav(toNavResp(item)),
                              },
                            ]
                          : []),
                      ]
                    : []
                  : menu
                    ? menu({
                        item: treeItem,
                        createItem: type => {
                          setCreateParentId(
                            item.explorerKind ? null : item.id || null,
                          );
                          setCreateType(type);
                        },
                        renameItem: () => {
                          setEditingId(item.id || null);
                          setEditingName(item.name || '');
                        },
                        isEditing: editingId === item.id,
                        removeItem: () => undefined,
                      })
                        .filter(
                          action =>
                            !(
                              ['folder', 'batch-import-zip'].includes(
                                action.key,
                              ) && item.can_create_folder === false
                            ) &&
                            !(
                              action.key === 'doc' &&
                              item.can_create_document === false
                            ),
                        )
                        .map(action => {
                          if (
                            action.key === ConstsCrawlerSource.CrawlerSourceFile
                          ) {
                            return {
                              ...action,
                              onClick: () => {
                                setDocImportTarget({
                                  parentId: item.id || null,
                                  navId: item.nav_id || activeNavId || null,
                                });
                                setDocImportOpen(true);
                              },
                            };
                          }
                          if (action.key === 'batch-import-zip') {
                            return {
                              ...action,
                              onClick: () => {
                                setBatchImportParentId(
                                  item.explorerKind ? null : item.id || null,
                                );
                                setBatchImportParentName(
                                  [
                                    ...breadcrumbs.map(crumb => crumb.name),
                                    item.name,
                                  ]
                                    .filter(Boolean)
                                    .join(' > ') || '根目录',
                                );
                                setBatchImportNavId(
                                  item.nav_id || activeNavId || null,
                                );
                                setBatchImportOpen(true);
                              },
                            };
                          }
                          if (action.key === 'folder') {
                            return {
                              ...action,
                              onClick: () => {
                                setCreateParentId(
                                  item.explorerKind ? null : item.id || null,
                                );
                                setCreateNavId(
                                  item.nav_id || activeNavId || null,
                                );
                                setCreateType(1);
                              },
                            };
                          }
                          if (action.key === 'doc') {
                            return {
                              ...action,
                              children: action.children?.map(child => ({
                                ...child,
                                onClick: () => {
                                  setCreateParentId(
                                    item.explorerKind ? null : item.id || null,
                                  );
                                  setCreateNavId(
                                    item.nav_id || activeNavId || null,
                                  );
                                  setCreateType(2);
                                },
                              })),
                            };
                          }
                          return action;
                        })
                    : [];
                const resolvedActionMenu =
                  !item.explorerKind && !itemReadOnly
                    ? withMoveAction(actionMenu, item)
                    : actionMenu.filter(
                        action =>
                          !(
                            ['folder', 'batch-import-zip'].includes(
                              action.key,
                            ) && item.can_create_folder === false
                          ) &&
                          !(
                            action.key === 'doc' &&
                            item.can_create_document === false
                          ),
                      );
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
                    onClick={() => setSelectedId(item.id || null)}
                    onDoubleClick={() => {
                      setSelectedId(item.id || null);
                      if (isFolder) onFolderChange(item.id || '');
                      else onDocumentSelect(item);
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
                      {!item.explorerKind && item.id && (
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
                    {isFolder && loading ? (
                      <CircularProgress
                        size={24}
                        aria-label={`正在加载 ${item.name} 的目录`}
                        sx={{ mx: '2px' }}
                      />
                    ) : isFolder ? (
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
                      {editingId === item.id ? (
                        <Stack direction='row' alignItems='center' gap={1}>
                          <TextField
                            size='small'
                            value={editingName}
                            autoFocus
                            onChange={event =>
                              setEditingName(event.target.value)
                            }
                            onClick={event => event.stopPropagation()}
                            onKeyDown={event => {
                              if (event.key === 'Enter') {
                                event.preventDefault();
                                if (!editingName.trim() || !kb_id || !item.id)
                                  return;
                                putApiV1NodeDetail({
                                  id: item.id,
                                  kb_id,
                                  nav_id: item.nav_id || navId || '',
                                  name: editingName.trim(),
                                }).then(() => {
                                  setEditingId(null);
                                  refresh();
                                });
                              }
                              if (event.key === 'Escape') setEditingId(null);
                            }}
                            sx={{ flex: 1 }}
                          />
                          <Button
                            size='small'
                            onClick={event => {
                              event.stopPropagation();
                              if (!editingName.trim() || !kb_id || !item.id)
                                return;
                              putApiV1NodeDetail({
                                id: item.id,
                                kb_id,
                                nav_id: item.nav_id || navId || '',
                                name: editingName.trim(),
                              }).then(() => {
                                setEditingId(null);
                                refresh();
                              });
                            }}
                          >
                            保存
                          </Button>
                        </Stack>
                      ) : (
                        <Typography variant='body2' noWrap title={item.name}>
                          {item.name || '未命名'}
                        </Typography>
                      )}
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
                    {isFolder && !item.explorerKind && !itemReadOnly && (
                      <Stack direction='row' alignItems='center' gap={0.25}>
                        <AddDocBtn
                          parentId={item.id}
                          navId={item.nav_id}
                          refresh={refresh}
                          exportFile={false}
                          batchImport={false}
                          context={
                            <IconButton
                              size='small'
                              onClick={event => event.stopPropagation()}
                              title='新建'
                            >
                              <IconJiahao sx={{ fontSize: 18 }} />
                            </IconButton>
                          }
                        />
                      </Stack>
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
      <BatchZipImport
        open={batchImportOpen}
        parentId={batchImportParentId}
        refresh={refresh}
        onClose={() => {
          setBatchImportOpen(false);
          setBatchImportParentId(null);
          setBatchImportParentName('根目录');
          setBatchImportNavId(null);
        }}
        parentName={batchImportParentName}
        navId={batchImportNavId || activeNavId}
      />
      <DocDelete
        open={deleteSelectedDocsOpen}
        onClose={() => setDeleteSelectedDocsOpen(false)}
        data={selectedItems}
        onDeleted={() => {
          setDeleteSelectedDocsOpen(false);
          setSelectedItemIds([]);
          setSelectedId(null);
          refresh();
        }}
      />
      <MoveDocs
        open={moveOpen}
        data={activeNodes}
        selected={moveSelectedItems}
        currentNavId={activeNavId}
        refresh={refresh}
        onMoved={() => {
          setSelectedItemIds([]);
          setSelectedId(null);
          refresh();
        }}
        onClose={() => {
          setMoveOpen(false);
          setMoveSelectedItems([]);
        }}
      />
      <DocAddByCustomText
        type={createType || 2}
        open={createType !== null}
        parentId={createParentId || undefined}
        navId={createNavId || activeNavId}
        refresh={refresh}
        onClose={() => {
          setCreateType(null);
          setCreateParentId(null);
          setCreateNavId(null);
        }}
      />
      <AddDocByType
        type={ConstsCrawlerSource.CrawlerSourceFile}
        open={docImportOpen}
        refresh={refresh}
        onCancel={() => {
          setDocImportOpen(false);
          setDocImportTarget(null);
        }}
        parentId={
          docImportTarget ? docImportTarget.parentId : currentFolderParentId
        }
        navId={docImportTarget ? docImportTarget.navId : activeNavId}
      />
      <NavEditModal
        open={addNavOpen}
        onClose={() => {
          setAddNavOpen(false);
          setEditingNav(null);
        }}
        onSuccess={() => {
          setAddNavOpen(false);
          setEditingNav(null);
          refresh();
        }}
        nav={editingNav}
        kb_id={kb_id}
      />
      <NavPermissionModal
        open={permissionOpen}
        nav={permissionNav}
        kb_id={kb_id}
        onClose={() => {
          setPermissionOpen(false);
          setPermissionNav(null);
        }}
        onSuccess={() => {
          setPermissionOpen(false);
          setPermissionNav(null);
          refresh();
        }}
      />
      <Modal
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <ErrorOutlineIcon sx={{ color: 'warning.main' }} />
            确认删除目录？
          </Stack>
        }
        open={!!deletingNav}
        width={480}
        okText='确认删除'
        okButtonProps={{
          sx: { bgcolor: 'error.main' },
          disabled: deleteConfirmInput !== (deletingNav?.name || '未命名'),
        }}
        onCancel={closeDeleteNavConfirm}
        onOk={handleDeleteNavConfirm}
      >
        <Box sx={{ fontSize: 14, lineHeight: 1.6, color: 'text.secondary' }}>
          <Box component='p' sx={{ m: 0, mb: 1 }}>
            删除目录「
            <Box component='span' sx={{ fontWeight: 600 }}>
              {deletingNav?.name || '未命名'}
            </Box>
            」后，将
            <Box component='span' sx={{ color: 'error.main', fontWeight: 600 }}>
              同步删除该目录下所有文档
            </Box>
            ，且
            <Box component='span' sx={{ color: 'error.main', fontWeight: 600 }}>
              不可恢复
            </Box>
            ，请谨慎操作。
          </Box>
          <Box sx={{ mt: 2 }}>
            <TextField
              fullWidth
              size='small'
              placeholder='请输入当前目录名称，并确认删除'
              value={deleteConfirmInput}
              onChange={event => setDeleteConfirmInput(event.target.value)}
              error={
                deleteConfirmInput.length > 0 &&
                deleteConfirmInput !== (deletingNav?.name || '未命名')
              }
              helperText={
                deleteConfirmInput.length > 0 &&
                deleteConfirmInput !== (deletingNav?.name || '未命名')
                  ? '名称不正确，请输入正确的目录名称'
                  : ''
              }
              sx={{ '& .MuiFormHelperText-root': { m: 0, mt: 0.5 } }}
            />
          </Box>
        </Box>
      </Modal>
    </Card>
  );
};

export default DocPageFileManager;
