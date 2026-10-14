import { ChatConversationPair } from '@/api';
import { getApiV1ConversationDetail } from '@/request/Conversation';
import { DomainConversationDetailResp } from '@/request/types';
import type { DomainChatTrace } from '@/request/types';
import Avatar from '@/components/Avatar';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import Card from '@/components/Card';
import MarkDown from '@/components/MarkDown';
import { useAppSelector } from '@/store';
import { getBasePath } from '@/utils/getBasePath';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Stack,
  useTheme,
  styled,
  alpha,
  Typography,
} from '@mui/material';
import { Ellipsis, Modal, Image } from '@/components/ui';
import { useEffect, useState } from 'react';
import { IconDitu_diqiu } from '@panda-wiki/icons';

const handleThinkingContent = (content: string) => {
  const thinkRegex = /<think>([\s\S]*?)(?:<\/think>|$)/g;
  const thinkMatches = [];
  let match;
  while ((match = thinkRegex.exec(content)) !== null) {
    thinkMatches.push(match[1]);
  }

  let answerContent = content.replace(/<think>[\s\S]*?<\/think>/g, '');
  answerContent = answerContent.replace(/<think>[\s\S]*$/, '');

  return {
    thinkingContent: thinkMatches.join(''),
    answerContent: answerContent,
  };
};

const chatTraceStepLabels: Record<string, string> = {
  request_received: '接收问题',
  save_user_question: '保存用户问题',
  load_chat_model: '读取推理模型配置',
  load_conversation_history: '读取历史会话',
  load_chat_prompt: '读取问答提示词',
  load_knowledge_base_for_retrieval: '准备检索知识库',
  load_question_filter: '加载问题过滤规则',
  input_filter: '问题检查',
  load_knowledge_base: '读取知识库',
  load_user_permissions: '读取用户等级',
  build_access_context: '生成访问权限',
  vector_retrieval: '向量检索',
  local_fallback_retrieval: '本地补充检索',
  document_resolution: '匹配文档和片段',
  permission_filter: '文档权限筛选',
  model_input: '发送给模型的上下文（超长字段会截断）',
  format_model_messages: '组装模型输入',
  prepare_model_context: '准备模型上下文',
  model_configuration: '配置模型',
  model_inference: '模型生成',
  final_answer: '最终回答',
  save_answer: '保存回答',
  update_model_usage: '更新模型用量',
};

const chatTraceStatusLabels: Record<string, string> = {
  in_progress: '处理中',
  completed: '已完成',
  failed: '失败',
  interrupted: '已中断',
  blocked: '已拦截',
  running: '进行中',
  skipped: '已跳过',
  completed_with_warnings: '已完成，但有权限读取异常',
};

const formatTraceTime = (value?: string) =>
  value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—';

