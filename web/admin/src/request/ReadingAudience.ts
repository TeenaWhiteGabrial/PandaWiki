import httpRequest, { ContentType } from './httpClient';
import type { DomainNodePermissions } from './types';

export interface ReadingAudienceUser {
  id: string;
  account: string;
  name: string;
  department_ids?: string[];
  permission_level_id?: string;
}

export interface ReadingAudienceResult {
  permissions: DomainNodePermissions;
  summary?: ReadingPermissionSummary;
  unrestricted: boolean;
  total: number;
  filtered_total: number;
  users: ReadingAudienceUser[];
  source?: {
    id: string;
    name: string;
    nav_id?: string;
    target_type: 'node' | 'nav';
  };
}

export interface ReadingPermissionSummary {
  group_allowed: boolean;
  department_groups: string[][];
  level_restricted: boolean;
  permission_level_ids: string[];
  min_permission_level_id?: string;
  min_permission_level_rank?: number;
  allowed_user_ids: string[];
  denied_user_ids: string[];
  denied_department_ids: string[];
  user_names?: Record<string, string>;
}

export const previewReadingAudience = (
  body: {
    kb_id: string;
    ids: string[];
    target_type: 'node' | 'nav';
    permissions?: DomainNodePermissions;
    include_users?: boolean;
    keyword?: string;
    page?: number;
    page_size?: number;
  },
  signal?: AbortSignal,
) =>
  httpRequest<ReadingAudienceResult>({
    path: '/api/v1/node/permission/audience',
    method: 'POST',
    body,
    type: ContentType.Json,
    format: 'json',
    signal,
  });
