import Card from '@/components/Card';
import DragTree, { type DragTreeHandle } from '@/components/Drag/DragTree';
import type { TreeMenuItem } from '@/components/Drag/DragTree/TreeMenu';
import type { ITreeItem } from '@/api';
import Cascader from '@/components/Cascader';
import EmptyState from '@/components/EmptyState';
import Loading from '@/components/Loading';
import { useURLSearchParams } from '@/hooks';
import { deleteApiV1NavDelete } from '@/request/Nav';
import { postApiV1NodeRestudy } from '@/request/Node';
import { getNodePermissionOptions } from '@/request/NodeManagement';
import { ConstsCrawlerSource } from '@/request/types';
import type { DomainNodeListItemResp, V1NavListResp } from '@/request/types';
import type { GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp } from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import { setIsRefreshDocList, setNavId } from '@/store/slices/config';
import { addOpacityToColor } from '@/utils';
import { Ellipsis, message, Modal } from '@/components/ui';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import {
  Box,
  Button,
  IconButton,
  LinearProgress,
  Stack,
  TextField,
  Typography,
  useTheme,
} from '@mui/material';
import {
  IconDrag,
  IconGengduo,
  IconJiahao,
  IconXiajiantou,
} from '@panda-wiki/icons';
import { convertToTree, type TreeDragHandlers } from '@/utils/drag';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import NavEditModal from './NavEditModal';
import NavPermissionModal from './NavPermissionModal';
import DocListModals from '../DocPageList/DocListModals';
import { useDocTreeMenu } from '../DocPageList/useDocTreeMenu';
import {
  collectOpenFolderIds,
  findItemInTree,
  removeDeep,
  reopenFolders,
} from '../DocPageList/utils';
import BatchZipImport from '../../component/BatchZipImport';
import {
  getApiV1FileFolderDownload,
  postApiV1FileFolderDownload,
} from '@/request/BatchImport';
import { getApiV1FileDownloadUrl } from '@/request/File';
import type { FolderDownloadTask } from '@/request/BatchImport';
import FileStorageInfo from '@/pages/document/editor/edit/FileStorageInfo';
import {
  addDocumentPermissionTags,
  type PermissionAudienceCatalog,
} from '@/pages/document/utils/permissionTags';

