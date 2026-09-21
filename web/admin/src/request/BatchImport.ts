import httpRequest, { ContentType, RequestParams } from './httpClient';

export interface BatchImportItem {
  path: string;
  node_id?: string;
  file_key?: string;
  status: 'imported' | 'failed';
  message?: string;
  content_ok?: boolean;
}

export interface BatchImportResult {
  batch_id: string;
  total: number;
  imported: number;
  failed: number;
  items: BatchImportItem[];
}

export const postApiV1FileImportZip = (
  body: {
    file: File;
    kb_id: string;
    nav_id: string;
    parent_id?: string;
  },
  params: RequestParams = {},
) =>
  httpRequest<BatchImportResult>({
    path: '/api/v1/file/import/zip',
    method: 'POST',
    body,
    secure: true,
    type: ContentType.FormData,
    format: 'json',
    ...params,
  });
