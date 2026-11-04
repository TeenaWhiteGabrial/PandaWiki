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
const moduleURL = source =>
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const requests = moduleURL(`
export const getLearningTasks = (...args) => globalThis.__learningRequests.list(...args);
export const getLearningTaskSummary = (...args) => globalThis.__learningRequests.summary(...args);
`);
const hookSource = ts.transpileModule(
  (await readFile(new URL('./useLearningTasks.ts', import.meta.url), 'utf8'))
    .replace(
      "from 'react'",
      `from '${pathToFileURL(require.resolve('react')).href}'`,
    )
    .replace("from '@/request/nodeLearning'", `from '${requests}'`)
    .replaceAll('setTimeout(', 'globalThis.__learningSetTimeout(')
    .replaceAll('clearTimeout(', 'globalThis.__learningClearTimeout('),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2020,
    },
  },
).outputText;
const { useLearningTasks } = await import(moduleURL(hookSource));
const page = (id, running = 0) => ({
  list: [{ id }],
  total: 1,
  running,
  page: 1,
  page_size: 10,
});
const counts = running => ({
  total: 1,
  attention: running,
  running,
  failed: 0,
  unavailable: 0,
  answerable: 1,
  needs_learning: 0,
});

async function fixture(t) {
  const window = new Window();
  globalThis.window = window;
  globalThis.document = window.document;
  globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  Object.defineProperty(document, 'visibilityState', {
    value: 'visible',
    writable: true,
    configurable: true,
  });
  let timerID = 0;
  const timers = new Map();
  globalThis.__learningSetTimeout = (fn, ms) => {
    const id = ++timerID;
    timers.set(id, { fn, ms });
    return id;
  };
  globalThis.__learningClearTimeout = id => timers.delete(id);
  const list = [],
    summary = [];
  const pending = (bucket, args) =>
    new Promise((resolve, reject) => bucket.push({ args, resolve, reject }));
  globalThis.__learningRequests = {
    list: (...args) => pending(list, args),
    summary: (...args) => pending(summary, args),
  };
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  let current;
  function Harness(props) {
    current = useLearningTasks(props.open, props.identity, props.query);
    return null;
  }
  let props = {
    open: true,
    identity: 'alice',
    query: { kb_id: 'kb-a', filter: 'all', page: 1, page_size: 10 },
  };
  const render = async changes => {
    props = { ...props, ...changes };
    await act(async () => root.render(React.createElement(Harness, props)));
  };
  t.after(async () => {
    await act(async () => root.unmount());
    window.happyDOM.abort();
  });
  await render({});
  return {
    render,
    list,
    summary,
    timers,
    get state() {
      return current;
    },
    tick: async () => {
      const [id, timer] = timers.entries().next().value;
      timers.delete(id);
      await act(async () => {
        timer.fn();
      });
    },
  };
}

test('renders the page before slow counts; reopen uses cache; closing aborts and stops polling', async t => {
  const f = await fixture(t);
  assert.equal(f.list.length, 1);
  assert.equal(f.summary.length, 1);
  await act(async () => f.list[0].resolve(page('first')));
  assert.equal(f.state.data.list[0].id, 'first');
  assert.equal(f.state.initialLoading, false);
  assert.equal(f.state.counts, undefined);
  assert.equal(f.timers.size, 0); // No overlapping requests while counts are pending.
  await act(async () => f.summary[0].resolve(counts(0)));
  assert.equal(f.timers.size, 1);
  assert.equal([...f.timers.values()][0].ms, 30000);
  await f.render({ open: false });
  assert.equal(f.timers.size, 0);
  await f.render({ open: true });
  assert.equal(f.state.data.list[0].id, 'first');
  assert.equal(f.state.initialLoading, false);
  assert.equal(f.list.length, 2);
  assert.equal(f.summary.length, 1); // Recent counts are reused on page/open changes.
  await f.render({ open: false });
  assert.equal(f.list[1].args[1].aborted, true);
  await act(async () => f.list[1].resolve(page('late')));
  assert.equal(f.state.data.list[0].id, 'first');
});

