import request, { ContentType } from './httpClient';
import {
  GetApiProV1ContributeDetailParams,
  GetApiProV1ContributeListParams,
  GithubComChaitinPandaWikiProApiContributeV1ContributeAuditReq,
  GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp,
  GithubComChaitinPandaWikiProApiContributeV1ContributeListResp,
} from './pro/types';

export const getContributeList = (query: GetApiProV1ContributeListParams) =>
  request<GithubComChaitinPandaWikiProApiContributeV1ContributeListResp>({
    path: '/api/pro/v1/contribute/list',
    method: 'GET',
    secure: true,
    query,
    type: ContentType.Json,
    format: 'json',
  });

export const getContributeDetail = (
  query: GetApiProV1ContributeDetailParams,
) =>
  request<GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp>({
    path: '/api/pro/v1/contribute/detail',
    method: 'GET',
    secure: true,
    query,
    type: ContentType.Json,
    format: 'json',
  });

export const auditContribute = (
  body: GithubComChaitinPandaWikiProApiContributeV1ContributeAuditReq,
) =>
  request({
    path: '/api/pro/v1/contribute/audit',
    method: 'POST',
    secure: true,
    body,
    type: ContentType.Json,
    format: 'json',
  });
