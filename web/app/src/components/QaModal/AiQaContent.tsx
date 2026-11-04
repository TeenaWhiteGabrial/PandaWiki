'use client';
import { useChatSession } from '@/hooks/useChatSession';
import ChatHistoryToolbar from '@/components/ChatHistoryToolbar';
import aiLoading from '@/assets/images/ai-loading.gif';
import Logo from '@/assets/images/logo.png';
import { ChunkResultItem } from '@/assets/type';
import Feedback from '@/components/feedback';
import { IconCopy } from '@/components/icons';
import MarkDown2 from '@/components/markdown2';
import { useBasePath, useSmartScroll } from '@/hooks';
import { useStore } from '@/provider';
import { postShareV1ChatFeedback } from '@/request/ShareChat';
import { getShareV1ConversationDetail } from '@/request/ShareConversation';
import { copyText } from '@/utils';
import SSEClient from '@/utils/fetch';
import { Image as ImagePreview, message } from '@ctzhian/ui';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import {
  Box,
  Button,
  IconButton,
  Stack,
  Typography,
  alpha,
  useTheme,
} from '@mui/material';
import {
  IconADiancaiWeixuanzhong2,
  IconDiancaiWeixuanzhong,
  IconDianzanWeixuanzhong,
  IconDianzanXuanzhong1,
  IconFasong,
  IconXingxing,
} from '@panda-wiki/icons';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import relativeTime from 'dayjs/plugin/relativeTime';
import Image from 'next/image';
import { useSearchParams } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { v4 as uuidv4 } from 'uuid';
import {
  assistantFileIntent,
  assistantTemplateIntent,
} from './assistantIntent.mjs';
import ChatLoading from '../../views/chat/ChatLoading';
import {
  StyledActionButtonStack,
  StyledActionStack,
  StyledAiBubble,
  StyledAiBubbleContent,
  StyledChunkAccordion,
  StyledChunkAccordionDetails,
  StyledChunkAccordionSummary,
  StyledChunkItem,
  StyledConversationContainer,
  StyledConversationItem,
  StyledFuzzySuggestionItem,
  StyledFuzzySuggestionsStack,
  StyledHotSearchColumn,
  StyledHotSearchColumnItem,
  StyledHotSearchContainer,
  StyledInputContainer,
  StyledInputWrapper,
  StyledMainContainer,
  StyledTextField,
  StyledThinkingAccordion,
  StyledThinkingAccordionDetails,
  StyledThinkingAccordionSummary,
  StyledUserBubble,
} from './StyledComponents';
import { handleThinkingContent } from './utils';
import AssistantTools, { emptyAssistantOptions } from './AssistantTools';
import AssistantRunCard from './AssistantRunCard';
import AssistantFilePanel from './AssistantFilePanel';
import {
  assistantConversationFiles,
  assistantRevisionTarget,
} from './assistantFiles.mjs';
import { reportAssistantIssue } from './assistantFeedback.mjs';
import {
  assistantRequest,
  AssistantOptions,
  AssistantRun,
  AssistantArtifact,
} from '@/request/Assistant';

import { getImagePath } from '@/utils/getImagePath';

export interface ConversationItem {
  image_paths: string[];
  q: string;
  a: string;
  score: number;
  update_time: string;
  message_id: string;
  source: 'history' | 'chat';
  chunk_result: ChunkResultItem[];
  result_expend: boolean;
  thinking_expend: boolean;
  thinking_content: string;
  id: string;
  assistantRun?: AssistantRun;
}

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

const AnswerStatus = {
  1: '正在搜索结果...',
  2: '思考中...',
  3: '正在回答',
  4: '',
};

const getChatAuthHeaders = (): Record<string, string> => ({ 'X-PW-CSRF': '1' });

const LoadingContent = ({
  thinking,
}: {
  thinking: keyof typeof AnswerStatus;
}) => {
  if (thinking === 4 || thinking === 2) return null;
  return (
    <Stack direction='row' alignItems='center' gap={1} sx={{ pb: 1 }}>
      <Image
        src={aiLoading}
        alt='ai-loading'
        unoptimized
        width={20}
        height={20}
      />
      <Typography
        variant='body2'
        sx={theme => ({
          fontSize: 12,
          color: alpha(theme.palette.text.primary, 0.5),
        })}
      >
        {AnswerStatus[thinking]}
      </Typography>
    </Stack>
  );
};

