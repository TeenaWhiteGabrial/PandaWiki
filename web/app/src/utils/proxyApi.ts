import { request as httpRequest, type IncomingMessage } from 'node:http';
import { request as httpsRequest } from 'node:https';
import { Readable } from 'node:stream';
import type { ReadableStream as NodeReadableStream } from 'node:stream/web';

function endToEndHeaders(input: Headers): Headers {
  const headers = new Headers(input);
  // Connection can nominate additional hop-by-hop fields.
  for (const name of (headers.get('connection') || '').split(',')) {
    if (name.trim()) headers.delete(name.trim());
  }
  for (const name of [
    'connection',
    'keep-alive',
    'proxy-authenticate',
    'proxy-authorization',
    'te',
    'trailer',
    'transfer-encoding',
    'upgrade',
  ])
    headers.delete(name);
  return headers;
}

// Node fetch can replace Host with the internal target authority. The API's
// Origin/Host CSRF check needs the browser-facing authority, so use the native
// HTTP transport and preserve Host explicitly. Never rewrite Origin or invent
// the CSRF header. Bodies remain streamed for uploads, downloads and chat SSE.
export function proxyApiRequest(
  request: Request,
  target: URL,
  forwardedHeaders: Headers = request.headers,
): Promise<Response> {
  if (target.protocol !== 'http:' && target.protocol !== 'https:') {
    return Promise.reject(new Error('Unsupported API target protocol'));
  }
  const headers = endToEndHeaders(forwardedHeaders);
  const outgoingHeaders = Object.fromEntries(headers.entries());
  outgoingHeaders.host =
    request.headers.get('host') || new URL(request.url).host;

  return new Promise((resolve, reject) => {
    let body: Readable | undefined;
    let response: IncomingMessage | undefined;
    const transport = target.protocol === 'https:' ? httpsRequest : httpRequest;
    const outgoing = transport(
      target,
      {
        method: request.method,
        headers: outgoingHeaders,
      },
      incoming => {
        response = incoming;
        incoming.once('close', cleanup);
        const responseHeaders = new Headers();
        for (let i = 0; i < incoming.rawHeaders.length; i += 2) {
          responseHeaders.append(
            incoming.rawHeaders[i],
            incoming.rawHeaders[i + 1],
          );
        }
        const status = incoming.statusCode || 502;
        const noBody =
          request.method === 'HEAD' || [204, 205, 304].includes(status);
        if (noBody) incoming.resume();
        resolve(
          new Response(
            noBody
              ? null
              : (Readable.toWeb(incoming) as ReadableStream<Uint8Array>),
            {
              status,
              statusText: incoming.statusMessage,
              headers: endToEndHeaders(responseHeaders),
            },
          ),
        );
      },
    );
    const cleanup = () => request.signal.removeEventListener('abort', abort);
    const abort = () => {
      const error = new Error('API request aborted');
      error.name = 'AbortError';
      outgoing.destroy(error);
      body?.destroy(error);
      response?.destroy(error);
    };
    outgoing.once('error', error => {
      cleanup();
      reject(error);
    });
    request.signal.addEventListener('abort', abort, { once: true });
    if (request.signal.aborted) {
      abort();
      return;
    }
    if (request.body && !['GET', 'HEAD'].includes(request.method)) {
      body = Readable.fromWeb(request.body as NodeReadableStream<Uint8Array>);
      body.once('error', error => outgoing.destroy(error));
      body.pipe(outgoing);
    } else {
      outgoing.end();
    }
  });
}
