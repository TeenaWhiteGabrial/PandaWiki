export type MarkdownBlockItem = {
  id: string;
  order: number;
  content: string;
  content_hash: string;
  text_length: number;
};

const TARGET_BLOCK_SIZE = 6000;
const MAX_BLOCK_SIZE = 10000;

export const hashMarkdownBlock = (value: string) => {
  let hash = 2166136261;
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(36);
};

const isPreferredBreakLine = (line: string) =>
  line.trim() === '' || /^#{1,6}\s+/.test(line);

export const buildMarkdownBlocks = (content: string): MarkdownBlockItem[] => {
  if (!content) {
    return [
      {
        id: 'mdb_0',
        order: 0,
        content: '',
        content_hash: hashMarkdownBlock(''),
        text_length: 0,
      },
    ];
  }

  const lines = content.match(/[^\n]*(?:\n|$)/g)?.filter(Boolean) || [content];
  const rawBlocks: string[] = [];
  let current = '';

  for (const line of lines) {
    const next = current + line;
    if (
      current &&
      next.length >= TARGET_BLOCK_SIZE &&
      (isPreferredBreakLine(line) || next.length >= MAX_BLOCK_SIZE)
    ) {
      rawBlocks.push(current);
      current = line;
      continue;
    }
    current = next;
  }
  if (current || rawBlocks.length === 0) {
    rawBlocks.push(current);
  }

  return rawBlocks.map((block, order) => ({
    id: `mdb_${order}`,
    order,
    content: block,
    content_hash: hashMarkdownBlock(block),
    text_length: block.length,
  }));
};

export const joinMarkdownBlocks = (blocks: MarkdownBlockItem[]) =>
  [...blocks]
    .sort((a, b) => a.order - b.order)
    .map(block => block.content)
    .join('');
