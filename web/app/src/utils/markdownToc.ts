import type { TocList } from '@ctzhian/tiptap';

const normalizeHeadingText = (text: string) =>
  text
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .replace(/~~([^~]+)~~/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, '$1')
    .replace(/<[^>]+>/g, '')
    .trim();

const slugifyHeading = (text: string) =>
  text
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '') || 'section';

export const extractMarkdownToc = (content: string): TocList => {
  const toc: TocList = [];
  const slugCount = new Map<string, number>();
  const lines = content.split(/\r?\n/);

  for (const line of lines) {
    const match = line.match(/^(#{1,6})\s+(.+?)\s*#*\s*$/);
    if (!match) {
      continue;
    }

    const level = match[1].length;
    const textContent = normalizeHeadingText(match[2]);
    if (!textContent) {
      continue;
    }

    const baseSlug = slugifyHeading(textContent);
    const seen = slugCount.get(baseSlug) || 0;
    slugCount.set(baseSlug, seen + 1);
    const id = seen === 0 ? baseSlug : `${baseSlug}-${seen}`;

    toc.push({
      id,
      textContent,
      level,
      originalLevel: level,
      itemIndex: toc.length,
      pos: toc.length,
      isActive: false,
      isScrolledOver: false,
    });
  }

  return toc;
};
