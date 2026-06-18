import { getCollabEndpoint, NodeCollabResp } from '@/request/collabCompat';
import Collaboration from '@tiptap/extension-collaboration';
import { prosemirrorJSONToYXmlFragment } from '@tiptap/y-tiptap';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { WebsocketProvider } from 'y-websocket';
import * as Y from 'yjs';

type TiptapExtension = ReturnType<typeof Collaboration.configure>;
type ProseMirrorSchema = Parameters<typeof prosemirrorJSONToYXmlFragment>[0];

interface UseProseMirrorCollabExtensionsParams {
  collab?: NodeCollabResp;
  enabled: boolean;
  initialContent?: Record<string, unknown>;
}

export const useProseMirrorCollabExtensions = ({
  collab,
  enabled,
  initialContent,
}: UseProseMirrorCollabExtensionsParams) => {
  const extensionInjectionSupported = true;
  const providerRef = useRef<WebsocketProvider | null>(null);
  const docRef = useRef<Y.Doc | null>(null);
  const initializedRef = useRef(false);
  const [status, setStatus] = useState<
    'idle' | 'connecting' | 'connected' | 'disconnected'
  >('idle');

  const config = useMemo(() => {
    const endpoint = getCollabEndpoint(collab?.endpoint);
    if (
      !enabled ||
      !extensionInjectionSupported ||
      collab?.mode !== 'prose_mirror' ||
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
    extensionInjectionSupported,
  ]);

  const doc = useMemo(() => {
    if (!config) return null;
    initializedRef.current = false;
    return new Y.Doc();
  }, [config]);

  const extensions = useMemo<TiptapExtension[]>(() => {
    if (!doc) return [];
    return [
      Collaboration.configure({
        document: doc,
        field: 'content',
      }),
    ];
  }, [doc]);

  const initializeFromEditor = useCallback(
    (editor?: { schema?: ProseMirrorSchema }) => {
      if (
        !doc ||
        !initialContent ||
        !editor?.schema ||
        initializedRef.current
      ) {
        initializedRef.current = true;
        return;
      }
      const fragment = doc.getXmlFragment('content');
      if (fragment.length > 0) {
        initializedRef.current = true;
        return;
      }
      prosemirrorJSONToYXmlFragment(editor.schema, initialContent, fragment);
      initializedRef.current = true;
    },
    [doc, initialContent],
  );

  useEffect(() => {
    if (!config || !doc) {
      setStatus('idle');
      return;
    }

    setStatus('connecting');
    docRef.current = doc;
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
    providerRef.current = provider;

    const handleStatus = (event: { status: string }) => {
      setStatus(event.status === 'connected' ? 'connected' : 'connecting');
    };
    const handleDisconnect = () => {
      setStatus('disconnected');
    };

    provider.on('status', handleStatus);
    provider.on('connection-error', handleDisconnect);
    provider.on('connection-close', handleDisconnect);
    const connectTimer = window.setTimeout(
      () => {
        provider.connect();
      },
      initialContent && !initializedRef.current ? 100 : 0,
    );

    return () => {
      window.clearTimeout(connectTimer);
      provider.off('status', handleStatus);
      provider.off('connection-error', handleDisconnect);
      provider.off('connection-close', handleDisconnect);
      provider.destroy();
      doc.destroy();
      providerRef.current = null;
      docRef.current = null;
      setStatus('idle');
    };
  }, [config, doc, initialContent]);

  return {
    enabled: !!config,
    extensions,
    initializeFromEditor,
    status,
  };
};
