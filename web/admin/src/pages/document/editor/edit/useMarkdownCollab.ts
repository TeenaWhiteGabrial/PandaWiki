import { getCollabEndpoint, NodeCollabResp } from '@/request/collabCompat';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { WebsocketProvider } from 'y-websocket';
import * as Y from 'yjs';

type MarkdownCollabStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'disconnected';

const findTextPatch = (current: string, next: string) => {
  if (current === next) return null;
  let prefix = 0;
  const minLength = Math.min(current.length, next.length);
  while (prefix < minLength && current[prefix] === next[prefix]) {
    prefix += 1;
  }

  let currentSuffix = current.length;
  let nextSuffix = next.length;
  while (
    currentSuffix > prefix &&
    nextSuffix > prefix &&
    current[currentSuffix - 1] === next[nextSuffix - 1]
  ) {
    currentSuffix -= 1;
    nextSuffix -= 1;
  }

  return {
    index: prefix,
    deleteLength: currentSuffix - prefix,
    insertText: next.slice(prefix, nextSuffix),
  };
};

interface UseMarkdownCollabParams {
  collab?: NodeCollabResp;
  enabled: boolean;
  initialContent: string;
  onRemoteChange: (value: string) => void;
}

export const useMarkdownCollab = ({
  collab,
  enabled,
  initialContent,
  onRemoteChange,
}: UseMarkdownCollabParams) => {
  const textRef = useRef<Y.Text | null>(null);
  const applyingRemoteRef = useRef(false);
  const initializedRef = useRef(false);
  const connectedRef = useRef(false);
  const [status, setStatus] = useState<MarkdownCollabStatus>('idle');

  const config = useMemo(() => {
    const endpoint = getCollabEndpoint(collab?.endpoint);
    if (
      !enabled ||
      collab?.mode !== 'markdown' ||
      !collab.enabled ||
      !endpoint ||
      !collab.auth_token ||
      !collab.room_id
    ) {
      return null;
    }
    return {
      endpoint,
      roomId: collab.room_id,
      token: collab.auth_token,
    };
  }, [
    collab?.auth_token,
    collab?.enabled,
    collab?.endpoint,
    collab?.mode,
    collab?.room_id,
    enabled,
  ]);

  useEffect(() => {
    if (!config) {
      textRef.current = null;
      initializedRef.current = false;
      connectedRef.current = false;
      setStatus('idle');
      return;
    }

    const doc = new Y.Doc();
    const text = doc.getText('content');
    textRef.current = text;
    initializedRef.current = false;
    connectedRef.current = false;
    setStatus('connecting');

    const handleTextUpdate = () => {
      const value = text.toString();
      applyingRemoteRef.current = true;
      try {
        onRemoteChange(value);
      } finally {
        window.setTimeout(() => {
          applyingRemoteRef.current = false;
        }, 0);
      }
    };

    text.observe(handleTextUpdate);
    const provider = new WebsocketProvider(
      config.endpoint,
      config.roomId,
      doc,
      {
        connect: false,
        params: {
          token: config.token,
          roomId: config.roomId,
        },
      },
    );

    const seedLocalContent = () => {
      if (initializedRef.current) return;
      initializedRef.current = true;
      if (text.length === 0 && initialContent) {
        text.insert(0, initialContent);
      } else {
        handleTextUpdate();
      }
    };
    const handleStatus = (event: { status: string }) => {
      if (event.status === 'connected') {
        connectedRef.current = true;
        setStatus('connected');
        window.setTimeout(seedLocalContent, 0);
      } else if (!connectedRef.current) {
        setStatus('connecting');
      }
    };
    const handleDisconnect = () => {
      connectedRef.current = false;
      setStatus('disconnected');
    };

    provider.on('status', handleStatus);
    provider.on('sync', seedLocalContent);
    provider.on('connection-error', handleDisconnect);
    provider.on('connection-close', handleDisconnect);
    provider.connect();

    return () => {
      provider.off('status', handleStatus);
      provider.off('sync', seedLocalContent);
      provider.off('connection-error', handleDisconnect);
      provider.off('connection-close', handleDisconnect);
      text.unobserve(handleTextUpdate);
      provider.destroy();
      doc.destroy();
      textRef.current = null;
      initializedRef.current = false;
      connectedRef.current = false;
      setStatus('idle');
    };
  }, [config, initialContent, onRemoteChange]);

  const applyLocalChange = useCallback((value: string) => {
    if (applyingRemoteRef.current) return;
    const text = textRef.current;
    if (!text) return;
    const current = text.toString();
    const patch = findTextPatch(current, value);
    if (!patch) return;
    if (patch.deleteLength > 0) {
      text.delete(patch.index, patch.deleteLength);
    }
    if (patch.insertText) {
      text.insert(patch.index, patch.insertText);
    }
  }, []);

  return {
    enabled: !!config,
    status,
    applyLocalChange,
  };
};
