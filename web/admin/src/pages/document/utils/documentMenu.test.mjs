import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';
const compile = text =>
  ts.transpileModule(text, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
const importCode = code =>
  import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`);
const { organizeDocumentMenu } = await importCode(
  compile(
    await readFile(new URL('./documentMenu.ts', import.meta.url), 'utf8'),
  ),
);
const hookSource = compile(
  await readFile(
    new URL('../layout/DocPageList/useDocTreeMenu.tsx', import.meta.url),
    'utf8',
  ),
)
  .replace(
    /import \{ organizeDocumentMenu \} from [^;]+;/,
    'const organizeDocumentMenu = globalThis.__menuOrganizer;',
  )
  .replace(
    /import \{ ConstsCrawlerSource \} from [^;]+;/,
    'const ConstsCrawlerSource = { CrawlerSourceFile: "file" };',
  )
  .replace(
    /import \{ useCallback \} from [^;]+;/,
    'const useCallback = fn => fn;',
  );
globalThis.__menuOrganizer = organizeDocumentMenu;
const { useDocTreeMenu } = await importCode(hookSource);
delete globalThis.__menuOrganizer;
const calls = [];
const handlers = Object.fromEntries(
  [
    'Url',
    'BatchImport',
    'FolderDownload',
    'DownloadFile',
    'FileStorageInfo',
    'Move',
    'Delete',
    'Publish',
    'Restudy',
    'Properties',
    'FrontDoc',
    'Edit',
    'View',
  ].map(name => ['handle' + name, (...args) => calls.push([name, ...args])]),
);
const menu = (changes = {}, readOnly = false) =>
  useDocTreeMenu(
    handlers,
    readOnly,
  )({
    item: {
      id: 'doc',
      type: 2,
      name: 'test',
      level: 0,
      status: 0,
      can_manage: true,
      ...changes,
    },
    createItem() {},
    renameItem() {},
    removeItem() {},
    isEditing: false,
    openFolder() {},
  });

test('document menus use agreed grouping, preserve handlers and nest technical actions', () => {
  const items = menu({
    status: 1,
    publisher_id: 'publisher',
    file_key: 'original.pdf',
  });
  assert.deepEqual(
    items.map(x => x.key),
    [
      'view-document',
      'edit-document',
      'front_doc',
      'download-original',
      'update_publish',
      'rename',
      'move',
      'properties',
      'advanced',
      'delete',
    ],
  );
  assert.deepEqual(
    items.filter(x => x.dividerBefore).map(x => x.key),
    ['update_publish', 'properties', 'delete'],
  );
  const advanced = items.find(x => x.key === 'advanced');
  assert.deepEqual(
    advanced.children.map(x => x.key),
    ['storage-info', 'restudy'],
  );
  assert.equal(advanced.children[0].label, '查看存储信息');
  calls.length = 0;
  items.find(x => x.key === 'download-original').onClick();
  assert.equal(calls[0][0], 'DownloadFile');
  assert.equal(calls[0][1].file_key, 'original.pdf');
});

test('unpublished view explains disabled state; previously published drafts remain accessible', () => {
  const first = menu().find(x => x.key === 'front_doc');
  assert.equal(first.disabled, true);
  assert.equal(first.disabledReason, '发布后可查看');
  assert.equal(
    menu({ status: 1, publisher_id: 'publisher' }).find(
      x => x.key === 'front_doc',
    ).disabled,
    false,
  );
  assert.equal(
    menu({ status: 2 }).find(x => x.key === 'front_doc').disabled,
    false,
  );
});

test('uploaded readonly files do not offer editing; a file key controls original downloads', () => {
  assert.ok(
    menu({ file_readonly: true, file_key: 'file.pdf' }).some(
      x => x.key === 'download-original',
    ),
  );
  assert.ok(
    !menu({ file_readonly: true, file_key: 'file.pdf' }).some(
      x => x.key === 'edit-document',
    ),
  );
  assert.ok(!menu().some(x => x.key === 'download-original'));
  assert.ok(!menu().some(x => x.key === 'advanced'));
});

test('folder ordering and capability filtering do not leave leading separators', () => {
  const items = menu({
    type: 1,
    can_create_document: false,
    can_create_folder: false,
  });
  assert.deepEqual(
    items.map(x => x.key),
    [
      'open-folder',
      'download-folder',
      'rename',
      'move',
      'properties',
      'delete',
    ],
  );
  assert.equal(items[0].dividerBefore, false);
  assert.equal(
    items.find(x => x.key === 'download-folder').label,
    '打包下载（ZIP）',
  );
  assert.deepEqual(
    menu({ type: 1 })
      .slice(0, 5)
      .map(x => x.label),
    ['打开', '新建文件夹', '新建文档', '导入文档', '批量导入 ZIP'],
  );
});

test('unauthorized and placeholder nodes retain empty action menus', () => {
  assert.deepEqual(menu({}, true), []);
  assert.deepEqual(menu({ can_manage: false }), []);
  assert.deepEqual(menu({ placeholder: 'loading' }), []);
});

test('column-specific commands retain their order and no folder rename/move leaks in', () => {
  const items = organizeDocumentMenu(
    [
      { key: 'delete', label: '删除栏目', color: 'error' },
      { key: 'permission', label: '栏目权限' },
      { key: 'edit', label: '修改栏目' },
      { key: 'folder', label: '创建文件夹' },
      { key: 'download-folder', label: '下载' },
    ],
    'nav',
  );
  assert.deepEqual(
    items.map(x => x.label),
    ['新建文件夹', '打包下载（ZIP）', '修改栏目', '栏目权限', '删除栏目'],
  );
  assert.ok(items.at(-1).dividerBefore);
});
