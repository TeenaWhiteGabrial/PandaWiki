import { NodeCollabResp } from '@/request/collabCompat';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { WebsocketProvider } from 'y-websocket';
import * as Y from 'yjs';
import {
  buildEditorBlocks,
  EditorBlockItem,
  getEditorBlockContent,
  getBlockRoomId,
  parseEditorBlocks,
  replaceEditorBlockContent,
} from './editorBlocks';

type BlockCollabPlan = {
  enabled: boolean;
  activeBlockIds: string[];
  activeRoomIds: string[];
  blocks: EditorBlockItem[];
  blockCount: number;
  windowSize: number;
  status: 'idle' | 'connecting' | 'connected' | 'disconnected';
  connectedRoomCount: number;
  mergedContent?: Record<string, unknown>;
  mergedContentOrigin?: 'local' | 'remote';
  publishBlockContent: (
    blockId: string,
    editorContent?: Record<string, unknown>,
  ) => void;
};

type BlockRoomSession = {
  doc: Y.Doc;
  provider: WebsocketProvider;
  status: 'connecting' | 'connected' | 'disconnected';
  initialized: boolean;
  cleanup: () => void;
};

const clampWindow = (value?: number) => {
  if (!value || value < 1) return 6;
  return Math.min(value, 12);
};

const blockClientId =
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : Math.random().toString(36).slice(2);

