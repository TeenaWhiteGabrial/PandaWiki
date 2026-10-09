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
import SSEClient, { SSEHttpError } from '@/utils/fetch';
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
}

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

const AnswerStatus = {
  1: '正在搜索结果...',
  2: '思考中...',
  3: '正在回答',
  4: '',
};

const getChatAuthHeaders = (): Record<string, string> => {
  if (typeof window === 'undefined') {
    return {};
  }
  const token = window.localStorage.getItem('panda_wiki_token');
  if (!token) {
    return {};
  }
  return {
    Authorization: `Bearer ${token}`,
  };
};

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
}> = ({ hotSearch, placeholder, inputRef }) => {
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
      message.error('验证失败');
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
      message.error((error as Error)?.message || '恢复会话失败，请重试');
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
            setConversation(prev => {
              const newConversation = [...prev];
              const lastConversation =
                newConversation[newConversation.length - 1];
              if (lastConversation) {
                lastConversation.a =
                  lastConversation.a +
                  (content
                    ? `\n\n回答出现错误：<error>${content}</error>`
                    : '\n\n回答出现错误，请重试');
              }
              return newConversation;
            });
            if (content) message.error(content);
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
    if (loading || restoringHistory || !q.trim()) return;
    if (reset) chatSession.reset();
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
    void chatAnswer(q);
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
    await postShareV1ChatFeedback(data);
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
        if (error instanceof SSEHttpError && error.status === 401) {
          const current = window.location;
          window.location.href = `${basePath}/auth/login?redirect=${encodeURIComponent(current.pathname + current.search)}`;
          return;
        }
        message.error(error.message || '请求失败');
      },
      onCancel: () => {
        setLoading(false);
        setThinking(4);
        setConversation(prev => {
          const newConversation = [...prev];
          const lastConversation = newConversation[newConversation.length - 1];
          if (lastConversation) {
            lastConversation.a =
              lastConversation.a + '\n\n<error>Request canceled</error>';
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
      setConversation(conversation);
      setShouldAutoScroll(false);
      return true;
    } catch (error) {
      if (version !== chatSession.generation.current) return false;
      setRestoringHistory(false);
      chatSession.reset();
      message.error((error as Error)?.message || '加载历史会话失败，请重试');
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
    <StyledMainContainer className={palette.mode === 'dark' ? 'md-dark' : ''}>
      <ChatHistoryToolbar
        conversationId={conversationId}
        disabled={restoringHistory}
        credentials={chatSession.historyCredentials}
        onSelect={loadHistory}
        onNew={onReset}
      />
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
              src={getImagePath(kbDetail?.settings?.icon || Logo.src, basePath)}
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
                  <Stack direction='row' gap={1} sx={{ alignSelf: 'flex-end' }}>
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
                        共找到 {item.chunk_result.length} 个结果
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
                  <MarkDown2 content={item.a} autoScroll={false} />
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
                            <IconDianzanXuanzhong1 sx={{ cursor: 'pointer' }} />
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
          <StyledTextField
            fullWidth
            multiline
            rows={2}
            disabled={loading || restoringHistory}
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
            placeholder={restoringHistory ? '正在恢复历史会话...' : placeholder}
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
                  disabled={input.length === 0 || restoringHistory}
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
  );
};

export default AiQaContent;
