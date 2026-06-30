import { ITreeItem } from '@/api';
import { getApiV1AppDetailCompat } from '@/request/appCompat';
import { getApiV1KnowledgeBaseList } from '@/request/KnowledgeBase';
import { getApiV1NodeListGroupNav, putApiV1NodeDetail } from '@/request/Node';
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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Outlet } from 'react-router-dom';
import Catalog from './Catalog';
import { CollabStatus } from '../component/EditorCollaboration';

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
  collabStatus: CollabStatus;
  setCollabStatus: (status: CollabStatus) => void;
  collabDisabledReason: string;
  setCollabDisabledReason: (reason: string) => void;
}

const DocEditor = () => {
  const catalogWidth = 292;
  const isWideScreen = useMediaQuery('(min-width:1400px)');
  const dispatch = useAppDispatch();
  const { kb_id = '' } = useAppSelector(state => state.config);
  const [nodeDetail, setNodeDetail] = useState<V1NodeDetailResp>({});
  const [catalogOpen, setCatalogOpen] = useState(true);
  const [groups, setGroups] = useState<
    GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[]
  >([]);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const nav_id = useAppSelector(state => state.config.nav_id) || '';

  const [docWidth, setDocWidth] = useState<string>('full');
  const saveCurrentDocRef = useRef<(() => Promise<void>) | null>(null);
  const [collabStatus, setCollabStatus] = useState<CollabStatus>('idle');
  const [collabDisabledReason, setCollabDisabledReason] = useState('');

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

  const refreshCatalog = useCallback(async (): Promise<
    GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[]
  > => {
    const params = {
      kb_id: kb_id || localStorage.getItem('kb_id') || '',
    };
    setCatalogLoading(true);
    try {
      const res = await getApiV1NodeListGroupNav(params);
      const list = res || [];
      setGroups(list);
      if (list.length > 0) {
        const storedNavId = localStorage.getItem(`nav_id_${params.kb_id}`);
        const validInList =
          storedNavId && list.some(g => g.nav_id === storedNavId);
        const idToUse = validInList ? storedNavId! : list[0].nav_id || '';
        dispatch(setNavId(idToUse));
      }
      return list;
    } finally {
      setCatalogLoading(false);
    }
  }, [dispatch, kb_id]);

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
    if (kb_id) {
      refreshCatalog();
    }
  }, [kb_id, refreshCatalog]);

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
            collabStatus,
            setCollabStatus,
            collabDisabledReason,
            setCollabDisabledReason,
          }}
        />
      </Box>
    </Stack>
  );
};

export default DocEditor;
