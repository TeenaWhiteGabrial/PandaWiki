'use client';

import CommentInput, {
  CommentInputRef,
  ImageItem,
} from '@/components/commentInput';
import { DocWidth } from '@/constant';
import { useBasePath } from '@/hooks';
import { getDocContentSx } from '@/utils/getDocContentSx';
import { useStore } from '@/provider';
import {
  getShareV1CommentList,
  postShareV1Comment,
} from '@/request/ShareComment';
import { ConstsCopySetting, V1ShareNodeDetailResp } from '@/request/types';
import { copyText, findAdjacentDocuments } from '@/utils';
import { UseTiptapReturn } from '@ctzhian/tiptap';
import type { TocList } from '@ctzhian/tiptap';
import { message } from '@ctzhian/ui';
import { Alert, Box, alpha } from '@mui/material';
import { EditorContent } from '@tiptap/react';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import relativeTime from 'dayjs/plugin/relativeTime';
import { useParams } from 'next/navigation';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import MarkDown from '@/components/markdown';
import DocSkeleton from '@/components/docSkeleton';
import FolderList from './folderList';
import AdjacentDocNav from './components/AdjacentDocNav';
import DocMetaInfo from './components/DocMetaInfo';
import CommentSection from './components/CommentSection';
import AttachmentList from './components/AttachmentList';
import FilePreview from './components/FilePreview';

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

