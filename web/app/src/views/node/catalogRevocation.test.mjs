import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { test } from 'node:test';
import { pathToFileURL } from 'node:url';
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const reactURL = pathToFileURL(require.resolve('react')).href;
const jsxURL = pathToFileURL(require.resolve('react/jsx-runtime')).href;
const packages = new URL('../../../../node_modules/.pnpm/', import.meta.url);
const happy = (await readdir(packages)).find(name =>
  name.startsWith('happy-dom@'),
);
const { Window } = await import(
  new URL(`${happy}/node_modules/happy-dom/lib/index.js`, packages)
);

const asModule = source =>
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
async function compile(file, stubs) {
  const source = ts.transpileModule(
    await readFile(new URL(file, import.meta.url), 'utf8'),
    {
      fileName: file,
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
        jsx: ts.JsxEmit.ReactJSX,
      },
    },
  ).outputText;
  const urls = { react: reactURL, 'react/jsx-runtime': jsxURL, ...stubs };
  return import(
    asModule(
      source.replace(/from ['"]([^'"]+)['"]/g, (_, name) => {
        assert.ok(urls[name], `Missing test stub for ${name}`);
        return `from '${urls[name]}'`;
      }),
    )
  );
}

const store = asModule(`import {createContext, useContext} from '${reactURL}';
  export const StoreContext = createContext({authInfo:{id:'reader'}});
  export const useStore = () => useContext(StoreContext);`);
const navigation = asModule(`export const useParams = () => ({id:'doc'});
  export const usePathname = () => '/node/doc';`);
