import { KnowledgeBaseListItem } from '@/api';
import {
  DomainAppDetailResp,
  DomainKnowledgeBaseDetail,
  GithubComChaitinPandaWikiDomainModelListItem,
} from '@/request/types';
import { UserInfoCompat } from '@/request/userCompat';
import { createSlice } from '@reduxjs/toolkit';

export interface config {
  user: UserInfoCompat;
  kb_id: string;
  nav_id: string;
  kbList: KnowledgeBaseListItem[] | null;
  modelList: GithubComChaitinPandaWikiDomainModelListItem[] | null;
  modelStatus: boolean;
  kbDetail: DomainKnowledgeBaseDetail;
  appPreviewData: DomainAppDetailResp | null;
  refreshAdminRequest: () => void;
  isRefreshDocList: boolean;
}

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
  setKbDetail,
  setRefreshAdminRequest,
  setModelList,
  setIsRefreshDocList,
} = configSlice.actions;
export default configSlice.reducer;