test('query changes abort old requests; late results cannot replace the current page', async t => {
  const f = await fixture(t);
  await f.render({
    query: { kb_id: 'kb-a', filter: 'failed', page: 2, page_size: 10 },
  });
  assert.equal(f.list[0].args[1].aborted, true);
  await act(async () => {
    f.list[1].resolve(page('failed-page'));
    f.summary[1].resolve(counts(1));
  });
  await act(async () => {
    f.list[0].resolve(page('old-page'));
    f.summary[0].resolve(counts(0));
  });
  assert.equal(f.state.data.list[0].id, 'failed-page');
  assert.equal(f.state.counts.running, 1);
  assert.equal(f.timers.size, 1);
  assert.equal([...f.timers.values()][0].ms, 5000);
  await f.tick();
  assert.equal(f.list.length, 3);
  await act(async () => {
    document.dispatchEvent(new window.Event('visibilitychange'));
  });
  assert.equal(f.list.length, 3); // In-flight refresh is not duplicated.
});

test('cache is isolated by user and KB; transient errors keep data, permission errors clear it', async t => {
  const f = await fixture(t);
  await act(async () => {
    f.list[0].resolve(page('alice'));
    f.summary[0].resolve(counts(0));
  });
  await f.render({ identity: 'bob' });
  assert.equal(f.state.data, undefined);
  assert.equal(f.state.counts, undefined);
  await act(async () => {
    f.list[1].resolve(page('bob'));
    f.summary[1].resolve(counts(0));
  });
  await act(async () => f.state.reload());
  await act(async () => {
    f.list[2].reject({ status: 500 });
    f.summary[2].reject({ status: 500 });
  });
  assert.equal(f.state.data.list[0].id, 'bob');
  assert.equal(f.state.loadError, 'refresh');
  await act(async () => f.state.reload());
  await act(async () => {
    f.list[3].reject({ status: 403 });
    f.summary[3].resolve(counts(0));
  });
  assert.equal(f.state.data, undefined);
  assert.equal(f.state.counts, undefined);
  assert.equal(f.state.loadError, 'initial');
  await f.render({
    query: { kb_id: 'kb-b', filter: 'all', page: 1, page_size: 10 },
  });
  assert.equal(f.state.data, undefined);
});

test('hidden tabs pause requests and resume immediately when visible', async t => {
  const f = await fixture(t);
  await act(async () => {
    f.list[0].resolve(page('first', 1));
    f.summary[0].resolve(counts(1));
  });
  document.visibilityState = 'hidden';
  await f.tick();
  assert.equal(f.list.length, 1);
  document.visibilityState = 'visible';
  await act(async () =>
    document.dispatchEvent(new window.Event('visibilitychange')),
  );
  assert.equal(f.list.length, 2);
});

test('refresh and mutation invalidation retain the visible page until new results arrive', async t => {
  const f = await fixture(t);
  await act(async () => {
    f.list[0].resolve(page('visible'));
    f.summary[0].resolve(counts(1));
  });
  const visible = f.state.data;
  await act(async () => f.state.reload());
  assert.equal(f.state.data, visible);
  assert.equal(f.state.initialLoading, false);
  assert.equal(f.state.fetching, true);
  await act(async () => {
    f.list[1].resolve(page('refreshed'));
    f.summary[1].resolve(counts(1));
  });
  const refreshed = f.state.data;
  await act(async () => f.state.invalidate());
  assert.equal(f.state.data, refreshed);
  assert.equal(f.state.initialLoading, false);
  await act(async () => {
    f.list[2].reject({ status: 500 });
    f.summary[2].reject({ status: 500 });
  });
  assert.equal(f.state.data, refreshed);
  assert.equal(f.state.loadError, 'refresh');
});
