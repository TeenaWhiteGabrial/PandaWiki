import httpRequest, { ContentType, RequestParams } from './httpClient';

export interface GeneratePreviewReq {
  file_key: string;
  file_name: string;
  kb_id: string;
}

export interface GeneratePreviewResp {
  preview_key?: string;
}

export const postApiV1FilePreviewGenerate = (
  body: GeneratePreviewReq,
  params: RequestParams = {},
) =>
  httpRequest<GeneratePreviewResp>({
    path: `/api/v1/file/preview/generate`,
    method: 'POST',
    body,
    secure: true,
    type: ContentType.Json,
    format: 'json',
    ...params,
  });
