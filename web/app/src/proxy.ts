import { allowedHost, pageSecurityHeaders } from '../../security.mjs';
import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import { v4 as uuidv4 } from 'uuid';
import { getShareV1AppWidgetInfo } from './request/ShareApp';

import { getBasePath, parsePathname } from '@/utils';
import { postShareV1StatPage } from '@/request/ShareStat';
import { getShareNodeTree } from '@/request/NodeTree';
import { getFreshWebAppInfo } from '@/request/appInfo';
import { proxyApiRequest } from '@/utils/proxyApi';

const StatPage = {
  welcome: 1,
  node: 2,
  chat: 3,
  auth: 4,
} as const;

const getFirstNode = async () => {
  const roots = await getShareNodeTree();
  const first = roots?.find(g => g.entry_node_id);
  return first?.entry_node_id ? { id: first.entry_node_id } : undefined;
};

const getHomePath = async () => {
  const info = await getFreshWebAppInfo();
  return {
    homePageSetting: info?.settings?.home_page_setting,
    hasLandingConfigs:
      (info?.settings?.web_app_landing_configs?.length || 0) > 0,
  };
};

const stripBasePath = (pathname: string, basePath: string) => {
  if (!basePath) return pathname;
  if (pathname === basePath) return '/';
  if (pathname.startsWith(`${basePath}/`)) {
    return pathname.slice(basePath.length) || '/';
  }
  return pathname;
};

const isLoginPath = (pathname: string) => pathname === '/auth/login';

const requireReaderLogin = async (
  request: NextRequest,
  basePath: string,
  pathname: string,
): Promise<NextResponse | undefined> => {
  const fileRequest =
    pathname === '/static-file' || pathname.startsWith('/static-file/');
  const loginResponse = (clearCookie = false, changePassword = false) => {
    const login = new URL(`${basePath}/auth/login`, request.url);
    login.searchParams.set('redirect', pathname + request.nextUrl.search);
    if (changePassword) login.searchParams.set('change_password', '1');
    const response = fileRequest
      ? NextResponse.json(
          { success: false, code: 40101, message: '请先登录后访问' },
          { status: 401 },
        )
      : NextResponse.redirect(login);
    response.headers.set('Cache-Control', 'private, no-store');
    if (clearCookie) {
      response.cookies.set('pw_reader_session', '', {
        path: '/',
        maxAge: 0,
        httpOnly: true,
        sameSite: 'strict',
      });
    }
    return response;
  };

  if (!request.cookies.get('pw_reader_session')?.value) return loginResponse();

  try {
    const authHeaders: Record<string, string> = {
      Cookie: request.headers.get('cookie') || '',
    };
    const kbID = request.headers.get('x-kb-id');
    if (kbID) authHeaders['x-kb-id'] = kbID;
    const auth = await fetch(`${process.env.TARGET}/share/v1/auth/user`, {
      headers: authHeaders,
      cache: 'no-store',
      redirect: 'manual',
    });
    if (auth.status === 401) return loginResponse(true);
    if (auth.status === 403) {
      return NextResponse.json(
        { success: false, message: '当前知识库禁止访问' },
        { status: 403, headers: { 'Cache-Control': 'private, no-store' } },
      );
    }
    if (!auth.ok) throw new Error('Authentication service unavailable');
    const result = await auth.json();
    if (!result.success || !result.data?.id) return loginResponse(true);
    if (result.data.password_change_required) return loginResponse(false, true);
  } catch {
    // An unavailable identity service must not turn a protected page public,
    // or erase an otherwise valid login as if it were an expired session.
    return NextResponse.json(
      { success: false, message: '暂时无法验证登录状态，请稍后重试' },
      { status: 503, headers: { 'Cache-Control': 'private, no-store' } },
    );
  }
};

const homeProxy = async (
  request: NextRequest,
  headers: Record<string, string>,
  session: string,
  pathname?: string,
) => {
  const url = request.nextUrl.clone();
  if (pathname) {
    url.pathname = pathname;
  }
  const { page, id } = parsePathname(url.pathname);
  try {
    // 获取节点列表
    if (url.pathname === '/') {
      const { homePageSetting, hasLandingConfigs } = await getHomePath();
      if (homePageSetting === 'custom' && hasLandingConfigs) {
        return NextResponse.rewrite(new URL('/home', request.url), {
          request: { headers: request.headers },
        });
      }

      const firstNode = await getFirstNode();
      if (firstNode) {
        return NextResponse.rewrite(
          new URL(`/node/${firstNode.id}`, request.url),
          { request: { headers: request.headers } },
        );
      }
      return NextResponse.rewrite(new URL('/node', request.url), {
        request: { headers: request.headers },
      });
    }

    // 页面上报
    const pages = Object.keys(StatPage);
    if (pages.includes(page) || pages.includes(id)) {
      postShareV1StatPage(
        {
          scene: StatPage[page as keyof typeof StatPage],
          node_id: id || '',
        },
        {
          headers: {
            'x-pw-session-id': session,
            ...headers,
          },
        },
      );
    }

    if (pathname && pathname !== request.nextUrl.pathname) {
      return NextResponse.rewrite(
        new URL(`${url.pathname}${url.search}`, request.url),
        { request: { headers: request.headers } },
      );
    }
    return NextResponse.next({ request: { headers: request.headers } });
  } catch (error) {
    if (
      typeof error === 'object' &&
      error !== null &&
      'message' in error &&
      error.message === 'NEXT_REDIRECT'
    ) {
      return NextResponse.redirect(
        new URL(
          `/auth/login?redirect=${encodeURIComponent(url.pathname + url.search)}`,
          request.url,
        ),
      );
    }
  }

  if (pathname && pathname !== request.nextUrl.pathname) {
    return NextResponse.rewrite(
      new URL(`${url.pathname}${url.search}`, request.url),
      { request: { headers: request.headers } },
    );
  }
  return NextResponse.next({ request: { headers: request.headers } });
};

