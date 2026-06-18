import { uploadFile } from '@/api';
import Emoji from '@/components/Emoji';
import { postApiV1CreationTabComplete, putApiV1NodeDetail } from '@/request';
import { V1NodeDetailResp } from '@/request/types';
import { completeIncompleteLinks } from '@/utils';
import {
  EditorMarkdown,
  MarkdownEditorRef,
  TocList,
  useTiptap,
  UseTiptapReturn,
} from '@ctzhian/tiptap';
import { message } from '@ctzhian/ui';
import { Box, Stack, TextField, Tooltip } from '@mui/material';
import {
  IconAShijian2,
  IconDJzhinengzhaiyao,
  IconTianjiawendang,
  IconZiti,
} from '@panda-wiki/icons';
import IconPageview1 from '@panda-wiki/icons/IconPageview1';
import dayjs from 'dayjs';
import { debounce } from 'lodash-es';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  useLocation,
  useNavigate,
  useOutletContext,
  useParams,
} from 'react-router-dom';
import { WrapContext } from '..';
import AIGenerate from './AIGenerate';
import BlockNavigator from './BlockNavigator';
import FullTextEditor from './FullTextEditor';
import Header from './Header';
import Summary from './Summary';
import Toc from './Toc';
import Toolbar from './Toolbar';
import {
  buildEditorBlocks,
  getBlockIdByDocumentPosition,
  getDocumentPositionByBlockId,
} from './editorBlocks';
import { useBlockCollabScheduler } from './useBlockCollabScheduler';
import { useMarkdownBlockCollab } from './useMarkdownBlockCollab';
import { useMarkdownCollab } from './useMarkdownCollab';
import { useProseMirrorCollabExtensions } from './useProseMirrorCollabExtensions';

interface WrapProps {
  detail: V1NodeDetailResp & {
    collab?: {
      enabled?: boolean;
      mode?: string;
      disabled_reason?: string;
      room_id?: string;
      endpoint?: string;
      auth_token?: string;
      snapshot?: string;
      strategy?: string;
      doc_room_id?: string;
      block_room_prefix?: string;
      long_document?: boolean;
      content_bytes?: number;
      block_count?: number;
      block_window?: number;
    };
  };
  readOnlyUser?: boolean;
}

const isEditorPerfLogEnabled = () => {
  try {
    return (
      import.meta.env.DEV || localStorage.getItem('panda-editor-perf') === '1'
    );
  } catch {
    return import.meta.env.DEV;
  }
};

const editorPerfLog = (event: string, payload?: Record<string, unknown>) => {
  if (!isEditorPerfLogEnabled()) return;
  console.info(`[PandaWiki Editor Perf] ${event}`, {
    ts: Math.round(performance.now()),
    ...payload,
  });
};

const getEditorCharacterCount = (editor: { storage?: unknown }) => {
  const storage = editor.storage as
    | { characterCount?: { characters?: () => number } }
    | undefined;
  return storage?.characterCount?.characters?.() || 0;
};

