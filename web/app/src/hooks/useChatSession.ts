'use client';

import { useRef, useState } from 'react';
import httpRequest, { ContentType } from '@/request/httpClient';
import { ChatSessionMemory, type ChatSession } from '@/utils/chatSession';

// Browser history keeps credentials locally; shared URLs contain only the conversation ID.
export function useChatSession(scope: string) {
  const [conversationId, setConversationId] = useState('');
  const memory = useRef<ChatSessionMemory | null>(null);
  if (!memory.current) {
    memory.current = new ChatSessionMemory(
      scope,
      () => sessionStorage,
      setConversationId,
      () => localStorage,
    );
  }
  const session = memory.current;

  const reset = () => {
    session.reset();
    const url = new URL(window.location.href);
    url.searchParams.delete('cid');
    url.searchParams.delete('ask');
    window.history.replaceState(null, '', url.toString());
  };

  const resolve = () =>
    session.resolve((id, nonce) =>
      httpRequest<{ data: ChatSession }>({
        path: '/share/v1/conversation/resume',
        method: 'POST',
        body: { id, nonce },
        type: ContentType.Json,
        format: 'json',
        isAlert: false,
      }),
    );

  return {
    conversationId,
    restore: (id: string) => session.restore(id),
    reset,
    acceptEvent: (type: string, content: string) =>
      session.acceptEvent(type, content),
    resolve,
    historyCredentials: () => session.historyCredentials(),
    generation: session.generation,
  };
}
