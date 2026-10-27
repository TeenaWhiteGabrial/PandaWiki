import { useLazyDocumentTree, branchKey } from '@/hooks/useLazyDocumentTree';
import { getTreeNodeStatuses } from '@/request/NodeTree';
import { useURLSearchParams } from '@/hooks';
import VersionPublish from '@/pages/release/components/VersionPublish';
import RagErrorReStart from '@/pages/document/component/RagErrorReStart';
import { postApiV1NavMove } from '@/request/Nav';
import { postApiV1NodeBatchMove, postApiV1NodeMoveNav } from '@/request/Node';
import { V1NavListResp } from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import { setIsRefreshDocList, setNavId } from '@/store/slices/config';
import type { TreeDragHandlers } from '@/utils/drag';
import { isOrdinaryReadOnlyUser } from '@/utils/permission';
import { resolveReaderBaseUrl } from '@/utils/readerUrl';
import { message } from '@/components/ui';
import {
  getRagStatusMeta,
  hasPollingRagTask,
} from '@/pages/document/utils/ragStatus';
import {
  DndContext,
  DragEndEvent,
  DragMoveEvent,
  DragOverEvent,
  DragStartEvent,
  PointerSensor,
  pointerWithin,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { arrayMove } from '@dnd-kit/sortable';
import { Box, Button, Stack } from '@mui/material';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import DocPageHeader from './DocPageHeader';
import DocPageNavs from './DocPageNavs';
import DocPagePreview from './DocPagePreview';
import DocPageFileManager from './DocPageFileManager';
import type { DocTreeMenuFn } from './DocPageList/types';
import ZipUploadTaskPanel from '../component/ZipUploadTaskPanel';
import LearningProgressPanel from '../component/LearningProgressPanel';

const Content = () => {
  const { kb_id, isRefreshDocList, kbDetail } = useAppSelector(
    state => state.config,
  );
  const user = useAppSelector(state => state.config.user);
  const readOnlyUser = isOrdinaryReadOnlyUser(user, kbDetail);
  const dispatch = useAppDispatch();
  const nav_id = useAppSelector(state => state.config.nav_id) || undefined;

  const [searchParams] = useURLSearchParams();
  const search = searchParams.get('search') || '';

  const [publishOpen, setPublishOpen] = useState(false);
  const [restudyOpen, setRestudyOpen] = useState(false);
  const [uploadTaskOpen, setUploadTaskOpen] = useState(false);
  const [learningProgressOpen, setLearningProgressOpen] = useState(false);
  const [publishIds, setPublishIds] = useState<string[]>([]);
  const {
    groups,
    setGroups,
    loading,
    error: treeError,
    branches,
    loadChildren,
    refresh: reloadTree,
  } = useLazyDocumentTree(kb_id || '', user?.id || '', search);
  const [navList, setNavList] = useState<V1NavListResp[]>([]);
  const [navWidth, setNavWidth] = useState(() =>
    Math.max(300, Math.round(window.innerWidth * 0.3)),
  );
  const [previewCollapsed, setPreviewCollapsed] = useState(true);
  const [fullscreen, setFullscreen] = useState(false);
  const [selectedDocumentId, setSelectedDocumentId] = useState<string | null>(
    null,
  );
  const [currentFolderId, setCurrentFolderId] = useState('');
  const [leftViewMode, setLeftViewMode] = useState<'tree' | 'explorer'>(() => {
    if (typeof window === 'undefined') return 'tree';
    const saved = localStorage.getItem('panda-wiki-document-view-mode');
    return saved === 'explorer' ? 'explorer' : 'tree';
  });
  const handleViewModeChange = useCallback((mode: 'tree' | 'explorer') => {
    setLeftViewMode(mode);
    localStorage.setItem('panda-wiki-document-view-mode', mode);
  }, []);
  const [explorerMenu, setExplorerMenu] = useState<DocTreeMenuFn | null>(null);
  const handleDocumentMenuReady = useCallback(
    (menu: DocTreeMenuFn) => setExplorerMenu(() => menu),
    [],
  );
  const minLeftWidth = Math.round(window.innerWidth * 0.3);
  const leftWidth = Math.max(navWidth, minLeftWidth);

  // The tree and explorer share the same left pane. Keep the tree wrapper in
  // the flex layout as well; without an explicit width/flex basis its inner
  // card can shrink to its content when the preview is collapsed.
  const leftPaneSx = {
    position: 'relative' as const,
    flex: previewCollapsed ? '1 1 auto' : `0 0 ${leftWidth}px`,
    width: previewCollapsed ? '100%' : leftWidth,
    height: 'calc(100vh - 183px)',
    minWidth: 0,
  };
  const explorerNavId = currentFolderId.startsWith('nav:')
    ? currentFolderId.slice(4)
    : groups.find(g => g.list?.some(n => n.id === currentFolderId))?.nav_id ||
      nav_id ||
      '';
  const explorerParentId = currentFolderId.startsWith('nav:')
    ? ''
    : currentFolderId;
  const explorerBranchState = currentFolderId
    ? branches[branchKey(explorerNavId, explorerParentId)]
    : undefined;
  const shouldPollRagStatus = hasPollingRagTask(groups);
  useEffect(() => {
    const next = groups
      .map(g => ({
        id: g.nav_id,
        name: g.nav_name,
        position: g.position ?? 0,
        permissions: g.permissions,
        can_manage: g.can_manage,
        can_create_document: g.can_create_document,
        can_create_folder: g.can_create_folder,
        can_configure: g.can_configure,
      }))
      .filter(n => n.id);
    setNavList(next);
    const stored = kb_id ? localStorage.getItem(`nav_id_${kb_id}`) : '';
    if (!next.some(n => n.id === nav_id))
      dispatch(
        setNavId(next.find(n => n.id === stored)?.id || next[0]?.id || ''),
      );
  }, [groups, kb_id, nav_id, dispatch]);

  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const refresh = useCallback(() => {
    void reloadTree();
    setRefreshTrigger(t => t + 1);
  }, [reloadTree]);

  const handleExplorerResizeStart = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      event.preventDefault();
      const startX = event.clientX;
      const startWidth = navWidth;
      const handleMove = (moveEvent: PointerEvent) => {
        setNavWidth(
          Math.min(
            Math.round(window.innerWidth * 0.65),
            Math.max(minLeftWidth, startWidth + moveEvent.clientX - startX),
          ),
        );
      };
      const handleUp = () => {
        window.removeEventListener('pointermove', handleMove);
        window.removeEventListener('pointerup', handleUp);
      };
      window.addEventListener('pointermove', handleMove);
      window.addEventListener('pointerup', handleUp);
    },
    [minLeftWidth, navWidth],
  );

  const handlePublishOpen = useCallback(
    (ids?: string[]) => {
      if (readOnlyUser) return;
      setPublishIds(ids ?? []);
      setPublishOpen(true);
    },
    [readOnlyUser],
  );

  const wikiUrl = useMemo(
    () => resolveReaderBaseUrl(kbDetail?.access_settings),
    [kbDetail?.access_settings],
  );
  const treeDragHandlersRef = useRef<TreeDragHandlers | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 3 } }),
  );

  const handleLayoutDragStart = useCallback((e: DragStartEvent) => {
    treeDragHandlersRef.current?.onDragStart?.(e);
  }, []);
  const handleLayoutDragMove = useCallback((e: DragMoveEvent) => {
    treeDragHandlersRef.current?.onDragMove?.(e);
  }, []);
  const handleLayoutDragOver = useCallback((e: DragOverEvent) => {
    treeDragHandlersRef.current?.onDragOver?.(e);
  }, []);
  const handleLayoutDragEnd = useCallback(
    (e: DragEndEvent) => {
      const { active, over } = e;
      if (!over) {
        treeDragHandlersRef.current?.onDragEnd?.(e);
        return;
      }
      const navIds = (navList || [])
        .map(n => n.id)
        .filter((id): id is string => !!id);
      const overIsNav = navIds.includes(over.id as string);
      const activeIsNav = navIds.includes(active.id as string);

      if (overIsNav && activeIsNav) {
        treeDragHandlersRef.current?.onDragCancel?.();
        // 目录之间拖拽排序
        const sorted = [...(navList || [])].sort(
          (a, b) => (a.position ?? 0) - (b.position ?? 0),
        );
        const oldIndex = sorted.findIndex(n => (n.id || '') === active.id);
        const newIndex = sorted.findIndex(n => (n.id || '') === over.id);
        if (oldIndex !== -1 && newIndex !== -1 && oldIndex !== newIndex) {
          const reordered = arrayMove(sorted, oldIndex, newIndex);
          const next = reordered.map((item, index) => ({
            ...item,
            position: index,
          }));
          setNavList(next);
          const prevId = next[newIndex - 1]?.id;
          const nextId = next[newIndex + 1]?.id;
          postApiV1NavMove({
            id: active.id as string,
            kb_id: kb_id!,
            prev_id: prevId,
            next_id: nextId,
          }).then(() => {
            message.success('顺序已更新');
            refresh();
          });
        }
        return;
      }
      if (activeIsNav && !overIsNav) {
        treeDragHandlersRef.current?.onDragCancel?.();
        return;
      }
      if (overIsNav && !activeIsNav) {
        const targetNavId = over.id as string;
        const activeNode = groups
          .flatMap(g => g.list || [])
          .find(item => item.id === active.id);
        if (!activeNode) {
          treeDragHandlersRef.current?.onDragCancel?.();
          return;
        }
        const isAlreadyAtTargetRoot =
          activeNode.nav_id === targetNavId && !activeNode.parent_id;
        if (isAlreadyAtTargetRoot) {
          treeDragHandlersRef.current?.onDragCancel?.();
          return;
        }

        treeDragHandlersRef.current?.onDragCancel?.();
        if (activeNode.nav_id === targetNavId) {
          postApiV1NodeBatchMove({
            ids: [active.id as string],
            kb_id: kb_id!,
            parent_id: '',
          }).then(() => {
            message.success('已移动到目录根');
            refresh();
          });
          return;
        }

        postApiV1NodeMoveNav({
          ids: [active.id as string],
          kb_id: kb_id!,
          nav_id: over.id as string,
        }).then(() => {
          message.success('已移动到该目录');
          refresh();
        });
        return;
      }
      treeDragHandlersRef.current?.onDragEnd?.(e);
    },
    [kb_id, navList, refresh, groups],
  );
  const handleLayoutDragCancel = useCallback(() => {
    treeDragHandlersRef.current?.onDragCancel?.();
  }, []);

  useEffect(() => {
    const revalidate = () => {
      if (document.visibilityState === 'visible') void reloadTree();
    };
    document.addEventListener('visibilitychange', revalidate);
    return () => document.removeEventListener('visibilitychange', revalidate);
  }, [reloadTree]);

  useEffect(() => {
    if (leftViewMode !== 'explorer' || !currentFolderId) return;
    const nav = currentFolderId.startsWith('nav:')
      ? currentFolderId.slice(4)
      : groups.find(g => g.list?.some(n => n.id === currentFolderId))?.nav_id;
    if (nav)
      void loadChildren(
        nav,
        currentFolderId.startsWith('nav:') ? '' : currentFolderId,
      );
  }, [currentFolderId, leftViewMode, loadChildren, groups]);

  useEffect(() => {
    setCurrentFolderId('');
  }, [nav_id]);

  useEffect(() => {
    if (leftViewMode === 'tree') {
      setPreviewCollapsed(true);
    }
  }, [leftViewMode]);

  useEffect(() => {
    if (isRefreshDocList) {
      refresh();
      dispatch(setIsRefreshDocList(false));
    }
  }, [isRefreshDocList, refresh, dispatch]);

  useEffect(() => {
    if (!kb_id || !shouldPollRagStatus) return;
    let stopped = false;
    let timer: number;
    const poll = async () => {
      try {
        const ids = groups
          .flatMap(g => g.list || [])
          .filter(
            n =>
              n.type === 2 &&
              n.can_manage !== false &&
              getRagStatusMeta(n)?.polling,
          )
          .map(n => n.id!);
        const statuses = new Map<
          string,
          Awaited<ReturnType<typeof getTreeNodeStatuses>>[number]
        >();
        for (let i = 0; i < ids.length; i += 1000) {
          const batch = await getTreeNodeStatuses(
            kb_id,
            ids.slice(i, i + 1000),
          );
          if (stopped) return;
          for (const status of batch)
            if (status.id) statuses.set(status.id, status);
        }
        if (!stopped)
          setGroups(prev =>
            prev.map(g => ({
              ...g,
              list: g.list?.map(n => {
                const status = statuses.get(n.id!);
                return status
                  ? { ...n, status: status.status, rag_info: status.rag_info }
                  : n;
              }),
            })),
          );
      } catch {
        // Request errors are reported by the shared response interceptor.
      } finally {
        if (!stopped) timer = window.setTimeout(poll, 5000);
      }
    };
    timer = window.setTimeout(poll, 5000);
    return () => {
      stopped = true;
      window.clearTimeout(timer);
    };
  }, [kb_id, shouldPollRagStatus, groups, setGroups]);

  useEffect(() => {
    document.body.classList.toggle('document-workbench-fullscreen', fullscreen);
    return () => {
      document.body.classList.remove('document-workbench-fullscreen');
    };
  }, [fullscreen]);

  return (
    <>
      {treeError && (
        <Button color='error' onClick={refresh}>
          目录加载失败，点击重试
        </Button>
      )}
      {!fullscreen && (
        <DocPageHeader
          onPublishClick={() => {
            if (readOnlyUser) return;
            setPublishIds([]);
            setPublishOpen(true);
          }}
          refreshTrigger={refreshTrigger}
          viewMode={leftViewMode}
          onViewModeChange={handleViewModeChange}
          onUploadTaskClick={() => setUploadTaskOpen(true)}
          onLearningProgressClick={() => setLearningProgressOpen(true)}
          onRestudyClick={() => setRestudyOpen(true)}
        />
      )}
      <ZipUploadTaskPanel
        open={uploadTaskOpen}
        onClose={() => setUploadTaskOpen(false)}
        kbId={kb_id}
        onTaskSettled={refresh}
      />
      <LearningProgressPanel
        open={learningProgressOpen}
        groups={groups}
        onClose={() => setLearningProgressOpen(false)}
        refresh={refresh}
      />
      <DndContext
        sensors={sensors}
        collisionDetection={pointerWithin}
        onDragStart={handleLayoutDragStart}
        onDragMove={handleLayoutDragMove}
        onDragOver={handleLayoutDragOver}
        onDragEnd={handleLayoutDragEnd}
        onDragCancel={handleLayoutDragCancel}
      >
        <Stack direction={'row'} sx={{ mt: fullscreen ? 0 : 2, width: '100%' }}>
          {!fullscreen && (
            <Box
              aria-hidden={leftViewMode !== 'tree'}
              sx={
                leftViewMode === 'tree'
                  ? leftPaneSx
                  : {
                      position: 'absolute',
                      left: -10000,
                      top: -10000,
                      width: 1,
                      height: 1,
                      overflow: 'hidden',
                      pointerEvents: 'none',
                      opacity: 0,
                    }
              }
            >
              <DocPageNavs
                navList={navList}
                groups={groups}
                onNavListChange={setNavList}
                onNavDeleted={navId => {
                  setGroups(prev => prev.filter(g => g.nav_id !== navId));
                }}
                refresh={refresh}
                isSearching={!!search}
                loading={loading && groups.length === 0}
                branches={branches}
                onLoadChildren={loadChildren}
                width={leftWidth}
                onWidthChange={setNavWidth}
                onDocumentSelect={item => {
                  setSelectedDocumentId(item.id);
                  setPreviewCollapsed(false);
                }}
                onFolderSelect={item => {
                  setCurrentFolderId(item.id || '');
                  setPreviewCollapsed(true);
                }}
                fill={previewCollapsed}
                wikiUrl={wikiUrl}
                onPublishOpen={handlePublishOpen}
                readOnlyUser={readOnlyUser}
                registerTreeDragHandlers={handlers => {
                  treeDragHandlersRef.current = handlers;
                }}
                // 菜单本身是函数，必须包装一层，避免被 React 当作 state updater 执行。
                onDocumentMenuReady={handleDocumentMenuReady}
              />
            </Box>
          )}
          {!fullscreen && leftViewMode === 'explorer' && (
            <Box
              sx={{
                ...leftPaneSx,
                mr: 2,
              }}
            >
              <DocPageFileManager
                loading={loading || explorerBranchState === 'loading'}
                loadError={explorerBranchState === 'error'}
                onRetry={() => {
                  if (explorerNavId)
                    void loadChildren(explorerNavId, explorerParentId);
                }}
                groups={groups}
                navId={nav_id}
                folderId={currentFolderId}
                onFolderChange={folderId => {
                  setCurrentFolderId(folderId);
                  setPreviewCollapsed(true);
                }}
                onDocumentSelect={item => {
                  setSelectedDocumentId(item.id || null);
                  setPreviewCollapsed(false);
                }}
                refresh={refresh}
                onNavDeleted={navId => {
                  const deletedGroup = groups.find(g => g.nav_id === navId);
                  setGroups(prev => prev.filter(g => g.nav_id !== navId));
                  setNavList(prev => prev.filter(nav => nav.id !== navId));
                  setCurrentFolderId(prev => {
                    if (prev === `nav:${navId}`) return '';
                    if (deletedGroup?.list?.some(item => item.id === prev)) {
                      return '';
                    }
                    return prev;
                  });
                  if (
                    deletedGroup?.list?.some(
                      item => item.id === selectedDocumentId,
                    )
                  ) {
                    setSelectedDocumentId(null);
                  }
                  setPreviewCollapsed(true);
                }}
                readOnlyUser={readOnlyUser}
                menu={explorerMenu || undefined}
                fill
              />
              {!previewCollapsed && (
                <Box
                  onPointerDown={handleExplorerResizeStart}
                  sx={{
                    position: 'absolute',
                    top: 0,
                    right: -6,
                    bottom: 0,
                    width: 8,
                    cursor: 'col-resize',
                    zIndex: 3,
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
                    '&:hover:after': { bgcolor: 'primary.main' },
                  }}
                />
              )}
            </Box>
          )}
          <DocPagePreview
            documentId={selectedDocumentId}
            collapsed={fullscreen ? false : previewCollapsed}
            onToggle={() => setPreviewCollapsed(value => !value)}
            fullscreen={fullscreen}
            onToggleFullscreen={() => {
              setFullscreen(value => !value);
              setPreviewCollapsed(false);
            }}
          />
        </Stack>
      </DndContext>
      {!readOnlyUser && (
        <>
          <VersionPublish
            open={publishOpen}
            defaultSelected={publishIds}
            onClose={() => {
              setPublishOpen(false);
              setPublishIds([]);
            }}
            refresh={refresh}
          />
          <RagErrorReStart
            open={restudyOpen}
            onClose={() => setRestudyOpen(false)}
            refresh={refresh}
          />
        </>
      )}
    </>
  );
};

export default Content;
