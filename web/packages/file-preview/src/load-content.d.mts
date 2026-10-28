export type AccessError = {
  status: number;
  data: {
    code?: number;
    success?: boolean;
    message?: string;
    trace_id?: string;
  };
};
export class PreviewLoadError extends Error {
  status: number;
  data: AccessError['data'];
}
export function loadPreviewContent(options: {
  url: string;
  headers?: Record<string, string>;
  kind: 'pdf' | 'text' | 'html';
  signal?: AbortSignal;
  timeoutMs?: number;
  onProgress?: (loaded: number, total: number) => void;
  onAccessError?: (error: AccessError) => void;
}): Promise<Uint8Array>;