const empty = asModule('export default () => null;');
const treeCode = await readFile(
  new URL('../../utils/tree.ts', import.meta.url),
  'utf8',
);
const tree = asModule(
  ts.transpileModule(treeCode, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText,
);
const { default: CatalogProvider } = await compile(
  '../../provider/LazyCatalogProvider.tsx',
  {
    '@/provider': store,
    'next/navigation': navigation,
    '@/request/NodeTree': asModule(
      'export const getShareNodeTree = query => globalThis.revocationFixture.tree(query);',
    ),
    '@/utils/tree': tree,
  },
);
const { default: Doc } = await compile('./index.tsx', {
  '@/provider': store,
  'next/navigation': navigation,
  '@/request/ShareNode': asModule(
    'export const getShareV1NodeDetail = (query, params) => globalThis.revocationFixture.detail(query, params);',
  ),
  '@/hooks/useBasePath': asModule("export const useBasePath = () => '';"),
  '@/hooks/useCopy': empty,
  '@/utils/getDocContentSx': asModule(
    'export const getDocContentSx = () => ({});',
  ),
  '@/utils/markdownToc': asModule(
    'export const extractMarkdownToc = () => [];',
  ),
  '@/request/types': asModule('export const ConstsCopySetting = {};'),
  '@ctzhian/tiptap': asModule(
    'export const useTiptap = () => ({editor:null});',
  ),
  '@mui/material': asModule(
    `import React from '${reactURL}'; export const Box = ({children}) => React.createElement('div',null,children);`,
  ),
  '@/components/error': asModule(
    "export default ({error}) => error.code === 40003 ? '无权限访问' : '其他错误';",
  ),
  '@/components/docFab': empty,
  '@/components/scrollToTopFab': empty,
  './DocAnchor': empty,
  './DocContent': asModule(`import React from '${reactURL}';
    export default ({info,loading}) => loading || !info ? '正在加载' : React.createElement('div', { 'data-preview': 'open' }, info.content);`),
});
const { useStore } = await import(store);

const documentEntry = {
  id: 'doc',
  type: 2,
  parent_id: 'folder',
  name: '已授权文档',
};
const folder = { id: 'folder', type: 1, parent_id: '', name: '已授权目录' };
const other = { id: 'other', type: 1, parent_id: '', name: '其他目录' };
const groups = (revision, allowed) => [
  {
    nav_id: 'nav',
    nav_name: '栏目',
    viewer_id: 'reader',
    tree_revision: revision,
    list: allowed ? [folder, documentEntry, other] : [other],
    loaded_parents: allowed ? ['', 'folder'] : [''],
  },
];
const detail = content => ({
  ...documentEntry,
  content,
  meta: { file_readonly: true, file_key: 'fixture.pdf' },
});

test('catalog refresh clears an open preview before the new detail returns and discards old tree responses', async () => {
  const browser = new Window();
  const globals = {
    window: browser,
    document: browser.document,
    IS_REACT_ACT_ENVIRONMENT: true,
  };
  const saved = Object.fromEntries(
    Object.keys(globals).map(key => [key, globalThis[key]]),
  );
  Object.assign(globalThis, globals);
  Object.defineProperty(browser.document, 'visibilityState', {
    value: 'visible',
  });
  const host = browser.document.createElement('div');
  browser.document.body.appendChild(host);
  const root = createRoot(host);
  const original = groups('allowed', true);
  let snapshot = original;
  let finishOldTree;
  const oldTree = new Promise(resolve => {
    finishOldTree = resolve;
  });
  let finishRevokedDetail;
  const revokedDetail = new Promise((_, reject) => {
    finishRevokedDetail = reject;
  });
  const requests = [];
  globalThis.revocationFixture = {
    tree: query =>
      query.parent_id === other.id ? oldTree : Promise.resolve(snapshot),
    detail: (_query, params) => {
      requests.push(params);
      if (snapshot[0].tree_revision === 'revoked') return revokedDetail;
      return Promise.resolve(
        detail(snapshot === original ? '原正文' : '重新授权正文'),
      );
    },
  };
  const App = () => {
    const { nodeList, loadCatalogChildren } = useStore();
    return React.createElement(
      React.Fragment,
      null,
      React.createElement(
        'aside',
        null,
        nodeList.map(node => node.name).join('、'),
      ),
      React.createElement(
        'button',
        { onClick: () => void loadCatalogChildren(other) },
        '展开其他目录',
      ),
      React.createElement(Doc, { nodeId: 'doc' }),
    );
  };
  const refresh = () =>
    act(async () => {
      browser.document.dispatchEvent(new browser.Event('visibilitychange'));
    });
  try {
    await act(async () =>
      root.render(
        React.createElement(
          CatalogProvider,
          { initialGroups: original },
          React.createElement(App),
        ),
      ),
    );
    assert.match(host.textContent, /原正文/);
    assert.ok(host.querySelector('[data-preview]'));
    await refresh();
    assert.equal(
      requests.length,
      1,
      'unchanged catalog keeps the open document',
    );

    await act(async () => host.querySelector('button').click());
    snapshot = groups('revoked', false);
    await refresh();
    assert.equal(requests.length, 2);
    assert.equal(requests[0].signal.aborted, true);
    assert.doesNotMatch(host.querySelector('aside').textContent, /已授权/);
    assert.doesNotMatch(host.textContent, /原正文/);
    assert.equal(host.querySelector('[data-preview]'), null);
    await act(async () => finishOldTree(original));
    assert.doesNotMatch(host.querySelector('aside').textContent, /已授权/);
    await act(async () => finishRevokedDetail({ code: 40003 }));
    assert.match(host.textContent, /无权限访问/);

    snapshot = groups('restored', true);
    await refresh();
    assert.match(host.querySelector('aside').textContent, /已授权文档/);
    assert.match(host.textContent, /重新授权正文/);
    assert.ok(host.querySelector('[data-preview]'));
  } finally {
    await act(async () => root.unmount());
    await browser.close();
    delete globalThis.revocationFixture;
    for (const [key, value] of Object.entries(saved)) {
      if (value === undefined) delete globalThis[key];
      else globalThis[key] = value;
    }
  }
});
