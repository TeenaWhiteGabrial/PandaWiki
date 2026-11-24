'use client';

import DocFab from '@/components/docFab';
import ErrorComponent from '@/components/error';
import ScrollToTopFab from '@/components/scrollToTopFab';
import { useBasePath } from '@/hooks/useBasePath';
import { getShareV1NodeDetail } from '@/request/ShareNode';
import type { V1ShareNodeDetailResp } from '@/request/types';
import { getDocContentSx } from '@/utils/getDocContentSx';
import { extractMarkdownToc } from '@/utils/markdownToc';
import useCopy from '@/hooks/useCopy';
import { useStore } from '@/provider';
import { ConstsCopySetting } from '@/request/types';
import { TocList, useTiptap, UseTiptapReturn } from '@ctzhian/tiptap';
import { Box } from '@mui/material';
import { usePathname } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
import DocAnchor from './DocAnchor';
import DocContent from './DocContent';

type DocPageError = Partial<Error> & { code?: number | string };

const DocView = ({ nodeId }: { nodeId: string }) => {
  const { kbDetail, mobile, catalogWidth } = useStore();
  const [loading, setLoading] = useState(true);
  const [editorReady, setEditorReady] = useState(false);
  const [headings, setHeadings] = useState<TocList>([]);
  const [characterCount, setCharacterCount] = useState(0);
  const [currentNode, setCurrentNode] = useState<V1ShareNodeDetailResp>();
  const [currentError, setCurrentError] = useState<DocPageError>();
  const isMarkdown = useMemo(() => {
    return currentNode?.meta?.content_type === 'md';
  }, [currentNode?.meta?.content_type]);
  const baseUrl = useBasePath();
  const isReadonlyFilePreview = useMemo(() => {
    return !!currentNode?.meta?.file_key && !!currentNode?.meta?.file_readonly;
  }, [currentNode?.meta?.file_key, currentNode?.meta?.file_readonly]);
  const shouldUseTiptap = useMemo(() => {
    return !isReadonlyFilePreview && !isMarkdown;
  }, [isMarkdown, isReadonlyFilePreview]);

  useEffect(() => {
    if (!nodeId) return;

    const controller = new AbortController();

    getShareV1NodeDetail(
      { id: nodeId, format: 'json' },
      { cache: 'no-store', signal: controller.signal, isAlert: false },
    )
      .then(res => {
        if (!controller.signal.aborted) {
          const detail = res as V1ShareNodeDetailResp | undefined;
          if (!detail) throw new Error('文档内容为空');
          setCurrentNode(detail);
          setLoading(false);
        }
      })
      .catch(err => {
        if (!controller.signal.aborted) {
          setCurrentError(err as DocPageError);
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, [nodeId]);

  useEffect(() => {
    if (isReadonlyFilePreview || isMarkdown) {
      setLoading(false);
      setEditorReady(false);
      setHeadings(
        isMarkdown ? extractMarkdownToc(currentNode?.content || '') : [],
      );
      setCharacterCount(currentNode?.content?.length ?? 0);
    }
  }, [currentNode?.content, isMarkdown, isReadonlyFilePreview]);
  const tiptapRef = useTiptap({
    content: shouldUseTiptap ? currentNode?.content || '' : '',
    editable: false,
    contentType: 'html',
    immediatelyRender: false,
    baseUrl: baseUrl,
    onTocUpdate: (toc: TocList) => {
      setHeadings(toc);
    },
    onBeforeCreate: () => {
      setLoading(true);
      setEditorReady(false);
    },
    onCreate: ({ editor }) => {
      setLoading(false);
      setEditorReady(true);
      setCharacterCount((editor.storage as any).characterCount.characters());
    },
  });
  const editorRef = (
    !shouldUseTiptap
      ? ({ editor: null } as unknown as UseTiptapReturn)
      : tiptapRef
  ) as UseTiptapReturn;

  const docWidth = useMemo(() => {
    return kbDetail?.settings?.theme_and_style?.doc_width || 'full';
  }, [kbDetail]);

  useCopy({
    mode:
      kbDetail?.settings?.copy_setting !== ConstsCopySetting.CopySettingDisabled
        ? 'allow'
        : 'disable',
    blockContextMenuWhenDisabled: false,
    suffix:
      kbDetail?.settings?.copy_setting === ConstsCopySetting.CopySettingAppend
        ? `\n\n-----------------------------------------\n内容来自 ${typeof window !== 'undefined' ? window.location.href : ''}`
        : '',
  });

  useEffect(() => {
    if (!shouldUseTiptap || !currentNode || !editorRef.editor) return;

    setEditorReady(false);
    setHeadings([]);
    const frame = requestAnimationFrame(() => {
      editorRef.setContent(currentNode.content || '', 'html');
      setEditorReady(true);
    });
    return () => cancelAnimationFrame(frame);
  }, [currentNode, editorRef.editor, shouldUseTiptap]);

  return (
    <>
      {currentError ? (
        <Box
          sx={{
            height: '100%',
            ...getDocContentSx({
              docWidth,
              mobile: mobile ?? false,
              catalogWidth: catalogWidth ?? 260,
              variant: 'error',
            }),
          }}
        >
          <ErrorComponent error={currentError} />
        </Box>
      ) : (
        <>
          <DocContent
            info={currentNode}
            docWidth={docWidth}
            editorRef={editorRef}
            editorReady={editorReady}
            characterCount={characterCount}
            loading={loading && !isReadonlyFilePreview}
            headings={headings}
          />
          {!mobile && !isReadonlyFilePreview && headings.length > 0 && (
            <DocAnchor headings={headings} />
          )}
          <DocFab />
          {!mobile && <ScrollToTopFab />}
        </>
      )}
    </>
  );
};

const Doc = ({ nodeId }: { nodeId: string }) => {
  const { authInfo, authRevision, catalogRevision } = useStore();
  const pathname = usePathname();

  useEffect(() => {
    document.querySelector('#scroll-container')?.scrollTo({ top: 0 });
  }, [pathname]);

  // Unmount the old content, editor and preview as soon as the catalog changes.
  // Only the fresh detail request can render content for this revision.
  const scope = JSON.stringify([
    nodeId,
    authInfo?.id,
    authRevision,
    catalogRevision,
  ]);
  return <DocView key={scope} nodeId={nodeId} />;
};

export default Doc;