const SortableNavItem = ({
  nav,
  selected,
  onSelect,
  onEdit,
  onPermission,
  onDelete,
  showDelete,
  isLast,
  selectedItemRef,
}: {
  nav: V1NavListResp;
  selected: boolean;
  onSelect: () => void;
  onEdit: () => void;
  onPermission: () => void;
  onDelete: () => void;
  showDelete: boolean;
  isLast: boolean;
  selectedItemRef?: React.RefObject<HTMLDivElement | null>;
}) => {
  const theme = useTheme();
  const id = nav.id || '';
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
    isOver,
  } = useSortable({ id });

  const setRef = useCallback(
    (el: HTMLDivElement | null) => {
      setNodeRef(el);
      if (selected && selectedItemRef) {
        (
          selectedItemRef as React.MutableRefObject<HTMLDivElement | null>
        ).current = el;
      }
    },
    [setNodeRef, selected, selectedItemRef],
  );

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  const menuItems = [
    {
      key: 'edit',
      label: (
        <Stack
          direction='row'
          alignItems='center'
          sx={{
            fontSize: 14,
            px: 2,
            lineHeight: '40px',
            height: 40,
            width: 140,
            borderRadius: '5px',
            cursor: 'pointer',
            '&:hover': {
              bgcolor: addOpacityToColor(theme.palette.primary.main, 0.1),
            },
          }}
        >
          修改栏目
        </Stack>
      ),
      onClick: onEdit,
    },
    {
      key: 'permission',
      label: (
        <Stack
          direction='row'
          alignItems='center'
          sx={{
            fontSize: 14,
            px: 2,
            lineHeight: '40px',
            height: 40,
            width: 140,
            borderRadius: '5px',
            cursor: 'pointer',
            '&:hover': {
              bgcolor: addOpacityToColor(theme.palette.primary.main, 0.1),
            },
          }}
        >
          目录权限
        </Stack>
      ),
      onClick: onPermission,
    },
    ...(showDelete
      ? [
          {
            key: 'delete',
            label: (
              <Stack
                direction='row'
                alignItems='center'
                sx={{
                  fontSize: 14,
                  px: 2,
                  lineHeight: '40px',
                  height: 40,
                  width: 140,
                  borderRadius: '5px',
                  cursor: 'pointer',
                  color: 'error.main',
                  '&:hover': {
                    bgcolor: addOpacityToColor(theme.palette.primary.main, 0.1),
                  },
                }}
              >
                删除栏目
              </Stack>
            ),
            onClick: onDelete,
          },
        ]
      : []),
  ];

  return (
    <Box
      ref={setRef}
      style={style}
      sx={{
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        px: 2,
        py: 2,
        cursor: 'pointer',
        ...(!isLast && {
          borderBottom: '1px dashed',
          borderColor: 'divider',
        }),
        '&:hover .nav-name': {
          color: 'primary.main',
        },
        opacity: isDragging ? 0.5 : 1,
        ...(isOver && {
          bgcolor: addOpacityToColor(theme.palette.primary.main, 0.12),
          borderRadius: 1,
        }),
      }}
      onClick={onSelect}
    >
      {selected && (
        <Box
          sx={{
            position: 'absolute',
            left: -2,
            top: '50%',
            transform: 'translateY(-50%)',
            width: 4,
            height: 24,
            borderRadius: 1,
            bgcolor: 'primary.main',
          }}
        />
      )}
      <Box
        {...attributes}
        {...listeners}
        component='span'
        onClick={e => e.stopPropagation()}
        sx={{
          display: 'flex',
          cursor: 'grab',
          '&:active': { cursor: 'grabbing' },
        }}
      >
        <IconDrag sx={{ fontSize: 16 }} />
      </Box>
      <Ellipsis
        className='nav-name'
        sx={{
          flex: 1,
          width: 0,
          fontSize: 14,
          fontWeight: selected ? 600 : 400,
          color: selected ? 'primary.main' : 'text.primary',
        }}
      >
        {nav.name || '未命名'}
      </Ellipsis>
      <Box onClick={e => e.stopPropagation()} sx={{ display: 'inline-flex' }}>
        <Cascader
          list={menuItems}
          context={
            <IconButton size='small'>
              <IconGengduo sx={{ fontSize: 16 }} />
            </IconButton>
          }
          anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
          transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        />
      </Box>
    </Box>
  );
};

interface DocPageNavsProps {
  navList: V1NavListResp[];
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  onNavListChange: React.Dispatch<React.SetStateAction<V1NavListResp[]>>;
  onNavDeleted?: (navId: string) => void;
  /** 目录修改/移动后刷新 Header 统计 */
  refresh?: () => void;
  isSearching?: boolean;
  loading?: boolean;
  width?: number;
  onWidthChange?: (width: number) => void;
  onDocumentSelect?: (item: ITreeItem) => void;
  onFolderSelect?: (item: ITreeItem) => void;
  fill?: boolean;
  wikiUrl?: string;
  onPublishOpen?: (ids?: string[]) => void;
  readOnlyUser?: boolean;
  registerTreeDragHandlers?: (handlers: TreeDragHandlers | null) => void;
  onDocumentMenuReady?: (menu: ReturnType<typeof useDocTreeMenu>) => void;
}

