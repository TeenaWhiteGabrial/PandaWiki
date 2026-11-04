import request, { ContentType } from './httpClient';

export interface AssistantUpload {
  id: string;
  name: string;
  format: string;
  size: number;
  status:
    | 'queued'
    | 'processing'
    | 'ready'
    | 'failed'
    | 'deleted'
    | 'unavailable';
  error?: string;
  report?: {
    template_supported?: boolean;
    template_error?: string;
    template_notice?: string;
    layouts?: { index: number; name: string }[];
  };
  expires_at: string;
}
export interface TemplateRef {
  kind: 'enterprise' | 'upload';
  id: string;
  version?: number;
}
export interface AssistantTemplate {
  id: string;
  name: string;
  version: number;
  is_default: boolean;
  state: string;
  upload_id: string;
}
export interface AssistantArtifact {
  outline?: { id: string; title: string; position: number; unit: string }[];
	family_id: string;
	base_id?: string;
	created_at?: string;
  id: string;
  name: string;
  format: string;
  size: number;
  revision: number;
  preview_state: string;
  expires_at: string;
}
export interface AssistantRun {
  artifact?: AssistantArtifact;
  generation?: {
    completed: number;
    total: number;
    unit: string;
    current_title?: string;
    parts: { id: string; position?: number; title: string; preview?: string; completed: boolean }[];
  };
  uploads?: AssistantUpload[];
  id: string;
  nonce?: string;
  conversation_id: string;
  user_message_id: string;
  assistant_message_id: string;
  state:
    | 'queued'
    | 'running'
    | 'completed'
    | 'failed'
    | 'canceled'
    | 'awaiting_input'
    | 'awaiting_sources'
    | 'awaiting_outline';
  stage: string;
  error?: string;
  answer?: string;
  artifact_id?: string;
  format?: string;
  question?: string;
  sources?: { kind: string; id: string; name: string; url?: string }[];
  created_at: string;
  updated_at: string;
  stage_started_at: string;
  planning?: AssistantPlan;
}
export interface AssistantOutlineItem {
  id: string;
  title: string;
  purpose: string;
  source_ids: string[];
}
export interface AssistantPlan {
  revision: number;
  format: string;
  title: string;
  summary: string;
  brief: Record<string, string>;
  questions: {
    id: string;
    question: string;
    options?: string[];
    default?: string;
  }[];
  sufficient: boolean;
  gaps: string[];
  outline: AssistantOutlineItem[];
  materials: { id: string; name: string; status: string; characters: number; total_characters?: number; read_mode?: string }[];
  framework_only: boolean;
}
export interface AssistantOptions {
  attachments: AssistantUpload[];
  template?: TemplateRef;
  web: boolean;
  publicQuery: string;
  scope: { type: 'all' | 'documents'; node_ids?: string[] };
  baseArtifact?: string;
  targetIDs?: string[];
}
export const assistantRequest = <T>(
  path: string,
  method = 'GET',
  body?: unknown,
  signal?: AbortSignal,
) =>
  request<T>({
    path: `/share/v1/assistant${path}`,
    method,
    body,
    signal,
    format: 'json',
    ...(body instanceof FormData
      ? { type: ContentType.FormData }
      : body !== undefined
        ? { type: ContentType.Json }
        : {}),
    isAlert: false,
  });

// Keep binary downloads inside the conversation when the response is not a
// file, rather than navigating the reader to an API error document.
export async function downloadAssistantContent(path: string, name: string) {
  const response = await fetch(
    `${window._BASE_PATH_ || ''}/share/v1/assistant${path}`,
    { credentials: 'same-origin', headers: { 'X-PW-CSRF': '1' } },
  );
  const type = response.headers.get('content-type') || '';
  if (
    !response.ok ||
    type.includes('application/json') ||
    type.includes('text/html')
  ) {
    throw Object.assign(new Error('Assistant content is not available'), {
      code: response.status,
    });
  }
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement('a');
  link.href = url;
  link.download = name;
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
