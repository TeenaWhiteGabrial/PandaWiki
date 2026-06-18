import MarkDown from '@/components/MarkDown';
import { Box, TextField } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import BlockNavigator from './BlockNavigator';
import {
  buildMarkdownBlocks,
  joinMarkdownBlocks,
  MarkdownBlockItem,
} from './markdownBlocks';

interface MarkdownBlockEditorProps {
  content: string;
  onChange: (value: string) => void;
}

type DisplayMode = 'edit' | 'preview';

type MarkdownNavBlock = MarkdownBlockItem & {
  type: string;
};

const toNavBlock = (block: MarkdownBlockItem): MarkdownNavBlock => {
  const firstLine = block.content.trimStart().split('\n')[0] || '';
  let type = 'paragraph';
  if (/^#{1,6}\s+/.test(firstLine)) type = 'heading';
  else if (/^>\s*/.test(firstLine)) type = 'blockquote';
  else if (/^```/.test(firstLine)) type = 'codeBlock';
  else if (/^\s*[-*]\s+/.test(firstLine)) type = 'bulletList';
  else if (/^\s*\d+\.\s+/.test(firstLine)) type = 'orderedList';
  return { ...block, type };
};

const MarkdownBlockEditor = ({
  content,
  onChange,
}: MarkdownBlockEditorProps) => {
  const [activeBlockId, setActiveBlockId] = useState<string>();
  const [displayMode, setDisplayMode] = useState<DisplayMode>('edit');
  const [blocks, setBlocks] = useState<MarkdownBlockItem[]>(() =>
    buildMarkdownBlocks(content),
  );
  const [lastEmittedContent, setLastEmittedContent] = useState(content);
  const navBlocks = useMemo(() => blocks.map(toNavBlock), [blocks]);

  useEffect(() => {
    if (content === lastEmittedContent) return;
    const nextBlocks = buildMarkdownBlocks(content);
    setBlocks(nextBlocks);
    setLastEmittedContent(content);
    if (
      !activeBlockId ||
      !nextBlocks.some(block => block.id === activeBlockId)
    ) {
      setActiveBlockId(nextBlocks[0]?.id);
    }
  }, [activeBlockId, content, lastEmittedContent]);

  const updateBlock = (blockId: string, nextContent: string) => {
    const nextBlocks = blocks.map(block =>
      block.id === blockId ? { ...block, content: nextContent } : block,
    );
    const nextValue = joinMarkdownBlocks(nextBlocks);
    setBlocks(nextBlocks);
    setLastEmittedContent(nextValue);
    onChange(nextValue);
  };

  const decorateBlock = (blockId: string, prefix: string, suffix = '') => {
    const block = blocks.find(item => item.id === blockId);
    if (!block) return;
    updateBlock(blockId, `${prefix}${block.content}${suffix}`);
  };

  return (
    <>
      <Box
        sx={{
          mb: 2,
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          flexWrap: 'wrap',
        }}
      >
        <Box
          onClick={() => setDisplayMode('edit')}
          sx={{
            px: 1.5,
            py: 0.75,
            borderRadius: 1,
            cursor: 'pointer',
            fontSize: 12,
            bgcolor:
              displayMode === 'edit' ? 'action.selected' : 'background.paper',
          }}
        >
          编辑模式
        </Box>
        <Box
          onClick={() => setDisplayMode('preview')}
          sx={{
            px: 1.5,
            py: 0.75,
            borderRadius: 1,
            cursor: 'pointer',
            fontSize: 12,
            bgcolor:
              displayMode === 'preview'
                ? 'action.selected'
                : 'background.paper',
          }}
        >
          预览模式
        </Box>
      </Box>
      {blocks
        .filter(block => block.id === activeBlockId || !activeBlockId)
        .slice(0, 1)
        .map(block => {
          const active = block.id === activeBlockId;
          return (
            <Box
              key={block.id}
              id={`markdown-block-${block.id}`}
              sx={{
                border: '1px solid',
                borderColor: active ? 'primary.main' : 'divider',
                borderRadius: 2,
                bgcolor: 'background.paper',
                boxShadow: active ? '0 0 0 1px rgba(25,118,210,0.08)' : 'none',
                p: 1.5,
              }}
            >
              <Box
                sx={{
                  mb: 1,
                  fontSize: 12,
                  fontWeight: 700,
                  color: active ? 'primary.main' : 'text.secondary',
                }}
              >
                块 #{block.order + 1} · {block.text_length} 字
              </Box>
              {displayMode === 'edit' && (
                <>
                  <Box
                    sx={{
                      mb: 1,
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1,
                      flexWrap: 'wrap',
                    }}
                  >
                    <Box
                      onClick={() => decorateBlock(block.id, '# ')}
                      sx={{
                        cursor: 'pointer',
                        fontSize: 12,
                        color: 'text.secondary',
                      }}
                    >
                      一级标题
                    </Box>
                    <Box
                      onClick={() => decorateBlock(block.id, '**', '**')}
                      sx={{
                        cursor: 'pointer',
                        fontSize: 12,
                        color: 'text.secondary',
                      }}
                    >
                      加粗
                    </Box>
                    <Box
                      onClick={() => decorateBlock(block.id, '- ')}
                      sx={{
                        cursor: 'pointer',
                        fontSize: 12,
                        color: 'text.secondary',
                      }}
                    >
                      列表
                    </Box>
                    <Box
                      onClick={() => decorateBlock(block.id, '> ')}
                      sx={{
                        cursor: 'pointer',
                        fontSize: 12,
                        color: 'text.secondary',
                      }}
                    >
                      引用
                    </Box>
                  </Box>
                  <TextField
                    multiline
                    fullWidth
                    minRows={Math.max(
                      8,
                      Math.ceil(block.content.split('\n').length),
                    )}
                    value={block.content}
                    onFocus={() => setActiveBlockId(block.id)}
                    onChange={event =>
                      updateBlock(block.id, event.target.value)
                    }
                    slotProps={{
                      input: {
                        sx: {
                          fontFamily:
                            'ui-monospace, SFMono-Regular, Menlo, monospace',
                          lineHeight: 1.7,
                          alignItems: 'flex-start',
                          '& textarea': {
                            lineHeight: 1.7,
                          },
                          '& fieldset': {
                            border: 'none !important',
                          },
                        },
                      },
                    }}
                  />
                </>
              )}
              {displayMode === 'preview' && (
                <Box sx={{ minHeight: 240 }}>
                  <MarkDown content={block.content} />
                </Box>
              )}
            </Box>
          );
        })}
      <BlockNavigator
        activeBlockId={activeBlockId}
        blocks={navBlocks}
        onSelect={blockId => {
          setActiveBlockId(blockId);
          document
            .getElementById(`markdown-block-${blockId}`)
            ?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }}
      />
    </>
  );
};

export default MarkdownBlockEditor;
