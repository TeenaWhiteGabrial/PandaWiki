import { message as antdMessage } from 'antd';
import type { NoticeType } from 'antd/es/message/interface';
import type React from 'react';

type MessageFn = (
  content: React.ReactNode,
  duration?: number,
  icon?: boolean,
) => void;

type Notify = Record<NoticeType, MessageFn> & {
  destroy: () => void;
};

const createMessage =
  (type: NoticeType): MessageFn =>
  (content, duration, icon) => {
    antdMessage.open({
      type,
      content,
      duration,
      icon: icon === false ? null : undefined,
    });
  };

export const message: Notify = {
  success: createMessage('success'),
  error: createMessage('error'),
  warning: createMessage('warning'),
  info: createMessage('info'),
  loading: createMessage('loading'),
  destroy: () => antdMessage.destroy(),
};

export default message;