const proxyShare = async (request: NextRequest, pathname?: string) => {
  // 转发到 process.env.TARGET
  const targetOrigin = process.env.TARGET!;
  const targetUrl = new URL(
    (pathname || request.nextUrl.pathname) + request.nextUrl.search,
    targetOrigin,
  );
  // 构造 fetch 选项
  const fetchHeaders = new Headers(request.headers);
  fetchHeaders.delete('content-security-policy');
  fetchHeaders.delete('x-nonce');
  const kbID = request.headers.get('x-kb-id');
  if (kbID) {
    fetchHeaders.set('x-kb-id', kbID);
  } else {
    fetchHeaders.delete('x-kb-id');
  }

  const proxyRes = await proxyApiRequest(request, targetUrl, fetchHeaders);
  const nextRes = new NextResponse(proxyRes.body, {
    status: proxyRes.status,
    headers: proxyRes.headers,
    statusText: proxyRes.statusText,
  });
  return nextRes;
};

async function appProxy(request: NextRequest) {
  const url = request.nextUrl.clone();
  const pathname = url.pathname;
  let basePath = '';
  try {
    const kbDetail = await getFreshWebAppInfo();
    basePath = getBasePath(kbDetail?.base_url || '');
  } catch {
    // Allow the request flow to continue even when app info is temporarily unavailable.
  }
  const appPathname = stripBasePath(pathname, basePath);
  const apiRequest =
    appPathname.startsWith('/api/') || appPathname.startsWith('/share/');
  if (!apiRequest && !isLoginPath(appPathname)) {
    const denied = await requireReaderLogin(request, basePath, appPathname);
    if (denied) return denied;
  }
  if (appPathname.startsWith('/widget')) {
    const widgetInfo: any = await getShareV1AppWidgetInfo();
    if (widgetInfo) {
      if (!widgetInfo?.settings?.widget_bot_settings?.is_open) {
        return NextResponse.rewrite(new URL('/not-found', request.url), {
          request: { headers: request.headers },
        });
      }
    }
    return NextResponse.next({ request: { headers: request.headers } });
  }

  const headers: Record<string, string> = {};
  for (const [key, value] of request.headers.entries()) {
    headers[key] = value;
  }

  let sessionId = request.cookies.get('x-pw-session-id')?.value || '';
  let needSetSessionId = false;

  if (!sessionId) {
    sessionId = uuidv4();
    needSetSessionId = true;
  }

  let response: NextResponse;

  if (appPathname.startsWith('/api/') || appPathname.startsWith('/share/')) {
    response = await proxyShare(request, appPathname);
  } else {
    response = await homeProxy(request, headers, sessionId, appPathname);
  }

  if (needSetSessionId) {
    response.cookies.set('x-pw-session-id', sessionId, {
      httpOnly: true,
      maxAge: 60 * 60 * 24 * 365, // 1 年
    });
  }
  if (!appPathname.startsWith('/share')) {
    response.headers.set('x-current-path', appPathname);
    response.headers.set('x-current-search', url.search);
  }
  if (!apiRequest && !isLoginPath(appPathname)) {
    response.headers.set('Cache-Control', 'private, no-store');
    response.headers.append('Vary', 'Cookie');
  }
  return response;
}

export async function proxy(request: NextRequest) {
  if (!allowedHost(request.headers.get('host') || ''))
    return new NextResponse(null, { status: 421 });
  const widget = /(?:^|\/)widget(?:\/|$)/.test(request.nextUrl.pathname);
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64');
  const headers = pageSecurityHeaders({
    widget,
    nonce,
    development: process.env.NODE_ENV !== 'production',
  });
  request.headers.set(
    'Content-Security-Policy',
    headers['Content-Security-Policy'],
  );
  request.headers.set('x-nonce', nonce);
  const response = await appProxy(request);
  // Preserve the strict binary response policy; the reader fetches bytes,
  // and direct navigation must never execute a file on the app origin.
  const fileContent = /\/(?:share\/v1\/common|api\/v1)\/file\/content$/.test(
    request.nextUrl.pathname,
  );
  if (!fileContent)
    for (const [key, value] of Object.entries(headers))
      response.headers.set(key, value);
  response.headers.delete('X-Powered-By');
  response.headers.delete('Server');
  return response;
}

export const config = {
  matcher: [
    '/',
    '/home',
    '/:basePath/home',
    '/share/:path*',
    '/:basePath/share/:path*',
    '/api/:path*',
    '/:basePath/api/:path*',
    '/chat/:path*',
    '/:basePath/chat/:path*',
    '/widget',
    '/:basePath/widget',
    '/widget/:path*',
    '/:basePath/widget/:path*',
    '/static-file/:path*',
    '/:basePath/static-file/:path*',
    '/welcome',
    '/:basePath/welcome',
    '/auth/login',
    '/:basePath/auth/login',
    '/node/:path*',
    '/:basePath/node/:path*',
    '/node',
    '/:basePath/node',
  ],
};
