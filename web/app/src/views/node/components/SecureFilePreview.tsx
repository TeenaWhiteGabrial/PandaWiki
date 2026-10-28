'use client';

import SecureFilePreview, { type AccessError } from '@panda-wiki/file-preview';
import { clearAuthStorage } from '@/utils/auth';

const onAccessError = ({ data }: AccessError) => {
  const base = (window._BASE_PATH_ || '').replace(/\/$/, '');
  if (data.code === 40101) {
    clearAuthStorage();
    if (location.pathname !== `${base}/auth/login`)
      location.href = `${base}/auth/login?redirect=${encodeURIComponent(location.pathname.slice(base.length) + location.search)}`;
  } else if (
    data.code === 40302 &&
    location.pathname !== `${base}/auth/login`
  ) {
    location.href = `${base}/auth/login?change_password=1`;
  }
};

export default function ShareSecureFilePreview({
  nodeId,
  kbId,
  fileKey,
  kind,
  title,
}: {
  nodeId: string;
  kbId: string;
  fileKey: string;
  kind: 'pdf' | 'text' | 'html';
  title?: string;
}) {
  const base =
    typeof window === 'undefined'
      ? ''
      : (window._BASE_PATH_ || '').replace(/\/$/, '');
  const query = new URLSearchParams({
    node_id: nodeId,
    key: fileKey,
    format: kind === 'pdf' ? 'pdf' : 'text',
  });
  return (
    <SecureFilePreview
      url={`${base}/share/v1/common/file/content?${query}`}
      headers={{ 'X-KB-ID': kbId }}
      kind={kind}
      title={title}
      onAccessError={onAccessError}
    />
  );
}
