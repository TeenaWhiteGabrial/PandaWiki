import { useURLSearchParams } from '@/hooks';
import VersionPublish from '@/pages/release/components/VersionPublish';
import { postApiV1NavMove } from '@/request/Nav';
import { getApiV1NodeListGroupNav, postApiV1NodeMoveNav } from '@/request/Node';
import {
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
  V1NavListResp,
} from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import { setIsRefreshDocList, setNavId } from '@/store/slices/config';
import type { TreeDragHandlers } from '@/utils/drag';
import { isOrdinaryReadOnlyUser } from '@/utils/permission';
import { message } from '@/components/ui';
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
import { Box, Stack } from '@mui/material';
import { useCallback, useEffect, useRef, useState } from 'react';
import DocPageHeader from './DocPageHeader';
import DocPageNavs from './DocPageNavs';
import DocPagePreview from './DocPagePreview';
import DocPageFileManager from './DocPageFileManager';
import type { DocTreeMenuFn } from './DocPageList/types';
import ZipUploadTaskPanel from '../component/ZipUploadTaskPanel';

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
  const [uploadTaskOpen, setUploadTaskOpen] = useState(false);
  const [publishIds, setPublishIds] = useState<string[]>([]);
  const [groups, setGroups] = useState<
    GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[]
  >([]);
  const [navList, setNavList] = useState<V1NavListResp[]>([]);
  const [loading, setLoading] = useState(false);
  const [hasLoadedOnce, setHasLoadedOnce] = useState(false);
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

  const getData = useCallback(() => {
    if (!kb_id) {
      setLoading(false);
      return;
    }
    const params: { kb_id: string; search?: string } = { kb_id };
    if (search) params.search = search;
    setLoading(true);
    getApiV1NodeListGroupNav(params)
      .then(res => {
        const list = res || [];
        setGroups(list);
        const nextNavList = list
          .map(g => ({
            id: g.nav_id,
            name: g.nav_name,
            position: g.position ?? 0,
          }))
          .filter(n => n.id)
          .sort((a, b) => (a.position ?? 0) - (b.position ?? 0));
        setNavList(nextNavList);
        if (nextNavList.length > 0) {
          const storedNavId = kb_id
            ? localStorage.getItem(`nav_id_${kb_id}`)
            : null;
          const validInList =
            storedNavId && nextNavList.some(n => n.id === storedNavId);
          const idToUse = validInList ? storedNavId! : nextNavList[0].id!;
          dispatch(setNavId(idToUse));
        } else {
          dispatch(setNavId(''));
        }
        setHasLoadedOnce(true);
      })
      .finally(() => setLoading(false));
  }, [search, kb_id, dispatch]);

  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const refresh = useCallback(() => {
    getData();
    setRefreshTrigger(t => t + 1);
  }, [getData]);

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

  const [wikiUrl, setWikiUrl] = useState<string>('');
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
      if (overIsNav && !activeIsNav) {
        // 如果文档/文件夹本来就在该目录中，则不调用移动接口
        const targetNavId = over.id as string;
        const isAlreadyInTargetNav = groups.some(
          g =>
            g.nav_id === targetNavId &&
            (g.list || []).some(item => item.id === active.id),
        );
        if (isAlreadyInTargetNav) {
          treeDragHandlersRef.current?.onDragCancel?.();
          return;
        }

        // 文档树节点拖到目录
        treeDragHandlersRef.current?.onDragCancel?.();
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
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible' && kb_id) {
        getData();
      }
    };
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [getData, kb_id]);

  useEffect(() => {
    if (kbDetail?.access_settings?.base_url) {
      setWikiUrl(kbDetail.access_settings.base_url);
      return;
    }
    const host = kbDetail?.access_settings?.hosts?.[0] || '';
    if (host === '') return;
    const { ssl_ports = [], ports = [] } = kbDetail?.access_settings || {};

    if (ssl_ports) {
      if (ssl_ports.includes(443)) setWikiUrl(`https://${host}`);
      else if (ssl_ports.length > 0)
        setWikiUrl(`https://${host}:${ssl_ports[0]}`);
    } else if (ports) {
      if (ports.includes(80)) setWikiUrl(`http://${host}`);
      else if (ports.length > 0) setWikiUrl(`http://${host}:${ports[0]}`);
    }
  }, [kbDetail]);

  useEffect(() => {
    if (kb_id) getData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, kb_id]);

  useEffect(() => {
    setCurrentFolderId('');
  }, [nav_id]);

  useEffect(() => {
    if (isRefreshDocList) {
      refresh();
      dispatch(setIsRefreshDocList(false));
    }
  }, [isRefreshDocList, refresh, dispatch]);

  useEffect(() => {
    document.body.classList.toggle('document-workbench-fullscreen', fullscreen);
    return () => {
      document.body.classList.remove('document-workbench-fullscreen');
    };
  }, [fullscreen]);

  return (
    <>
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
        />
      )}
      <ZipUploadTaskPanel
        open={uploadTaskOpen}
        onClose={() => setUploadTaskOpen(false)}
        kbId={kb_id}
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
                  ? undefined
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
                loading={loading && !hasLoadedOnce}
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
                // 菜单本身是函数，必须包装一层，避免被 React 当作 state updater 执行。
                onDocumentMenuReady={handleDocumentMenuReady}
              />
            </Box>
          )}
          {!fullscreen && leftViewMode === 'explorer' && (
            <Box
              sx={{
                position: 'relative',
                flex: previewCollapsed ? '1 1 auto' : `0 0 ${leftWidth}px`,
                width: previewCollapsed ? '100%' : leftWidth,
                height: 'calc(100vh - 183px)',
                mr: 2,
                minWidth: 0,
              }}
            >
              <DocPageFileManager
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
        <VersionPublish
          open={publishOpen}
          defaultSelected={publishIds}
          onClose={() => {
            setPublishOpen(false);
            setPublishIds([]);
          }}
          refresh={refresh}
        />
      )}
    </>
  );
};

export default Content;
