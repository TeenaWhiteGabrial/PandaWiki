import { getCollabEndpoint, NodeCollabResp } from '@/request/collabCompat';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { WebsocketProvider } from 'y-websocket';
import * as Y from 'yjs';
import {
  buildMarkdownBlocks,
  hashMarkdownBlock,
  joinMarkdownBlocks,
  MarkdownBlockItem,
} from './markdownBlocks';

type MarkdownBlockCollabStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'disconnected';

type SharedMarkdownBlock = {
  id: string;
  order: number;
  content: string;
  content_hash: string;
  text_length: number;
  client_id: string;
  updated_at?: string;
};

interface UseMarkdownBlockCollabParams {
  collab?: NodeCollabResp;
  enabled: boolean;
  initialContent: string;
  currentContent: string;
  onRemoteChange: (value: string) => void;
}

const markdownBlockClientId =
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : Math.random().toString(36).slice(2);

const toSharedBlock = (block: MarkdownBlockItem): SharedMarkdownBlock => ({
  id: block.id,
  order: block.order,
  content: block.content,
  content_hash: block.content_hash,
  text_length: block.text_length,
  client_id: markdownBlockClientId,
  updated_at: new Date().toISOString(),
});

const readBlocks = (sharedBlocks: Y.Map<SharedMarkdownBlock>) =>
  Array.from(sharedBlocks.values())
    .filter(
      (block): block is SharedMarkdownBlock =>
        !!block &&
        typeof block.id === 'string' &&
        typeof block.order === 'number' &&
        typeof block.content === 'string',
    )
    .sort((a, b) => a.order - b.order)
    .map(
      (block): MarkdownBlockItem => ({
        id: block.id,
        order: block.order,
        content: block.content,
        content_hash:
          typeof block.content_hash === 'string'
            ? block.content_hash
            : hashMarkdownBlock(block.content),
        text_length:
          typeof block.text_length === 'number'
            ? block.text_length
            : block.content.length,
      }),
    );

const writeBlocks = (
  doc: Y.Doc,
  sharedBlocks: Y.Map<SharedMarkdownBlock>,
  content: string,
  origin: string,
) => {
  const nextBlocks = buildMarkdownBlocks(content);
  const nextIds = new Set(nextBlocks.map(block => block.id));
  doc.transact(() => {
    for (const key of Array.from(sharedBlocks.keys())) {
      if (!nextIds.has(key)) {
        sharedBlocks.delete(key);
      }
    }
    for (const block of nextBlocks) {
      const existing = sharedBlocks.get(block.id);
      if (
        existing &&
        existing.content_hash === block.content_hash &&
        existing.order === block.order
      ) {
        continue;
      }
      sharedBlocks.set(block.id, toSharedBlock(block));
    }
  }, origin);
};

export const useMarkdownBlockCollab = ({
  collab,
  enabled,
  initialContent,
  currentContent,
  onRemoteChange,
}: UseMarkdownBlockCollabParams) => {
  const docRef = useRef<Y.Doc | null>(null);
  const blocksRef = useRef<Y.Map<SharedMarkdownBlock> | null>(null);
  const applyingRemoteRef = useRef(false);
  const initializedRef = useRef(false);
  const contentRef = useRef(currentContent);
  const connectedRef = useRef(false);
  const [status, setStatus] = useState<MarkdownBlockCollabStatus>('idle');

  useEffect(() => {
    contentRef.current = currentContent;
  }, [currentContent]);

  const config = useMemo(() => {
    const endpoint = getCollabEndpoint(collab?.endpoint);
    if (
      !enabled ||
      collab?.mode !== 'markdown_block' ||
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

  const applyRemoteBlocks = useCallback(() => {
    const sharedBlocks = blocksRef.current;
    if (!sharedBlocks) return;
    const value = joinMarkdownBlocks(readBlocks(sharedBlocks));
    if (value === contentRef.current) return;
    applyingRemoteRef.current = true;
    try {
      onRemoteChange(value);
    } finally {
      window.setTimeout(() => {
        applyingRemoteRef.current = false;
      }, 0);
    }
  }, [onRemoteChange]);

  useEffect(() => {
    if (!config) {
      docRef.current = null;
      blocksRef.current = null;
      initializedRef.current = false;
      connectedRef.current = false;
      setStatus('idle');
      return;
    }

    const doc = new Y.Doc();
    const sharedBlocks = doc.getMap<SharedMarkdownBlock>('markdown_blocks');
    docRef.current = doc;
    blocksRef.current = sharedBlocks;
    initializedRef.current = false;
    connectedRef.current = false;
    setStatus('connecting');

    const seedLocalContent = () => {
      if (initializedRef.current) return;
      initializedRef.current = true;
      const localValue = contentRef.current || initialContent;
      if (sharedBlocks.size === 0) {
        writeBlocks(
          doc,
          sharedBlocks,
          localValue,
          'markdown-block-initializer',
        );
        return;
      }

      const remoteValue = joinMarkdownBlocks(readBlocks(sharedBlocks));
      if (
        localValue &&
        localValue !== initialContent &&
        remoteValue !== localValue
      ) {
        writeBlocks(doc, sharedBlocks, localValue, 'markdown-block-local');
        return;
      }

      if (remoteValue !== localValue) {
        applyRemoteBlocks();
      }
    };
    const handleStatus = (event: { status: string }) => {
      if (event.status === 'connected') {
        connectedRef.current = true;
        setStatus('connected');
      } else if (!connectedRef.current) {
        setStatus('connecting');
      }
      if (event.status === 'connected') {
        window.setTimeout(seedLocalContent, 0);
      }
    };
    const handleSync = () => seedLocalContent();
    const handleDisconnect = () => {
      connectedRef.current = false;
      setStatus('disconnected');
    };
    const handleBlocksChange = (
      _event: unknown,
      transaction: Y.Transaction,
    ) => {
      if (
        transaction.origin === 'markdown-block-local' ||
        transaction.origin === 'markdown-block-initializer'
      ) {
        return;
      }
      applyRemoteBlocks();
    };

    sharedBlocks.observe(handleBlocksChange);
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

    provider.on('status', handleStatus);
    provider.on('sync', handleSync);
    provider.on('connection-error', handleDisconnect);
    provider.on('connection-close', handleDisconnect);
    provider.connect();

    return () => {
      provider.off('status', handleStatus);
      provider.off('sync', handleSync);
      provider.off('connection-error', handleDisconnect);
      provider.off('connection-close', handleDisconnect);
      sharedBlocks.unobserve(handleBlocksChange);
      provider.destroy();
      doc.destroy();
      docRef.current = null;
      blocksRef.current = null;
      initializedRef.current = false;
      connectedRef.current = false;
      setStatus('idle');
    };
  }, [applyRemoteBlocks, config, initialContent]);

  const applyLocalChange = useCallback((value: string) => {
    if (applyingRemoteRef.current) return;
    const doc = docRef.current;
    const sharedBlocks = blocksRef.current;
    if (!doc || !sharedBlocks) return;
    writeBlocks(doc, sharedBlocks, value, 'markdown-block-local');
  }, []);

  return {
    enabled: !!config,
    status,
    applyLocalChange,
  };
};