const RichTextWrap = ({
  detail: defaultDetail,
  readOnlyUser = false,
}: WrapProps) => {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const state = useLocation().state as { node?: V1NodeDetailResp };
  const {
    catalogOpen,
    setCatalogOpen,
    nodeDetail,
    setNodeDetail,
    onSave,
    catalogData,
    saveCurrentDocRef,
    setCollabStatus,
    setCollabDisabledReason,
  } = useOutletContext<WrapContext>();

  const storageTocOpen = localStorage.getItem('toc-open');
  const postApiV1CreationTabCompleteController = useRef<AbortController | null>(
    null,
  );
  const markdownEditorRef = useRef<MarkdownEditorRef>(null);
  const markdownApplyingLocalRef = useRef(false);
  const collabApplyingRemoteRef = useRef(false);
  const nodeDetailRef = useRef(nodeDetail);
  const editorRefRef = useRef<UseTiptapReturn | null>(null);
  const checkIfEditedRef = useRef<() => void>(() => {});
  const activeBlockUpdateTimerRef = useRef<number | null>(null);
  const mountedAtRef = useRef(performance.now());
  const editorUpdateCountRef = useRef(0);
  const lastEditorUpdateLogRef = useRef(0);
  const updateDetailRef = useRef<(value: V1NodeDetailResp) => void>(() => {});
  const [activeBlockId, setActiveBlockId] = useState<string | undefined>();

  const isMarkdown = useMemo(
    () => defaultDetail.meta?.content_type === 'md',
    [defaultDetail.meta?.content_type],
  );
  const collabDisabledReason =
    defaultDetail.collab?.disabled_reason ||
    (defaultDetail.collab?.long_document
      ? `文档大小 ${defaultDetail.collab.content_bytes || 0}B，块数 ${defaultDetail.collab.block_count || 0}`
      : '');
  const proseMirrorCollab = useProseMirrorCollabExtensions({
    collab: defaultDetail.collab,
    enabled: !readOnlyUser && !isMarkdown,
    initialContent: defaultDetail.editor_content,
  });
  const blockCollabPlan = useBlockCollabScheduler({
    activeBlockId,
    collab: {
      ...defaultDetail.collab,
      enabled: false,
      mode: 'disabled',
    },
    editorBlocks: defaultDetail.editor_blocks,
    editorContent: defaultDetail.editor_content,
  });
  const excludedExtensions = proseMirrorCollab.enabled
    ? ['invisibleCharacters', 'youtube', 'mention', 'undoRedo']
    : ['invisibleCharacters', 'youtube', 'mention', 'emoji'];
  const markdownContent = nodeDetail?.content || defaultDetail.content || '';
  const richTextInitialContent =
    !isMarkdown && defaultDetail.editor_content
      ? defaultDetail.editor_content
      : defaultDetail.content || '';
  const tiptapInitialContent =
    !isMarkdown && proseMirrorCollab.enabled ? '' : richTextInitialContent;

  const [title, setTitle] = useState(nodeDetail?.name || defaultDetail.name);
  const [summary, setSummary] = useState(
    nodeDetail?.meta?.summary || defaultDetail.meta?.summary || '',
  );
  const [characterCount, setCharacterCount] = useState(0);
  const [headings, setHeadings] = useState<TocList>([]);
  const [fixedToc, setFixedToc] = useState(!!storageTocOpen);
  const [selectionText, setSelectionText] = useState('');
  const [aiGenerateOpen, setAiGenerateOpen] = useState(false);
  const [showSummary, setShowSummary] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const initialStateRef = useRef({
    content: defaultDetail.content || '',
    summary: defaultDetail.meta?.summary || '',
    emoji: defaultDetail.meta?.emoji || '',
  });

  useEffect(() => {
    const mountedAt = mountedAtRef.current;
    editorPerfLog('wrap:mount', {
      id: defaultDetail.id,
      contentType: defaultDetail.meta?.content_type,
      contentLength: defaultDetail.content?.length || 0,
      collabEnabled: !!defaultDetail.collab?.enabled,
      collabMode: defaultDetail.collab?.mode,
      collabDisabledReason,
      readOnlyUser,
    });

    return () => {
      editorPerfLog('wrap:unmount', {
        id: defaultDetail.id,
        aliveMs: Math.round(performance.now() - mountedAt),
      });
    };
  }, [
    defaultDetail.collab?.enabled,
    defaultDetail.collab?.mode,
    defaultDetail.content?.length,
    defaultDetail.id,
    defaultDetail.meta?.content_type,
    collabDisabledReason,
    readOnlyUser,
  ]);

  useEffect(() => {
    nodeDetailRef.current = nodeDetail;
  }, [nodeDetail]);

  const updateDetail = useCallback(
    (value: V1NodeDetailResp) => {
      setNodeDetail({
        ...nodeDetailRef.current,
        ...value,
      });
    },
    [setNodeDetail],
  );

  useEffect(() => {
    updateDetailRef.current = updateDetail;
  }, [updateDetail]);

  const handleMarkdownCollabRemoteChange = useCallback((value: string) => {
    if (markdownApplyingLocalRef.current) {
      return;
    }
    updateDetailRef.current({ content: value });
  }, []);

  const markdownCollab = useMarkdownCollab({
    collab: defaultDetail.collab,
    enabled: false,
    initialContent: defaultDetail.content || '',
    onRemoteChange: handleMarkdownCollabRemoteChange,
  });
  const markdownBlockCollab = useMarkdownBlockCollab({
    collab: defaultDetail.collab,
    enabled: false,
    initialContent: defaultDetail.content || '',
    currentContent: markdownContent,
    onRemoteChange: handleMarkdownCollabRemoteChange,
  });

  useEffect(() => {
    if (
      !proseMirrorCollab.enabled &&
      !markdownCollab.enabled &&
      !markdownBlockCollab.enabled
    ) {
      if (isMarkdown) {
        setCollabStatus('disabled');
        setCollabDisabledReason('Markdown 实时协同已暂停');
      }
      return;
    }
    setCollabStatus(
      proseMirrorCollab.enabled
        ? proseMirrorCollab.status
        : markdownBlockCollab.enabled
          ? markdownBlockCollab.status
          : markdownCollab.status,
    );
    setCollabDisabledReason('');
  }, [
    defaultDetail.collab?.disabled_reason,
    defaultDetail.collab?.enabled,
    isMarkdown,
    markdownBlockCollab.enabled,
    markdownBlockCollab.status,
    markdownCollab.enabled,
    markdownCollab.status,
    proseMirrorCollab.enabled,
    proseMirrorCollab.status,
    setCollabDisabledReason,
    setCollabStatus,
  ]);

  useEffect(() => {
    if (!blockCollabPlan.enabled) return;
    editorPerfLog('block-collab:plan', {
      id: defaultDetail.id,
      blockCount: blockCollabPlan.blockCount,
      windowSize: blockCollabPlan.windowSize,
      activeBlockIds: blockCollabPlan.activeBlockIds,
      activeRoomIds: blockCollabPlan.activeRoomIds,
      status: blockCollabPlan.status,
      connectedRoomCount: blockCollabPlan.connectedRoomCount,
    });
  }, [
    blockCollabPlan.activeBlockIds,
    blockCollabPlan.activeRoomIds,
    blockCollabPlan.blockCount,
    blockCollabPlan.connectedRoomCount,
    blockCollabPlan.enabled,
    blockCollabPlan.status,
    blockCollabPlan.windowSize,
    defaultDetail.id,
  ]);

  useEffect(() => {
    if (
      !blockCollabPlan.enabled ||
      blockCollabPlan.mergedContentOrigin !== 'remote' ||
      !blockCollabPlan.mergedContent ||
      isMarkdown ||
      readOnlyUser
    ) {
      return;
    }
    const currentJSON = editorRefRef.current?.getJSON();
    if (
      currentJSON &&
      JSON.stringify(currentJSON) ===
        JSON.stringify(blockCollabPlan.mergedContent)
    ) {
      return;
    }
    collabApplyingRemoteRef.current = true;
    try {
      (
        editorRefRef.current?.setContent as
          | ((content: Record<string, unknown>) => void)
          | undefined
      )?.(blockCollabPlan.mergedContent);
      updateDetailRef.current({
        editor_content: blockCollabPlan.mergedContent,
      });
      editorPerfLog('block-collab:remote-patched-editor', {
        id: defaultDetail.id,
        blockCount: blockCollabPlan.blockCount,
      });
    } finally {
      window.setTimeout(() => {
        collabApplyingRemoteRef.current = false;
      }, 0);
    }
  }, [
    blockCollabPlan.blockCount,
    blockCollabPlan.enabled,
    blockCollabPlan.mergedContent,
    blockCollabPlan.mergedContentOrigin,
    defaultDetail.id,
    isMarkdown,
    readOnlyUser,
  ]);

  const htmlToMarkdown = useCallback((html: string) => {
    const container = document.createElement('div');
    container.innerHTML = html;

    const walk = (node: Node): string => {
      if (node.nodeType === Node.TEXT_NODE) {
        return node.textContent || '';
      }
      if (node.nodeType !== Node.ELEMENT_NODE) {
        return '';
      }
      const el = node as HTMLElement;
      const children = Array.from(el.childNodes).map(walk).join('');
      switch (el.tagName.toLowerCase()) {
        case 'h1':
          return `# ${children.trim()}\n\n`;
        case 'h2':
          return `## ${children.trim()}\n\n`;
        case 'h3':
          return `### ${children.trim()}\n\n`;
        case 'h4':
          return `#### ${children.trim()}\n\n`;
        case 'h5':
          return `##### ${children.trim()}\n\n`;
        case 'h6':
          return `###### ${children.trim()}\n\n`;
        case 'p':
          return `${children.trim()}\n\n`;
        case 'br':
          return '\n';
        case 'strong':
        case 'b':
          return `**${children}**`;
        case 'em':
        case 'i':
          return `*${children}*`;
        case 'code':
          return el.closest('pre') ? children : `\`${children}\``;
        case 'pre':
          return `\`\`\`\n${children.trim()}\n\`\`\`\n\n`;
        case 'blockquote':
          return `${children
            .trim()
            .split('\n')
            .map(line => `> ${line}`)
            .join('\n')}\n\n`;
        case 'a':
          return `[${children}](${el.getAttribute('href') || ''})`;
        case 'img':
          return `![${el.getAttribute('alt') || ''}](${el.getAttribute('src') || ''})`;
        case 'li':
          return `- ${children.trim()}\n`;
        case 'ul':
        case 'ol':
          return `${children}\n`;
        default:
          return children;
      }
    };

    return Array.from(container.childNodes)
      .map(walk)
      .join('')
      .replace(/\n{3,}/g, '\n\n')
      .trim();
  }, []);

  const markdownToHtml = useCallback((markdown: string) => {
    const escapeHtml = (value: string) =>
      value
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');

    const inline = (value: string) =>
      escapeHtml(value)
        .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img alt="$1" src="$2" />')
        .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>')
        .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
        .replace(/\*([^*]+)\*/g, '<em>$1</em>')
        .replace(/`([^`]+)`/g, '<code>$1</code>');

    const lines = markdown.split(/\r?\n/);
    const html: string[] = [];
    let listItems: string[] = [];
    let codeBlock: string[] | null = null;

    const flushList = () => {
      if (listItems.length > 0) {
        html.push(
          `<ul>${listItems.map(item => `<li>${inline(item)}</li>`).join('')}</ul>`,
        );
        listItems = [];
      }
    };

    for (const line of lines) {
      if (line.trim().startsWith('```')) {
        if (codeBlock) {
          html.push(
            `<pre><code>${escapeHtml(codeBlock.join('\n'))}</code></pre>`,
          );
          codeBlock = null;
        } else {
          flushList();
          codeBlock = [];
        }
        continue;
      }
      if (codeBlock) {
        codeBlock.push(line);
        continue;
      }

      const heading = line.match(/^(#{1,6})\s+(.*)$/);
      if (heading) {
        flushList();
        html.push(
          `<h${heading[1].length}>${inline(heading[2])}</h${heading[1].length}>`,
        );
        continue;
      }

      const list = line.match(/^\s*[-*]\s+(.*)$/);
      if (list) {
        listItems.push(list[1]);
        continue;
      }

      if (!line.trim()) {
        flushList();
        continue;
      }

      flushList();
      html.push(`<p>${inline(line)}</p>`);
    }
    flushList();
    if (codeBlock) {
      html.push(`<pre><code>${escapeHtml(codeBlock.join('\n'))}</code></pre>`);
    }
    return html.join('\n');
  }, []);

  const handleUpload = async (
    file: File,
    onProgress?: (progress: { progress: number }) => void,
    abortSignal?: AbortSignal,
  ) => {
    const formData = new FormData();
    formData.append('file', file);
    const { key } = await uploadFile(formData, {
      onUploadProgress: ({ progress }) => {
        onProgress?.({ progress: progress / 100 });
      },
      abortSignal,
    });
    return Promise.resolve('/static-file/' + key);
  };

  const handleTocUpdate = (toc: TocList) => {
    setHeadings(toc);
  };

  const handleError = (error: Error) => {
    if (error.message) {
      message.error(error.message);
    }
  };

  const handleAiWritingGetSuggestion = async ({
    prefix,
    suffix,
  }: {
    prefix: string;
    suffix: string;
  }): Promise<string> => {
    if (postApiV1CreationTabCompleteController.current) {
      postApiV1CreationTabCompleteController.current.abort();
    }
    postApiV1CreationTabCompleteController.current = new AbortController();
    const signal = postApiV1CreationTabCompleteController.current.signal;
    const suggestion = await postApiV1CreationTabComplete(
      {
        prefix: prefix.length > 300 ? prefix.slice(-300) : prefix,
        suffix: suffix.slice(0, 300),
      },
      {
        signal,
      },
    );
    return suggestion || '';
  };

  const editorRef = useTiptap({
    editable: !isMarkdown && !readOnlyUser,
    contentType: isMarkdown ? 'markdown' : 'html',
    immediatelyRender: !isMarkdown,
    content: isMarkdown ? '' : tiptapInitialContent,
    baseUrl: window.__BASENAME__ || '',
    extensions: proseMirrorCollab.extensions,
    exclude: excludedExtensions,
    onCreate: ({ editor: tiptapEditor }: { editor: any }) => {
      const startedAt = performance.now();
      const characterCount = getEditorCharacterCount(tiptapEditor);
      setCharacterCount(characterCount);
      editorPerfLog('tiptap:create', {
        id: defaultDetail.id,
        sinceMountMs: Math.round(performance.now() - mountedAtRef.current),
        durationMs: Math.round(performance.now() - startedAt),
        characterCount,
        contentLength: defaultDetail.content?.length || 0,
      });
      proseMirrorCollab.initializeFromEditor(tiptapEditor);
    },
    onError: handleError,
    onUpload: handleUpload,
    onUpdate: ({ editor }: { editor: any }) => {
      const startedAt = performance.now();
      editorUpdateCountRef.current += 1;
      setCharacterCount(getEditorCharacterCount(editor));
      if (
        defaultDetail.collab?.mode === 'block' &&
        defaultDetail.collab?.enabled &&
        !collabApplyingRemoteRef.current &&
        !activeBlockUpdateTimerRef.current
      ) {
        const selectionFrom = editor.state.selection.from;
        activeBlockUpdateTimerRef.current = window.setTimeout(() => {
          activeBlockUpdateTimerRef.current = null;
          const currentEditorContent =
            editor.getJSON() || defaultDetail.editor_content;
          const nextActiveBlockId = getBlockIdByDocumentPosition(
            currentEditorContent,
            selectionFrom,
          );
          if (nextActiveBlockId) {
            setActiveBlockId(nextActiveBlockId);
            blockCollabPlan.publishBlockContent(
              nextActiveBlockId,
              currentEditorContent,
            );
          }
        }, 200);
      }
      checkIfEditedRef.current();
      const durationMs = performance.now() - startedAt;
      const now = performance.now();
      if (durationMs > 16 || now - lastEditorUpdateLogRef.current > 2000) {
        lastEditorUpdateLogRef.current = now;
        editorPerfLog(
          durationMs > 16 ? 'tiptap:update:slow' : 'tiptap:update',
          {
            id: defaultDetail.id,
            durationMs: Math.round(durationMs),
            count: editorUpdateCountRef.current,
            isMarkdown,
            proseMirrorCollab: proseMirrorCollab.enabled,
          },
        );
      }
    },
    onTocUpdate: handleTocUpdate,
    onAiWritingGetSuggestion: handleAiWritingGetSuggestion,
  });

  useEffect(() => {
    editorRefRef.current = editorRef;
  }, [editorRef]);

  const handleConvertContentType = useCallback(
    async (targetType: 'html' | 'md') => {
      if (readOnlyUser || !defaultDetail.id || !defaultDetail.kb_id) return;

      const currentType = isMarkdown ? 'md' : 'html';
      if (currentType === targetType) return;

      let nextContent = '';
      if (targetType === 'md') {
        nextContent = htmlToMarkdown(
          editorRef.getHTML() || nodeDetail?.content || '',
        );
      } else {
        nextContent = markdownToHtml(markdownContent);
      }

      await putApiV1NodeDetail({
        id: defaultDetail.id,
        kb_id: defaultDetail.kb_id,
        nav_id: defaultDetail.nav_id || '',
        name: nodeDetail?.name || defaultDetail.name || '',
        content: nextContent,
        content_type: targetType,
        editor_format: 'legacy-string',
        editor_version: 1,
      });

      updateDetail({
        content: nextContent,
        meta: {
          ...nodeDetail?.meta,
          content_type: targetType,
        },
      });
      message.success(`已转为${targetType === 'md' ? 'Markdown' : '富文本'}`);
      navigate(`/doc/editor/${defaultDetail.id}?type=${targetType}`, {
        replace: true,
      });
      window.location.reload();
    },
    [
      defaultDetail.id,
      defaultDetail.kb_id,
      defaultDetail.name,
      defaultDetail.nav_id,
      editorRef,
      htmlToMarkdown,
      isMarkdown,
      markdownToHtml,
      markdownContent,
      navigate,
      nodeDetail?.content,
      nodeDetail?.meta,
      nodeDetail?.name,
      readOnlyUser,
      updateDetail,
    ],
  );

  const exportFile = (value: string, type: string) => {
    if (!value) return;
    const completed = completeIncompleteLinks(value);
    let content = completed;
    let mimeType = `text/${type}`;
    if (type === 'html') {
      mimeType = 'text/html;charset=utf-8';
      const safeTitle = (nodeDetail?.name || '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
      content = `<!DOCTYPE html>\n<html>\n<head>\n<meta charset="utf-8" />\n<title>${safeTitle}</title>\n</head>\n<body>\n${completed}\n</body>\n</html>`;
    } else if (type === 'md') {
      mimeType = 'text/markdown;charset=utf-8';
    }
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${nodeDetail?.name}.${type}`;
    a.click();
    URL.revokeObjectURL(url);
    message.success('导出成功');
  };

  const getEditorContent = useCallback(() => {
    if (isMarkdown) {
      return markdownContent;
    }
    return editorRef.getContent() || '';
  }, [editorRef, isMarkdown, markdownContent]);

  const saveContent = useCallback(async () => {
    let content = nodeDetail?.content || '';
    let editorState:
      | {
          editor_blocks?: Record<string, unknown>;
          editor_content?: Record<string, unknown>;
          editor_format?: string;
          editor_version?: number;
        }
      | undefined;
    if (!isMarkdown) {
      content = getEditorContent();
      const editorContent =
        blockCollabPlan.mergedContent || editorRef.getJSON() || undefined;
      updateDetail({ content });
      editorState = {
        editor_blocks: buildEditorBlocks(editorContent),
        editor_content: editorContent,
        editor_format: 'prose_mirror',
        editor_version: 1,
      };
    }
    await onSave(content, editorState);
    initialStateRef.current = {
      content,
      summary,
      emoji: nodeDetail?.meta?.emoji || '',
    };
    setIsEditing(false);
    return content;
  }, [
    blockCollabPlan.mergedContent,
    editorRef,
    getEditorContent,
    isMarkdown,
    nodeDetail?.content,
    nodeDetail?.meta?.emoji,
    onSave,
    summary,
    updateDetail,
  ]);

  const checkIfEdited = useCallback(() => {
    if (editorRef) {
      let value = nodeDetail?.content || '';
      if (!isMarkdown) {
        value = getEditorContent();
      }
      const currentSummary = summary;
      const currentEmoji = nodeDetail?.meta?.emoji || '';
      setIsEditing(
        value !== initialStateRef.current.content ||
          currentSummary !== initialStateRef.current.summary ||
          currentEmoji !== initialStateRef.current.emoji,
      );
    }
  }, [
    editorRef,
    getEditorContent,
    isMarkdown,
    nodeDetail?.content,
    nodeDetail?.meta?.emoji,
    summary,
  ]);

  useEffect(() => {
    checkIfEditedRef.current = checkIfEdited;
  }, [checkIfEdited]);

  const handleAiGenerate = useCallback(() => {
    if (editorRef.editor) {
      const { from, to } = editorRef.editor.state.selection;
      const text = editorRef.editor.state.doc.textBetween(from, to, '\n');
      if (!text) {
        message.error('请先选择文本');
        return;
      }
      setSelectionText(text);
      setAiGenerateOpen(true);
    }
  }, [editorRef.editor]);

  const handleSelectBlock = useCallback(
    (blockId: string) => {
      setActiveBlockId(blockId);
      if (!editorRef.editor || isMarkdown) return;

      const editorContent =
        blockCollabPlan.mergedContent ||
        editorRef.getJSON() ||
        defaultDetail.editor_content;
      const position = getDocumentPositionByBlockId(editorContent, blockId);
      if (!position) return;

      editorRef.editor
        .chain()
        .focus(position)
        .setTextSelection(position)
        .scrollIntoView()
        .run();
    },
    [
      blockCollabPlan.mergedContent,
      defaultDetail.editor_content,
      editorRef,
      isMarkdown,
    ],
  );

  const changeCatalogItem = useCallback(() => {
    if (editorRef && editorRef.editor) {
      let content = nodeDetail?.content || '';
      if (!isMarkdown) {
        content = getEditorContent();
        updateDetail({ content });
      }
      onSave(content);
      initialStateRef.current = {
        content,
        summary,
        emoji: nodeDetail?.meta?.emoji || '',
      };
      setIsEditing(false);
    }
  }, [
    editorRef,
    getEditorContent,
    isMarkdown,
    nodeDetail?.content,
    nodeDetail?.meta?.emoji,
    onSave,
    summary,
    updateDetail,
  ]);

  const handleGlobalKeydown = useCallback(
    (event: KeyboardEvent) => {
      if (
        !readOnlyUser &&
        (event.ctrlKey || event.metaKey) &&
        event.key === 's'
      ) {
        event.preventDefault();
        saveContent();
      }
      if ((event.ctrlKey || event.metaKey) && event.key === 'b') {
        event.preventDefault();
        setCatalogOpen(!catalogOpen);
      }
    },
    [catalogOpen, readOnlyUser, saveContent, setCatalogOpen],
  );

  const renderEditorTitleEmojiSummary = () => (
    <>
      <Stack
        direction={'row'}
        alignItems={'center'}
        gap={1}
        sx={{ mb: 2, position: 'relative' }}
      >
        <Emoji
          type={2}
          sx={{ flexShrink: 0, width: 36, height: 36 }}
          iconSx={{ fontSize: 28 }}
          value={nodeDetail?.meta?.emoji}
          onChange={value => {
            if (readOnlyUser) return;
            putApiV1NodeDetail({
              id: defaultDetail.id!,
              kb_id: defaultDetail.kb_id!,
              nav_id: defaultDetail.nav_id || '',
              emoji: value,
            }).then(() => {
              updateDetail({
                meta: {
                  ...nodeDetail?.meta,
                  emoji: value,
                },
              });
              setTimeout(() => checkIfEdited(), 0);
            });
          }}
        />
        <TextField
          sx={{ flex: 1 }}
          value={title}
          slotProps={{
            input: {
              sx: {
                fontSize: 28,
                fontWeight: 'bold',
                bgcolor: 'background.default',
                '& input': { p: 0, lineHeight: '36px', height: '36px' },
                '& fieldset': { border: 'none !important' },
              },
            },
          }}
          onChange={e => {
            if (readOnlyUser) return;
            setTitle(e.target.value);
            updateDetail({ name: e.target.value });
            debouncedUpdateTitle(e.target.value);
          }}
        />
      </Stack>
      <Stack direction={'row'} alignItems={'center'} gap={2} sx={{ mb: 4 }}>
        {nodeDetail?.editor_account && (
          <Tooltip
            arrow
            title={
              nodeDetail?.creator_account || nodeDetail?.publisher_account ? (
                <Stack>
                  {nodeDetail?.creator_account && (
                    <Box>创建：{nodeDetail?.creator_account}</Box>
                  )}
                  {nodeDetail?.publisher_account && (
                    <Box>上次发布：{nodeDetail?.publisher_account}</Box>
                  )}
                </Stack>
              ) : null
            }
          >
            <Stack
              direction={'row'}
              alignItems={'center'}
              gap={0.5}
              sx={{ cursor: 'pointer', fontSize: 12, color: 'text.tertiary' }}
            >
              <IconTianjiawendang sx={{ fontSize: 9 }} />
              {nodeDetail?.editor_account} 编辑
            </Stack>
          </Tooltip>
        )}
        <Tooltip arrow title='查看历史版本'>
          <Stack
            direction={'row'}
            alignItems={'center'}
            gap={0.5}
            sx={{
              fontSize: 12,
              color: 'text.tertiary',
              cursor: 'pointer',
              ':hover': { color: 'primary.main' },
            }}
            onClick={() => navigate(`/doc/editor/history/${defaultDetail.id}`)}
          >
            <IconAShijian2 sx={{ fontSize: 12 }} />
            {dayjs(defaultDetail.created_at).format('YYYY-MM-DD HH:mm:ss')} 创建
          </Stack>
        </Tooltip>
        <Stack
          direction={'row'}
          alignItems={'center'}
          gap={0.5}
          sx={{ fontSize: 12, color: 'text.tertiary' }}
        >
          <IconZiti sx={{ fontSize: 12 }} />
          {characterCount} 字
        </Stack>
        <Stack
          direction={'row'}
          alignItems={'center'}
          gap={0.5}
          sx={{ fontSize: 12, color: 'text.tertiary' }}
        >
          <IconPageview1 sx={{ fontSize: 12 }} />
          浏览量 {nodeDetail?.pv}
        </Stack>
      </Stack>
      <Box
        sx={{
          mb: 6,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px',
          bgcolor: 'background.paper2',
          p: 2,
          position: 'relative',
          '.ai-generate-summary-left-icon': {
            opacity: '0',
            transition: 'opacity 0.3s ease-in-out',
          },
          ':hover': {
            '.ai-generate-summary-left-icon': { opacity: '1' },
          },
          '.MuiInputBase-root': { p: 0 },
        }}
      >
        <Stack
          className='ai-generate-summary-left-icon'
          direction={'row'}
          alignItems={'center'}
          gap={0.5}
          onClick={() => setShowSummary(true)}
          sx={{
            position: 'absolute',
            top: -18,
            left: 0,
            zIndex: 1,
            lineHeight: '18px',
            cursor: 'pointer',
            fontSize: 12,
            color: 'text.tertiary',
            ':hover': { color: 'text.primary' },
          }}
        >
          <IconDJzhinengzhaiyao sx={{ fontSize: 12 }} />
          文档摘要
        </Stack>
        {nodeDetail?.meta?.summary ? (
          <TextField
            value={summary}
            multiline
            fullWidth
            placeholder='暂无摘要，可在此处输入摘要'
            slotProps={{
              input: {
                sx: {
                  bgcolor: 'background.paper2',
                  fontSize: 14,
                  lineHeight: '28px',
                  letterSpacing: '1px',
                  fontWeight: 'normal',
                  color: 'text.secondary',
                  '& fieldset': { border: 'none !important' },
                },
              },
            }}
            onChange={e => {
              if (readOnlyUser) return;
              setSummary(e.target.value);
              debouncedUpdateSummary(e.target.value);
            }}
          />
        ) : (
          <Box sx={{ fontSize: 12, color: 'text.tertiary' }}>
            暂无摘要，点击
            <Box
              component='span'
              sx={{ color: 'primary.main', cursor: 'pointer' }}
              onClick={() => setShowSummary(true)}
            >
              生成摘要
            </Box>
          </Box>
        )}
      </Box>
    </>
  );

  const debouncedUpdateSummary = useMemo(
    () =>
      debounce((newSummary: string) => {
        if (readOnlyUser) return;
        putApiV1NodeDetail({
          id: defaultDetail.id!,
          kb_id: defaultDetail.kb_id!,
          nav_id: defaultDetail.nav_id || '',
          summary: newSummary,
        }).then(() => {
          updateDetail({
            meta: {
              ...nodeDetail?.meta,
              summary: newSummary,
            },
          });
        });
      }, 500),
    [
      defaultDetail.id,
      defaultDetail.kb_id,
      defaultDetail.nav_id,
      nodeDetail?.meta,
      readOnlyUser,
      updateDetail,
    ],
  );

  const debouncedUpdateTitle = useMemo(
    () =>
      debounce((newTitle: string) => {
        if (readOnlyUser) return;
        putApiV1NodeDetail({
          id: defaultDetail.id!,
          kb_id: defaultDetail.kb_id!,
          nav_id: defaultDetail.nav_id || '',
          name: newTitle,
        });
      }, 500),
    [defaultDetail.id, defaultDetail.kb_id, defaultDetail.nav_id, readOnlyUser],
  );

  useEffect(() => {
    if (readOnlyUser) {
      setCollabStatus('idle');
      setCollabDisabledReason('');
      return;
    }
    if (
      !proseMirrorCollab.enabled &&
      !markdownCollab.enabled &&
      !markdownBlockCollab.enabled &&
      defaultDetail.collab?.mode !== 'block'
    ) {
      setCollabStatus(
        defaultDetail.collab?.mode === 'disabled' ? 'disabled' : 'disconnected',
      );
      setCollabDisabledReason(collabDisabledReason);
      return;
    }
    setCollabDisabledReason('');
  }, [
    collabDisabledReason,
    defaultDetail.collab?.mode,
    markdownBlockCollab.enabled,
    markdownCollab.enabled,
    proseMirrorCollab.enabled,
    readOnlyUser,
    setCollabDisabledReason,
    setCollabStatus,
  ]);

  useEffect(() => {
    setSummary(nodeDetail?.meta?.summary || '');
  }, [nodeDetail]);

  useEffect(() => {
    checkIfEdited();
  }, [checkIfEdited, summary]);

  useEffect(() => {
    setTitle(defaultDetail?.name || '');
    setSummary(defaultDetail?.meta?.summary || '');
    initialStateRef.current = {
      content: defaultDetail.content || '',
      summary: defaultDetail.meta?.summary || '',
      emoji: defaultDetail.meta?.emoji || '',
    };
    setIsEditing(false);
  }, [defaultDetail]);

  useEffect(() => {
    document.addEventListener('keydown', handleGlobalKeydown);
    return () => document.removeEventListener('keydown', handleGlobalKeydown);
  }, [handleGlobalKeydown]);

  useEffect(() => {
    if (state && state.node && editorRef.editor) {
      const newContent = state.node.content || nodeDetail?.content || '';
      const newSummary =
        state.node.meta?.summary || nodeDetail?.meta?.summary || '';
      const newEmoji = state.node.meta?.emoji || nodeDetail?.meta?.emoji || '';
      updateDetail({
        name: state.node.name || nodeDetail?.name || '',
        meta: { summary: newSummary, emoji: newEmoji },
        content: newContent,
      });
      if (!proseMirrorCollab.enabled || isMarkdown) {
        editorRef.setContent(newContent);
      }
      initialStateRef.current = {
        content: newContent,
        summary: newSummary,
        emoji: newEmoji,
      };
      setIsEditing(false);
      navigate(`/doc/editor/${defaultDetail.id}`);
    }
  }, [
    defaultDetail.id,
    editorRef,
    editorRef.editor,
    navigate,
    nodeDetail?.content,
    nodeDetail?.meta?.emoji,
    nodeDetail?.meta?.summary,
    nodeDetail?.name,
    proseMirrorCollab.enabled,
    state,
    updateDetail,
  ]);

  useEffect(() => {
    const handleTabClose = () => {
      if (isEditing) {
        saveContent();
      }
    };
    const handleVisibilityChange = () => {
      if (document.hidden && isEditing) {
        saveContent();
      }
    };
    window.addEventListener('beforeunload', handleTabClose);
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      window.removeEventListener('beforeunload', handleTabClose);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [isEditing, saveContent]);

  useEffect(() => {
    const editor = editorRef.editor;
    return () => {
      if (activeBlockUpdateTimerRef.current) {
        window.clearTimeout(activeBlockUpdateTimerRef.current);
      }
      editor?.destroy();
    };
  }, [editorRef.editor]);

  useEffect(() => {
    saveCurrentDocRef.current = async () => {
      if (editorRef?.editor) {
        await saveContent();
      }
    };
    return () => {
      saveCurrentDocRef.current = null;
    };
  }, [editorRef, saveContent, saveCurrentDocRef]);

  useEffect(() => {
    if (id !== defaultDetail.id) {
      const checkDocExists = (items: typeof catalogData): boolean => {
        for (const item of items) {
          if (item.id === defaultDetail.id) return true;
          if (item.children && checkDocExists(item.children)) return true;
        }
        return false;
      };
      if (checkDocExists(catalogData)) {
        changeCatalogItem();
      }
    }
  }, [id, catalogData, defaultDetail.id, changeCatalogItem]);

  return (
    <>
      <Box
        sx={{
          position: 'fixed',
          top: 0,
          left: catalogOpen ? 292 : 0,
          right: 0,
          zIndex: 10,
          bgcolor: 'background.default',
          transition: 'left 0.3s ease-in-out',
        }}
      >
        <Header
          edit={isEditing}
          detail={nodeDetail!}
          updateDetail={updateDetail}
          readOnlyUser={readOnlyUser}
          handleSave={async () => {
            if (readOnlyUser) return;
            await saveContent();
          }}
          handleExport={type => {
            if (type === 'html') {
              exportFile(editorRef.getHTML() || '', type);
            } else if (type === 'md') {
              exportFile(
                isMarkdown
                  ? nodeDetail?.content || ''
                  : editorRef.getMarkdown() || '',
                type,
              );
            }
          }}
          handleConvertContentType={handleConvertContentType}
        />
        {!readOnlyUser && !isMarkdown && (
          <Toolbar editorRef={editorRef} handleAiGenerate={handleAiGenerate} />
        )}
      </Box>
      <Box
        sx={{ ...(fixedToc && { display: 'flex' }) }}
        onKeyDown={event => {
          if ((event.ctrlKey || event.metaKey) && event.key === 's') return;
          event.stopPropagation();
        }}
      >
        {isMarkdown ? (
          <Box
            sx={{
              mt: '56px',
              px: 10,
              pt: 4,
              pb: 3,
              flex: 1,
            }}
          >
            <Box>{renderEditorTitleEmojiSummary()}</Box>
            {editorRef.editor ? (
              <EditorMarkdown
                ref={markdownEditorRef}
                editor={editorRef.editor}
                value={nodeDetail?.content || ''}
                onUpload={handleUpload}
                placeholder='请输入文档内容'
                onAceChange={value => {
                  if (readOnlyUser) return;
                  markdownApplyingLocalRef.current = true;
                  try {
                    if (markdownCollab.enabled) {
                      markdownCollab.applyLocalChange(value);
                    }
                    updateDetail({
                      content: value,
                    });
                  } finally {
                    window.setTimeout(() => {
                      markdownApplyingLocalRef.current = false;
                    }, 0);
                  }
                }}
                height='calc(100vh - 127px)'
              />
            ) : null}
          </Box>
        ) : (
          <FullTextEditor
            editor={editorRef.editor}
            fixed={fixedToc}
            header={renderEditorTitleEmojiSummary()}
          />
        )}
      </Box>
      <Toc
        headings={headings}
        fixed={fixedToc}
        isMarkdown={isMarkdown}
        setFixed={setFixedToc}
        setShowSummary={setShowSummary}
        scrollToHeading={
          isMarkdown
            ? headingText =>
                markdownEditorRef.current?.scrollToHeading(headingText)
            : undefined
        }
      />
      {blockCollabPlan.enabled && (
        <BlockNavigator
          activeBlockId={activeBlockId}
          blocks={blockCollabPlan.blocks}
          onSelect={handleSelectBlock}
        />
      )}
      <AIGenerate
        open={aiGenerateOpen}
        selectText={selectionText}
        onClose={() => setAiGenerateOpen(false)}
        editorRef={editorRef}
      />
      <Summary
        open={showSummary}
        updateDetail={updateDetail}
        onClose={() => setShowSummary(false)}
      />
    </>
  );
};

const Wrap = ({ detail, readOnlyUser = false }: WrapProps) => {
  return <RichTextWrap detail={detail} readOnlyUser={readOnlyUser} />;
};

export default Wrap;
