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

export interface ZipUploadTicket {
  upload_url: string;
  file_key: string;
  expires_in: number;
}

export const postApiV1FileUploadZipPresign = (body: {
  kb_id: string;
  filename: string;
  size: number;
}) =>
  httpRequest<ZipUploadTicket>({
    path: '/api/v1/file/upload/zip/presign',
    method: 'POST',
    body,
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });

export const postApiV1FileImportZipCommit = (body: {
  kb_id: string;
  nav_id: string;
  parent_id?: string;
  file_key: string;
  filename: string;
  size: number;
}) =>
  httpRequest<{ node_id: string; file_key: string }>({
    path: '/api/v1/file/import/zip/commit',
    method: 'POST',
    body,
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });

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

export interface FolderDownloadTask {
  id: string;
  kb_id: string;
  node_id: string;
  status: 'waiting' | 'processing' | 'completed' | 'failed' | 'expired';
  total_files: number;
  processed_files: number;
  total_bytes: number;
  processed_bytes: number;
  retry_count: number;
  error_message?: string;
  expires_at?: string;
  created_at?: string;
  updated_at?: string;
}

export interface ZipUploadTask {
  id: string;
  kb_id: string;
  nav_id: string;
  parent_id?: string;
  filename: string;
  size: number;
  part_size: number;
  total_parts: number;
  uploaded_bytes: number;
  status:
    | 'waiting'
    | 'uploading'
    | 'interrupted'
    | 'processing'
    | 'completed'
    | 'failed'
    | 'canceled';
  retry_count: number;
  error_message?: string;
  node_id?: string;
  file_key?: string;
  uploaded_parts?: Record<string, string> | string;
  uploaded_part_count?: number;
  missing_parts?: number[];
  stale_seconds?: number;
  can_resume?: boolean;
  total_files?: number;
  processed_files?: number;
  total_bytes?: number;
  processed_bytes?: number;
  created_at?: string;
  updated_at?: string;
}

export const postApiV1FileUploadZipTasks = (body: { kb_id: string; nav_id: string; parent_id?: string; filename: string; size: number }) =>
  httpRequest<ZipUploadTask>({ path: '/api/v1/file/upload/zip/tasks', method: 'POST', body, secure: true, type: ContentType.Json, format: 'json' });
export const getApiV1FileUploadZipTasks = (kb_id: string) =>
  httpRequest<ZipUploadTask[]>({ path: `/api/v1/file/upload/zip/tasks?kb_id=${encodeURIComponent(kb_id)}`, method: 'GET', secure: true, format: 'json' });
export const getApiV1FileUploadZipTask = (id: string) =>
  httpRequest<ZipUploadTask>({ path: `/api/v1/file/upload/zip/tasks/${id}`, method: 'GET', secure: true, format: 'json' });
export const getApiV1FileUploadZipPartUrl = (id: string, part: number) =>
  httpRequest<{ url: string; part_number: number }>({ path: `/api/v1/file/upload/zip/tasks/${id}/parts/${part}/presign`, method: 'GET', secure: true, format: 'json' });
export const putApiV1FileUploadZipPart = (id: string, part: number, body: Blob, onUploadProgress?: (loaded: number, total: number) => void) =>
  httpRequest<{ etag: string; size: number; part_number: number }>({
    path: `/api/v1/file/upload/zip/tasks/${id}/parts/${part}`,
    method: 'PUT',
    body,
    secure: true,
    format: 'json',
    headers: { 'Content-Type': 'application/octet-stream' },
    onUploadProgress: event => {
      if (onUploadProgress && event.total) onUploadProgress(event.loaded, event.total);
    },
  });
export const postApiV1FileUploadZipPartComplete = (id: string, part: number, body: { etag: string; size: number }) =>
  httpRequest<ZipUploadTask>({ path: `/api/v1/file/upload/zip/tasks/${id}/parts/${part}/complete`, method: 'POST', body, secure: true, type: ContentType.Json, format: 'json' });
export const postApiV1FileUploadZipTaskComplete = (id: string) =>
  httpRequest<ZipUploadTask>({ path: `/api/v1/file/upload/zip/tasks/${id}/complete`, method: 'POST', secure: true, type: ContentType.Json, format: 'json' });
export const postApiV1FileUploadZipTaskRetry = (id: string) =>
  httpRequest<ZipUploadTask>({ path: `/api/v1/file/upload/zip/tasks/${id}/retry`, method: 'POST', secure: true, type: ContentType.Json, format: 'json' });
export const postApiV1FileUploadZipTaskCancel = (id: string) =>
  httpRequest<ZipUploadTask>({ path: `/api/v1/file/upload/zip/tasks/${id}/cancel`, method: 'POST', secure: true, type: ContentType.Json, format: 'json' });
export const postApiV1FileUploadZipTaskFail = (id: string, error: string) =>
  httpRequest<ZipUploadTask>({ path: `/api/v1/file/upload/zip/tasks/${id}/fail`, method: 'POST', body: { error }, secure: true, type: ContentType.Json, format: 'json' });

export const postApiV1FileFolderDownload = (body: {
  kb_id: string;
  node_id: string;
}) =>
  httpRequest<FolderDownloadTask>({
    path: '/api/v1/file/folder-download',
    method: 'POST',
    body,
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });

export const getApiV1FileFolderDownload = (id: string) =>
  httpRequest<{ task: FolderDownloadTask; download_url?: string }>({
    path: `/api/v1/file/folder-download/${id}`,
    method: 'GET',
    secure: true,
    format: 'json',
  });
export const getApiV1FileFolderDownloads = (kb_id: string) =>
  httpRequest<FolderDownloadTask[]>({ path: `/api/v1/file/folder-download?kb_id=${encodeURIComponent(kb_id)}`, method: 'GET', secure: true, format: 'json' });

export const postApiV1FileFolderDownloadRetry = (id: string) =>
  httpRequest<FolderDownloadTask>({
    path: `/api/v1/file/folder-download/${id}/retry`,
    method: 'POST',
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });
