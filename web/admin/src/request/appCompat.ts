import {
  getApiV1AppDetail as getApiV1AppDetailRaw,
  putApiV1App as putApiV1AppRaw,
  deleteApiV1App as deleteApiV1AppRaw,
} from '@/request/App';
import type {
  DeleteApiV1AppParams,
  GetApiV1AppDetailParams,
  PutApiV1AppParams,
  DomainUpdateAppReq,
} from '@/request/types';

type OptionalKbId<T extends { kb_id?: string }> = Omit<T, 'kb_id'> & {
  kb_id?: string;
};

export const getApiV1AppDetailCompat = (
  query: OptionalKbId<GetApiV1AppDetailParams>,
) => {
  return getApiV1AppDetailRaw(query as GetApiV1AppDetailParams);
};

export const putApiV1AppCompat = (
  query: PutApiV1AppParams,
  app: OptionalKbId<DomainUpdateAppReq>,
) => {
  return putApiV1AppRaw(query, app as DomainUpdateAppReq);
};

export const deleteApiV1AppCompat = (
  query: OptionalKbId<DeleteApiV1AppParams>,
) => {
  return deleteApiV1AppRaw(query as DeleteApiV1AppParams);
};
