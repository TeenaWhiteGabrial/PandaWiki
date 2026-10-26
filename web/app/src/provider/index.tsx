'use client';

import { ITreeItem, KBDetail, NodeListItem, WidgetInfo } from '@/assets/type';
import { getApiV1User } from '@/request/User';
import { V1UserInfoResp } from '@/request/types';
import { useMediaQuery } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  Dispatch,
  SetStateAction,
} from 'react';
import { useParams, useRouter } from 'next/navigation';
import {
  filterEmptyFolders,
  convertToTree,
  findNavIdByNodeId,
  addExpandState,
  type NavItem,
} from '@/utils/tree';
import { usePathname } from 'next/navigation';
import {
  AUTH_CLEARED_EVENT,
  clearAuthStorage,
  clearLegacyCredentials,
} from '@/utils/auth';

interface StoreContextType {
  authInfo?: V1UserInfoResp;
  authRevision?: number;
  setAuthInfo?: Dispatch<SetStateAction<V1UserInfoResp | undefined>>;
  widget?: WidgetInfo;
  kbDetail?: KBDetail;
  catalogShow?: boolean;
  tree?: ITreeItem[];
  themeMode?: 'light' | 'dark';
  mobile?: boolean;
  nodeList?: NodeListItem[];
  setNodeList?: (list: NodeListItem[]) => void;
  setTree?: Dispatch<SetStateAction<ITreeItem[] | undefined>>;
  setCatalogShow?: (value: boolean) => void;
  catalogWidth?: number;
  setCatalogWidth?: (value: number) => void;
  qaModalOpen?: boolean;
  setQaModalOpen?: (value: boolean) => void;
  /** 栏目列表，多栏目时展示导航栏 */
  navList?: NavItem[];
  /** 当前选中的栏目 id */
  selectedNavId?: string;
  setSelectedNavId?: Dispatch<SetStateAction<string | undefined>>;
  /** 各栏目对应的文档列表 nav_id -> NodeListItem[] */
  navDataMap?: Record<string, NodeListItem[]>;
}

export const StoreContext = createContext<StoreContextType | undefined>(
  undefined,
);

const EMPTY_NODE_LIST: NodeListItem[] = [];
const EMPTY_TREE: ITreeItem[] = [];
const EMPTY_NAV_LIST: NavItem[] = [];
const EMPTY_NAV_DATA: Record<string, NodeListItem[]> = {};

export const useStore = () => {
  const context = useContext(StoreContext);
  if (!context) {
    throw new Error('useStore must be used within a StoreProvider');
  }
  return context;
};