const ChatTracePanel = ({ trace }: { trace: DomainChatTrace }) => (
  <StyledAccordion>
    <StyledAccordionSummary
      expandIcon={<ExpandMoreIcon sx={{ fontSize: 16 }} />}
    >
      <Typography variant='body2' sx={{ fontSize: 12, fontWeight: 600 }}>
        排查回答过程 ·{' '}
        {chatTraceStatusLabels[trace.status || ''] ||
          trace.status ||
          '未知状态'}{' '}
        · 追踪编号 {trace.trace_id}
      </Typography>
    </StyledAccordionSummary>
    <StyledAccordionDetails>
      <Stack gap={1.5}>
        <Typography variant='caption' color='text.secondary'>
          日志包含用户标识、问题、检索片段和模型上下文，保留 30
          天；单段过长或历史消息过多时会截断。
        </Typography>
        <Typography variant='caption' color='text.secondary'>
          请求开始：{formatTraceTime(trace.started_at)} · 结束：
          {formatTraceTime(trace.finished_at)}
        </Typography>
        {(trace.steps || []).map((step, index) => (
          <Box
            key={`${step.name}-${index}`}
            sx={{
              borderLeft: '2px solid',
              borderColor:
                step.status === 'failed' || step.status === 'blocked'
                  ? 'error.main'
                  : step.status === 'completed_with_warnings'
                    ? 'warning.main'
                    : 'divider',
              pl: 1.5,
            }}
          >
            <Stack direction='row' justifyContent='space-between' gap={1}>
              <Typography variant='body2' sx={{ fontWeight: 600 }}>
                {chatTraceStepLabels[step.name || ''] ||
                  step.name ||
                  '处理步骤'}
              </Typography>
              <Typography
                variant='caption'
                color={
                  step.status === 'failed' || step.status === 'blocked'
                    ? 'error.main'
                    : step.status === 'completed_with_warnings'
                      ? 'warning.main'
                      : 'text.secondary'
                }
              >
                {chatTraceStatusLabels[step.status || ''] ||
                  step.status ||
                  '未知'}
                {step.duration_ms ? ` · ${step.duration_ms} ms` : ''}
              </Typography>
            </Stack>
            <Typography variant='caption' color='text.secondary'>
              {formatTraceTime(step.started_at)} →{' '}
              {formatTraceTime(step.finished_at)}
            </Typography>
            {step.error && (
              <Typography variant='body2' color='error.main' sx={{ mt: 0.5 }}>
                {step.error}
              </Typography>
            )}
            {step.input && <TracePayload title='输入' value={step.input} />}
            {step.output && <TracePayload title='输出' value={step.output} />}
          </Box>
        ))}
      </Stack>
    </StyledAccordionDetails>
  </StyledAccordion>
);

const TracePayload = ({
  title,
  value,
}: {
  title: string;
  value: Record<string, unknown>;
}) => (
  <Box sx={{ mt: 0.75 }}>
    <Typography variant='caption' color='text.secondary'>
      {title}
    </Typography>
    <Box
      component='pre'
      sx={{
        m: 0,
        mt: 0.25,
        p: 1,
        maxHeight: 320,
        overflow: 'auto',
        whiteSpace: 'pre-wrap',
        overflowWrap: 'anywhere',
        borderRadius: 1,
        bgcolor: 'background.paper3',
        fontSize: 11,
      }}
    >
      {JSON.stringify(value, null, 2)}
    </Box>
  </Box>
);

export const StyledConversationItem = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(2),
}));

// 聊天气泡相关组件
export const StyledUserBubble = styled(Box)(({ theme }) => ({
  alignSelf: 'flex-end',
  maxWidth: '75%',
  padding: theme.spacing(1, 2),
  borderRadius: '10px 10px 0px 10px',
  backgroundColor: theme.palette.primary.main,
  color: theme.palette.primary.contrastText,
  fontSize: 14,
  wordBreak: 'break-word',
}));

export const StyledAiBubble = styled(Box)(({ theme }) => ({
  alignSelf: 'flex-start',
  display: 'flex',
  flexDirection: 'column',
  width: '100%',
  gap: theme.spacing(3),
}));

export const StyledAiBubbleContent = styled(Box)(() => ({
  wordBreak: 'break-word',
}));

// 对话相关组件
export const StyledAccordion = styled(Accordion)(() => ({
  padding: 0,
  border: 'none',
  '&:before': {
    content: '""',
    height: 0,
  },
  background: 'transparent',
  backgroundImage: 'none',
}));

export const StyledAccordionSummary = styled(AccordionSummary)(({ theme }) => ({
  paddingLeft: theme.spacing(2),
  paddingRight: theme.spacing(2),
  paddingTop: theme.spacing(1),
  paddingBottom: theme.spacing(1),
  userSelect: 'text',
  borderRadius: '10px',
  backgroundColor: theme.palette.background.paper3,
  border: '1px solid',
  borderColor: theme.palette.divider,
}));

export const StyledAccordionDetails = styled(AccordionDetails)(({ theme }) => ({
  padding: theme.spacing(2),
  borderTop: 'none',
}));

export const StyledQuestionText = styled(Box)(() => ({
  fontWeight: '700',
  fontSize: 16,
  lineHeight: '24px',
  wordBreak: 'break-all',
}));

