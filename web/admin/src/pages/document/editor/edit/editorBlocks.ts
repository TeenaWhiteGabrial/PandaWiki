export type EditorBlockItem = {
  id: string;
  type: string;
  order: number;
  path: number[];
  text_length: number;
  content_hash: string;
  attrs?: Record<string, unknown>;
};

export type EditorBlocks = {
  version: 1;
  source: 'prose_mirror';
  block_count: number;
  text_length: number;
  blocks: EditorBlockItem[];
  generated_at: string;
};

type ProseMirrorNode = {
  type?: string;
  text?: string;
  attrs?: Record<string, unknown>;
  content?: ProseMirrorNode[];
};

const BLOCK_NODE_TYPES = new Set([
  'paragraph',
  'heading',
  'blockquote',
  'codeBlock',
  'bulletList',
  'orderedList',
  'listItem',
  'table',
  'tableRow',
  'tableCell',
  'tableHeader',
  'image',
  'horizontalRule',
]);

const hashText = (value: string) => {
  let hash = 2166136261;
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(36);
};

const normalizeText = (value: string) =>
  value.replace(/\s+/g, ' ').trim().slice(0, 256);

const collectText = (node: ProseMirrorNode): string => {
  if (node.text) return node.text;
  return (node.content || []).map(collectText).join('');
};

const getBlockAttrs = (node: ProseMirrorNode) => {
  if (!node.attrs) return undefined;
  const attrs: Record<string, unknown> = {};
  for (const key of ['level', 'language', 'src', 'alt', 'title']) {
    if (node.attrs[key] !== undefined) {
      attrs[key] = node.attrs[key];
    }
  }
  return Object.keys(attrs).length > 0 ? attrs : undefined;
};

export const buildEditorBlocks = (
  editorContent?: Record<string, unknown>,
): EditorBlocks | undefined => {
  if (!editorContent) return undefined;
  const root = editorContent as ProseMirrorNode;
  const blocks: EditorBlockItem[] = [];
  const seen = new Map<string, number>();

  const visit = (node: ProseMirrorNode, path: number[]) => {
    const type = node.type || 'unknown';
    if (BLOCK_NODE_TYPES.has(type)) {
      const normalizedText = normalizeText(collectText(node));
      const attrs = getBlockAttrs(node);
      const identity = `${type}:${attrs?.level || ''}:${normalizedText || JSON.stringify(attrs || {})}`;
      const contentHash = hashText(identity);
      const occurrence = seen.get(contentHash) || 0;
      seen.set(contentHash, occurrence + 1);
      blocks.push({
        id: `blk_${type}_${contentHash}_${occurrence}`,
        type,
        order: blocks.length,
        path,
        text_length: normalizedText.length,
        content_hash: contentHash,
        ...(attrs ? { attrs } : {}),
      });
    }
    for (const [index, child] of (node.content || []).entries()) {
      visit(child, [...path, index]);
    }
  };

  for (const [index, child] of (root.content || []).entries()) {
    visit(child, [index]);
  }

  return {
    version: 1,
    source: 'prose_mirror',
    block_count: blocks.length,
    text_length: blocks.reduce((sum, block) => sum + block.text_length, 0),
    blocks,
    generated_at: new Date().toISOString(),
  };
};

export const parseEditorBlocks = (
  value?: Record<string, unknown>,
): EditorBlocks | undefined => {
  if (!value || value.version !== 1 || !Array.isArray(value.blocks)) {
    return undefined;
  }
  const blocks = value.blocks
    .map((block, index): EditorBlockItem | undefined => {
      if (!block || typeof block !== 'object') return undefined;
      const item = block as Partial<EditorBlockItem>;
      if (!item.id || !item.type) return undefined;
      return {
        id: item.id,
        type: item.type,
        order: typeof item.order === 'number' ? item.order : index,
        path: Array.isArray(item.path)
          ? item.path.filter(
              (value): value is number => typeof value === 'number',
            )
          : [index],
        text_length:
          typeof item.text_length === 'number' ? item.text_length : 0,
        content_hash: item.content_hash || '',
        ...(item.attrs ? { attrs: item.attrs } : {}),
      };
    })
    .filter((block): block is EditorBlockItem => !!block)
    .sort((a, b) => a.order - b.order);

  return {
    version: 1,
    source: 'prose_mirror',
    block_count:
      typeof value.block_count === 'number' ? value.block_count : blocks.length,
    text_length:
      typeof value.text_length === 'number'
        ? value.text_length
        : blocks.reduce((sum, block) => sum + block.text_length, 0),
    blocks,
    generated_at:
      typeof value.generated_at === 'string' ? value.generated_at : '',
  };
};

export const getBlockRoomId = (blockRoomPrefix: string, blockId: string) =>
  `${blockRoomPrefix}${blockId}`;

const getNodeByPath = (
  editorContent: Record<string, unknown>,
  path: number[],
) => {
  let node: ProseMirrorNode | undefined = editorContent as ProseMirrorNode;
  for (const index of path) {
    node = node?.content?.[index];
    if (!node) return undefined;
  }
  return node;
};

export const getEditorBlockContent = (
  editorContent: Record<string, unknown> | undefined,
  block: EditorBlockItem,
): Record<string, unknown> | undefined => {
  if (!editorContent) return undefined;
  const pathNode = getNodeByPath(editorContent, block.path);
  if (pathNode) return pathNode as Record<string, unknown>;
  const rebuiltBlock = buildEditorBlocks(editorContent)?.blocks.find(
    item => item.id === block.id,
  );
  return rebuiltBlock
    ? (getNodeByPath(editorContent, rebuiltBlock.path) as
        | Record<string, unknown>
        | undefined)
    : undefined;
};

const replaceNodeByPath = (
  node: ProseMirrorNode,
  path: number[],
  replacement: ProseMirrorNode,
): ProseMirrorNode => {
  if (path.length === 0) {
    return replacement;
  }
  const [index, ...rest] = path;
  const content = [...(node.content || [])];
  const child = content[index];
  if (!child) return node;
  content[index] = replaceNodeByPath(child, rest, replacement);
  return {
    ...node,
    content,
  };
};

export const replaceEditorBlockContent = (
  editorContent: Record<string, unknown> | undefined,
  block: EditorBlockItem,
  nextContent: Record<string, unknown>,
): Record<string, unknown> | undefined => {
  if (!editorContent) return undefined;
  return replaceNodeByPath(
    editorContent as ProseMirrorNode,
    block.path,
    nextContent as ProseMirrorNode,
  ) as Record<string, unknown>;
};

export const getBlockIdByDocumentPosition = (
  editorContent: Record<string, unknown> | undefined,
  position: number,
): string | undefined => {
  const blocks = buildEditorBlocks(editorContent)?.blocks;
  if (!blocks?.length) return undefined;

  let currentPos = 1;
  let matched: EditorBlockItem | undefined;
  const visit = (node: ProseMirrorNode, path: number[]) => {
    const start = currentPos;
    currentPos += 1;
    for (const [index, child] of (node.content || []).entries()) {
      visit(child, [...path, index]);
    }
    const end = currentPos;
    currentPos += 1;
    if (position < start || position > end) return;
    const block = blocks.find(
      item =>
        item.path.length === path.length &&
        item.path.every((value, index) => value === path[index]),
    );
    if (block) {
      matched = block;
    }
  };

  for (const [index, child] of (
    (editorContent as ProseMirrorNode | undefined)?.content || []
  ).entries()) {
    visit(child, [index]);
  }

  return matched?.id;
};
