import { getShareV1AppWebInfo } from './ShareApp';

export const getFreshWebAppInfo = () =>
  getShareV1AppWebInfo({ cache: 'no-store' });
