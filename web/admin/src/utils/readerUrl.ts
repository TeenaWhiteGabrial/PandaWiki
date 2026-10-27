interface ReaderAccessSettings {
  base_url?: string;
  hosts?: string[];
  ports?: number[];
  ssl_ports?: number[];
}

export function resolveReaderBaseUrl(settings?: ReaderAccessSettings): string {
  const configured = settings?.base_url?.trim();
  if (configured) return configured.replace(/\/+$/, '');

  const host = settings?.hosts
    ?.find(value => value.trim() && value !== '*')
    ?.trim();
  if (!host) return '';
  const sslPorts = settings?.ssl_ports || [];
  const ports = settings?.ports || [];
  if (sslPorts.length > 0) {
    return sslPorts.includes(443)
      ? `https://${host}`
      : `https://${host}:${sslPorts[0]}`;
  }
  if (ports.length > 0) {
    return ports.includes(80) ? `http://${host}` : `http://${host}:${ports[0]}`;
  }
  return '';
}

// Never allow a missing reader origin to turn into /node/:id on the admin
// origin. Its SPA fallback returns admin HTML for both the page and its CSS.
export function getReaderDocumentUrl(
  baseUrl: string | undefined,
  nodeID: string,
): string {
  if (!baseUrl?.trim() || !nodeID) return '';
  try {
    const url = new URL(baseUrl.trim());
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return '';
    url.pathname = `${url.pathname.replace(/\/+$/, '')}/node/${encodeURIComponent(nodeID)}`;
    url.search = '';
    url.hash = '';
    return url.href;
  } catch {
    return '';
  }
}
