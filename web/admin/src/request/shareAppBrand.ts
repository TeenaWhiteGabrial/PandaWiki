import type { DomainAppInfoResp } from './types';
import httpRequest from './httpClient';

export const getShareWebAppBrand = () =>
  httpRequest<DomainAppInfoResp>({
    path: '/share/v1/app/web/info',
    method: 'GET',
    format: 'json',
  });
