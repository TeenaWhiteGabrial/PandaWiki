import request, { ContentType } from './httpClient';
import {
  DomainGetNodeReleaseDetailResp,
  DomainNodeReleaseListItem,
  GetApiProV1NodeReleaseDetailParams,
  GetApiProV1NodeReleaseListParams,
} from './pro/types';

export interface RestoreNodeReleaseReq {
  id: string;
  kb_id: string;
}

export const getNodeReleaseList = (query: GetApiProV1NodeReleaseListParams) =>
  request<DomainNodeReleaseListItem[]>({
    path: '/api/v1/node/release/list',
    method: 'GET',
    secure: true,
    query,
    type: ContentType.Json,
    format: 'json',
  });

export const getNodeReleaseDetail = (
  query: GetApiProV1NodeReleaseDetailParams,
) =>
  request<DomainGetNodeReleaseDetailResp>({
    path: '/api/v1/node/release/detail',
    method: 'GET',
    secure: true,
    query,
    type: ContentType.Json,
    format: 'json',
  });

export const restoreNodeRelease = (body: RestoreNodeReleaseReq) =>
  request({
    path: '/api/v1/node/release/restore',
    method: 'POST',
    secure: true,
    body,
    type: ContentType.Json,
    format: 'json',
  });