// 搜索结果相关组件
export const StyledChunkAccordion = styled(Accordion)(({ theme }) => ({
  backgroundImage: 'none',
  background: 'transparent',
  border: 'none',
  padding: 0,
}));

export const StyledChunkAccordionSummary = styled(AccordionSummary)(
  ({ theme }) => ({
    justifyContent: 'flex-start',
    gap: theme.spacing(2),
    '.MuiAccordionSummary-content': {
      flexGrow: 0,
    },
  }),
);

export const StyledChunkAccordionDetails = styled(AccordionDetails)(
  ({ theme }) => ({
    paddingTop: 0,
    paddingLeft: theme.spacing(2),
    borderTop: 'none',
    borderLeft: '1px solid',
    borderColor: theme.palette.divider,
  }),
);

export const StyledChunkItem = styled(Box)(({ theme }) => ({
  cursor: 'pointer',
  '&:hover': {
    '.hover-primary': {
      color: theme.palette.primary.main,
    },
  },
}));

// 思考过程相关组件
export const StyledThinkingAccordion = styled(Accordion)(({ theme }) => ({
  backgroundColor: 'transparent',
  border: 'none',
  padding: 0,
  paddingBottom: theme.spacing(2),
  '&:before': {
    content: '""',
    height: 0,
  },
}));

export const StyledThinkingAccordionSummary = styled(AccordionSummary)(
  ({ theme }) => ({
    justifyContent: 'flex-start',
    gap: theme.spacing(2),
    '.MuiAccordionSummary-content': {
      flexGrow: 0,
    },
  }),
);

export const StyledThinkingAccordionDetails = styled(AccordionDetails)(
  ({ theme }) => ({
    paddingTop: 0,
    paddingLeft: theme.spacing(2),
    borderTop: 'none',
    borderLeft: '1px solid',
    borderColor: theme.palette.divider,
    '.markdown-body': {
      opacity: 0.75,
      fontSize: 12,
    },
  }),
);

