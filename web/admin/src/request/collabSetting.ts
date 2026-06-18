import request, { ContentType } from './httpClient';

export type CollabMode = 'disabled' | 'prose_mirror';

export type CollabSetting = {
  mode: CollabMode;
  max_doc_bytes: number;
  block_min_bytes: number;
  block_min_count: number;
  block_window: number;
};

export const getApiV1CollabSetting = () =>
  request<CollabSetting>({
    path: '/api/v1/collab/setting',
    method: 'GET',
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });

export const putApiV1CollabSetting = (body: CollabSetting) =>
  request<CollabSetting>({
    path: '/api/v1/collab/setting',
    method: 'PUT',
    body,
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });
