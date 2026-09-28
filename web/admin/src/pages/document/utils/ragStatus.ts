import {
  ConstsNodeLearningTaskAction,
  ConstsNodeLearningTaskStatus,
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
  available?: boolean;
};

export function hasAvailableRagIndex(item?: RagStatusSource | null): boolean {
  return (
    !!item &&
    item.type === 2 &&
    item.status === DomainNodeStatus.NodeStatusPublished &&
    (item.rag_info?.status || item.rag_status) ===
      ConstsNodeRagInfoStatus.NodeRagStatusSucceeded
  );
}

export function isLearningTaskActive(
  status?: ConstsNodeLearningTaskStatus,
): boolean {
  return (
    status === ConstsNodeLearningTaskStatus.NodeLearningTaskWaiting ||
    status === ConstsNodeLearningTaskStatus.NodeLearningTaskRunning ||
    status === ConstsNodeLearningTaskStatus.NodeLearningTaskCancelRequested
  );
}

const getTaskRunningLabel = (action?: ConstsNodeLearningTaskAction) => {
  if (action === ConstsNodeLearningTaskAction.NodeLearningTaskActionRebuild) {
    return '重建中';
  }
  return '学习中';
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
  const taskStatus = item.rag_info?.task_status;
  const taskAction = item.rag_info?.task_action;
  const taskMessage = item.rag_info?.task_message || message;
  const available = hasAvailableRagIndex(item);

  switch (taskStatus) {
    case ConstsNodeLearningTaskStatus.NodeLearningTaskWaiting:
    case ConstsNodeLearningTaskStatus.NodeLearningTaskRunning:
      return {
        label: getTaskRunningLabel(taskAction),
        color: 'info',
        tagColor: 'info',
        message: taskMessage,
        polling: true,
        available,
      };
    case ConstsNodeLearningTaskStatus.NodeLearningTaskCancelRequested:
      return {
        label: '停止中',
        color: 'warning',
        tagColor: 'warning',
        message: taskMessage,
        polling: true,
        available,
      };
    case ConstsNodeLearningTaskStatus.NodeLearningTaskCanceled:
      if (!available) {
        return {
          label: '已停止',
          color: 'warning',
          tagColor: 'warning',
          message: taskMessage,
          polling: false,
          available,
        };
      }
      break;
    case ConstsNodeLearningTaskStatus.NodeLearningTaskFailed:
      return {
        label: available ? '本次失败' : '学习失败',
        color: available ? 'warning' : 'error',
        tagColor: available ? 'warning' : 'error',
        message: taskMessage,
        polling: false,
        available,
      };
  }

  switch (status) {
    case ConstsNodeRagInfoStatus.NodeRagStatusSucceeded:
      return {
        label: '学习成功',
        color: 'success',
        tagColor: 'success',
        message,
        polling: false,
        available: true,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusFailed:
      return {
        label: '学习失败',
        color: 'error',
        tagColor: 'error',
        message,
        polling: false,
        available: false,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusRunning:
      return {
        label: '学习中',
        color: 'info',
        tagColor: 'info',
        message,
        polling: true,
        available: false,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusReindexing:
      return {
        label: '重建中',
        color: 'info',
        tagColor: 'info',
        message,
        polling: true,
        available: false,
      };
    case ConstsNodeRagInfoStatus.NodeRagStatusPending:
      return {
        label: '等待学习',
        color: 'warning',
        tagColor: 'warning',
        message,
        polling: true,
        available: false,
      };
    default:
      return {
        label: '待学习',
        color: 'warning',
        tagColor: 'warning',
        message,
        polling: false,
        available: false,
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