const Detail = ({
  id,
  open,
  onClose,
}: {
  id: string;
  open: boolean;
  onClose: () => void;
}) => {
  const theme = useTheme();
  const { kb_id = '' } = useAppSelector(state => state.config);
  const [detail, setDetail] = useState<DomainConversationDetailResp | null>(
    null,
  );
  const [conversations, setConversations] = useState<
    ChatConversationPair[] | null
  >(null);

  const getDetail = () => {
    getApiV1ConversationDetail({ id, kb_id }).then(res => {
      setDetail(res);
      const pairs: ChatConversationPair[] = [];
      let currentPair: Partial<ChatConversationPair> = {};
      const appendUnansweredPair = () => {
        if (
          currentPair.user ||
          (currentPair.image_paths && currentPair.image_paths.length > 0)
        ) {
          pairs.push({
            user: currentPair.user || '',
            image_paths: currentPair.image_paths || [],
            assistant: '',
            thinking_content: '',
            created_at: '',
            info: {
              feedback_content: '',
              feedback_type: 0,
              score: 0,
            },
            trace: currentPair.trace,
          });
          currentPair = {};
        }
      };
      res.messages?.forEach(message => {
        if (message.role === 'user') {
          appendUnansweredPair();
          currentPair = {
            user: message.content,
            image_paths: message.image_paths,
            trace: res.chat_traces?.find(
              trace => trace.user_message_id === message.id,
            ),
          };
        } else if (message.role === 'assistant') {
          if (
            currentPair.user ||
            (currentPair.image_paths && currentPair.image_paths.length > 0)
          ) {
            const { thinkingContent, answerContent } = handleThinkingContent(
              message.content || '',
            );
            currentPair.assistant = answerContent;
            currentPair.trace =
              res.chat_traces?.find(
                trace => trace.assistant_message_id === message.id,
              ) || currentPair.trace;
            currentPair.thinking_content = thinkingContent;
            currentPair.created_at = message.created_at;
            // @ts-expect-error 类型不兼容
            currentPair.info = message.info;
            pairs.push(currentPair as ChatConversationPair);
            currentPair = {};
          }
        }
      });

      appendUnansweredPair();

      setConversations(pairs);
    });
  };

  useEffect(() => {
    if (open && id) getDetail();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, open]);

  return (
    <Modal
      title={
        <Ellipsis
          sx={{
            fontWeight: 'bold',
            fontSize: 20,
            lineHeight: '22px',
            width: 700,
          }}
        >
          问答记录
        </Ellipsis>
      }
      width={800}
      open={open}
      onCancel={onClose}
      footer={null}
    >
      {detail ? (
        <Box sx={{ fontSize: 14 }}>
          {(detail.references?.length || 0) > 0 && (
            <>
              <Stack
                direction={'row'}
                alignItems={'center'}
                gap={1}
                sx={{
                  fontWeight: 'bold',
                  mt: 2,
                  mb: 1,
                  '&::before': {
                    content: '""',
                    display: 'inline-block',
                    width: '4px',
                    height: '12px',
                    borderRadius: '2px',
                    backgroundColor: theme.palette.primary.main,
                  },
                }}
              >
                内容来源
              </Stack>
              <Card sx={{ p: 2, bgcolor: 'background.paper3' }}>
                {detail.references?.map((item, index) => (
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    key={index}
                  >
                    <Avatar
                      // @ts-expect-error 类型不兼容
                      src={item.favicon}
                      sx={{ width: 18, height: 18 }}
                      errorIcon={
                        <IconDitu_diqiu
                          sx={{ fontSize: 18, color: 'text.tertiary' }}
                        />
                      }
                    />
                    <Ellipsis>
                      <Box
                        component={'a'}
                        href={item.url}
                        target='_blank'
                        sx={{
                          color: 'text.primary',
                          '&:hover': { color: 'primary.main' },
                        }}
                      >
                        {/* @ts-expect-error 类型不兼容 */}
                        {item.title}
                      </Box>
                    </Ellipsis>
                  </Stack>
                ))}
              </Card>
            </>
          )}
          <Stack gap={2}>
            {conversations &&
              conversations.map((item, index) => (
                <StyledConversationItem key={index}>
                  {item.image_paths && item.image_paths.length > 0 && (
                    <Image.PreviewGroup>
                      <Stack
                        direction='row'
                        gap={1}
                        sx={{ alignSelf: 'flex-end' }}
                      >
                        {item.image_paths.map((url: string) => (
                          <Image
                            key={url}
                            alt={url}
                            src={getBasePath(url)}
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
                    </Image.PreviewGroup>
                  )}
                  {/* 用户问题气泡 - 右对齐 */}
                  {item.user && (
                    <StyledUserBubble>{item.user}</StyledUserBubble>
                  )}

                  {/* AI回答气泡 - 左对齐 */}
                  <StyledAiBubble>
                    {item.trace && <ChatTracePanel trace={item.trace} />}
                    {/* 思考过程 */}
                    {!!item.thinking_content && (
                      <StyledThinkingAccordion defaultExpanded>
                        <StyledThinkingAccordionSummary
                          expandIcon={<ExpandMoreIcon sx={{ fontSize: 16 }} />}
                        >
                          <Stack direction='row' alignItems='center' gap={1}>
                            <Typography
                              variant='body2'
                              sx={theme => ({
                                fontSize: 12,
                                color: alpha(theme.palette.text.primary, 0.5),
                              })}
                            >
                              已思考
                            </Typography>
                          </Stack>
                        </StyledThinkingAccordionSummary>

                        <StyledThinkingAccordionDetails>
                          <MarkDown content={item.thinking_content || ''} />
                        </StyledThinkingAccordionDetails>
                      </StyledThinkingAccordion>
                    )}

                    {/* AI回答内容 */}
                    <StyledAiBubbleContent>
                      <MarkDown content={item.assistant} />
                    </StyledAiBubbleContent>
                  </StyledAiBubble>
                </StyledConversationItem>
              ))}
          </Stack>
        </Box>
      ) : (
        <Box></Box>
      )}
    </Modal>
  );
};

export default Detail;
