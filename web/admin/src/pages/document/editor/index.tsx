import { useLazyDocumentTree } from '@/hooks/useLazyDocumentTree';
import { ITreeItem } from '@/api';
import { getApiV1AppDetailCompat } from '@/request/appCompat';
import { getApiV1KnowledgeBaseList } from '@/request/KnowledgeBase';
import { putApiV1NodeDetail } from '@/request/Node';
import {
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
  V1NodeDetailResp,
} from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import {
  setKbDetail,
  setKbId,
  setKbList,
  setNavId,
} from '@/store/slices/config';
import { convertToTree } from '@/utils/drag';
import { message } from '@/components/ui';
import { Box, Drawer, Stack, useMediaQuery } from '@mui/material';
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Outlet, useOutletContext, useParams } from 'react-router-dom';
import Catalog from './Catalog';

export interface WrapContext {
  catalogOpen: boolean;
  setCatalogOpen: (open: boolean) => void;
  nodeDetail: V1NodeDetailResp | null;
  setNodeDetail: (detail: V1NodeDetailResp) => void;
  onSave: (
    content: string,
    editorState?: {
      editor_blocks?: Record<string, unknown>;
      editor_content?: Record<string, unknown>;
      editor_format?: string;
      editor_version?: number;
    },
  ) => void;
  docWidth: string;
  catalogData: ITreeItem[];
  groups: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  nav_id: string;
  refreshCatalog: () => Promise<
    GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[]
  >;
  saveCurrentDocRef: React.MutableRefObject<(() => Promise<void>) | null>;
}

// The editor is used both under the legacy editor route and inside the
// document workbench. Keep one context contract for both entry points.
export const EditorContext = createContext<WrapContext | null>(null);

export const useEditorContext = () => {
  const embeddedContext = useContext(EditorContext);
  const routeContext = useOutletContext<WrapContext | null>();
  return embeddedContext || routeContext;
};

const DocEditor = () => {
  const catalogWidth = 292;
  const isWideScreen = useMediaQuery('(min-width:1400px)');
  const dispatch = useAppDispatch();
  const { kb_id = '', user } = useAppSelector(state => state.config);
  const { id: pathID = '' } = useParams();
  const [nodeDetail, setNodeDetail] = useState<V1NodeDetailResp>({});
  const [catalogOpen, setCatalogOpen] = useState(true);
  const {
    groups,
    loading: catalogLoading,
    branches,
    loadChildren,
    refresh: refreshCatalog,
  } = useLazyDocumentTree(kb_id, user?.id || '', '', pathID);
  const nav_id = useAppSelector(state => state.config.nav_id) || '';

  const [docWidth, setDocWidth] = useState<string>('full');
  const saveCurrentDocRef = useRef<(() => Promise<void>) | null>(null);

  const catalogData = useMemo(() => {
    const curGroup = groups.find(g => g.nav_id === nav_id);
    const nodeList = curGroup?.list ?? [];
    return convertToTree(nodeList);
  }, [groups, nav_id]);

  const getInfo = async () => {
    const res = await getApiV1AppDetailCompat({ type: '1' });
    setDocWidth(res.settings?.theme_and_style?.doc_width || 'full');
  };

  const getKbList = useCallback(
    (id?: string) => {
      const kb_id = id || localStorage.getItem('kb_id') || '';
      getApiV1KnowledgeBaseList().then(res => {
        if (res.length > 0) {
          dispatch(setKbList(res));
          const kbDetail = res.find(item => item.id === kb_id);
          if (kbDetail) {
            dispatch(setKbId(kb_id));
            dispatch(setKbDetail(kbDetail));
          } else {
            dispatch(setKbId(res[0]?.id || ''));
          }
        }
      });
    },
    [dispatch],
  );

  const onSave = async (
    content: string,
    editorState?: {
      editor_blocks?: Record<string, unknown>;
      editor_content?: Record<string, unknown>;
      editor_format?: string;
      editor_version?: number;
    },
  ) => {
    if (!kb_id || !nodeDetail.id) return;
    try {
      await putApiV1NodeDetail({
        kb_id,
        id: nodeDetail.id,
        nav_id: nodeDetail.nav_id || '',
        content,
        editor_format: editorState?.editor_format || 'legacy-string',
        editor_version: editorState?.editor_version || 1,
        ...(editorState?.editor_content
          ? { editor_content: editorState.editor_content }
          : {}),
        ...(editorState?.editor_blocks
          ? { editor_blocks: editorState.editor_blocks }
          : {}),
        name: nodeDetail.name || '',
      });
      message.success('保存成功');
    } catch (error) {
      console.error(error);
    }
  };

  useEffect(() => {
    setCatalogOpen(isWideScreen);
  }, [isWideScreen]);

  useEffect(() => {
    if (!kb_id) {
      getKbList();
    } else {
      getInfo();
    }
  }, [getKbList, kb_id]);

  useEffect(() => {
    if (nav_id && groups.some(g => g.nav_id === nav_id))
      void loadChildren(nav_id);
    else if (groups[0]?.nav_id) dispatch(setNavId(groups[0].nav_id));
  }, [nav_id, groups, loadChildren, dispatch]);

  useEffect(() => {
    if (nodeDetail.nav_id && groups.some(g => g.nav_id === nodeDetail.nav_id)) {
      dispatch(setNavId(nodeDetail.nav_id));
    }
  }, [nodeDetail.nav_id, groups, dispatch]);

  return (
    <Stack
      direction='row'
      sx={{ color: 'text.primary', bgcolor: 'background.default' }}
    >
      <Drawer
        variant='persistent'
        anchor='left'
        open={catalogOpen}
        sx={{
          width: catalogOpen ? catalogWidth : 0,
          flexShrink: 0,
          transition: 'width 0.3s ease-in-out',
          '.MuiPaper-root': {
            width: catalogWidth,
            boxShadow: 'none !important',
            boxSizing: 'border-box',
          },
        }}
      >
        <Catalog
          curNode={nodeDetail}
          setCatalogOpen={setCatalogOpen}
          catalogData={catalogData}
          groups={groups}
          nav_id={nav_id}
          loading={catalogLoading}
          branches={branches}
          onLoadChildren={loadChildren}
          onRefresh={refreshCatalog}
          onSaveCurrentDoc={() =>
            saveCurrentDocRef.current?.() ?? Promise.resolve()
          }
        />
      </Drawer>
      <Box sx={{ flexGrow: 1 }}>
        <Outlet
          context={{
            catalogOpen,
            setCatalogOpen,
            nodeDetail,
            setNodeDetail,
            onSave,
            docWidth,
            catalogData,
            groups,
            nav_id,
            refreshCatalog,
            saveCurrentDocRef,
          }}
        />
      </Box>
    </Stack>
  );
};

export default DocEditor;
