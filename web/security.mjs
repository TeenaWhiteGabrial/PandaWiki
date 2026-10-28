import { isIP } from 'node:net';

export function allowedHost(
  raw,
  configured = process.env.SECURITY_ALLOWED_HOSTS || '',
) {
  if (/[\s/@\\?#]/.test(raw)) return false;
  let host;
  try {
    host = new URL(`http://${raw}`).hostname.toLowerCase();
  } catch {
    return false;
  }
  if (configured.trim())
    return configured
      .split(',')
      .some(
        value => value.trim().toLowerCase() === host.replace(/^\[|\]$/g, ''),
      );
  return (
    host === 'localhost' ||
    isIP(host.replace(/^\[|\]$/g, '')) !== 0 ||
    /^(\d{1,3}\.){3}\d{1,3}$/.test(host) ||
    host === 'app' ||
    host === 'pandawiki-app'
  );
}

export function pageSecurityHeaders({
  widget = false,
  nonce = '',
  development = false,
} = {}) {
  const scripts = nonce
    ? `'nonce-${nonce}' 'strict-dynamic'`
    : "'self' 'unsafe-inline'";
  const headers = {
    'X-Content-Type-Options': 'nosniff',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
    'Content-Security-Policy': `default-src 'self'; script-src ${scripts} 'wasm-unsafe-eval'${development ? " 'unsafe-eval'" : ''}; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: http: https:; font-src 'self' data:; connect-src 'self' http: https: ws: wss:; media-src 'self' blob: http: https:; frame-src 'self' blob: http: https:; worker-src 'self' blob:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors ${widget ? '*' : "'self'"}`,
  };
  if (!widget) headers['X-Frame-Options'] = 'SAMEORIGIN';
  return headers;
}
