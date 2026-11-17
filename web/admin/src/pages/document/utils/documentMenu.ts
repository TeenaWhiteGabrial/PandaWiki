import type { TreeMenuItem } from '@/components/Drag/DragTree/TreeMenu';

// Keep grouping independent of the renderer: tree, explorer and future context
// menus must share order and actions. Empty groups never create stray dividers.
export function organizeDocumentMenu(
  actions: TreeMenuItem[],
  kind: 'document' | 'folder' | 'nav',
): TreeMenuItem[] {
  const names: Record<string, string> = {
    folder: '新建文件夹',
    doc: '新建文档',
    rich_text: '富文本',
    md: 'Markdown',
    'download-folder': '打包下载（ZIP）',
    'storage-info': '查看存储信息',
  };
  const items = new Map<string, TreeMenuItem>(
    actions.map(action => [
      action.key,
      {
        ...action,
        dividerBefore: false,
        label: names[action.key] || action.label,
        children: action.children?.map(child => ({
          ...child,
          label: names[child.key] || child.label,
        })),
      },
    ]),
  );
  const advanced = ['storage-info', 'restudy'].flatMap(key =>
    items.has(key) ? [items.get(key)!] : [],
  );
  if (advanced.length)
    items.set('advanced', {
      key: 'advanced',
      label: '高级操作',
      children: advanced,
    });
  const groups =
    kind === 'document'
      ? [
          ['view-document', 'edit-document', 'front_doc', 'download-original'],
          ['update_publish', 'rename', 'move'],
          ['properties', 'advanced'],
          ['delete'],
        ]
      : kind === 'nav'
        ? [
            ['folder', 'doc', 'file', 'batch-import-zip'],
            ['download-folder'],
            ['edit', 'permission'],
            ['delete'],
          ]
        : [
            ['open-folder'],
            ['folder', 'doc', 'file', 'batch-import-zip'],
            ['download-folder', 'rename', 'move'],
            ['properties'],
            ['delete'],
          ];
  const result: TreeMenuItem[] = [];
  for (const keys of groups) {
    const group = keys.flatMap(key =>
      items.has(key) ? [items.get(key)!] : [],
    );
    if (!group.length) continue;
    group[0] = { ...group[0], dividerBefore: result.length > 0 };
    result.push(...group);
  }
  return result;
}