const DocPageNavs = ({
  navList: navListProp,
  groups,
  onNavListChange,
  onNavDeleted,
  refresh,
  isSearching = false,
  loading = false,
  width = 300,
  onWidthChange,
  onDocumentSelect,
  onFolderSelect,
  fill = false,
  wikiUrl = '',
  onPublishOpen,
  readOnlyUser = false,
  registerTreeDragHandlers,
  onDocumentMenuReady,
}: DocPageNavsProps) => {
  const dispatch = useAppDispatch();
  const { kb_id } = useAppSelector(state => state.config);
  const [permissionCatalog, setPermissionCatalog] =
    useState<PermissionAudienceCatalog>({
      departments: {},
      levels: {},
      users: {},
    });
  const [searchParams, setSearchParams] = useURLSearchParams();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [editingNav, setEditingNav] = useState<V1NavListResp | null>(null);
  const [permissionOpen, setPermissionOpen] = useState(false);
  const [permissionNav, setPermissionNav] = useState<V1NavListResp | null>(
    null,
  );
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deletingNav, setDeletingNav] = useState<V1NavListResp | null>(null);
  const [deleteConfirmInput, setDeleteConfirmInput] = useState('');
  const [opraData, setOpraData] = useState<DomainNodeListItemResp[]>([]);
  const [modalList, setModalList] = useState<DomainNodeListItemResp[]>([]);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [urlOpen, setUrlOpen] = useState(false);
  const [summaryOpen, setSummaryOpen] = useState(false);
  const [moreSummaryOpen, setMoreSummaryOpen] = useState(false);
  const [statusOpen, setStatusOpen] = useState<'delete' | null>(null);
  const [batchImportParentId, setBatchImportParentId] = useState<string | null>(
    null,
  );
  const [batchImportParentName, setBatchImportParentName] = useState('根目录');
  const [batchImportNavId, setBatchImportNavId] = useState<string | null>(null);
  const [batchImportOpen, setBatchImportOpen] = useState(false);
  const [folderDownloadTask, setFolderDownloadTask] =
    useState<FolderDownloadTask | null>(null);
  const [folderDownloadOpen, setFolderDownloadOpen] = useState(false);
  const [folderDownloadURL, setFolderDownloadURL] = useState('');
  const [storageInfoOpen, setStorageInfoOpen] = useState(false);
  const [storageInfoFileKey, setStorageInfoFileKey] = useState('');
  const [moveOpen, setMoveOpen] = useState(false);
  const [propertiesOpen, setPropertiesOpen] = useState(false);
  const [importKey, setImportKey] = useState<ConstsCrawlerSource | null>(null);
  const [isBatch, setIsBatch] = useState(false);
  const dragTreeRef = useRef<DragTreeHandle>(null);
  const selectedItemRef = useRef<HTMLDivElement | null>(null);
  const hasScrolledToSelectedRef = useRef(false);
  const expend = true;
  const navs = navListProp || [];

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
        // Generic permission tags remain useful if the audience catalog is unavailable.
      });
    return () => {
      active = false;
    };
  }, [kb_id]);

  const sortedNavs = useMemo(
    () => [...navs].sort((a, b) => (a.position ?? 0) - (b.position ?? 0)),
    [navs],
  );

  const unifiedTree = useMemo<ITreeItem[]>(() => {
    const attachNav = (items: ITreeItem[], navId: string): ITreeItem[] =>
      items.map(item => ({
        ...item,
        nav_id: navId,
        canHaveChildren:
          item.type === 1
            ? (dragItem: ITreeItem) => dragItem.kind !== 'nav'
            : false,
        children: item.children
          ? attachNav(item.children, navId)
          : item.children,
      }));

    const tree: ITreeItem[] = sortedNavs.map(nav => {
      const group = groups.find(item => item.nav_id === nav.id);
      const children = attachNav(convertToTree(group?.list || []), nav.id!);
      // The grouped node-list response is also the authoritative source for
      // visible columns. Keep its permissions as a fallback so column tags do
      // not disappear when the separate nav-list state has not been populated.
      const permissions = nav.permissions ?? group?.permissions;
      return {
        id: nav.id!,
        name: nav.name || '未命名目录',
        level: 0,
        type: 1,
        kind: 'nav',
        permissions,
        effective_management_grants: permissions?.management_grants,
        can_manage: nav.can_manage ?? group?.can_manage,
        can_create_document:
          nav.can_create_document ?? group?.can_create_document,
        can_create_folder: nav.can_create_folder ?? group?.can_create_folder,
        nav_id: nav.id!,
        children,
        canHaveChildren: (dragItem: ITreeItem) => dragItem.kind !== 'nav',
        // Entering the document workspace starts compact; users can expand
        // any directory on demand.
        collapsed: true,
      };
    });
    return addDocumentPermissionTags(tree, permissionCatalog);
  }, [groups, sortedNavs, permissionCatalog]);
  const [treeData, setTreeData] = useState<ITreeItem[]>(unifiedTree);

  useEffect(() => {
    setTreeData(prev => {
      if (prev.length === 0) return unifiedTree;
      const openIds = collectOpenFolderIds(prev);
      if (openIds.size === 0) return unifiedTree;
      return reopenFolders(unifiedTree, openIds);
    });
  }, [unifiedTree]);

  useEffect(() => {
    const navIdFromStorage = kb_id
      ? localStorage.getItem(`nav_id_${kb_id}`)
      : null;
    const validInList = (id: string | null) =>
      id && sortedNavs.some(n => n.id === id);
    if (sortedNavs.length > 0) {
      const idToUse = validInList(navIdFromStorage)
        ? navIdFromStorage!
        : sortedNavs[0]?.id || null;
      if (idToUse) {
        setSelectedId(idToUse);
        dispatch(setNavId(idToUse));
      }
    } else {
      setSelectedId(null);
      const rest: Record<string, string> = {};
      searchParams.forEach((v, k) => {
        if (k !== 'nav_id') rest[k] = v;
      });
      setSearchParams(Object.keys(rest).length ? rest : null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kb_id, navListProp]);

  useEffect(() => {
    if (
      !selectedId ||
      !sortedNavs.length ||
      hasScrolledToSelectedRef.current ||
      !selectedItemRef.current
    ) {
      return;
    }
    hasScrolledToSelectedRef.current = true;
    selectedItemRef.current.scrollIntoView({
      behavior: 'auto',
      block: 'nearest',
      inline: 'nearest',
    });
  }, [selectedId, sortedNavs.length]);

  useEffect(() => {
    hasScrolledToSelectedRef.current = false;
  }, [kb_id]);

  useEffect(() => {
    setModalList(groups.find(group => group.nav_id === selectedId)?.list || []);
  }, [groups, selectedId]);

  const handleSelect = useCallback(
    (id: string) => {
      setSelectedId(id);
      dispatch(setNavId(id));
    },
    [dispatch],
  );

  const handleEdit = useCallback((nav: V1NavListResp) => {
    setEditingNav(nav);
    setEditOpen(true);
  }, []);

  const handleAdd = useCallback(() => {
    setEditingNav(null);
    setEditOpen(true);
  }, []);

  const handlePermission = useCallback((nav: V1NavListResp) => {
    setPermissionNav(nav);
    setPermissionOpen(true);
  }, []);

  const handleEditClose = useCallback(() => {
    setEditOpen(false);
    setEditingNav(null);
  }, []);

  const handleEditSuccess = useCallback(
    (updated?: { id: string; name: string }) => {
      if (updated) {
        onNavListChange(prev =>
          prev.map(n =>
            n.id === updated.id ? { ...n, name: updated.name } : n,
          ),
        );
        refresh?.();
      } else {
        dispatch(setIsRefreshDocList(true));
      }
      handleEditClose();
    },
    [onNavListChange, handleEditClose, dispatch, refresh],
  );

  const handleDeleteClick = useCallback((nav: V1NavListResp) => {
    setDeletingNav(nav);
    setDeleteConfirmOpen(true);
  }, []);

  const handleFolderDownload = useCallback(
    async (item: ITreeItem) => {
      if (!kb_id || !item.id) return;
      const popup = window.open('about:blank', '_blank');
      try {
        setFolderDownloadURL('');
        const task = await postApiV1FileFolderDownload({
          kb_id,
          ...(item.kind === 'nav'
            ? { nav_id: item.nav_id || item.id }
            : { node_id: item.id }),
        });
        setFolderDownloadTask(task);
        setFolderDownloadOpen(true);
        message.info('文件夹压缩任务已创建，可继续进行其他操作');
        let attempts = 0;
        const poll = async () => {
          attempts += 1;
          const result = await getApiV1FileFolderDownload(task.id);
          setFolderDownloadTask(result.task);
          if (result.task.status === 'completed' && result.download_url) {
            setFolderDownloadURL(result.download_url);
            if (popup) popup.location.href = result.download_url;
            message.success('文件夹压缩包已生成');
            return;
          }
          if (
            result.task.status === 'failed' ||
            result.task.status === 'expired'
          ) {
            popup?.close();
            message.error(result.task.error_message || '文件夹下载任务失败');
            return;
          }
          if (attempts < 720) window.setTimeout(poll, 5000);
          else popup?.close();
        };
        window.setTimeout(poll, 1000);
      } catch (error) {
        popup?.close();
        message.error(
          error instanceof Error ? error.message : '创建下载任务失败',
        );
      }
    },
    [kb_id],
  );

  const navMenu = useCallback(
    ({
      item,
      createItem,
    }: Parameters<typeof documentMenu>[0]): TreeMenuItem[] => {
      if (item.kind !== 'nav') return [];
      const nav = sortedNavs.find(value => value.id === item.id);
      if (!nav || nav.can_manage === false) return [];

      // 目录根节点和普通文件夹使用同一套内容操作：目录本身就是
      // 该导航下的第一层文件夹，创建时 parent_id 保持为空即可。
      const folderActions: TreeMenuItem[] = [
        ...(!readOnlyUser
          ? [
              {
                key: 'folder',
                label: '创建文件夹',
                onClick: () => createItem(1),
              },
              {
                key: 'doc',
                label: '创建文档',
                children: [
                  {
                    label: '创建富文本',
                    key: 'rich_text',
                    onClick: () => createItem(2, 'html'),
                  },
                  {
                    label: '创建 Markdown',
                    key: 'md',
                    onClick: () => createItem(2, 'md'),
                  },
                ],
              },
            ]
          : []),
        {
          key: ConstsCrawlerSource.CrawlerSourceFile,
          label: '导入文档',
          onClick: () => handleUrl(item, ConstsCrawlerSource.CrawlerSourceFile),
        },
        ...(!readOnlyUser
          ? [
              {
                key: 'batch-import-zip',
                label: '批量导入 ZIP',
                onClick: () => {
                  setBatchImportParentId(null);
                  setBatchImportParentName(item.name || '根目录');
                  setBatchImportNavId(nav.id || item.id);
                  setBatchImportOpen(true);
                },
              },
              {
                key: 'download-folder',
                label: '下载整个目录（ZIP）',
                onClick: () => handleFolderDownload(item),
              },
            ]
          : []),
      ];

      return [
        ...folderActions,
        { key: 'edit', label: '修改目录', onClick: () => handleEdit(nav) },
        {
          key: 'permission',
          label: '目录权限',
          onClick: () => handlePermission(nav),
        },
        ...(sortedNavs.length > 1
          ? [
              {
                key: 'delete',
                label: '删除目录',
                color: 'error' as const,
                onClick: () => handleDeleteClick(nav),
              },
            ]
          : []),
      ];
    },
    [
      handleDeleteClick,
      handleEdit,
      handleFolderDownload,
      handlePermission,
      readOnlyUser,
      sortedNavs,
    ],
  );

  const getNodeData = useCallback(
    (item: ITreeItem): DomainNodeListItemResp[] => {
      const group = groups.find(value => value.nav_id === item.nav_id);
      const found = group?.list?.find(node => node.id === item.id);
      const node = found || findItemInTree([item], item.id);
      return node ? [node] : [];
    },
    [groups],
  );

  const handleUrl = useCallback(
    (item: ITreeItem, key: ConstsCrawlerSource) => {
      setImportKey(key);
      setUrlOpen(true);
      setOpraData(getNodeData(item));
    },
    [getNodeData],
  );
  const handleDelete = useCallback(
    (item: ITreeItem) => {
      setDeleteOpen(true);
      setOpraData(getNodeData(item));
    },
    [getNodeData],
  );
  const handleMove = useCallback(
    (item: ITreeItem) => {
      setMoveOpen(true);
      setIsBatch(false);
      setOpraData(getNodeData(item));
    },
    [getNodeData],
  );
  const handleProperties = useCallback(
    (item: ITreeItem) => {
      setPropertiesOpen(true);
      setIsBatch(false);
      setOpraData(getNodeData(item));
    },
    [getNodeData],
  );
  const handleFrontDoc = useCallback(
    (id: string) => {
      const item = groups
        .flatMap(group => group.list || [])
        .find(value => value.id === id);
      if (item?.status !== 2 && !item?.publisher_id) {
        message.warning('当前文档未发布，无法查看前台文档');
        return;
      }
      window.open(`${wikiUrl}/node/${id}`, '_blank');
    },
    [groups, wikiUrl],
  );
  const handleDownloadFile = useCallback(async (item: ITreeItem) => {
    if (!item.file_key) return;
    try {
      const { url } = await getApiV1FileDownloadUrl({
        key: item.file_key,
        attachment: true,
        filename: item.file_name || item.name,
      });
      window.location.assign(url);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '原文件下载失败');
    }
  }, []);
  const handleFileStorageInfo = useCallback((item: ITreeItem) => {
    if (!item.file_key) return;
    setStorageInfoFileKey(item.file_key);
    setStorageInfoOpen(true);
  }, []);
  const handleBatchImport = useCallback((item: ITreeItem) => {
    setBatchImportParentId(item.kind === 'nav' ? null : item.id);
    setBatchImportParentName(item.name || '根目录');
    setBatchImportNavId(item.nav_id || (item.kind === 'nav' ? item.id : null));
    setBatchImportOpen(true);
  }, []);
  const handlePublish = useCallback(
    (item: ITreeItem) => onPublishOpen?.([item.id]),
    [onPublishOpen],
  );
  const handleRestudy = useCallback(
    (item: ITreeItem) => {
      if (!kb_id || !item.id) return;
      postApiV1NodeRestudy({
        kb_id,
        node_ids: [item.id],
        priority: 100,
      }).then(() => {
        message.success('正在重建问答索引');
        refresh?.();
      });
    },
    [kb_id, refresh],
  );
  const documentMenu = useDocTreeMenu(
    {
      handleUrl,
      handleBatchImport,
      handleFolderDownload,
      handleDownloadFile,
      handleFileStorageInfo,
      handleMove,
      handleDelete,
      handlePublish,
      handleRestudy,
      handleProperties,
      handleFrontDoc,
    },
    readOnlyUser,
  );

  useEffect(() => {
    onDocumentMenuReady?.(documentMenu);
  }, [documentMenu, onDocumentMenuReady]);
  const treeMenu = useCallback(
    (options: Parameters<typeof documentMenu>[0]) =>
      options.item.kind === 'nav' ? navMenu(options) : documentMenu(options),
    [documentMenu, navMenu],
  );

  const handleTreeItemClick = useCallback(
    (item: ITreeItem) => {
      const targetNavId = item.kind === 'nav' ? item.id : item.nav_id;
      if (targetNavId && targetNavId !== selectedId) {
        handleSelect(targetNavId);
      }
      if (item.kind !== 'nav' && item.type === 2) {
        onDocumentSelect?.(item);
      } else if (item.kind !== 'nav' && item.type === 1) {
        onFolderSelect?.(item);
      }
    },
    [handleSelect, onDocumentSelect, selectedId],
  );

  const handleResizeStart = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (!onWidthChange || !expend) return;
      event.preventDefault();
      const startX = event.clientX;
      const startWidth = width;
      const minWidth = Math.max(240, Math.round(window.innerWidth * 0.3));
      const handleMove = (moveEvent: PointerEvent) => {
        const nextWidth = Math.min(
          Math.round(window.innerWidth * 0.65),
          Math.max(minWidth, startWidth + moveEvent.clientX - startX),
        );
        onWidthChange(nextWidth);
      };
      const handleUp = () => {
        window.removeEventListener('pointermove', handleMove);
        window.removeEventListener('pointerup', handleUp);
      };
      window.addEventListener('pointermove', handleMove);
      window.addEventListener('pointerup', handleUp);
    },
    [expend, onWidthChange, width],
  );

  const handleDeleteConfirmClose = useCallback(() => {
    setDeleteConfirmOpen(false);
    setDeletingNav(null);
    setDeleteConfirmInput('');
  }, []);

  const handleDeleteConfirm = useCallback(() => {
    if (!deletingNav) return;
    const nav = deletingNav;
    deleteApiV1NavDelete({ id: nav.id!, kb_id }).then(() => {
      message.success('删除成功');
      handleDeleteConfirmClose();
      const next = (navListProp || []).filter(n => n.id !== nav.id);
      onNavListChange(next);
      onNavDeleted?.(nav.id!);
      refresh?.();
      if (selectedId === nav.id && next.length > 0) {
        const first = next[0];
        if (first?.id) {
          setSelectedId(first.id);
          dispatch(setNavId(first.id));
        }
      } else if (next.length === 0) {
        setSelectedId(null);
        dispatch(setNavId(''));
        const rest: Record<string, string> = {};
        searchParams.forEach((v, k) => {
          if (k !== 'nav_id') rest[k] = v;
        });
        setSearchParams(Object.keys(rest).length ? rest : null);
      }
    });
  }, [
    deletingNav,
    kb_id,
    selectedId,
    navListProp,
    searchParams,
    setSearchParams,
    handleDeleteConfirmClose,
    onNavListChange,
    onNavDeleted,
    refresh,
    dispatch,
  ]);

  const showEmptySearch = isSearching && sortedNavs.length === 0;

  return (
    <Box
      sx={{
        position: 'relative',
        flex: fill && expend ? 1 : expend ? '0 0 auto' : '0 0 40px',
        minWidth: expend ? 0 : 40,
      }}
    >
      <Card
        sx={{
          width: fill && expend ? '100%' : expend ? width : 0,
          height: '100%',
          mr: expend ? 2 : 0,
          transition: 'width 0.15s ease-in-out, margin-right 0.15s ease-in-out',
        }}
      >
        {loading ? (
          <Loading sx={{ py: 4 }} />
        ) : showEmptySearch ? (
          <EmptyState text='无搜索结果' sx={{ p: 4 }} />
        ) : (
          <>
            <Box
              sx={{
                height: 'calc(100vh - 216px)',
                overflowY: 'auto',
                p: 1,
              }}
            >
              <DragTree
                readOnly={readOnlyUser}
                supportSelect={false}
                ref={dragTreeRef}
                menu={treeMenu}
                onItemClick={handleTreeItemClick}
                data={treeData}
                updateData={data => setTreeData([...data])}
                canRootHaveChildren={dragItem => dragItem.kind === 'nav'}
                registerDragHandlers={
                  readOnlyUser ? undefined : registerTreeDragHandlers
                }
                virtualized
              />
            </Box>
            {!isSearching &&
              !readOnlyUser &&
              (sortedNavs.length === 0 ||
                sortedNavs.some(nav => nav.can_configure !== false)) && (
                <Box
                  sx={{ px: 2, py: 1, borderTop: 1, borderColor: 'divider' }}
                >
                  <Button
                    fullWidth
                    variant='text'
                    startIcon={
                      <IconJiahao sx={{ fontSize: '10px !important' }} />
                    }
                    onClick={handleAdd}
                    sx={{
                      justifyContent: 'center',
                      textTransform: 'none',
                    }}
                  >
                    添加目录
                  </Button>
                </Box>
              )}
          </>
        )}
      </Card>
      {expend && onWidthChange && !fill && (
        <Box
          onPointerDown={handleResizeStart}
          sx={{
            position: 'absolute',
            top: 0,
            bottom: 0,
            left: width - 4,
            width: 8,
            cursor: 'col-resize',
            zIndex: 3,
            bgcolor: 'transparent',
            '&:after': {
              content: '""',
              position: 'absolute',
              left: 3,
              top: '50%',
              width: 2,
              height: 40,
              borderRadius: 2,
              bgcolor: 'divider',
              transform: 'translateY(-50%)',
            },
            '&:hover': {
              bgcolor: 'action.hover',
              '&:after': { bgcolor: 'primary.main' },
            },
          }}
        />
      )}
      <NavEditModal
        open={editOpen}
        onClose={handleEditClose}
        onSuccess={handleEditSuccess}
        nav={editingNav}
        kb_id={kb_id}
      />
      <DocListModals
        kb_id={kb_id}
        deleteOpen={deleteOpen}
        opraData={opraData}
        data={treeData}
        list={modalList}
        dragTreeRef={dragTreeRef}
        importKey={importKey}
        urlOpen={urlOpen}
        summaryOpen={summaryOpen}
        moreSummaryOpen={moreSummaryOpen}
        statusOpen={statusOpen}
        moveOpen={moveOpen}
        propertiesOpen={propertiesOpen}
        isBatch={isBatch}
        refresh={refresh || (() => {})}
        setData={setTreeData}
        onCloseDelete={() => {
          setDeleteOpen(false);
          setOpraData([]);
        }}
        onCancelAddDoc={() => {
          setUrlOpen(false);
          setOpraData([]);
        }}
        onCloseSummary={() => setSummaryOpen(false)}
        onCloseMoreSummary={() => setMoreSummaryOpen(false)}
        onCloseStatus={() => setStatusOpen(null)}
        onCloseMove={() => setMoveOpen(false)}
        onCloseProperties={() => setPropertiesOpen(false)}
        onOkProperties={() => {
          setPropertiesOpen(false);
          refresh?.();
        }}
        removeDeep={removeDeep}
      />
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
        navId={batchImportNavId || undefined}
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
          refresh?.();
        }}
      />
      <Modal
        title='文件夹下载'
        open={folderDownloadOpen && !!folderDownloadTask}
        width={460}
        footer={null}
        onCancel={() => {
          setFolderDownloadOpen(false);
          setFolderDownloadURL('');
        }}
      >
        {folderDownloadTask && (
          <Stack gap={1.5}>
            <Box sx={{ color: 'text.secondary', fontSize: 14 }}>
              {folderDownloadTask.status === 'completed'
                ? '压缩包已生成，可点击下方按钮下载。'
                : folderDownloadTask.status === 'failed'
                  ? folderDownloadTask.error_message || '压缩任务失败'
                  : '正在后台生成压缩包，可以关闭此窗口继续操作。'}
            </Box>
            {folderDownloadTask.status === 'waiting' ||
            folderDownloadTask.status === 'processing' ? (
              <>
                <LinearProgress
                  variant='determinate'
                  value={
                    folderDownloadTask.total_files > 0
                      ? (folderDownloadTask.processed_files /
                          folderDownloadTask.total_files) *
                        100
                      : 0
                  }
                />
                <Typography variant='body2' color='text.secondary'>
                  已处理 {folderDownloadTask.processed_files} /{' '}
                  {folderDownloadTask.total_files} 个文件
                </Typography>
              </>
            ) : null}
            {folderDownloadTask.status === 'completed' && folderDownloadURL ? (
              <Button
                component='a'
                href={folderDownloadURL}
                target='_blank'
                rel='noreferrer'
                variant='contained'
              >
                下载 ZIP
              </Button>
            ) : null}
          </Stack>
        )}
      </Modal>
      <Modal
        title='文件存储地址'
        open={storageInfoOpen}
        width={720}
        footer={null}
        onCancel={() => {
          setStorageInfoOpen(false);
          setStorageInfoFileKey('');
        }}
      >
        <FileStorageInfo fileKey={storageInfoFileKey} />
      </Modal>
      <Modal
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <ErrorOutlineIcon sx={{ color: 'warning.main' }} />
            确认删除栏目？
          </Stack>
        }
        open={deleteConfirmOpen}
        width={480}
        okText='确认删除'
        okButtonProps={{
          sx: { bgcolor: 'error.main' },
          disabled: deleteConfirmInput !== (deletingNav?.name || '未命名'),
        }}
        onCancel={handleDeleteConfirmClose}
        onOk={handleDeleteConfirm}
      >
        <Box sx={{ fontSize: 14, lineHeight: 1.6, color: 'text.secondary' }}>
          <Box component='p' sx={{ m: 0, mb: 1 }}>
            删除栏目「
            <Box component='span' sx={{ fontWeight: 600 }}>
              {deletingNav?.name || '未命名'}
            </Box>
            」后，将
            <Box component='span' sx={{ color: 'error.main', fontWeight: 600 }}>
              同步删除该栏目下所有文档
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
              placeholder={`请输入当前栏目名称，并确认删除`}
              value={deleteConfirmInput}
              onChange={e => setDeleteConfirmInput(e.target.value)}
              error={
                deleteConfirmInput.length > 0 &&
                deleteConfirmInput !== (deletingNav?.name || '未命名')
              }
              helperText={
                deleteConfirmInput.length > 0 &&
                deleteConfirmInput !== (deletingNav?.name || '未命名')
                  ? '名称不正确，请输入正确的栏目名称'
                  : ''
              }
              sx={{ '& .MuiFormHelperText-root': { m: 0, mt: 0.5 } }}
            />
          </Box>
        </Box>
      </Modal>
    </Box>
  );
};

export default DocPageNavs;
