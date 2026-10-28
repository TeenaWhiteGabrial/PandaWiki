import SecureFilePreview from '@panda-wiki/file-preview';
import { handleAdminAuthError } from '@/request/authErrors';

export function adminFileContentUrl(
  nodeId: string,
  kbId: string,
  key: string,
  format: 'pdf' | 'text' | 'download',
) {
  const base = (window.__BASENAME__ || '').replace(/\/$/, '');
  return `${base}/api/v1/file/content?${new URLSearchParams({ node_id: nodeId, kb_id: kbId, key, format })}`;
}

export default function AdminSecureFilePreview({
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
  return (
    <SecureFilePreview
      url={adminFileContentUrl(
        nodeId,
        kbId,
        fileKey,
        kind === 'pdf' ? 'pdf' : 'text',
      )}
      kind={kind}
      title={title}
      assetBase={window.__BASENAME__ || ''}
      onAccessError={handleAdminAuthError}
    />
  );
}
