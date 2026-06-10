import httpRequest, { ContentType, RequestParams } from './httpClient';
import { V1LoginReq, V1LoginResp, V1UserInfoResp } from './types';

export const getApiV1User = (params: RequestParams = {}) =>
  httpRequest<V1UserInfoResp>({
    path: '/api/v1/user',
    method: 'GET',
    type: ContentType.Json,
    ...params,
  });

export const postApiV1UserLogin = (
  body: V1LoginReq,
  params: RequestParams = {},
) =>
  httpRequest<V1LoginResp>({
    path: '/api/v1/user/login',
    method: 'POST',
    body,
    type: ContentType.Json,
    format: 'json',
    ...params,
  });