const DocContent = ({
  info,
  docWidth,
  editorRef,
  editorReady,
  commentList: propsCommentList,
  characterCount,
  loading = false,
  headings = [],
}: {
  docWidth?: string;
  info?: V1ShareNodeDetailResp;
  editorRef: UseTiptapReturn;
  editorReady: boolean;
  commentList?: any[];
  characterCount?: number;
  loading?: boolean;
  headings?: TocList;
}) => {
  const { mobile = false, authInfo, kbDetail, catalogWidth, tree } = useStore();
  const basePath = useBasePath();
  const params = useParams() || {};
  const [commentLoading, setCommentLoading] = useState(false);
  const docId = params.id as string;
  const [commentList, setCommentList] = useState<any[]>(propsCommentList ?? []);
  const [canRenderEditor, setCanRenderEditor] = useState(false);
  const [contentReady, setContentReady] = useState(false);
  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm({
    defaultValues: { content: '', name: '' },
  });

  const commentInputRef = useRef<CommentInputRef>(null);
  const [contentFocused, setContentFocused] = useState(false);
  const [commentImages, setCommentImages] = useState<ImageItem[]>([]);

  const adjacentDocs = useMemo(() => {
    if (!tree || !docId || info?.type !== 2) return undefined;
    return findAdjacentDocuments(tree, docId);
  }, [tree, docId, info?.type]);
  const appDetail: any = kbDetail?.settings;
  const isMarkdown = info?.meta?.content_type === 'md';
  const shouldPageMarkdown = (info?.content?.length || 0) > 120000;
  const isReadonlyFilePreview = useMemo(() => {
    return !!info?.meta?.file_key && !!info?.meta?.file_readonly;
  }, [info?.meta?.file_key, info?.meta?.file_readonly]);

  const getComment = async () => {
    if (!info?.kb_id) return;
    const res = await getShareV1CommentList(
      { id: docId },
      {
        headers: {
          'X-KB-ID': info.kb_id,
        },
      },
    );
    setCommentList(res.data ?? []);
  };

  useEffect(() => {
    if (
      docId &&
      info?.kb_id &&
      !info?.path_only &&
      appDetail?.web_app_comment_settings?.is_enable
    ) {
      getComment();
    }
  }, [
    docId,
    info?.kb_id,
    info?.path_only,
    appDetail?.web_app_comment_settings?.is_enable,
  ]);

  const onSubmit = handleSubmit(
    async (data: { content: string; name: string }) => {
      if (!authInfo?.id) {
        message.error('请登录后发表评论');
        return;
      }
      setCommentLoading(true);
      let token = '';
      try {
        const Cap = (await import('@cap.js/widget')).default;
        const cap = new Cap({ apiEndpoint: `${basePath}/share/v1/captcha/` });
        const solution = await cap.solve();
        token = solution.token;
      } catch (error) {
        message.error('验证失败');
        setCommentLoading(false);
        return;
      }
      try {
        const kbID = info?.kb_id || '';
        if (!kbID) {
          message.error('知识库信息缺失');
          return;
        }
        let imageUrls: string[] = [];
        if (commentImages.length > 0 && commentInputRef.current) {
          imageUrls = await commentInputRef.current.uploadImages();
        }
        await postShareV1Comment(
          {
            content: data.content,
            pic_urls: imageUrls,
            node_id: docId,
            user_name: authInfo?.account || '',
            captcha_token: token,
          },
          {
            headers: {
              'X-KB-ID': kbID,
            },
          },
        );
        getComment();
        reset();
        commentInputRef.current?.clearImages();
        setCommentImages([]);
        message.success(
          appDetail?.web_app_comment_settings?.moderation_enable
            ? '评论已提交，请耐心等待审核'
            : '评论成功',
        );
      } catch (error: any) {
        console.log(error.message || '评论发布失败');
      } finally {
        setCommentLoading(false);
      }
    },
  );

  useEffect(() => {
    window.CAP_CUSTOM_WASM_URL =
      window.location.origin + `${basePath}/cap@0.0.6/cap_wasm.min.js`;
  }, [basePath]);

  useEffect(() => {
    if (isMarkdown || info?.type !== 2 || !editorRef.editor || !editorReady) {
      setCanRenderEditor(false);
      return;
    }

    const timer = window.setTimeout(() => {
      setCanRenderEditor(true);
    }, 0);

    return () => {
      window.clearTimeout(timer);
      setCanRenderEditor(false);
    };
  }, [editorReady, editorRef.editor, info?.type, isMarkdown]);

  useEffect(() => {
    if (!info) {
      setContentReady(false);
      return;
    }

    setContentReady(false);

    const markReady = () => {
      window.requestAnimationFrame(() => {
        setContentReady(true);
      });
    };

    if (info?.type === 1) {
      markReady();
      return;
    }

    if (isReadonlyFilePreview || isMarkdown) {
      const timer = window.setTimeout(markReady, 0);
      return () => window.clearTimeout(timer);
    }

    if (
      info?.type === 2 &&
      editorReady &&
      canRenderEditor &&
      editorRef.editor
    ) {
      markReady();
    }
  }, [
    canRenderEditor,
    editorReady,
    editorRef.editor,
    info?.id,
    info?.type,
    isMarkdown,
    isReadonlyFilePreview,
  ]);

  const onCopyDocMd = () => {
    if (!info) return;
    let context = isMarkdown
      ? info?.content || ''
      : editorRef.getMarkdown() || '';
    if (
      kbDetail?.settings?.copy_setting === ConstsCopySetting.CopySettingAppend
    ) {
      context += `\n\n-----------------------------------------\n内容来自 ${typeof window !== 'undefined' ? window.location.href : ''}`;
    }
    copyText(context);
  };

  return (
    <Box
      id='doc-content'
      sx={theme => ({
        wordBreak: 'break-all',
        color: 'text.primary',
        position: 'relative',
        zIndex: 1,
        '& ::selection': {
          backgroundColor: `${alpha(theme.palette.primary.main, 0.2)} !important`,
        },
        ...(getDocContentSx({
          docWidth: docWidth || 'full',
          mobile,
          catalogWidth: catalogWidth ?? 260,
        }) as Record<string, unknown>),
      })}
    >
      {!info || loading ? (
        <DocSkeleton showSummary={false} />
      ) : (
        <>
          <DocMetaInfo
            info={info}
            characterCount={characterCount}
            kbDetailCopySetting={kbDetail?.settings?.copy_setting}
            onCopyDocMd={onCopyDocMd}
          />

          {info.path_only && (
            <Alert severity='info' sx={{ mb: 3 }}>
              此目录仅用于定位您有权查看的下级内容，目录自身内容和信息不可见。
            </Alert>
          )}

          {info?.meta?.summary && (
            <Box
              sx={{
                mb: 6,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: '10px',
                bgcolor: 'background.paper3',
                p: '20px',
                fontSize: 14,
                lineHeight: '28px',
                backdropFilter: 'blur(5px)',
              }}
            >
              <Box sx={{ fontWeight: 'bold', mb: 2, lineHeight: '22px' }}>
                内容摘要
              </Box>
              <Box>{info?.meta?.summary}</Box>
            </Box>
          )}

          <AttachmentList
            attachments={info?.meta?.attachments}
            nodeId={info?.id || ''}
            kbId={info?.kb_id || ''}
          />

          <Box
            className='editor-container'
            sx={{
              mt: 6,
              '.tiptap.ProseMirror': {
                '.tableWrapper': {
                  width:
                    docWidth === 'full'
                      ? '100%'
                      : DocWidth[docWidth as keyof typeof DocWidth].value,
                  overflowX: 'auto',
                  ...(docWidth !== 'full' && { maxWidth: '100%' }),
                  ...(mobile && { width: '100%' }),
                },
              },
            }}
          >
            {!contentReady ? (
              <DocSkeleton showSummary={false} />
            ) : isReadonlyFilePreview ? (
              <FilePreview info={info} />
            ) : isMarkdown ? (
              <MarkDown
                content={info.content || ''}
                paged={shouldPageMarkdown}
                headings={headings}
              />
            ) : (
              <>
                {info.type === 2 &&
                  editorRef.editor &&
                  editorReady &&
                  canRenderEditor && (
                    <EditorContent editor={editorRef.editor} />
                  )}
                {info.type === 1 && <FolderList list={info.list} />}
              </>
            )}
          </Box>

          {adjacentDocs && (adjacentDocs.prev || adjacentDocs.next) && (
            <AdjacentDocNav
              prev={adjacentDocs.prev}
              next={adjacentDocs.next}
              basePath={basePath}
              hasCommentSection={appDetail?.web_app_comment_settings?.is_enable}
            />
          )}

          {!info.path_only &&
            appDetail?.web_app_comment_settings?.is_enable && (
              <CommentSection
                commentList={commentList}
                canComment={!!authInfo?.id}
                contentFocused={contentFocused}
                control={control}
                errors={errors}
                onSubmit={onSubmit}
                commentLoading={commentLoading}
                commentInputRef={commentInputRef}
                commentImages={commentImages}
                onContentFocus={() => setContentFocused(true)}
                onContentBlur={() => setContentFocused(false)}
                onImagesChange={setCommentImages}
                basePath={basePath}
              />
            )}
        </>
      )}
    </Box>
  );
};

export default DocContent;
