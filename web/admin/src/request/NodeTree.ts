import type { GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp } from './types';
import httpRequest, { ContentType } from './httpClient';
import type { DomainResponse, DomainNodeListItemResp } from './types';
export const getTreeNodeStatuses = (kb_id: string, ids: string[]) =>
  httpRequest<
    DomainResponse & {
      data?: Pick<DomainNodeListItemResp, 'id' | 'status' | 'rag_info'>[];
    }
  >({
    path: '/api/v1/node/tree/status',
    method: 'POST',
    body: { kb_id, ids },
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });

export const getDocumentTree = (body: {
  kb_id: string;
  nav_id?: string;
  parent_id?: string;
  path_id?: string;
  search?: string;
  parents?: string[];
}) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
    }
  >({
    path: '/api/v1/node/tree',
    method: 'POST',
    body,
    secure: true,
    type: ContentType.Json,
    format: 'json',
    isAlert: false,
  });
