import { KnowledgeBaseListItem } from '@/api';
import {
  DomainAppDetailResp,
  DomainKnowledgeBaseDetail,
  GithubComChaitinPandaWikiDomainModelListItem,
} from '@/request/types';
import { UserInfoCompat } from '@/request/userCompat';
import { createSlice } from '@reduxjs/toolkit';
import { loadAppBrand, type AppBrand } from '@/utils/appBrand';

export interface config {
  user: UserInfoCompat;
  kb_id: string;
  nav_id: string;
  kbList: KnowledgeBaseListItem[] | null;
  modelList: GithubComChaitinPandaWikiDomainModelListItem[] | null;
  modelStatus: boolean;
  kbDetail: DomainKnowledgeBaseDetail;
  appPreviewData: DomainAppDetailResp | null;
  appBrand: AppBrand;
  refreshAdminRequest: () => void;
  isRefreshDocList: boolean;
}

const initialBrand = loadAppBrand() || {
  title: '知识库管理台',
  logo: `${window.__BASENAME__ || ''}/logo.png`,
  favicon: `${window.__BASENAME__ || ''}/logo.png`,
};

const initialState: config = {
  user: {
    id: '',
    account: '',
    created_at: '',
  },
  kb_id: '',
  nav_id: '',
  kbList: null,
  modelList: null,
  modelStatus: false,
  kbDetail: {} as DomainKnowledgeBaseDetail,
  appPreviewData: null,
  appBrand: initialBrand,
  refreshAdminRequest: () => {},
  isRefreshDocList: false,
};

const configSlice = createSlice({
  name: 'config',
  initialState,
  reducers: {
    setUser(state, { payload }) {
      state.user = payload;
    },
    setKbId(state, { payload }) {
      localStorage.setItem('kb_id', payload);
      state.kb_id = payload;
    },
    setNavId(state, { payload }) {
      state.nav_id = payload;
      if (state.kb_id) {
        if (payload) {
          localStorage.setItem(`nav_id_${state.kb_id}`, payload);
        } else {
          localStorage.removeItem(`nav_id_${state.kb_id}`);
        }
      }
    },
    setKbList(state, { payload }) {
      state.kbList = payload;
    },
    setModelList(state, { payload }) {
      state.modelList = payload;
    },
    setModelStatus(state, { payload }) {
      state.modelStatus = payload;
    },
    setAppPreviewData(state, { payload }) {
      state.appPreviewData = payload;
    },
    setAppBrand(state, { payload }) {
      state.appBrand = payload;
    },
    setKbDetail(state, { payload }) {
      state.kbDetail = payload;
    },
    setRefreshAdminRequest(state, { payload }) {
      state.refreshAdminRequest = payload;
    },
    setIsRefreshDocList(state, { payload }) {
      state.isRefreshDocList = payload;
    },
  },
});

export const {
  setUser,
  setKbId,
  setNavId,
  setKbList,
  setModelStatus,
  setAppPreviewData,
  setAppBrand,
  setKbDetail,
  setRefreshAdminRequest,
  setModelList,
  setIsRefreshDocList,
} = configSlice.actions;
export default configSlice.reducer;