const AiQaContent: React.FC<{
  hotSearch: string[];
  placeholder: string;
  inputRef: React.RefObject<HTMLInputElement | null>;
  onOpenFile?: () => void;
}> = ({ hotSearch, placeholder, inputRef, onOpenFile }) => {
  const sseClientRef = useRef<SSEClient<{
    type: string;
    content: string;
    chunk_result: ChunkResultItem;
  }> | null>(null);
  const { palette } = useTheme();
  const messageIdRef = useRef('');
  const lastResultExpendRef = useRef(false);
  const [fullAnswer, setFullAnswer] = useState<string>('');
  const [conversation, setConversation] = useState<ConversationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [thinking, setThinking] = useState<keyof typeof AnswerStatus>(4);
  const chatSession = useChatSession(useBasePath());
  const { conversationId } = chatSession;
  const [restoringHistory, setRestoringHistory] = useState(false);
  const initialQueryRef = useRef<string | null>(null);
  const [input, setInput] = useState('');
  const [assistantOptions, setAssistantOptions] = useState<AssistantOptions>(
    emptyAssistantOptions,
  );
  const assistantOptionsRef = useRef(assistantOptions);
  assistantOptionsRef.current = assistantOptions;
  const [assistantBusy, setAssistantBusy] = useState(false);
  const [assistantHint, setAssistantHint] = useState('');
  const [selectedFile, setSelectedFile] = useState<AssistantArtifact | null>(
    null,
  );
  const [followLatest, setFollowLatest] = useState(true);
  const [targetChoices, setTargetChoices] = useState<AssistantArtifact[]>([]);
  const artifacts = [
    ...new Map(
      conversation
        .flatMap(c =>
          c.assistantRun?.artifact ? [c.assistantRun.artifact] : [],
        )
        .map(f => [f.id, f]),
    ).values(),
  ];
  const selectFile = (file: AssistantArtifact) => {
    setSelectedFile(file);
    setFollowLatest(
      !artifacts.some(
        f => f.family_id === file.family_id && f.revision > file.revision,
      ),
    );
    onOpenFile?.();
  };
  const reviseFile = (id: string, targetIDs?: string[]) => {
    setAssistantOptions(prev => ({ ...prev, baseArtifact: id, targetIDs }));
    setInput(targetIDs?.length ? '请修改所选部分：' : '请修改这个文件：');
    if (mobile) setSelectedFile(null);
    inputRef.current?.focus();
  };
  useEffect(() => {
    setSelectedFile(null);
    setTargetChoices([]);
  }, [conversationId]);
  useEffect(() => {
    if (!selectedFile || !followLatest) return;
    const latest = artifacts
      .filter(f => f.family_id === selectedFile.family_id)
      .sort((a, b) => b.revision - a.revision)[0];
    if (latest && latest.id !== selectedFile.id) setSelectedFile(latest);
  }, [conversation, selectedFile?.id, followLatest]);

  const pauseLastAnswer = (notice: string) => {
    setConversation(items =>
      items.map((item, index) =>
        index === items.length - 1
          ? {
              ...item,
              a: item.a.includes(notice)
                ? item.a
                : [item.a, notice].filter(Boolean).join('\n\n'),
              update_time: dayjs().format('YYYY-MM-DD HH:mm:ss'),
            }
          : item,
      ),
    );
  };
  const [uploadingAttachments, setUploadingAttachments] = useState(false);
  const pendingAssistant = useRef<{
    fingerprint: string;
    requestId: string;
  } | null>(null);
  const submitAssistant = async (
    q: string,
    opts = assistantOptionsRef.current,
  ) => {
    const format =
      assistantFileIntent(q) ||
      (opts.template && assistantTemplateIntent(q) ? 'pptx' : undefined);
    if (opts.attachments.some(a => a.status === 'failed')) {
      setAssistantHint('请先继续处理尚未准备好的附件，或移除后再次发送。');
      setLoading(false);
      return;
    }
    if (opts.web && !opts.publicQuery.trim()) {
      setAssistantHint('填写本轮可公开的检索词后，就可以继续。');
      setLoading(false);
      return;
    }
    setAssistantBusy(true);
    const version = chatSession.generation.current;
    try {
      const session = await chatSession.resolve();
      if (version !== chatSession.generation.current) return;
      const fingerprint = JSON.stringify({
        q,
        cid: session?.conversation_id,
        attachments: opts.attachments.map(a => a.id),
        template: opts.template,
        web: opts.web,
        query: opts.publicQuery,
        scope: opts.scope,
        base: opts.baseArtifact,
        targets: opts.targetIDs,
      });
      if (pendingAssistant.current?.fingerprint !== fingerprint)
        pendingAssistant.current = { fingerprint, requestId: uuidv4() };
      const run = await assistantRequest<AssistantRun>('/runs', 'POST', {
        client_request_id: pendingAssistant.current.requestId,
        conversation_id: session?.conversation_id || '',
        message: q,
        attachment_ids: opts.attachments
          .filter(a => a.id !== opts.template?.id)
          .map(a => a.id),
        template_ref: format && format !== 'pptx' ? undefined : opts.template,
        format,
        web_search_enabled: opts.web,
        public_search_query: opts.web ? opts.publicQuery : '',
        source_scope: opts.scope,
        base_artifact_id: opts.baseArtifact,
        target_ids: opts.targetIDs,
      });
      if (version !== chatSession.generation.current) return;
      pendingAssistant.current = null;
      chatSession.acceptEvent('conversation_id', run.conversation_id);
      if (run.nonce) chatSession.acceptEvent('nonce', run.nonce);
      setConversation(prev =>
        prev.map((item, i) =>
          i === prev.length - 1
            ? {
                ...item,
                assistantRun: run,
                message_id: run.assistant_message_id,
              }
            : item,
        ),
      );
      setAssistantOptions({
        ...opts,
        attachments: [],
        baseArtifact: undefined,
        targetIDs: undefined,
        publicQuery: '',
      });
    } catch (e) {
      if (version !== chatSession.generation.current) return;
      const err = reportAssistantIssue('submit', e);
      setConversation(prev =>
        prev.map((item, i) =>
          i === prev.length - 1 ? { ...item, a: err } : item,
        ),
      );
    } finally {
      if (version === chatSession.generation.current) {
        setAssistantBusy(false);
        setLoading(false);
        setThinking(4);
      }
    }
  };
  const continueAssistant = async (q: string, previous: AssistantRun) => {
    const version = chatSession.generation.current;
    const opts = assistantOptionsRef.current;
    const confirming =
      previous.state === 'awaiting_outline' &&
      /^(确认|确认大纲|按大纲生成|开始生成|就按这个生成|就这么做|可以生成)[。！!\s]*$/.test(
        q.trim(),
      );
    const action =
      previous.state === 'awaiting_input'
        ? 'answer'
        : previous.state === 'awaiting_sources'
          ? 'supplement'
          : confirming
            ? 'confirm_outline'
            : 'revise';
    const body = {
      revision: previous.planning!.revision,
      action,
      message: q,
      outline: confirming ? previous.planning!.outline : undefined,
      attachment_ids:
        action === 'supplement' ? opts.attachments.map(a => a.id) : [],
      node_ids:
        action === 'supplement' && opts.scope.type === 'documents'
          ? opts.scope.node_ids || []
          : [],
    };
    const fingerprint = JSON.stringify({ run: previous.id, ...body });
    if (pendingAssistant.current?.fingerprint !== fingerprint)
      pendingAssistant.current = { fingerprint, requestId: uuidv4() };
    setAssistantBusy(true);
    try {
      const current = await assistantRequest<AssistantRun>(
        `/runs/${previous.id}/continue`,
        'POST',
        { ...body, client_request_id: pendingAssistant.current.requestId },
      );
      if (version !== chatSession.generation.current) return;
      pendingAssistant.current = null;
      setConversation(items =>
        items.map((item, index) =>
          item.assistantRun?.id === current.id
            ? { ...item, assistantRun: current }
            : index === items.length - 1
              ? { ...item, a: '已记录，继续处理当前文件任务。' }
              : item,
        ),
      );
      if (action === 'supplement')
        setAssistantOptions({ ...opts, attachments: [] });
    } catch (e) {
      if (version !== chatSession.generation.current) return;
      const error = reportAssistantIssue('continue', e);
      setConversation(items =>
        items.map((item, index) =>
          index === items.length - 1 ? { ...item, a: error } : item,
        ),
      );
    } finally {
      if (version === chatSession.generation.current) {
        setAssistantBusy(false);
        setLoading(false);
        setThinking(4);
      }
    }
  };
  const [open, setOpen] = useState(false);
  const [conversationItem, setConversationItem] =
    useState<ConversationItem | null>(null);
  const [fuzzySuggestions, setFuzzySuggestions] = useState<string[]>([]);
  const [showFuzzySuggestions, setShowFuzzySuggestions] = useState(false);

  const searchParams = useSearchParams();
  const basePath = useBasePath();

  // 使用智能滚动 hook（内置 ResizeObserver 自动监听内容高度变化，自动滚动）
  const { setShouldAutoScroll } = useSmartScroll({
    container: '.conversation-container',
    behavior: 'smooth',
  });

  const onReset = () => {
    if (loading) {
      handleSearchAbort();
    }
    chatSession.reset();
    setRestoringHistory(false);
    setConversation([]);
    setAssistantOptions(emptyAssistantOptions());
    setFullAnswer('');
    setInput('');
    setLoading(false);
  };

  const handleSearch = (reset: boolean = false) => {
    if (input.length > 0) {
      onSearch(input, reset);
    }
  };

  const onSuggestionClick = (text: string) => {
    setInput('');
    onSearch(text);
  };

  // 处理输入变化，显示模糊搜索建议
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setInput(value);

    // if (value.trim().length > 0) {
    //   // 改进的模糊搜索逻辑
    //   const filtered = mockFuzzySuggestions
    //     .filter(suggestion => {
    //       const lowerSuggestion = suggestion.toLowerCase();
    //       const lowerValue = value.toLowerCase();
    //       // 支持前缀匹配和包含匹配
    //       return (
    //         lowerSuggestion.startsWith(lowerValue) ||
    //         lowerSuggestion.includes(lowerValue)
    //       );
    //     })
    //     .slice(0, 5); // 限制显示数量

    //   setFuzzySuggestions(filtered);
    //   setShowFuzzySuggestions(true);
    // } else {
    //   setShowFuzzySuggestions(false);
    //   setFuzzySuggestions([]);
    // }
  };

  // 选择模糊搜索建议
  const handleFuzzySuggestionClick = (suggestion: string) => {
    setInput(suggestion);
    setShowFuzzySuggestions(false);
    setFuzzySuggestions([]);
  };

  // 高亮显示匹配的文本
  const highlightMatch = (text: string, query: string) => {
    if (!query.trim()) return text;

    // 转义特殊字符，避免正则表达式错误
    const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(`(${escapedQuery})`, 'gi');
    const parts = text.split(regex);

    return parts.map((part, index) => {
      // 检查是否匹配（不区分大小写）
      if (part.toLowerCase() === query.toLowerCase()) {
        return (
          <Box
            component='span'
            key={index}
            sx={{
              color: 'primary.main',
            }}
          >
            {part}
          </Box>
        );
      }
      return part;
    });
  };

  // 处理输入框失去焦点
  const handleInputBlur = () => {
    // 延迟隐藏，让用户有时间点击建议
    setTimeout(() => {
      setShowFuzzySuggestions(false);
    }, 200);
  };

  // 处理输入框获得焦点
  const handleInputFocus = () => {
    if (input.trim().length > 0) {
      setShowFuzzySuggestions(true);
    }
  };

  const chatAnswer = async (q: string) => {
    setLoading(true);
    setThinking(1);
    const version = chatSession.generation.current;

    let token = '';

    const Cap = (await import(`@cap.js/widget`)).default;
    const cap = new Cap({
      apiEndpoint: `${basePath}/share/v1/captcha/`,
    });
    try {
      const solution = await cap.solve();
      token = solution.token;
    } catch (error) {
      if (version !== chatSession.generation.current) return;
      setLoading(false);
      setThinking(4);
      pauseLastAnswer(reportAssistantIssue('chat', error));
      return;
    }

    const reqData = {
      message: q,
      image_paths: [],
      nonce: '',
      conversation_id: '',
      app_type: 1,
      captcha_token: token,
    };
    if (version !== chatSession.generation.current) return;
    try {
      const session = await chatSession.resolve();
      if (!session || version !== chatSession.generation.current) return;
      reqData.conversation_id = session.conversation_id;
      reqData.nonce = session.nonce;
    } catch (error) {
      if (version !== chatSession.generation.current) return;
      setLoading(false);
      setThinking(4);
      pauseLastAnswer(reportAssistantIssue('history', error));
      return;
    }

    if (sseClientRef.current) {
      sseClientRef.current.subscribe(
        JSON.stringify(reqData),
        ({ type, content, chunk_result }) => {
          if (version !== chatSession.generation.current) return;
          if (type === 'conversation_id') {
            chatSession.acceptEvent(type, content);
          } else if (type === 'message_id') {
            messageIdRef.current = content;
          } else if (type === 'nonce') {
            chatSession.acceptEvent(type, content);
          } else if (type === 'error') {
            setLoading(false);
            setThinking(4);
            pauseLastAnswer(reportAssistantIssue('chat', { message: content }));
          } else if (type === 'done') {
            setConversation(prev => {
              const newConversation = [...prev];
              const lastConversation =
                newConversation[newConversation.length - 1];
              if (lastConversation) {
                lastConversation.update_time = dayjs().format(
                  'YYYY-MM-DD HH:mm:ss',
                );
                lastConversation.message_id = messageIdRef.current;
                lastConversation.source = 'chat';
              }
              return newConversation;
            });

            setFullAnswer('');
            setLoading(false);

            setThinking(4);
          } else if (type === 'data') {
            setFullAnswer(prevFullAnswer => {
              const newFullAnswer = prevFullAnswer + content;

              const { thinkingContent, answerContent } =
                handleThinkingContent(newFullAnswer);

              // 更新状态
              if (newFullAnswer.includes('</think>')) {
                setThinking(3);
              } else if (newFullAnswer.includes('<think>')) {
                setThinking(2);
              } else {
                setThinking(3);
              }
              setConversation(preConversation => {
                const newConversation = [...preConversation];
                const lastConversation =
                  newConversation[newConversation.length - 1];
                if (lastConversation) {
                  lastConversation.a = answerContent;
                  lastConversation.thinking_content = thinkingContent;
                  lastConversation.result_expend = lastResultExpendRef.current;
                  lastConversation.thinking_expend = false;
                }
                return newConversation;
              });

              return newFullAnswer;
            });
          } else if (type === 'chunk_result') {
            setConversation(preConversation => {
              const newConversation = [...preConversation];
              const lastConversation =
                newConversation[newConversation.length - 1];
              if (lastConversation) {
                lastConversation.chunk_result = [
                  ...lastConversation.chunk_result,
                  chunk_result,
                ];
              }
              return newConversation;
            });
          }
        },
      );
    }
  };

  useEffect(() => {
    // @ts-ignore
    window.CAP_CUSTOM_WASM_URL =
      window.location.origin + `${basePath}/cap@0.0.6/cap_wasm.min.js`;
  }, []);

  const onSearch = (q: string, reset: boolean = false) => {
    if (loading || restoringHistory || uploadingAttachments || !q.trim())
      return;
    const pending = reset
      ? undefined
      : [...conversation]
          .reverse()
          .find(
            item =>
              [
                'awaiting_input',
                'awaiting_sources',
                'awaiting_outline',
              ].includes(item.assistantRun?.state || '') &&
              !!item.assistantRun?.planning,
          )?.assistantRun;
    if (
      pending &&
      pending.state !== 'awaiting_sources' &&
      assistantOptionsRef.current.attachments.length
    ) {
      setAssistantHint(
        '请先完成卡片中的需求或大纲确认；需要补充资料时可在资料检查阶段上传。',
      );
      return;
    }
    if (reset) chatSession.reset();
    setAssistantHint('');
    setLoading(true);
    setShouldAutoScroll(true); // 开始新搜索时，重置为自动滚动
    const newConversation = reset ? [] : [...conversation];
    lastResultExpendRef.current = false;
    newConversation.push({
      image_paths: [],
      q,
      a: '',
      score: 0,
      message_id: '',
      update_time: '',
      source: 'chat',
      chunk_result: [],
      thinking_content: '',
      result_expend: true,
      thinking_expend: true,
      id: uuidv4(),
    });
    messageIdRef.current = '';
    setConversation(newConversation);
    setFullAnswer('');
    if (pending) {
      void continueAssistant(q, pending);
      setInput('');
      return;
    }
    const opts = { ...assistantOptionsRef.current };
    const target = assistantRevisionTarget(
      q,
      conversation.flatMap(c => (c.assistantRun ? [c.assistantRun] : [])),
    );
    if (target.fresh) {
      opts.baseArtifact = undefined;
      opts.targetIDs = undefined;
    }
    if (!opts.baseArtifact && target.ambiguous) {
      setConversation(conversation);
      setLoading(false);
      setInput(q);
      setTargetChoices(target.files as AssistantArtifact[]);
      setAssistantHint('你指的是哪一份？选择后会继续处理这条要求。');
      return;
    }
    if (!opts.baseArtifact && target.id) opts.baseArtifact = target.id;
    setTargetChoices([]);
    const useAssistant =
      opts.attachments.length > 0 ||
      (!!opts.template && assistantTemplateIntent(q)) ||
      opts.web ||
      !!opts.baseArtifact ||
      opts.scope.type !== 'all' ||
      !!assistantFileIntent(q);
    if (useAssistant) void submitAssistant(q, opts);
    else void chatAnswer(q);
    setInput('');
  };

  const handleSearchAbort = () => {
    chatSession.generation.current++;
    sseClientRef.current?.unsubscribe();
    setLoading(false);
    setThinking(4);
  };

  const { mobile = false, kbDetail, qaModalOpen } = useStore();

  const isFeedbackEnabled =
    // @ts-ignore
    kbDetail?.settings?.ai_feedback_settings?.is_enabled ?? true;

  const handleScore = async (
    message_id: string,
    score: number,
    type?: string,
    content?: string,
  ) => {
    const data: any = {
      conversation_id: conversationId,
      message_id,
      score,
    };
    if (type) data.type = type;
    if (content) data.feedback_content = content;
    try {
      await postShareV1ChatFeedback(data, { isAlert: false });
    } catch (problem) {
      reportAssistantIssue('feedback', problem);
      setAssistantHint('这次评价尚未提交，可稍后再次评价。');
      return;
    }
    message.success('反馈成功');
    setConversation(
      conversation.map(item => {
        return item.message_id === message_id ? { ...item, score } : item;
      }),
    );
  };

  useEffect(() => {
    sseClientRef.current = new SSEClient({
      url: `${basePath}/share/v1/chat/message`,
      headers: {
        'Content-Type': 'application/json',
        ...getChatAuthHeaders(),
      },
      onError: error => {
        setLoading(false);
        setThinking(4);
        pauseLastAnswer(reportAssistantIssue('chat', error));
      },
      onCancel: () => {
        setLoading(false);
        setThinking(4);
        setConversation(prev => {
          const newConversation = [...prev];
          const lastConversation = newConversation[newConversation.length - 1];
          if (lastConversation) {
            lastConversation.a = lastConversation.a + '\n\n已停止本次回答。';
            lastConversation.update_time = dayjs().format(
              'YYYY-MM-DD HH:mm:ss',
            );
            lastConversation.message_id = messageIdRef.current;
          }
          return newConversation;
        });
      },
    });
    const searchQuery =
      initialQueryRef.current ||
      sessionStorage.getItem('chat_search_query') ||
      searchParams.get('ask');
    initialQueryRef.current = searchQuery;
    if (searchQuery) {
      sessionStorage.removeItem('chat_search_query');
      const newSearchParams = new URLSearchParams(searchParams.toString());
      newSearchParams.delete('cid');
      newSearchParams.delete('ask');
      const url = new URL(window.location.href);
      url.search = newSearchParams.toString();
      window.history.replaceState(null, '', url.toString());
      onSearch(searchQuery, true);
    }
    return () => {
      handleSearchAbort();
    };
  }, []);

  useEffect(() => {
    if (conversationId) {
      const currentUrl = new URL(window.location.href);
      currentUrl.searchParams.set('cid', conversationId);
      currentUrl.searchParams.delete('ask');
      window.history.replaceState(null, '', currentUrl.toString());
    }
  }, [conversationId]);

  const loadHistory = async (cid: string): Promise<boolean> => {
    handleSearchAbort();
    chatSession.reset();
    chatSession.restore(cid);
    const version = chatSession.generation.current;
    setRestoringHistory(true);
    setConversation([]);
    setFullAnswer('');
    setInput('');
    messageIdRef.current = '';
    try {
      const res = await getShareV1ConversationDetail(
        { id: cid },
        { isAlert: false },
      );
      if (version !== chatSession.generation.current) return false;
      const conversation: ConversationItem[] = [];
      if (res.messages) {
        let current: Partial<ConversationItem> = {
          chunk_result: [],
        };
        res.messages.forEach(message => {
          if (message.role === 'user') {
            if (current.q || current.image_paths?.length) {
              conversation.push({
                image_paths: current.image_paths || [],
                q: current.q || '',
                a: '',
                score: 0,
                update_time: '',
                message_id: '',
                source: 'history',
                chunk_result: [],
                thinking_content: '',
                id: uuidv4(),
                result_expend: true,
                thinking_expend: true,
              });
            }
            current = {
              image_paths: message.image_paths || [],
              q: message.content,
              chunk_result: [],
            };
          } else if (message.role === 'assistant') {
            if (
              current.q ||
              (current.image_paths && current.image_paths.length > 0)
            ) {
              const { thinkingContent, answerContent } = handleThinkingContent(
                message.content || '',
              );
              current.a = answerContent;
              current.update_time = message.created_at;
              current.score = 0;
              current.message_id = '';
              current.thinking_content = thinkingContent;
              current.source = 'history';
              current.id = uuidv4();
              conversation.push(current as ConversationItem);
              current = {};
            }
          }
        });
        if (
          current.q ||
          (current.image_paths && current.image_paths.length > 0)
        ) {
          conversation.push({
            image_paths: current.image_paths || [],
            q: current.q || '',
            a: '',
            score: 0,
            update_time: '',
            message_id: '',
            source: 'history',
            chunk_result: [],
            thinking_content: '',
            id: uuidv4(),
            result_expend: true,
            thinking_expend: true,
          });
        }
      }
      const runs = await assistantRequest<AssistantRun[]>(
        `/runs?conversation_id=${encodeURIComponent(cid)}`,
      );
      if (version !== chatSession.generation.current) return false;
      runs.forEach(run => {
        const at = conversation.findIndex(
          c => c.q === run.question && !c.assistantRun,
        );
        if (at >= 0)
          conversation[at] = { ...conversation[at], assistantRun: run, a: '' };
        else
          conversation.push({
            q: run.question || '后台任务',
            a: '',
            assistantRun: run,
            image_paths: [],
            score: 0,
            update_time: run.created_at,
            message_id: run.assistant_message_id,
            source: 'history',
            chunk_result: [],
            thinking_content: '',
            result_expend: false,
            thinking_expend: false,
            id: run.id,
          });
      });
      setConversation(conversation);
      setAssistantBusy(false);
      setAssistantOptions(emptyAssistantOptions());
      setShouldAutoScroll(false);
      return true;
    } catch (error) {
      if (version !== chatSession.generation.current) return false;
      setRestoringHistory(false);
      chatSession.reset();
      setAssistantHint(reportAssistantIssue('history', error));
      return false;
    } finally {
      if (version === chatSession.generation.current)
        setRestoringHistory(false);
    }
  };

  useEffect(() => {
    const cid = searchParams.get('cid');
    if (cid && !initialQueryRef.current) void loadHistory(cid);
    return () => {
      chatSession.generation.current++;
    };
  }, []);

  useEffect(() => {
    if (!qaModalOpen) {
      conversation.forEach(item => {
        item.image_paths.forEach(image => {
          if (image.startsWith('blob:')) {
            URL.revokeObjectURL(image);
          }
        });
      });
    }
  }, [qaModalOpen, conversation]);

  return (
    <Box
      sx={{
        display: 'flex',
        flex: 1,
        minHeight: 0,
        minWidth: 0,
        gap: 2,
        width: '100%',
      }}
    >
      <StyledMainContainer
        sx={{ display: selectedFile && mobile ? 'none' : 'flex' }}
        className={palette.mode === 'dark' ? 'md-dark' : ''}
      >
        <ChatHistoryToolbar
          conversationId={conversationId}
          disabled={restoringHistory || uploadingAttachments || assistantBusy}
          credentials={chatSession.historyCredentials}
          onSelect={loadHistory}
          onNew={onReset}
        />
        {artifacts.length > 0 && (
          <Stack
            direction='row'
            gap={1}
            py={0.5}
            alignItems='center'
            sx={{ overflowX: 'auto', flexShrink: 0 }}
          >
            <Typography variant='caption' sx={{ whiteSpace: 'nowrap' }}>
              会话文件
            </Typography>
            {(
              assistantConversationFiles(
                conversation.flatMap(c =>
                  c.assistantRun ? [c.assistantRun] : [],
                ),
              ) as AssistantArtifact[]
            ).map(file => (
              <Button
                key={file.id}
                size='small'
                sx={{ whiteSpace: 'nowrap', flexShrink: 0 }}
                onClick={() => selectFile(file)}
              >
                {file.name} · v{file.revision}
              </Button>
            ))}
          </Stack>
        )}
        {/* 无对话时显示欢迎界面 */}
        {conversation.length === 0 && (
          <Box
            sx={{
              flex: 1,
              minHeight: 0,
              overflowY: 'auto',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 3,
              py: 3,
            }}
          >
            {/* Logo区域 */}
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 2,
                my: { xs: 2, sm: 4 },
                flexShrink: 0,
              }}
            >
              <Image
                src={getImagePath(
                  kbDetail?.settings?.icon || Logo.src,
                  basePath,
                )}
                alt='logo'
                width={46}
                height={46}
                unoptimized
                style={{
                  objectFit: 'contain',
                }}
              />
              <Typography
                variant='h6'
                sx={{ fontSize: 32, color: 'text.primary', fontWeight: 700 }}
              >
                {kbDetail?.settings?.title}
              </Typography>
            </Box>

            {/* 热门搜索区域 */}
            {hotSearch.length > 0 && (
              <Box sx={{ width: '100%' }}>
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    mb: 2,
                  }}
                >
                  <Typography
                    sx={{
                      fontSize: 12,
                      fontWeight: 500,
                      color: 'primary.main',
                      display: 'flex',
                      alignItems: 'center',
                      gap: 0.5,
                    }}
                  >
                    <IconXingxing sx={{ fontSize: 14 }} />
                    大家都在搜什么?
                  </Typography>
                </Box>

                {/* 热门搜索列表 - 两列布局 */}
                <StyledHotSearchContainer>
                  {/* 左列 */}
                  <StyledHotSearchColumn>
                    {hotSearch
                      .filter((_, index) => index % 2 === 0)
                      .map((suggestion, index) => (
                        <StyledHotSearchColumnItem
                          key={index * 2}
                          onClick={() => onSuggestionClick(suggestion)}
                        >
                          • {suggestion}
                        </StyledHotSearchColumnItem>
                      ))}
                  </StyledHotSearchColumn>

                  {/* 右列 */}
                  <StyledHotSearchColumn>
                    {hotSearch
                      .filter((_, index) => index % 2 === 1)
                      .map((suggestion, index) => (
                        <StyledHotSearchColumnItem
                          key={index * 2 + 1}
                          onClick={() => onSuggestionClick(suggestion)}
                        >
                          • {suggestion}
                        </StyledHotSearchColumnItem>
                      ))}
                  </StyledHotSearchColumn>
                </StyledHotSearchContainer>
              </Box>
            )}
          </Box>
        )}

        {/* 有对话时显示对话历史 */}
        <StyledConversationContainer
          direction='column'
          className='conversation-container'
          sx={{
            mb: conversation?.length > 0 ? 2 : 0,
            display: conversation.length > 0 ? 'flex' : 'none',
          }}
        >
          <Stack gap={2}>
            {conversation.map((item, index) => (
              <StyledConversationItem key={item.id}>
                {item.image_paths.length > 0 && (
                  <ImagePreview.PreviewGroup>
                    <Stack
                      direction='row'
                      gap={1}
                      sx={{ alignSelf: 'flex-end' }}
                    >
                      {item.image_paths.map((url: string) => (
                        <ImagePreview
                          alt={url}
                          key={url}
                          src={getImagePath(url, basePath)}
                          width={100}
                          height={100}
                          style={{
                            borderRadius: '10px',
                            objectFit: 'cover',
                            cursor: 'pointer',
                          }}
                          referrerPolicy='no-referrer'
                        />
                      ))}
                    </Stack>
                  </ImagePreview.PreviewGroup>
                )}

                {/* 用户问题气泡 - 右对齐 */}
                {item.q && <StyledUserBubble>{item.q}</StyledUserBubble>}
                {/* AI回答气泡 - 左对齐 */}
                <StyledAiBubble>
                  {/* 搜索结果 */}
                  {item.chunk_result.length > 0 && (
                    <StyledChunkAccordion
                      expanded={item.result_expend}
                      onChange={(event, expanded) => {
                        setConversation(prev => {
                          const newConversation = [...prev];
                          if (index === conversation.length - 1) {
                            lastResultExpendRef.current = expanded;
                          }
                          newConversation[index].result_expend = expanded;
                          return newConversation;
                        });
                      }}
                    >
                      <StyledChunkAccordionSummary
                        expandIcon={<ExpandMoreIcon sx={{ fontSize: 16 }} />}
                      >
                        <Typography
                          variant='body2'
                          sx={theme => ({
                            fontSize: 12,
                            color: alpha(theme.palette.text.primary, 0.5),
                          })}
                        >
                          本回答引用了 {item.chunk_result.length} 份文档
                        </Typography>
                      </StyledChunkAccordionSummary>

                      <StyledChunkAccordionDetails>
                        <Stack gap={1} alignItems='flex-start'>
                          {item.chunk_result.map((chunk, chunkIndex) => (
                            <StyledChunkItem key={chunkIndex}>
                              <Typography
                                variant='body2'
                                className='hover-primary'
                                sx={theme => ({
                                  fontSize: 12,
                                  color: alpha(theme.palette.text.primary, 0.5),
                                })}
                                onClick={() => {
                                  window.open(
                                    `${basePath}/node/${chunk.node_id}`,
                                    '_blank',
                                  );
                                }}
                              >
                                {chunk.name}
                              </Typography>
                            </StyledChunkItem>
                          ))}
                        </Stack>
                      </StyledChunkAccordionDetails>
                    </StyledChunkAccordion>
                  )}

                  {/* 加载状态 */}
                  {index === conversation.length - 1 && loading && (
                    <LoadingContent thinking={thinking} />
                  )}

                  {/* 思考过程 */}
                  {!!item.thinking_content && (
                    <StyledThinkingAccordion
                      expanded={item.thinking_expend}
                      onChange={(event, expanded) => {
                        setConversation(prev => {
                          const newConversation = [...prev];
                          newConversation[index].thinking_expend = expanded;
                          return newConversation;
                        });
                      }}
                    >
                      <StyledThinkingAccordionSummary
                        expandIcon={<ExpandMoreIcon sx={{ fontSize: 16 }} />}
                      >
                        <Stack direction='row' alignItems='center' gap={1}>
                          {thinking === 2 &&
                            index === conversation.length - 1 && (
                              <Image
                                src={aiLoading}
                                alt='ai-loading'
                                width={20}
                                height={20}
                              />
                            )}

                          <Typography
                            variant='body2'
                            sx={theme => ({
                              fontSize: 12,
                              color: alpha(theme.palette.text.primary, 0.5),
                            })}
                          >
                            {thinking === 2 && index === conversation.length - 1
                              ? '思考中...'
                              : '已思考'}
                          </Typography>
                        </Stack>
                      </StyledThinkingAccordionSummary>

                      <StyledThinkingAccordionDetails>
                        <MarkDown2
                          content={item.thinking_content || ''}
                          autoScroll={false}
                        />
                      </StyledThinkingAccordionDetails>
                    </StyledThinkingAccordion>
                  )}

                  {/* AI回答内容 */}
                  <StyledAiBubbleContent>
                    {item.assistantRun ? (
                      <AssistantRunCard
                        initial={item.assistantRun}
                        onUpdate={run =>
                          setConversation(prev =>
                            prev.map(c =>
                              c.assistantRun?.id === run.id
                                ? { ...c, assistantRun: run }
                                : c,
                            ),
                          )
                        }
                        onPreview={selectFile}
                        onRevise={reviseFile}
                      />
                    ) : (
                      <MarkDown2 content={item.a} autoScroll={false} />
                    )}
                  </StyledAiBubbleContent>

                  {/* 操作按钮 */}
                  {(index !== conversation.length - 1 || !loading) && (
                    <StyledActionStack
                      direction={mobile ? 'column' : 'row'}
                      alignItems={mobile ? 'flex-start' : 'center'}
                      justifyContent='space-between'
                      gap={mobile ? 1 : 3}
                    >
                      <Stack direction='row' gap={3} alignItems='center'>
                        <span>生成于 {dayjs(item.update_time).fromNow()}</span>

                        <IconCopy
                          sx={{ cursor: 'pointer' }}
                          onClick={() => {
                            copyText(item.a);
                          }}
                        />

                        {isFeedbackEnabled && item.source === 'chat' && (
                          <>
                            {item.score === 1 && (
                              <IconDianzanXuanzhong1
                                sx={{ cursor: 'pointer' }}
                              />
                            )}
                            {item.score !== 1 && (
                              <IconDianzanWeixuanzhong
                                sx={{ cursor: 'pointer' }}
                                onClick={() => {
                                  if (item.score === 0)
                                    handleScore(item.message_id, 1);
                                }}
                              />
                            )}
                            {item.score !== -1 && (
                              <IconDiancaiWeixuanzhong
                                sx={{ cursor: 'pointer' }}
                                onClick={() => {
                                  if (item.score === 0) {
                                    setConversationItem(item);
                                    setOpen(true);
                                  }
                                }}
                              />
                            )}
                            {item.score === -1 && (
                              <IconADiancaiWeixuanzhong2
                                sx={{ cursor: 'pointer' }}
                              />
                            )}
                          </>
                        )}
                      </Stack>
                      <Box>
                        {kbDetail?.settings?.disclaimer_settings?.content}
                      </Box>
                    </StyledActionStack>
                  )}
                </StyledAiBubble>
              </StyledConversationItem>
            ))}
          </Stack>
        </StyledConversationContainer>

        <StyledInputContainer>
          <StyledInputWrapper>
            {assistantHint && (
              <Typography
                role='status'
                aria-live='polite'
                variant='body2'
                color='text.secondary'
              >
                {assistantHint}
              </Typography>
            )}
            <AssistantTools
              value={assistantOptions}
              onChange={setAssistantOptions}
              onUploadingChange={setUploadingAttachments}
              disabled={loading || restoringHistory || assistantBusy}
              conversationId={conversationId}
            />
            {targetChoices.length > 0 && (
              <Stack direction='row' gap={1} flexWrap='wrap'>
                {targetChoices.map(file => (
                  <Button
                    key={file.id}
                    size='small'
                    variant='outlined'
                    onClick={() => {
                      const next = {
                        ...assistantOptionsRef.current,
                        baseArtifact: file.id,
                        targetIDs: undefined,
                      };
                      assistantOptionsRef.current = next;
                      setAssistantOptions(next);
                      setTargetChoices([]);
                      onSearch(input);
                    }}
                  >
                    {file.name} · v{file.revision}
                  </Button>
                ))}
              </Stack>
            )}
            {assistantOptions.baseArtifact && (
              <Stack direction='row' alignItems='center' gap={1}>
                <Typography variant='caption'>
                  正在修改：
                  {artifacts.find(f => f.id === assistantOptions.baseArtifact)
                    ?.name || '所选文件'}{' '}
                  · v
                  {artifacts.find(f => f.id === assistantOptions.baseArtifact)
                    ?.revision || ''}
                  {assistantOptions.targetIDs?.length
                    ? ` · 仅修改所选 ${assistantOptions.targetIDs.length} 个部分`
                    : ''}
                </Typography>
                <Button
                  size='small'
                  onClick={() =>
                    setAssistantOptions(prev => ({
                      ...prev,
                      baseArtifact: undefined,
                      targetIDs: undefined,
                    }))
                  }
                >
                  取消选择
                </Button>
              </Stack>
            )}
            <StyledTextField
              fullWidth
              multiline
              rows={2}
              disabled={loading || restoringHistory || uploadingAttachments}
              ref={inputRef}
              size='small'
              value={input}
              onChange={handleInputChange}
              onFocus={handleInputFocus}
              onBlur={handleInputBlur}
              onKeyDown={e => {
                const isComposing =
                  e.nativeEvent.isComposing || e.nativeEvent.keyCode === 229;
                if (
                  e.key === 'Enter' &&
                  !e.shiftKey &&
                  input.length > 0 &&
                  !isComposing
                ) {
                  e.preventDefault();
                  handleSearch();
                }
              }}
              placeholder={
                restoringHistory ? '正在恢复历史会话...' : placeholder
              }
              autoComplete='off'
            />
            <StyledActionButtonStack
              direction='row'
              alignItems='center'
              justifyContent='flex-end'
            >
              <Box
                sx={{
                  fontSize: 12,
                  flexShrink: 0,
                  cursor: 'pointer',
                }}
              >
                {loading ? (
                  <ChatLoading
                    thinking={thinking}
                    onClick={() => {
                      setThinking(4);
                      handleSearchAbort();
                    }}
                  />
                ) : (
                  <IconButton
                    size='small'
                    disabled={
                      input.length === 0 ||
                      restoringHistory ||
                      uploadingAttachments
                    }
                    onClick={() => {
                      if (input.length > 0) {
                        setThinking(1);
                        handleSearch();
                      }
                    }}
                  >
                    <IconFasong
                      sx={{
                        fontSize: 16,
                        color:
                          input.length > 0 ? 'primary.main' : 'text.disabled',
                      }}
                    />
                  </IconButton>
                )}
              </Box>
            </StyledActionButtonStack>
          </StyledInputWrapper>
        </StyledInputContainer>
        {/* 模糊搜索建议列表 */}
        {showFuzzySuggestions &&
          fuzzySuggestions.length > 0 &&
          conversation.length === 0 && (
            <StyledFuzzySuggestionsStack gap={0.5}>
              {fuzzySuggestions.map((suggestion, index) => (
                <StyledFuzzySuggestionItem
                  key={index}
                  onClick={() => handleFuzzySuggestionClick(suggestion)}
                >
                  {highlightMatch(suggestion, input)}
                </StyledFuzzySuggestionItem>
              ))}
            </StyledFuzzySuggestionsStack>
          )}

        <Feedback
          open={open}
          onClose={() => setOpen(false)}
          onSubmit={handleScore}
          data={conversationItem}
        />
      </StyledMainContainer>
      {selectedFile && (
        <AssistantFilePanel
          file={selectedFile}
          files={
            artifacts.some(f => f.id === selectedFile.id)
              ? artifacts
              : [...artifacts, selectedFile]
          }
          onSelect={selectFile}
          onClose={() => setSelectedFile(null)}
          onRevise={reviseFile}
        />
      )}
    </Box>
  );
};

export default AiQaContent;
