import httpRequest, { ContentType } from './httpClient';
import { DomainResponse, DomainDocumentManagementGrant } from './types';

export interface NodeManagementResp {
  id: string;
  grants?: DomainDocumentManagementGrant[];
  can_configure: boolean;
}

export const getNodeManagement = (query: { kb_id: string; id: string }) =>
  httpRequest<DomainResponse & { data?: NodeManagementResp }>({
    path: '/api/v1/node/management',
    method: 'GET',
    query,
    secure: true,
    format: 'json',
  });

export const getNavManagement = (query: { kb_id: string; id: string }) =>
  httpRequest<DomainResponse & { data?: NodeManagementResp }>({
    path: '/api/v1/nav/management',
    method: 'GET',
    query,
    secure: true,
    format: 'json',
  });

export const editNavManagement = (body: {
  kb_id: string;
  id: string;
  grants: DomainDocumentManagementGrant[];
}) =>
  httpRequest<DomainResponse>({
    path: '/api/v1/nav/management',
    method: 'PATCH',
    body,
    type: ContentType.Json,
    secure: true,
    format: 'json',
  });

export const editNodeManagement = (body: {
  kb_id: string;
  ids: string[];
  grants: DomainDocumentManagementGrant[];
}) =>
  httpRequest<DomainResponse>({
    path: '/api/v1/node/management',
    method: 'PATCH',
    body,
    type: ContentType.Json,
    secure: true,
    format: 'json',
  });

export const getNodePermissionOptions = (kb_id: string) =>
  httpRequest<
    DomainResponse & {
      data?: {
        users: {
          id: string;
          account: string;
          name: string;
          role?: import('./types').ConstsUserRole;
        }[];
        departments: import('./Enterprise').Department[];
        levels: import('./Enterprise').PermissionLevel[];
      };
    }
  >({
    path: '/api/v1/node/permission/options',
    method: 'GET',
    query: { kb_id },
    secure: true,
    format: 'json',
  });
