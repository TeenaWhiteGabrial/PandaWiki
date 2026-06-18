import { Box, Stack } from '@mui/material';
import { useEffect, useRef, useState } from 'react';

export type CollabStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'disconnected'
  | 'disabled';

interface EditorCollaborationProps {
  status: CollabStatus;
  disabledReason?: string;
}

const statusText: Record<Exclude<CollabStatus, 'idle'>, string> = {
  connecting: '协同连接中',
  connected: '协同已连接',
  disconnected: '协同未连接',
  disabled: '协同已暂停',
};

const statusColor: Record<Exclude<CollabStatus, 'idle'>, string> = {
  connecting: 'warning.main',
  connected: 'success.main',
  disconnected: 'text.disabled',
  disabled: 'text.disabled',
};

const EditorCollaboration = ({
  status,
  disabledReason,
}: EditorCollaborationProps) => {
  const [displayStatus, setDisplayStatus] = useState<CollabStatus>(status);
  const connectedAtRef = useRef<number | null>(null);
  const timerRef = useRef<number | null>(null);

  useEffect(() => {
    if (timerRef.current) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    if (status === 'connected') {
      connectedAtRef.current = Date.now();
      setDisplayStatus('connected');
      return;
    }

    if (
      status === 'connecting' &&
      displayStatus === 'connected' &&
      connectedAtRef.current
    ) {
      const elapsed = Date.now() - connectedAtRef.current;
      if (elapsed < 2000) {
        timerRef.current = window.setTimeout(() => {
          setDisplayStatus('connecting');
          timerRef.current = null;
        }, 2000 - elapsed);
        return;
      }
    }

    setDisplayStatus(status);

    return () => {
      if (timerRef.current) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [displayStatus, status]);

  if (displayStatus === 'idle') return null;

  const color = statusColor[displayStatus];

  return (
    <Stack
      direction='row'
      alignItems='center'
      gap={0.75}
      sx={{ fontSize: 12, color }}
    >
      <Box
        sx={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          bgcolor: color,
        }}
      />
      <Box title={disabledReason}>{statusText[displayStatus]}</Box>
    </Stack>
  );
};

export default EditorCollaboration;
