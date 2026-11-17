import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import { test } from 'node:test';
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import ts from 'typescript';
const require = createRequire(import.meta.url);
const reactURL = pathToFileURL(require.resolve('react')).href;
const runtimeURL = pathToFileURL(require.resolve('react/jsx-runtime')).href;
const packages = new URL('../../../../../node_modules/.pnpm/', import.meta.url);
const happy = (await readdir(packages)).find(name =>
  name.startsWith('happy-dom@'),
);
const { Window } = await import(
  new URL(`${happy}/node_modules/happy-dom/lib/index.js`, packages)
);
const dataURL = source =>
  `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const ui = dataURL(`import React from '${reactURL}';
const native = tag => ({children,onClick,disabled,role,...p}) => React.createElement(tag,{onClick,disabled,role,...Object.fromEntries(Object.entries(p).filter(([k])=>k.startsWith('aria-')))},children);
export const Box=native('div'),Stack=native('div'),Typography=native('span'),Button=native('button'),Table=native('table'),TableBody=native('tbody'),TableHead=native('thead'),TableRow=native('tr'),TableCell=native('td'),TableContainer=native('div'),Alert=native('div');
export const Tooltip=({children})=>children;
export const CircularProgress=()=>React.createElement('span',null,'加载中');
export const Checkbox=({onChange,checked,disabled,...p})=>React.createElement('input',{type:'checkbox',checked,disabled,'aria-label':p['aria-label'],onChange:e=>onChange(e,e.target.checked)});
export const Select=({options,value,onChange,mode,...p})=>React.createElement('select',{'aria-label':p['aria-label'],multiple:mode==='multiple',value,disabled:p.disabled,onChange:e=>onChange(mode==='multiple'?[...e.target.selectedOptions].map(o=>o.value):e.target.value)},options.map(o=>React.createElement('option',{key:o.value,value:o.value},o.label)));
export const Modal=({open,children,onOk,onCancel,okText,okButtonProps})=>open?React.createElement('section',null,children,React.createElement('button',{'aria-label':'弹窗确认',disabled:okButtonProps?.disabled,onClick:onOk},okText),React.createElement('button',{'aria-label':'关闭弹窗',onClick:onCancel},'关闭弹窗')):null;
export const message={success(){}};
`);
const requests = dataURL(`
export const getApiV1NodePermission=async()=>globalThis.__readingModal.detail;
export const getApiV1NavPermission=getApiV1NodePermission;
export const getNodePermissionOptions=async()=>globalThis.__readingModal.options;
export const patchApiV1NodePermissionEdit=async p=>{globalThis.__readingModal.saved.push(p)};
export const patchApiV1NavPermission=patchApiV1NodePermissionEdit;
export const previewReadingAudience=async p=>({permissions:p.permissions?.inherit?globalThis.__readingModal.parent:p.permissions||globalThis.__readingModal.detail.effective_permissions,total:2,users:[{id:'a',account:'alice'}]});
`);
const store = dataURL(
  `export const useAppSelector=fn=>fn({config:{user:{permission_levels_disabled:false}}});`,
);
const dept = dataURL(`export default ()=>null;`);
const enterprise = dataURL(`export const buildDepartmentTree=value=>value;`);
const management = dataURL(
  `import React from '${reactURL}';export default ({open,onBack,onClose})=>open?React.createElement('section',null,React.createElement('button',{onClick:onBack},'返回查阅'),React.createElement('button',{onClick:onClose},'管理关闭')):null;`,
);
async function compile(file, extra = {}) {
  const js = ts.transpileModule(
    await readFile(new URL(file, import.meta.url), 'utf8'),
    {
      fileName: file,
      compilerOptions: {
        module: ts.ModuleKind.ESNext,
        target: ts.ScriptTarget.ES2022,
        jsx: ts.JsxEmit.ReactJSX,
      },
    },
  ).outputText;
  const urls = {
    react: reactURL,
    'react/jsx-runtime': runtimeURL,
    '@mui/material': ui,
    antd: ui,
    '@/components/ui': ui,
    '@/store': store,
    '@/utils/enterprise': enterprise,
    '@/components/DepartmentTreeSelect': dept,
    ...extra,
  };
  return dataURL(
    js.replace(
      /from\s+["']([^"']+)["']/g,
      (all, name) =>
        `from '${urls[name] || (name.startsWith('@/request/') ? requests : name)}'`,
    ),
  );
}
const rules = await compile('../utils/readingRules.ts');
const editor = await compile('./ReadingRuleEditor.tsx', {
  '../utils/readingRules': rules,
});
const layout = await compile('./PermissionDialogLayout.tsx');
const { default: ReadingRulesModal } = await import(
  await compile('./ReadingRulesModal.tsx', {
    '../utils/readingRules': rules,
    './ReadingRuleEditor': editor,
    './PermissionDialogLayout': layout,
    './NodeManagementModal': management,
  })
);
async function fixture(t, { inherit = false, readonly = false } = {}) {
  const window = new Window();
  globalThis.window = window;
  globalThis.document = window.document;
  globalThis.Element = window.Element;
  globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  const parent = {
    reading_version: 2,
    reading_rules: [
      {
        permissions: { scope_type: 'user', allowed_user_ids: ['a'] },
        include_descendants: false,
      },
    ],
    denied_user_ids: ['b'],
  };
  const local = {
    reading_version: 2,
    inherit: false,
    reading_rules: [
      {
        permissions: { scope_type: 'public', allow_all_users: true },
        include_descendants: true,
      },
    ],
  };
  const state = (globalThis.__readingModal = {
    detail: {
      permissions: inherit ? { reading_version: 2, inherit: true } : local,
      effective_permissions: inherit ? parent : local,
    },
    parent,
    saved: [],
    options: {
      users: [
        { id: 'a', account: 'alice', name: '甲' },
        { id: 'b', account: 'bob', name: '乙' },
      ],
      departments: [],
      levels: [],
    },
    closes: 0,
  });
  const element = document.createElement('div');
  document.body.append(element);
  const root = createRoot(element);
  await act(async () =>
    root.render(
      React.createElement(ReadingRulesModal, {
        open: true,
        kbID: 'kb',
        ids: ['folder'],
        names: ['研发资料'],
        kind: '目录',
        canConfigure: !readonly,
        onClose() {
          state.closes++;
        },
        onSuccess() {},
      }),
    ),
  );
  await act(async () => {
    await new Promise(resolve => setTimeout(resolve, 400));
  });
  t.after(async () => {
    await act(async () => root.unmount());
    window.happyDOM.abort();
  });
  return {
    state,
    element,
    window,
    button: text =>
      [...element.querySelectorAll('button')].find(
        button => button.textContent === text,
      ),
    async click(element) {
      assert.ok(element, 'missing control');
      await act(async () => element.click());
    },
    async select(label, value) {
      const select = element.querySelector(`select[aria-label="${label}"]`);
      assert.ok(select);
      select.value = value;
      await act(async () =>
        select.dispatchEvent(new window.Event('change', { bubbles: true })),
      );
    },
  };
}

test('compact rows edit locally; cancel discards, confirm and save retain the exact rules', async t => {
  const f = await fixture(t);
  await f.click(f.button('添加规则'));
  assert.equal(
    f.element.querySelector('[aria-label="弹窗确认"]').disabled,
    true,
  );
  await f.click(f.button('取消'));
  assert.equal(f.state.saved.length, 0);
  await f.click(f.button('添加规则'));
  await f.select('规则授权方式', 'users');
  await f.select('规则用户', 'b');
  await f.click(f.button('确认规则'));
  await f.click(f.element.querySelector('[aria-label="弹窗确认"]'));
  assert.equal(f.state.saved.length, 1);
  const rules = f.state.saved[0].permissions.reading_rules;
  assert.equal(rules.length, 2);
  assert.equal(rules[0].permissions.allow_all_users, true);
  assert.deepEqual(rules[1].permissions.allowed_user_ids, ['b']);
  assert.equal(rules[1].include_descendants, true);
});
test('inherit label does not toggle, checkbox copies audience, exclusions and exact directory scope', async t => {
  const f = await fixture(t, { inherit: true });
  const checkbox = f.element.querySelector('input[aria-label="使用上级设置"]');
  await f.click(
    [...f.element.querySelectorAll('span')].find(
      span => span.textContent === '使用上级设置',
    ),
  );
  assert.equal(checkbox.checked, true);
  await f.click(checkbox);
  assert.equal(checkbox.checked, false);
  await f.click(f.element.querySelector('[aria-label="弹窗确认"]'));
  const p = f.state.saved[0].permissions;
  assert.equal(p.inherit, false);
  assert.deepEqual(p.denied_user_ids, ['b']);
  assert.deepEqual(p.reading_rules[0].permissions.allowed_user_ids, ['a']);
  assert.equal(p.reading_rules[0].include_descendants, false);
});
test('management close closes the permission dialog; readonly footer also closes without writing', async t => {
  const f = await fixture(t);
  await f.click(f.button('管理权限'));
  await f.click(f.button('管理关闭'));
  assert.equal(f.state.closes, 1);
  assert.equal(f.state.saved.length, 0);
});
test('readonly close never writes permission settings', async t => {
  const f = await fixture(t, { readonly: true });
  await f.click(f.element.querySelector('[aria-label="弹窗确认"]'));
  assert.equal(f.state.closes, 1);
  assert.equal(f.state.saved.length, 0);
});
