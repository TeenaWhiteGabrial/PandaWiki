import httpRequest, { ContentType, RequestParams } from './httpClient';
import { V1LoginReq, V1LoginResp, V1UserInfoResp } from './types';

export const getApiV1User = (params: RequestParams = {}) =>
  httpRequest<V1UserInfoResp>({
    path: '/share/v1/auth/user',
    method: 'GET',
    type: ContentType.Json,
    ...params,
  });

export const postApiV1UserLogin = (
  body: V1LoginReq,
  params: RequestParams = {},
) =>
  httpRequest<V1LoginResp>({
    path: '/share/v1/auth/login/account',
    method: 'POST',
    body,
    type: ContentType.Json,
    format: 'json',
    ...params,
  });
