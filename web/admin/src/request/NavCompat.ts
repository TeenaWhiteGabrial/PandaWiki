import httpClient, { ContentType } from './httpClient';
import type { DomainNodePermissions } from './types';
export type NavEnterprisePermissions = DomainNodePermissions;

export interface NavPermissionResp {
  id: string;
  permissions: NavEnterprisePermissions;
}

export const getApiV1NavPermission = (query: { kb_id: string; id: string }) =>
  httpClient<NavPermissionResp>({
    path: '/api/v1/nav/permission',
    method: 'GET',
    query,
    format: 'json',
  });

export const patchApiV1NavPermission = (body: {
  kb_id: string;
  id: string;
  permissions: NavEnterprisePermissions;
}) =>
  httpClient<{ success: boolean }>({
    path: '/api/v1/nav/permission',
    method: 'PATCH',
    body,
    type: ContentType.Json,
    format: 'json',
  });
