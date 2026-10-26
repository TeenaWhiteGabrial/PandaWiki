import httpClient, { ContentType } from './httpClient';

export type DepartmentStatus = 'active' | 'disabled';
export type PermissionLevelStatus = 'active' | 'disabled';

export interface Department {
  id: string;
  name: string;
  parent_id?: string | null;
  path: string;
  /** Ancestor IDs used to evaluate inherited audience rules in scoped selectors. */
  ancestor_ids?: string[];
  sort: number;
  status: DepartmentStatus;
  children?: Department[];
}

export interface PermissionLevel {
  id: string;
  name: string;
  code: string;
  rank: number;
  description: string;
  is_default: boolean;
  status: PermissionLevelStatus;
}

export interface DepartmentListResp {
  departments: Department[];
}

export interface PermissionLevelListResp {
  levels: PermissionLevel[];
}

export interface DepartmentUpsertReq {
  id?: string;
  name: string;
  parent_id?: string | null;
  path?: string;
  sort?: number;
  status?: DepartmentStatus;
}

export interface PermissionLevelUpsertReq {
  id?: string;
  name: string;
  code: string;
  rank: number;
  description?: string;
  is_default?: boolean;
  status?: PermissionLevelStatus;
}

export interface UpdateUserEnterpriseAccessReq {
  user_id: string;
  permission_level_id?: string;
  department_ids: string[];
}

export const getApiV1EnterpriseDepartments = () =>
  httpClient<DepartmentListResp>({
    path: '/api/v1/enterprise/departments',
    method: 'GET',
    format: 'json',
  });

export const postApiV1EnterpriseDepartments = (body: DepartmentUpsertReq) =>
  httpClient<{ id: string }>({
    path: '/api/v1/enterprise/departments',
    method: 'POST',
    body,
    type: ContentType.Json,
    format: 'json',
  });

export const putApiV1EnterpriseDepartments = (body: DepartmentUpsertReq) =>
  httpClient<{ id: string }>({
    path: '/api/v1/enterprise/departments',
    method: 'PUT',
    body,
    type: ContentType.Json,
    format: 'json',
  });

export const deleteApiV1EnterpriseDepartments = (id: string) =>
  httpClient<{ success: boolean }>({
    path: '/api/v1/enterprise/departments',
    method: 'DELETE',
    query: { id },
    format: 'json',
  });

export const getApiV1EnterprisePermissionLevels = () =>
  httpClient<PermissionLevelListResp>({
    path: '/api/v1/enterprise/permission_levels',
    method: 'GET',
    format: 'json',
  });

export const postApiV1EnterprisePermissionLevels = (
  body: PermissionLevelUpsertReq,
) =>
  httpClient<{ id: string }>({
    path: '/api/v1/enterprise/permission_levels',
    method: 'POST',
    body,
    type: ContentType.Json,
    format: 'json',
  });

export const putApiV1EnterprisePermissionLevels = (
  body: PermissionLevelUpsertReq,
) =>
  httpClient<{ id: string }>({
    path: '/api/v1/enterprise/permission_levels',
    method: 'PUT',
    body,
    type: ContentType.Json,
    format: 'json',
  });

export const deleteApiV1EnterprisePermissionLevels = (id: string) =>
  httpClient<{ success: boolean }>({
    path: '/api/v1/enterprise/permission_levels',
    method: 'DELETE',
    query: { id },
    format: 'json',
  });

export const putApiV1EnterpriseUsersAccess = (
  body: UpdateUserEnterpriseAccessReq,
) =>
  httpClient<{ success: boolean }>({
    path: '/api/v1/enterprise/users/access',
    method: 'PUT',
    body,
    type: ContentType.Json,
    format: 'json',
  });
