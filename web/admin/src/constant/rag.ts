import { ConstsNodeRagInfoStatus } from '@/request';

const RAG_SOURCES = {
  [ConstsNodeRagInfoStatus.NodeRagStatusReindexing]: {
    name: '重新索引中',
    color: 'warning',
  },
  [ConstsNodeRagInfoStatus.NodeRagStatusPending]: {
    name: '待建立索引',
    color: 'warning',
  },
  [ConstsNodeRagInfoStatus.NodeRagStatusRunning]: {
    name: '正在建立索引',
    color: 'warning',
  },
  [ConstsNodeRagInfoStatus.NodeRagStatusFailed]: {
    name: '索引失败',
    color: 'error',
  },
  [ConstsNodeRagInfoStatus.NodeRagStatusSucceeded]: {
    name: '索引完成',
    color: 'success',
  },
};

export default RAG_SOURCES;
