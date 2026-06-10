import httpClient, { ContentType } from './httpClient';
export interface NavEnterprisePermissions {
  scope_type?: string;
  min_permission_level_id?: string;
  allowed_department_ids?: string[];
  allowed_user_ids?: string[];
}

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
