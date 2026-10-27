'use client';

import { useEffect, useState } from 'react';
import { Visibility, VisibilityOff } from '@mui/icons-material';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
} from '@mui/material';

export const PASSWORD_POLICY =
  '密码需为 12～72 字节，包含大写字母、小写字母、数字和特殊字符，且不能包含账号';
export function passwordError(
  password: string,
  account = '',
): string | undefined {
  const size = new TextEncoder().encode(password).length;
  if (
    size < 12 ||
    size > 72 ||
    !/[A-Z]/.test(password) ||
    !/[a-z]/.test(password) ||
    !/[0-9]/.test(password) ||
    !/[^A-Za-z0-9\s]/.test(password) ||
    /[\s\x00-\x1f\x7f]/.test(password)
  )
    return PASSWORD_POLICY;
  if (
    account.length >= 3 &&
    password.toLowerCase().includes(account.toLowerCase())
  )
    return PASSWORD_POLICY;
  return undefined;
}

export function PasswordChangeDialog({
  open,
  account,
  initialPassword = '',
  required = true,
  onSubmit,
  onCancel,
  onComplete,
}: {
  open: boolean;
  account: string;
  initialPassword?: string;
  required?: boolean;
  onSubmit: (currentPassword: string, newPassword: string) => Promise<unknown>;
  onCancel: () => Promise<void>;
  onComplete: () => void;
}) {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [visible, setVisible] = useState({
    current: false,
    next: false,
    confirmation: false,
  });
  useEffect(() => {
    setVisible({ current: false, next: false, confirmation: false });
    if (open) {
      setCurrent(initialPassword);
      setNext('');
      setConfirmation('');
      setError('');
    }
  }, [open, initialPassword]);
  const passwordToggle = (field: keyof typeof visible, label: string) => (
    <InputAdornment position='end'>
      <IconButton
        type='button'
        edge='end'
        aria-label={`${visible[field] ? '隐藏' : '显示'}${label}`}
        aria-pressed={visible[field]}
        disabled={loading}
        onMouseDown={event => event.preventDefault()}
        onClick={() =>
          setVisible(previous => ({ ...previous, [field]: !previous[field] }))
        }
      >
        {visible[field] ? <VisibilityOff /> : <Visibility />}
      </IconButton>
    </InputAdornment>
  );
  const submit = async () => {
    const invalid = passwordError(next, account);
    if (!current || invalid || next !== confirmation || current === next) {
      setError(
        !current
          ? '请输入当前密码'
          : invalid ||
              (current === next
                ? '新密码不能与当前密码相同'
                : '两次输入的新密码不一致'),
      );
      return;
    }
    setLoading(true);
    try {
      await onSubmit(current, next);
      setCurrent('');
      setNext('');
      setConfirmation('');
      onComplete();
    } catch (e) {
      setError(e instanceof Error ? e.message : '修改失败，请重试');
    } finally {
      setLoading(false);
    }
  };
  return (
    <Dialog open={open} maxWidth='sm' fullWidth disableEscapeKeyDown>
      <DialogTitle>{required ? '请修改初始密码' : '修改密码'}</DialogTitle>
      <DialogContent>
        <Stack gap={2} sx={{ pt: 1 }}>
          <Alert severity='info'>
            {required
              ? `账号 ${account} 需要先设置自己的密码，才能继续使用。`
              : '修改后，此账号的全部登录会话将失效。'}
            修改后请重新登录。
          </Alert>
          <TextField
            label='当前密码'
            value={current}
            type={visible.current ? 'text' : 'password'}
            slotProps={{
              input: { endAdornment: passwordToggle('current', '当前密码') },
            }}
            autoComplete='current-password'
            onChange={e => setCurrent(e.target.value)}
            disabled={loading}
          />
          <TextField
            label='新密码'
            value={next}
            type={visible.next ? 'text' : 'password'}
            slotProps={{
              input: { endAdornment: passwordToggle('next', '新密码') },
            }}
            autoComplete='new-password'
            helperText={PASSWORD_POLICY}
            onChange={e => setNext(e.target.value)}
            disabled={loading}
          />
          <TextField
            label='再次输入新密码'
            value={confirmation}
            type={visible.confirmation ? 'text' : 'password'}
            slotProps={{
              input: {
                endAdornment: passwordToggle(
                  'confirmation',
                  '再次输入的新密码',
                ),
              },
            }}
            autoComplete='new-password'
            onChange={e => setConfirmation(e.target.value)}
            disabled={loading}
            onKeyDown={e => {
              if (e.key === 'Enter') void submit();
            }}
          />
          {error && <Alert severity='error'>{error}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button disabled={loading} onClick={() => void onCancel()}>
          {required ? '退出登录' : '取消'}
        </Button>
        <Button
          variant='contained'
          disabled={loading}
          onClick={() => void submit()}
        >
          {loading ? '正在修改…' : '保存并重新登录'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
