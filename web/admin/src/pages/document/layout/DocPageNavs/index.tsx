import { branchKey, type BranchState } from '@/hooks/useLazyDocumentTree';
import Card from '@/components/Card';
import DragTree, { type DragTreeHandle } from '@/components/Drag/DragTree';
import type { ITreeItem } from '@/api';
import EmptyState from '@/components/EmptyState';
import Loading from '@/components/Loading';
import type {
  V1NavListResp,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
} from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import { setNavId } from '@/store/slices/config';
import { Box, Button } from '@mui/material';
import { IconJiahao } from '@panda-wiki/icons';
import { convertToTree, type TreeDragHandlers } from '@/utils/drag';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { collectOpenFolderIds, reopenFolders } from '../DocPageList/utils';
import { addDocumentPermissionTags } from '@/pages/document/utils/permissionTags';
import { useDocumentActions } from '../DocumentActionsProvider';

const EMPTY_BRANCHES: Record<string, BranchState> = {};

interface DocPageNavsProps {
  navList: V1NavListResp[];
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  /** 目录修改/移动后刷新 Header 统计 */
  refresh?: () => void;
  isSearching?: boolean;
  loading?: boolean;
  branches?: Record<string, BranchState>;
  onLoadChildren?: (nav: string, parent?: string) => Promise<void>;
  openFolderIds: Set<string>;
  onOpenFolderIdsChange: (ids: Set<string>) => void;
  width?: number;
  onWidthChange?: (width: number) => void;
  onDocumentSelect?: (item: ITreeItem) => void;
  onFolderSelect?: (item: ITreeItem) => void;
  fill?: boolean;
  readOnlyUser?: boolean;
  registerTreeDragHandlers?: (handlers: TreeDragHandlers | null) => void;
}

const DocPageNavs = ({
  navList: navListProp,
  groups,
  refresh,
  isSearching = false,
  loading = false,
  branches = EMPTY_BRANCHES,
  onLoadChildren,
  openFolderIds,
  onOpenFolderIdsChange,
  width = 300,
  onWidthChange,
  onDocumentSelect,
  onFolderSelect,
  fill = false,
  readOnlyUser = false,
  registerTreeDragHandlers,
}: DocPageNavsProps) => {
  const dispatch = useAppDispatch();
  const selectedId = useAppSelector(state => state.config.nav_id);
  const {
    menu: treeMenu,
    permissionCatalog,
    openAddNav,
    viewDocument,
  } = useDocumentActions();
  const dragTreeRef = useRef<DragTreeHandle>(null);
  const expend = true;
  const navs = navListProp || [];

  const sortedNavs = useMemo(
    () => [...navs].sort((a, b) => (a.position ?? 0) - (b.position ?? 0)),
    [navs],
  );

  const unifiedTree = useMemo<ITreeItem[]>(() => {
    const attachNav = (items: ITreeItem[], navId: string): ITreeItem[] =>
      items.map(item => ({
        ...item,
        nav_id: navId,
        collapsed: isSearching ? false : item.collapsed,
        children:
          item.type === 1 && branches[branchKey(navId, item.id)]
            ? [
                {
                  id: `loading:${item.id}`,
                  parentId: item.id,
                  name: '',
                  type: 2,
                  level: item.level + 1,
                  placeholder: branches[branchKey(navId, item.id)],
                  disableSorting: true,
                  retry: () => void onLoadChildren?.(navId, item.id),
                },
              ]
            : item.children
              ? attachNav(item.children, navId)
              : item.children,
        canHaveChildren:
          item.type === 1
            ? (dragItem: ITreeItem) => dragItem.kind !== 'nav'
            : false,
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
        hasChildren: group?.has_children,
        children: branches[branchKey(nav.id!)]
          ? [
              {
                id: `loading:${nav.id}`,
                parentId: nav.id,
                name: '',
                type: 2,
                level: 1,
                placeholder: branches[branchKey(nav.id!)],
                disableSorting: true,
                retry: () => void onLoadChildren?.(nav.id!, ''),
              },
            ]
          : children,
        canHaveChildren: (dragItem: ITreeItem) => dragItem.kind !== 'nav',
        // Entering the document workspace starts compact; users can expand
        // any directory on demand.
        collapsed: !isSearching,
      };
    });
    return addDocumentPermissionTags(tree, permissionCatalog);
  }, [
    groups,
    sortedNavs,
    permissionCatalog,
    branches,
    onLoadChildren,
    isSearching,
  ]);
  const [treeData, setTreeData] = useState<ITreeItem[]>(() =>
    reopenFolders(unifiedTree, openFolderIds),
  );

  useEffect(() => {
    setTreeData(prev => {
      if (prev.length === 0) return unifiedTree;
      const openIds = collectOpenFolderIds(prev);
      if (openIds.size === 0) return unifiedTree;
      return reopenFolders(unifiedTree, openIds);
    });
  }, [unifiedTree]);

  const handleSelect = useCallback(
    (id: string) => {
      dispatch(setNavId(id));
    },
    [dispatch],
  );

  const handleTreeItemClick = useCallback(
    (item: ITreeItem) => {
      const targetNavId = item.kind === 'nav' ? item.id : item.nav_id;
      if (targetNavId && targetNavId !== selectedId) {
        handleSelect(targetNavId);
      }
      if (item.type === 1 && !isSearching)
        void onLoadChildren?.(
          targetNavId || item.id,
          item.kind === 'nav' ? '' : item.id,
        );
      if (item.kind !== 'nav' && item.type === 2) {
        onDocumentSelect?.(item);
      } else if (item.type === 1) {
        onFolderSelect?.(item);
      }
    },
    [
      handleSelect,
      onDocumentSelect,
      onFolderSelect,
      selectedId,
      isSearching,
      onLoadChildren,
    ],
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
                onExpand={item =>
                  !isSearching
                    ? onLoadChildren?.(
                        item.nav_id || item.id,
                        item.kind === 'nav' ? '' : item.id,
                      )
                    : undefined
                }
                refresh={refresh}
                readOnly={readOnlyUser}
                supportSelect={false}
                ref={dragTreeRef}
                menu={treeMenu}
                onItemClick={handleTreeItemClick}
                onItemDoubleClick={viewDocument}
                data={treeData}
                updateData={data => {
                  setTreeData([...data]);
                  if (!isSearching)
                    onOpenFolderIdsChange(collectOpenFolderIds(data));
                }}
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
                    onClick={openAddNav}
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
    </Box>
  );
};

export default DocPageNavs;
