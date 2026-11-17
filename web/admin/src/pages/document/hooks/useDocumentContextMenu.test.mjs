import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import { test } from 'node:test';
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import ts from 'typescript';
const require = createRequire(import.meta.url);
const packages = new URL('../../../../../node_modules/.pnpm/', import.meta.url);
const happyDOM = (await readdir(packages)).find(name =>
  name.startsWith('happy-dom@'),
);
const { Window } = await import(
  new URL(`${happyDOM}/node_modules/happy-dom/lib/index.js`, packages)
);
const compile = text =>
  ts.transpileModule(text, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
const url = text =>
  `data:text/javascript;base64,${Buffer.from(text).toString('base64')}`;
const utility = url(
  compile(
    await readFile(new URL('../utils/contextMenu.ts', import.meta.url), 'utf8'),
  ),
);
const { contextSelection } = await import(utility);
const source = (
  await readFile(
    new URL('./useDocumentContextMenu.ts', import.meta.url),
    'utf8',
  )
)
  .replace(
    "from 'react'",
    `from '${pathToFileURL(require.resolve('react')).href}'`,
  )
  .replace("from '../utils/contextMenu'", `from '${utility}'`);
const { useDocumentContextMenu } = await import(url(compile(source)));
async function fixture(t) {
  const window = new Window();
  globalThis.window = window;
  globalThis.document = window.document;
  globalThis.Element = window.Element;
  globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  let current;
  let key = 'folder-a';
  function Harness() {
    current = useDocumentContextMenu(key);
    return null;
  }
  const render = async value => {
    if (value) key = value;
    await act(async () => root.render(React.createElement(Harness)));
  };
  await render();
  t.after(async () => {
    await act(async () => root.unmount());
    window.happyDOM.abort();
  });
  return {
    window,
    get current() {
      return current;
    },
    render,
    async open(target, menu, x = 99, y = 123) {
      let prevented = false,
        stopped = false;
      await act(async () =>
        current.open(
          {
            target,
            clientX: x,
            clientY: y,
            preventDefault() {
              prevented = true;
            },
            stopPropagation() {
              stopped = true;
            },
          },
          menu,
        ),
      );
      return { prevented, stopped };
    },
    async emit(target, event) {
      await act(async () => target.dispatchEvent(event));
    },
  };
}

test('opens at cursor, changes target without invoking actions, and closes on Escape', async t => {
  const f = await fixture(t);
  const row = document.createElement('div');
  document.body.append(row);
  let invoked = 0;
  const first = [
    {
      key: 'view',
      label: '查看',
      onClick() {
        invoked++;
      },
    },
  ];
  assert.deepEqual(await f.open(row, first), {
    prevented: true,
    stopped: true,
  });
  assert.deepEqual(f.current.state.position, { top: 123, left: 99 });
  assert.equal(invoked, 0);
  const batch = [{ key: 'move', label: '移动所选项' }];
  await f.open(row, batch, 180, 220);
  assert.equal(f.current.state.menu, batch);
  assert.equal(invoked, 0);
  await f.emit(
    document,
    new f.window.KeyboardEvent('keydown', { key: 'Escape' }),
  );
  assert.equal(f.current.state, null);
});

test('keeps native context menus for text editing, links, controls and empty action lists', async t => {
  const f = await fixture(t);
  for (const tag of ['input', 'textarea', 'a', 'button']) {
    const target = document.createElement(tag);
    document.body.append(target);
    assert.deepEqual(await f.open(target, [{ key: 'delete', label: '删除' }]), {
      prevented: false,
      stopped: false,
    });
    assert.equal(f.current.state, null);
  }
  const editable = document.createElement('div');
  editable.setAttribute('contenteditable', 'true');
  assert.equal(
    (await f.open(editable, [{ key: 'delete', label: '删除' }])).prevented,
    false,
  );
  assert.equal((await f.open(document.body, [])).prevented, false);
});

test('outside interactions, scrolling, resize and navigation close; internal submenu interactions do not', async t => {
  const f = await fixture(t);
  const list = document.createElement('div');
  list.className = 'cascader-sub-list';
  document.body.append(list);
  const menu = [{ key: 'move', label: '移动' }];
  await f.open(document.body, menu);
  await f.emit(list, new f.window.Event('scroll', { bubbles: false }));
  assert.ok(f.current.state);
  await f.emit(list, new f.window.Event('pointerdown', { bubbles: true }));
  assert.ok(f.current.state);
  for (const type of ['scroll', 'resize', 'blur', 'popstate']) {
    await f.open(document.body, menu);
    await f.emit(window, new f.window.Event(type));
    assert.equal(f.current.state, null, type);
  }
  await f.open(document.body, menu);
  await f.emit(
    document.body,
    new f.window.Event('pointerdown', { bubbles: true }),
  );
  assert.equal(f.current.state, null);
  await f.open(document.body, menu);
  await f.render('folder-b');
  assert.equal(f.current.state, null);
});

test('right-click selection keeps an existing multi-selection and replaces it for a different target', () => {
  const selected = ['a', 'b'];
  assert.equal(contextSelection('a', selected, true), selected);
  assert.deepEqual(contextSelection('c', selected, true), ['c']);
  assert.deepEqual(contextSelection('a', selected, false), []);
  assert.deepEqual(selected, ['a', 'b']);
});
