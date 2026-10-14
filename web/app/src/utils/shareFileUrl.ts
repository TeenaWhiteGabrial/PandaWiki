'use client';

import { getShareV1CommonFileDownloadUrl } from '@/request/ShareFile';

const signedUrlCache = new Map<string, { url: string; expiresAt: number }>();
const cacheTTL = 25 * 60 * 1000;

const normalizeFileKey = (key: string) => {
  const trimmed = key.trim();
  if (!trimmed) return '';
  if (trimmed.startsWith('/static-file/')) {
    return trimmed.slice('/static-file/'.length);
  }
  return trimmed.replace(/^\/+/, '');
};

export const getStaticFileFallbackUrl = (key: string) => {
  const normalized = normalizeFileKey(key);
  return normalized ? `/static-file/${normalized}` : '';
};

export const resolveShareFileUrl = async (
  key: string,
  nodeId: string,
  kbId: string,
) => {
  const normalized = normalizeFileKey(key);
  if (!normalized || !kbId) return '';

  const cacheKey = `${kbId}:${nodeId}:${normalized}`;
  const cached = signedUrlCache.get(cacheKey);
  if (cached && cached.expiresAt > Date.now()) {
    return cached.url;
  }

  const result = await getShareV1CommonFileDownloadUrl(
    {
      key: normalized,
      node_id: nodeId,
    },
    {
      headers: {
        'X-KB-ID': kbId,
      },
    },
  );
  const url = result?.url || getStaticFileFallbackUrl(normalized);
  if (url) {
    signedUrlCache.set(cacheKey, {
      url,
      expiresAt: Date.now() + cacheTTL,
    });
  }
  return url;
};

export const openShareFile = async (
  key: string,
  nodeId: string,
  kbId: string,
  target = '_blank',
) => {
  const popup =
    typeof window !== 'undefined' ? window.open('about:blank', target) : null;
  try {
    const url = await resolveShareFileUrl(key, nodeId, kbId);
    if (url) {
      if (popup) popup.location.href = url;
      else window.open(url, target, 'noopener,noreferrer');
    } else {
      popup?.close();
    }
  } catch (error) {
    popup?.close();
    throw error;
  }
};