export default function StoreProvider({
  children,
  ...props
}: StoreContextType & { children: React.ReactNode }) {
  const context = useContext(StoreContext) || {};
  const {
    widget = context.widget,
    kbDetail = context.kbDetail,
    themeMode = context.themeMode,
    nodeList: initialNodeList = context.nodeList || EMPTY_NODE_LIST,
    mobile = context.mobile,
    authInfo = context.authInfo,
    tree: initialTree = context.tree || EMPTY_TREE,
    navList: initialNavList = context.navList || EMPTY_NAV_LIST,
    selectedNavId: initialSelectedNavId = context.selectedNavId,
    navDataMap: initialNavDataMap = context.navDataMap || EMPTY_NAV_DATA,
  } = props;

  const NAV_ID_STORAGE_KEY = 'kb-selected-nav-id';
  const pathname = usePathname();
  const router = useRouter();

  // 使用 props 传入的 defaultNavId，避免 SSR 与 CSR 不一致导致 Hydration 错误
  const initialNavId = initialSelectedNavId;

  const catalogSettings = kbDetail?.settings?.catalog_settings;

  const [catalogWidth, setCatalogWidth] = useState<number>(() => {
    return catalogSettings?.catalog_width || 260;
  });
  const [nodeList, setNodeList] = useState<NodeListItem[] | undefined>(
    initialNodeList,
  );
  const [tree, setTree] = useState<ITreeItem[] | undefined>(() => {
    if (
      initialNavId !== undefined &&
      initialNavId !== '' &&
      initialNavDataMap[initialNavId]
    ) {
      return filterEmptyFolders(convertToTree(initialNavDataMap[initialNavId]));
    }
    return initialTree;
  });
  const [qaModalOpen, setQaModalOpen] = useState(false);
  const [authInfoState, setAuthInfo] = useState<V1UserInfoResp | undefined>(
    authInfo,
  );
  const [authRevision, setAuthRevision] = useState(0);
  const [navList, setNavList] = useState<NavItem[]>(initialNavList);
  const [navDataMap, setNavDataMap] =
    useState<Record<string, NodeListItem[]>>(initialNavDataMap);
  const [selectedNavId, setSelectedNavIdState] = useState<string | undefined>(
    initialNavId,
  );

  const setSelectedNavId: Dispatch<
    SetStateAction<string | undefined>
  > = value => {
    setSelectedNavIdState(prev => {
      const next = typeof value === 'function' ? value(prev) : value;
      if (typeof window !== 'undefined' && next) {
        localStorage.setItem(NAV_ID_STORAGE_KEY, next);
      }
      return next;
    });
  };

  const [catalogShow, setCatalogShow] = useState(
    catalogSettings?.catalog_visible !== 2,
  );
  const [isMobile, setIsMobile] = useState(mobile);
  const theme = useTheme();
  const mediaQueryResult = useMediaQuery(theme.breakpoints.down('lg'), {
    noSsr: true,
  });

  useEffect(() => {
    if (kbDetail) {
      setCatalogShow(catalogSettings?.catalog_visible !== 2);
    }
  }, [kbDetail]);

  useEffect(() => {
    const savedWidth = window.localStorage.getItem('CATALOG_WIDTH');
    if (Number(savedWidth) > 0) {
      setCatalogWidth(Number(savedWidth));
    }
  }, []);

  useEffect(() => {
    setIsMobile(mediaQueryResult);
  }, [mediaQueryResult]);

  useEffect(() => {
    setAuthInfo(authInfo);
  }, [authInfo]);

  // router.refresh() returns a newly permission-filtered server snapshot.
  // Do not retain the initial navigation lists after the identity changes.
  useEffect(() => {
    setNodeList(initialNodeList);
    setNavList(initialNavList);
    setNavDataMap(initialNavDataMap);
    setTree(initialTree);
    setSelectedNavIdState(previous =>
      previous && initialNavDataMap[previous] ? previous : initialSelectedNavId,
    );
  }, [
    initialNodeList,
    initialNavList,
    initialNavDataMap,
    initialTree,
    initialSelectedNavId,
  ]);

  useEffect(() => {
    const handleAuthCleared = () => {
      setAuthInfo(undefined);
      setAuthRevision(value => value + 1);
      setNodeList([]);
      setTree([]);
      setNavList([]);
      setNavDataMap({});
      setSelectedNavIdState(undefined);
      setQaModalOpen(false);
      // Only the outer store refreshes the route; nested stores all discard
      // their old data immediately while the anonymous snapshot is loaded.
      if (!context.setAuthInfo) router.refresh();
    };
    const handleStorage = (event: StorageEvent) => {
      if (
        event.key === 'pw-reader-auth-change' &&
        event.newValue !== event.oldValue
      ) {
        handleAuthCleared();
      }
    };

    window.addEventListener(AUTH_CLEARED_EVENT, handleAuthCleared);
    window.addEventListener('storage', handleStorage);
    return () => {
      window.removeEventListener(AUTH_CLEARED_EVENT, handleAuthCleared);
      window.removeEventListener('storage', handleStorage);
    };
  }, [context.setAuthInfo, router]);

  useEffect(() => {
    clearLegacyCredentials();
    if (authInfoState?.id) return;

    let disposed = false;
    getApiV1User({ isAlert: false })
      .then(user => {
        if (!disposed) {
          setAuthInfo(user);
        }
      })
      .catch(() => {
        if (!disposed) {
          setAuthInfo(undefined);
        }
      });

    return () => {
      disposed = true;
    };
  }, [authInfoState?.id, pathname]);

  // HttpOnly credentials remain unreadable; the server returns only their
  // non-secret absolute expiry so the username/content still clear on time.
  useEffect(() => {
    const expiresAt = Number(authInfoState?.session_expires_at) * 1000;
    if (!Number.isFinite(expiresAt) || expiresAt <= 0) return;
    let timer: number | undefined;
    const expire = () => {
      const remaining = expiresAt - Date.now();
      if (remaining > 0) {
        timer = window.setTimeout(expire, Math.min(remaining, 2_147_483_647));
      } else {
        clearAuthStorage();
      }
    };
    expire();
    document.addEventListener('visibilitychange', expire);
    return () => {
      if (timer !== undefined) window.clearTimeout(timer);
      document.removeEventListener('visibilitychange', expire);
    };
  }, [authInfoState?.session_expires_at]);

  useEffect(() => {
    const verifyAfterReturn = () => {
      if (document.visibilityState !== 'visible' || !authInfoState?.id) return;
      getApiV1User({ isAlert: false })
        .then(setAuthInfo)
        .catch(() => undefined);
    };
    document.addEventListener('visibilitychange', verifyAfterReturn);
    return () =>
      document.removeEventListener('visibilitychange', verifyAfterReturn);
  }, [authInfoState?.id]);

  const params = useParams();
  const docId = (params?.id as string) || undefined;
  const catalogFolderExpand = catalogSettings?.catalog_folder !== 2;

  useEffect(() => {
    if (
      navDataMap &&
      selectedNavId !== undefined &&
      selectedNavId !== '' &&
      navDataMap[selectedNavId]
    ) {
      const nodeList = navDataMap[selectedNavId];
      let newTree = filterEmptyFolders(convertToTree(nodeList));
      if (docId) {
        const { tree: expandedTree } = addExpandState(
          newTree,
          docId,
          catalogFolderExpand,
        );
        newTree = expandedTree;
      }
      setTree(newTree);
    }
  }, [selectedNavId, navDataMap, docId, catalogFolderExpand]);

  return (
    <StoreContext.Provider
      value={{
        widget,
        kbDetail,
        themeMode,
        nodeList,
        catalogShow,
        setCatalogShow,
        mobile: isMobile,
        authInfo: authInfoState,
        authRevision,
        setAuthInfo,
        setNodeList,
        catalogWidth,
        tree,
        setTree,
        setCatalogWidth: value => {
          setCatalogWidth(value);
          window.localStorage.setItem('CATALOG_WIDTH', value.toString());
        },
        qaModalOpen,
        setQaModalOpen,
        navList,
        selectedNavId,
        setSelectedNavId,
        navDataMap,
      }}
    >
      {children}
    </StoreContext.Provider>
  );
}
