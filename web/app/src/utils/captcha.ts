// Cap uses its own fetch rather than our authenticated request client. Its POSTs
// carry the reader cookie, so they must meet the same CSRF checks as chat.
export function createCaptchaFetch(
  origin: string,
  transport: typeof fetch,
): typeof fetch {
  return async (input, init) => {
    const url = new URL(
      input instanceof Request ? input.url : String(input),
      origin,
    );
    const method = (
      init?.method || (input instanceof Request ? input.method : 'GET')
    ).toUpperCase();
    const captcha =
      url.origin === origin &&
      /\/share\/v1\/captcha\/(challenge|redeem)$/.test(url.pathname) &&
      method === 'POST';
    if (!captcha) return transport(input, init);
    const headers = new Headers(
      init?.headers ?? (input instanceof Request ? input.headers : undefined),
    );
    headers.set('X-PW-CSRF', '1');
    const response = await transport(input, { ...init, headers });
    if (!response.ok) {
      // Keep diagnostics available without exposing arbitrary server text in UI.
      const error = Object.assign(
        new Error(`Captcha request failed: HTTP ${response.status}`),
        {
          status: response.status,
          path: url.pathname,
        },
      );
      throw error;
    }
    return response;
  };
}

let installed = false;
export async function loadCaptcha() {
  if (!installed) {
    const capWindow = window as Window & { CAP_CUSTOM_FETCH?: typeof fetch };
    const transport = capWindow.CAP_CUSTOM_FETCH || window.fetch.bind(window);
    capWindow.CAP_CUSTOM_FETCH = createCaptchaFetch(
      window.location.origin,
      transport,
    );
    installed = true;
  }
  return (await import('@cap.js/widget')).default;
}
