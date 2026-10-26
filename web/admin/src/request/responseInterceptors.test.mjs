import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import axios, { AxiosError } from 'axios';
import ts from 'typescript';

const transpile = source =>
  ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2020,
    },
  }).outputText;
const moduleURL = source =>
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const authSource = transpile(
  await readFile(new URL('./authErrors.ts', import.meta.url), 'utf8'),
);
const responseSource = transpile(
  await readFile(new URL('./responseInterceptors.ts', import.meta.url), 'utf8'),
);

const cases = [
  {
    name: 'login-page identity probe is silent and does not reload login',
    path: '/api/v1/user',
    status: 401,
    body: { success: false, code: 40101 },
    clears: 1,
  },
  {
    name: 'expired identity redirects once, preserving the route and base path',
    base: '/pc/admin',
    page: '/pc/admin/conversation',
    search: '?page=3',
    path: '/api/v1/user',
    status: 401,
    body: { success: false, code: 40101 },
    repeats: 2,
    clears: 2,
    redirects: ['/pc/admin/login?redirect=%2Fconversation%3Fpage%3D3'],
  },
  {
    name: 'ordinary 401 permission failure is visible and keeps the session',
    page: '/setting',
    path: '/api/v1/node/permission',
    status: 401,
    body: { success: false, message: '没有此目录的管理权限' },
    messages: ['没有此目录的管理权限'],
  },
  {
    name: 'ordinary 403 permission failure stays visible',
    page: '/setting',
    path: '/api/v1/user/list',
    status: 403,
    body: { success: false, message: '没有操作权限' },
    messages: ['没有操作权限'],
  },
  {
    name: 'incorrect login credentials still show the actual error',
    path: '/api/v1/user/login',
    status: 200,
    body: { success: false, message: '用户名或密码错误' },
    messages: ['用户名或密码错误'],
  },
  {
    name: 'login rejection cannot trigger a redirect loop',
    path: '/api/v1/user/login',
    status: 401,
    body: { success: false, code: 40101, message: '用户名或密码错误' },
    messages: ['用户名或密码错误'],
  },
  {
    name: 'an explicit session-invalid response redirects silently',
    page: '/setting',
    path: '/api/v1/user/list',
    status: 401,
    body: { success: false, code: 40101 },
    clears: 1,
    redirects: ['/login?redirect=%2Fsetting'],
  },
  {
    name: 'required password change navigates without an authentication toast',
    page: '/setting',
    path: '/api/v1/user/list',
    status: 403,
    body: { success: false, code: 40302 },
    redirects: ['/login?change_password=1'],
  },
  {
    name: 'optional branding failure leaves the login page usable',
    path: '/share/v1/app/web/brand',
    status: 404,
    isAlert: false,
    body: { success: false, message: 'Not Found' },
  },
  {
    name: 'public branding succeeds without a session',
    path: '/share/v1/app/web/brand',
    status: 200,
    body: { success: true, data: { settings: { admin_title: '知识库' } } },
  },
];

for (const [index, item] of cases.entries()) {
  test(item.name, async t => {
    const messages = [],
      redirects = [],
      cleared = [];
    const oldWindow = globalThis.window,
      oldStorage = globalThis.localStorage;
    t.after(() => {
      globalThis.window = oldWindow;
      globalThis.localStorage = oldStorage;
      delete globalThis.__adminRequestTestMessages;
    });
    globalThis.__adminRequestTestMessages = messages;
    globalThis.window = {
      __BASENAME__: item.base || '',
      location: {
        pathname: item.page || `${item.base || ''}/login`,
        search: item.search || '',
        set href(value) {
          redirects.push(value);
        },
      },
    };
    globalThis.localStorage = { removeItem: key => cleared.push(key) };

    const authURL = moduleURL(`${authSource}\n// independent module ${index}`);
    const uiURL = moduleURL(
      'export const message = { error(value) { globalThis.__adminRequestTestMessages.push(value); } };',
    );
    const { installResponseInterceptors } = await import(
      moduleURL(
        responseSource
          .replace(/from ['"]\.\/authErrors['"]/, `from '${authURL}'`)
          .replace(/from ['"]@\/components\/ui['"]/, `from '${uiURL}'`),
      )
    );
    const client = axios.create({
      adapter: async config => {
        const response = {
          config,
          data: item.body,
          status: item.status,
          statusText: 'Request failed',
          headers: {},
        };
        if (item.status >= 400) {
          throw new AxiosError(
            'Request failed',
            'ERR_BAD_RESPONSE',
            config,
            {},
            response,
          );
        }
        return response;
      },
    });
    installResponseInterceptors(client);
    for (let attempt = 0; attempt < (item.repeats || 1); attempt++) {
      const request = client.get(item.path, { isAlert: item.isAlert });
      if (item.body.success) assert.deepEqual(await request, item.body.data);
      else await assert.rejects(request);
    }
    assert.deepEqual(messages, item.messages || []);
    assert.deepEqual(redirects, item.redirects || []);
    assert.equal(cleared.length, item.clears || 0);
    assert.ok(cleared.every(key => key === 'panda_wiki_token'));
  });
}