export const useBlockCollabScheduler = ({
  collab,
  editorBlocks,
  editorContent,
  activeBlockId,
}: {
  collab?: NodeCollabResp;
  editorBlocks?: Record<string, unknown>;
  editorContent?: Record<string, unknown>;
  activeBlockId?: string;
}): BlockCollabPlan => {
  const providersRef = useRef<Map<string, BlockRoomSession>>(new Map());
  const [status, setStatus] = useState<
    'idle' | 'connecting' | 'connected' | 'disconnected'
  >('idle');
  const [connectedRoomCount, setConnectedRoomCount] = useState(0);
  const [mergedContent, setMergedContent] = useState<
    Record<string, unknown> | undefined
  >();
  const [mergedContentOrigin, setMergedContentOrigin] = useState<
    'local' | 'remote' | undefined
  >();
  const mergedContentRef = useRef<Record<string, unknown> | undefined>(
    undefined,
  );
  const parsedBlocks = useMemo(
    () => parseEditorBlocks(editorBlocks) || buildEditorBlocks(editorContent),
    [editorBlocks, editorContent],
  );
  const windowSize = clampWindow(collab?.block_window);
  const [anchorBlockId, setAnchorBlockId] = useState<string | undefined>(
    activeBlockId,
  );

  useEffect(() => {
    if (activeBlockId) {
      setAnchorBlockId(activeBlockId);
    }
  }, [activeBlockId]);

  useEffect(() => {
    mergedContentRef.current = editorContent;
    setMergedContent(undefined);
    setMergedContentOrigin(undefined);
  }, [editorContent]);

  const activeBlocks = useMemo<EditorBlockItem[]>(() => {
    if (
      !parsedBlocks?.blocks.length ||
      collab?.mode !== 'block' ||
      !collab.enabled
    )
      return [];
    const anchorIndex = Math.max(
      0,
      parsedBlocks.blocks.findIndex(block => block.id === anchorBlockId),
    );
    const halfWindow = Math.floor(windowSize / 2);
    const start = Math.max(0, anchorIndex - halfWindow);
    const end = Math.min(parsedBlocks.blocks.length, start + windowSize);
    return parsedBlocks.blocks.slice(start, end);
  }, [anchorBlockId, collab?.enabled, collab?.mode, parsedBlocks, windowSize]);

  const enabled =
    collab?.mode === 'block' && !!collab.enabled && !!collab.block_room_prefix;
  const activeBlockIds = useMemo(
    () => activeBlocks.map(block => block.id),
    [activeBlocks],
  );
  const activeRoomIds = useMemo(
    () =>
      collab?.block_room_prefix && collab.mode === 'block' && collab.enabled
        ? activeBlocks.map(block =>
            getBlockRoomId(collab.block_room_prefix!, block.id),
          )
        : [],
    [activeBlocks, collab?.block_room_prefix, collab?.enabled, collab?.mode],
  );
  const activeRoomKey = activeRoomIds.join('\n');
  const activeBlockByRoomId = useMemo(() => {
    const map = new Map<string, EditorBlockItem>();
    if (
      !collab?.block_room_prefix ||
      collab.mode !== 'block' ||
      !collab.enabled
    )
      return map;
    for (const block of activeBlocks) {
      map.set(getBlockRoomId(collab.block_room_prefix, block.id), block);
    }
    return map;
  }, [activeBlocks, collab?.block_room_prefix, collab?.enabled, collab?.mode]);

  const initializeBlockRoom = useCallback(
    (roomId: string) => {
      const session = providersRef.current.get(roomId);
      const block = activeBlockByRoomId.get(roomId);
      if (!session || !block || session.initialized) return;
      session.initialized = true;
      const sharedBlock = session.doc.getMap('block');
      if (sharedBlock.size > 0) return;
      const content = getEditorBlockContent(editorContent, block);
      if (!content) return;
      session.doc.transact(() => {
        sharedBlock.set('id', block.id);
        sharedBlock.set('type', block.type);
        sharedBlock.set('order', block.order);
        sharedBlock.set('content_hash', block.content_hash);
        sharedBlock.set('base_hash', block.content_hash);
        sharedBlock.set('revision', 1);
        sharedBlock.set('client_id', blockClientId);
        sharedBlock.set('content', content);
        sharedBlock.set('initialized_at', new Date().toISOString());
      }, 'block-initializer');
    },
    [activeBlockByRoomId, editorContent],
  );

  const applyBlockRoomContent = useCallback(
    (roomId: string, origin: unknown = 'remote') => {
      if (origin === 'local-block-editor' || origin === 'block-initializer') {
        return;
      }
      const session = providersRef.current.get(roomId);
      const block = activeBlockByRoomId.get(roomId);
      if (!session || !block) return;
      const sharedBlock = session.doc.getMap('block');
      const content = sharedBlock.get('content');
      if (!content || typeof content !== 'object') return;
      const nextContent = replaceEditorBlockContent(
        mergedContentRef.current || editorContent,
        block,
        content as Record<string, unknown>,
      );
      if (!nextContent) return;
      mergedContentRef.current = nextContent;
      setMergedContent(nextContent);
      setMergedContentOrigin('remote');
    },
    [activeBlockByRoomId, editorContent],
  );

  const publishBlockContent = useCallback(
    (blockId: string, nextEditorContent?: Record<string, unknown>) => {
      if (
        !collab?.block_room_prefix ||
        collab.mode !== 'block' ||
        !collab.enabled
      )
        return;
      const roomId = getBlockRoomId(collab.block_room_prefix, blockId);
      const session = providersRef.current.get(roomId);
      const block = activeBlockByRoomId.get(roomId);
      if (!session || !block) return;
      const content = getEditorBlockContent(nextEditorContent, block);
      if (!content) return;
      const sharedBlock = session.doc.getMap('block');
      const remoteHash = sharedBlock.get('content_hash');
      const remoteClientId = sharedBlock.get('client_id');
      if (
        remoteHash &&
        remoteHash !== block.content_hash &&
        remoteClientId !== blockClientId
      ) {
        sharedBlock.set('conflict', {
          local_base_hash: block.content_hash,
          remote_hash: remoteHash,
          detected_at: new Date().toISOString(),
          client_id: blockClientId,
        });
        return;
      }
      const nextRevision =
        typeof sharedBlock.get('revision') === 'number'
          ? Number(sharedBlock.get('revision')) + 1
          : 1;
      session.doc.transact(() => {
        sharedBlock.set('id', block.id);
        sharedBlock.set('type', block.type);
        sharedBlock.set('order', block.order);
        sharedBlock.set('content_hash', block.content_hash);
        sharedBlock.set('base_hash', remoteHash || block.content_hash);
        sharedBlock.set('revision', nextRevision);
        sharedBlock.set('client_id', blockClientId);
        sharedBlock.set('content', content);
        sharedBlock.set('updated_at', new Date().toISOString());
        sharedBlock.delete('conflict');
      }, 'local-block-editor');
      const merged = replaceEditorBlockContent(
        mergedContentRef.current || nextEditorContent || editorContent,
        block,
        content,
      );
      if (merged) {
        mergedContentRef.current = merged;
        setMergedContent(merged);
        setMergedContentOrigin('local');
      }
    },
    [
      activeBlockByRoomId,
      collab?.block_room_prefix,
      collab?.enabled,
      collab?.mode,
      editorContent,
    ],
  );

  const refreshConnectionState = () => {
    const sessions = Array.from(providersRef.current.values());
    setConnectedRoomCount(
      sessions.filter(session => session.status === 'connected').length,
    );
    if (sessions.length === 0) {
      setStatus('idle');
      return;
    }
    if (sessions.some(session => session.status === 'disconnected')) {
      setStatus('disconnected');
      return;
    }
    if (sessions.every(session => session.status === 'connected')) {
      setStatus('connected');
      return;
    }
    setStatus('connecting');
  };

  useEffect(() => {
    if (!enabled || !collab?.endpoint || !collab.auth_token || !activeRoomKey) {
      for (const { cleanup, doc, provider } of providersRef.current.values()) {
        cleanup();
        provider.destroy();
        doc.destroy();
      }
      providersRef.current.clear();
      setConnectedRoomCount(0);
      setStatus('idle');
      return;
    }

    setStatus('connecting');
    const nextRoomIds = activeRoomKey ? activeRoomKey.split('\n') : [];
    const activeRoomSet = new Set(nextRoomIds);
    for (const [roomId, session] of providersRef.current.entries()) {
      if (!activeRoomSet.has(roomId)) {
        session.cleanup();
        session.provider.destroy();
        session.doc.destroy();
        providersRef.current.delete(roomId);
      }
    }

    for (const roomId of nextRoomIds) {
      if (providersRef.current.has(roomId)) continue;
      const doc = new Y.Doc();
      const provider = new WebsocketProvider(collab.endpoint, roomId, doc, {
        connect: false,
        params: {
          token: collab.auth_token,
          roomId,
        },
      });
      const handleStatus = (event: { status: string }) => {
        const session = providersRef.current.get(roomId);
        if (!session) return;
        if (event.status === 'connected') {
          session.status = 'connected';
        } else {
          session.status = 'connecting';
        }
        refreshConnectionState();
      };
      const handleSync = (synced: boolean) => {
        if (synced) {
          initializeBlockRoom(roomId);
          applyBlockRoomContent(roomId);
        }
      };
      const handleBlockChange = (
        _event: unknown,
        transaction: Y.Transaction,
      ) => {
        applyBlockRoomContent(roomId, transaction.origin);
      };
      const handleDisconnect = () => {
        const session = providersRef.current.get(roomId);
        if (!session) return;
        session.status = 'disconnected';
        refreshConnectionState();
      };
      provider.on('status', handleStatus);
      provider.on('sync', handleSync);
      provider.on('connection-error', handleDisconnect);
      provider.on('connection-close', handleDisconnect);
      doc.getMap('block').observe(handleBlockChange);
      const cleanup = () => {
        doc.getMap('block').unobserve(handleBlockChange);
        provider.off('status', handleStatus);
        provider.off('sync', handleSync);
        provider.off('connection-error', handleDisconnect);
        provider.off('connection-close', handleDisconnect);
      };
      providersRef.current.set(roomId, {
        cleanup,
        doc,
        initialized: false,
        provider,
        status: 'connecting',
      });
      provider.connect();
    }

    refreshConnectionState();
  }, [
    activeRoomKey,
    applyBlockRoomContent,
    collab?.auth_token,
    collab?.endpoint,
    enabled,
    initializeBlockRoom,
  ]);

  useEffect(() => {
    const sessions = providersRef.current;
    return () => {
      for (const { cleanup, doc, provider } of sessions.values()) {
        cleanup();
        provider.destroy();
        doc.destroy();
      }
      sessions.clear();
    };
  }, []);

  return {
    enabled,
    activeBlockIds,
    activeRoomIds,
    blocks: parsedBlocks?.blocks || [],
    blockCount: parsedBlocks?.block_count || 0,
    windowSize,
    status,
    connectedRoomCount,
    mergedContent,
    mergedContentOrigin,
    publishBlockContent,
  };
};
