import httpRequest, { ContentType } from './httpClient';
import type { DomainResponse } from './types';

export const cancelPendingLearning = (kbId: string) =>
  httpRequest<
    DomainResponse & {
      data?: { count: number; protected_count: number; legacy_cutoff: number };
    }
  >({
    path: '/api/v1/node/learning/cancel-pending',
    method: 'POST',
    body: { kb_id: kbId },
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });

export type LearningFilter =
  | 'attention'
  | 'running'
  | 'failed'
  | 'unavailable'
  | 'answerable'
  | 'all'
  | 'needs_learning';

export type LearningTaskItem = import('./types').DomainNodeListItemResp & {
  folder_path: string;
};

export interface LearningTaskCounts {
  total: number;
  attention: number;
  running: number;
  failed: number;
  unavailable: number;
  answerable: number;
  needs_learning: number;
}

export interface LearningTaskPage {
  list: LearningTaskItem[];
  total: number;
  running: number;
  page: number;
  page_size: number;
}

export interface LearningTaskQuery {
  kb_id: string;
  filter?: LearningFilter;
  keyword?: string;
  page?: number;
  page_size?: number;
  only_running?: boolean;
}

export const getLearningTasks = (
  query: LearningTaskQuery,
  signal?: AbortSignal,
) =>
  httpRequest<DomainResponse & { data?: LearningTaskPage }>({
    path: '/api/v1/node/learning/tasks',
    method: 'GET',
    query,
    signal,
    secure: true,
    format: 'json',
    isAlert: false,
  });

export const getLearningTaskSummary = (kb_id: string, signal?: AbortSignal) =>
  httpRequest<DomainResponse & { data?: LearningTaskCounts }>({
    path: '/api/v1/node/learning/summary',
    method: 'GET',
    query: { kb_id },
    signal,
    secure: true,
    format: 'json',
    isAlert: false,
  });

export const getLearningTaskTargets = (query: LearningTaskQuery) =>
  httpRequest<DomainResponse & { data?: { ids: string[] } }>({
    path: '/api/v1/node/learning/targets',
    method: 'GET',
    query,
    secure: true,
    format: 'json',
  });

export const getLearningTaskDetail = (
  kb_id: string,
  id: string,
  signal?: AbortSignal,
) =>
  httpRequest<DomainResponse & { data?: LearningTaskItem }>({
    path: '/api/v1/node/learning/detail',
    method: 'GET',
    query: { kb_id, id },
    signal,
    secure: true,
    format: 'json',
    isAlert: false,
  });
