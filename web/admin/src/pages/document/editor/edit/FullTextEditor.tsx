import { DocWidth } from '@/constant/enums';
import { Editor, UseTiptapReturn } from '@ctzhian/tiptap';
import { Box } from '@mui/material';
import { useEditorContext } from '..';

interface FullTextEditorProps {
  editor: UseTiptapReturn['editor'];
  header: React.ReactNode;
  fixed: boolean;
  embedded?: boolean;
}

const FullTextEditor = ({
  editor,
  header,
  fixed,
  embedded = false,
}: FullTextEditorProps) => {
  const { catalogOpen, docWidth } = useEditorContext()!;

  return (
    <Box
      sx={{
        width: embedded
          ? '100%'
          : docWidth === 'full'
            ? `calc(100vw - 160px - ${catalogOpen ? 292 : 0}px - ${fixed ? 292 : 0}px)`
            : DocWidth[docWidth as keyof typeof DocWidth].value,
        maxWidth: '100%',
        p: embedded ? '32px 40px 100px' : '72px 80px 150px',
        mt: embedded ? '72px' : '102px',
        mx: 'auto',
      }}
    >
      {header}
      <Box
        sx={{
          wordBreak: 'break-all',
          '.tiptap.ProseMirror': {
            overflowX: 'hidden',
            minHeight: embedded
              ? 'calc(100vh - 72px - 48px)'
              : 'calc(100vh - 102px - 48px)',
          },
          '.tableWrapper': {
            maxWidth: embedded
              ? '100%'
              : `calc(100vw - 160px - ${catalogOpen ? 292 : 0}px - ${fixed ? 292 : 0}px)`,
            overflowX: 'auto',
          },
        }}
      >
        <Editor editor={editor} />
      </Box>
    </Box>
  );
};

export default FullTextEditor;
