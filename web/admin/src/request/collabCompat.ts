import request, { ContentType } from './httpClient';

export type NodeCollabResp = {
  enabled?: boolean;
  mode?:
    | 'disabled'
    | 'legacy'
    | 'prose_mirror'
    | 'markdown'
    | string;
  disabled_reason?: string;
  strategy?: 'document' | 'text' | string;
  doc_name?: string;
  room_id?: string;
  doc_room_id?: string;
  block_room_prefix?: string;
  endpoint?: string;
  auth_token?: string;
  snapshot?: string;
  content_type?: string;
  auto_save_interval?: number;
  long_document?: boolean;
  max_doc_bytes?: number;
  content_bytes?: number;
  block_count?: number;
  block_window?: number;
};

export const getApiV1NodeCollabCompat = (query: {
  kb_id: string;
  id: string;
}) =>
  request<NodeCollabResp>({
    path: '/api/v1/node/collab',
    method: 'GET',
    query,
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });

export const getCollabEndpoint = (endpoint?: string) =>
  (import.meta.env.VITE_COLLAB_ENDPOINT as string | undefined) ||
  (endpoint && endpoint.trim()) ||
  (window as Window & { __COLLAB_ENDPOINT__?: string }).__COLLAB_ENDPOINT__ ||
  'ws://127.0.0.1:1234/collab/ws';
