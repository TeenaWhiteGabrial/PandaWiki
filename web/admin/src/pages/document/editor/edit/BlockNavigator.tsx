import { Box, Stack } from '@mui/material';

export interface NavigableBlockItem {
  id: string;
  type: string;
  order: number;
  text_length: number;
}

interface BlockNavigatorProps {
  blocks: NavigableBlockItem[];
  activeBlockId?: string;
  onSelect: (blockId: string) => void;
}

const typeLabel: Record<string, string> = {
  paragraph: '段落',
  heading: '标题',
  blockquote: '引用',
  codeBlock: '代码',
  bulletList: '列表',
  orderedList: '有序列表',
  listItem: '列表项',
  table: '表格',
  image: '图片',
};

const BlockNavigator = ({
  activeBlockId,
  blocks,
  onSelect,
}: BlockNavigatorProps) => {
  if (blocks.length === 0) return null;

  return (
    <Stack
      sx={{
        position: 'fixed',
        top: 168,
        right: 304,
        width: 180,
        maxHeight: 'calc(100vh - 220px)',
        overflowY: 'auto',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: 'background.paper',
        boxShadow: '0 8px 24px rgba(0,0,0,0.08)',
        p: 1,
        zIndex: 3,
      }}
      gap={0.5}
    >
      <Box sx={{ fontSize: 12, fontWeight: 700, color: 'text.secondary' }}>
        长文档块
      </Box>
      {blocks.slice(0, 200).map(block => {
        const active = block.id === activeBlockId;
        return (
          <Stack
            key={block.id}
            onClick={() => onSelect(block.id)}
            sx={{
              cursor: 'pointer',
              borderRadius: 1,
              px: 1,
              py: 0.75,
              bgcolor: active ? 'action.selected' : 'transparent',
              ':hover': { bgcolor: 'action.hover' },
            }}
          >
            <Box
              sx={{
                fontSize: 12,
                color: active ? 'primary.main' : 'text.primary',
              }}
            >
              #{block.order + 1} {typeLabel[block.type] || block.type}
            </Box>
            <Box sx={{ fontSize: 11, color: 'text.disabled' }}>
              {block.text_length} 字
            </Box>
          </Stack>
        );
      })}
    </Stack>
  );
};

export default BlockNavigator;
