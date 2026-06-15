import { ContentType } from './httpClient';
import httpRequest from './httpClient';
import {
  DeleteApiV1CommentListParams,
  DomainCommentStatus,
  DomainPWResponse,
  DomainResponse,
  GetApiV1CommentParams,
  V1CommentLists,
} from './types';

export interface ModerateApiV1CommentListReq {
  ids: string[];
  status: DomainCommentStatus;
}

export const getCommentList = (
  query: GetApiV1CommentParams,
  params = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: V1CommentLists;
    }
  >({
    path: '/api/v1/comment',
    method: 'GET',
    query,
    type: ContentType.Json,
    format: 'json',
    ...params,
  });

export const deleteCommentList = (
  query: DeleteApiV1CommentListParams,
  params = {},
) =>
  httpRequest<DomainResponse>({
    path: '/api/v1/comment/list',
    method: 'DELETE',
    query,
    type: ContentType.Json,
    format: 'json',
    ...params,
  });

export const moderateCommentList = (
  body: ModerateApiV1CommentListReq,
  params = {},
) =>
  httpRequest<DomainResponse>({
    path: '/api/v1/comment/moderate',
    method: 'POST',
    body,
    type: ContentType.Json,
    format: 'json',
    ...params,
  });
