export function allowedHost(raw: string, configured?: string): boolean;
export function pageSecurityHeaders(options?: {
  widget?: boolean;
  nonce?: string;
  development?: boolean;
}): Record<string, string>;
