import {
  ConstsNodeRagInfoStatus,
  DomainNodeListItemResp,
  DomainNodeStatus,
} from '@/request/types';
import type { ChipProps } from '@mui/material';
import type { Theme } from '@mui/material';

export type RagStatusSource = Pick<
  DomainNodeListItemResp,
  'rag_info' | 'status' | 'type'
> & {
  rag_status?: ConstsNodeRagInfoStatus;
  rag_message?: string;
};

export type RagStatusMeta = {
  label: string;
  color: ChipProps['color'];
  tagColor: keyof Theme['palette'];
  message?: string;
  polling: boolean;
};

export function getRagStatusMeta(
  item?: RagStatusSource | null,
): RagStatusMeta | null {
  if (
    !item ||
    item.type !== 2 ||
    item.status !== DomainNodeStatus.NodeStatusPublished
  ) {
    return null;
  }

  const status = item.rag_info?.status || item.rag_status;
  const message = item.rag_info?.message || item.rag_message || '';

  switch (status) {
    case ConstsNodeRagInfoStatus.NodeRagStatusSucceeded:
      return {
        label: '学习成功',
        color: 'success',
        tagColor: 'success',
        message,
        polling: false,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusFailed:
      return {
        label: '学习失败',
        color: 'error',
        tagColor: 'error',
        message,
        polling: false,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusRunning:
      return {
        label: '学习中',
        color: 'info',
        tagColor: 'info',
        message,
        polling: true,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusReindexing:
      return {
        label: '重建中',
        color: 'info',
        tagColor: 'info',
        message,
        polling: true,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusPending:
      return {
        label: '等待学习',
        color: 'warning',
        tagColor: 'warning',
        message,
        polling: true,
      };
    default:
      return {
        label: '待学习',
        color: 'warning',
        tagColor: 'warning',
        message,
        polling: true,
      };
  }
}

export function hasPollingRagTask(
  groups: { list?: DomainNodeListItemResp[] }[] | undefined,
): boolean {
  return (groups || []).some(group =>
    (group.list || []).some(item => getRagStatusMeta(item)?.polling),
  );
}
