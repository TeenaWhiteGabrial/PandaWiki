/**
 * Build a URL for an object stored in the configured static-file storage.
 *
 * Object storage is private, so the browser must go through the authenticated
 * API endpoint. The API responds with a short-lived signed OSS URL.
 */
export const getStaticFileUrl = (fileKey?: string | null): string => {
  if (!fileKey) return '';
  if (/^(https?:|blob:|data:)/i.test(fileKey)) return fileKey;

  const key = fileKey.replace(/^\/+/, '');
  return `/api/v1/file/download?key=${encodeURIComponent(key)}`;
};

export const resolveStaticFileUrl = async (
  fileKey?: string | null,
): Promise<string> => {
  if (!fileKey) return '';
  if (/^(https?:|blob:|data:)/i.test(fileKey)) return fileKey;
  const { url } = await getApiV1FileDownloadUrl({
    key: fileKey.replace(/^\/+/, ''),
  });
  return url;
};
import { getApiV1FileDownloadUrl } from '@/request/File';
