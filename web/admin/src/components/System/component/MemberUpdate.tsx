import { passwordError, PASSWORD_POLICY } from '@panda-wiki/ui/security';
import Card from '@/components/Card';
import { Modal } from '@/components/ui';
import { putApiV1UserResetPassword } from '@/request/User';
import { V1UserListItemResp } from '@/request/types';
import { copyText, generatePassword } from '@/utils';
import { CheckCircle } from '@mui/icons-material';
import {
  Box,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
} from '@mui/material';
import { Visibility, VisibilityOff } from '@mui/icons-material';
import { IconShuaxin } from '@panda-wiki/icons';
import { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

type UpdateMemberProps = {
  user: V1UserListItemResp;
  refresh: () => void;
  type: 'reset' | 'update';
};

const MemberUpdate = ({ user, refresh, type }: UpdateMemberProps) => {
  const [updateOpen, setUpdateOpen] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
    setValue,
  } = useForm({
    defaultValues: {
      password: '',
      confirmPassword: '',
    },
  });

  const close = () => {
    setResetOpen(false);
    setUpdateOpen(false);
    setPassword('');
    setShowPassword(false);
    setShowConfirmPassword(false);
    reset();
    setLoading(false);
    refresh();
  };

  const copyUserInfo = () => {
    copyText(`用户名: ${user.account}\n密码: ${password}`, close);
  };

  const onSumbit = (data: { password: string; confirmPassword: string }) => {
    setLoading(true);
    putApiV1UserResetPassword({ id: user.id!, new_password: data.password })
      .then(() => {
        setPassword(data.password);
        setUpdateOpen(false);
        setResetOpen(true);
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    if (type === 'reset') {
      const password = generatePassword();
      setPassword(password);
    }
    setUpdateOpen(true);
  }, [user, type]);

  return (
    <>
      <Modal
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <CheckCircle sx={{ color: 'success.main' }} />
            密码修改成功
          </Stack>
        }
        open={resetOpen}
        onCancel={close}
        okText={'复制用户信息'}
        cancelText={'关闭'}
        closable={false}
        okButtonProps={{ sx: { minWidth: '120px' } }}
        onOk={copyUserInfo}
      >
        <Card sx={{ p: 2, fontSize: 14, bgcolor: 'background.paper3' }}>
          <Stack direction={'row'}>
            <Box sx={{ width: 80 }}>用户名</Box>
            <Box sx={{ fontWeight: 700 }}>{user.account}</Box>
          </Stack>
          <Stack direction={'row'} sx={{ mt: 1 }}>
            <Box sx={{ width: 80 }}>{'新密码'}</Box>
            <Box sx={{ fontWeight: 700 }}>{password}</Box>
          </Stack>
        </Card>
      </Modal>
      <Modal
        title={type === 'reset' ? '重置密码？' : '修改密码'}
        open={updateOpen}
        onOk={handleSubmit(onSumbit)}
        onCancel={close}
        okButtonProps={{
          loading,
        }}
      >
        <Box sx={{ fontSize: 14, lineHeight: '32px' }}>
          用户名{' '}
          <Box component={'span'} sx={{ color: 'red', mb: 1 }}>
            *
          </Box>
        </Box>
        <Box
          sx={{
            lineHeight: '36px',
            bgcolor: 'background.paper3',
            px: '14px',
            borderRadius: '10px',
            fontSize: 14,
            fontWeight: 700,
            cursor: 'not-allowed',
          }}
        >
          {user.account}
        </Box>
        <Box sx={{ fontSize: 14, lineHeight: '32px', mt: 2, mb: 1 }}>
          密码{' '}
          <Box component={'span'} sx={{ color: 'red' }}>
            *
          </Box>
        </Box>
        <Stack direction={'row'} alignItems={'center'} gap={2}>
          <Controller
            control={control}
            name='password'
            rules={{
              required: {
                value: true,
                message: '密码不能为空',
              },
              validate: value => passwordError(value, user.account) || true,
            }}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                autoFocus
                size='small'
                placeholder='输入新密码'
                autoComplete='new-password'
                type={showPassword ? 'text' : 'password'}
                error={!!errors.password}
                helperText={errors.password?.message || PASSWORD_POLICY}
                InputProps={{
                  endAdornment: (
                    <InputAdornment position='end'>
                      <IconButton
                        size='small'
                        onClick={() => setShowPassword(value => !value)}
                        edge='end'
                      >
                        {showPassword ? <VisibilityOff /> : <Visibility />}
                      </IconButton>
                    </InputAdornment>
                  ),
                }}
              />
            )}
          />
          <IconButton
            color='primary'
            size='small'
            onClick={() => setValue('password', generatePassword())}
            sx={{ flexShrink: 0 }}
          >
            <IconShuaxin sx={{ fontSize: 16 }} />
          </IconButton>
        </Stack>
        <Box sx={{ fontSize: 14, lineHeight: '32px', mt: 2, mb: 1 }}>
          确认密码{' '}
          <Box component={'span'} sx={{ color: 'red' }}>
            *
          </Box>
        </Box>
        <Controller
          control={control}
          name='confirmPassword'
          rules={{
            required: '请再次输入密码',
            validate: (value, values) =>
              value === values.password || '两次输入的密码不一致',
          }}
          render={({ field }) => (
            <TextField
              {...field}
              fullWidth
              size='small'
              placeholder='再次输入密码'
              type={showConfirmPassword ? 'text' : 'password'}
              error={!!errors.confirmPassword}
              helperText={errors.confirmPassword?.message}
              InputProps={{
                endAdornment: (
                  <InputAdornment position='end'>
                    <IconButton
                      size='small'
                      onClick={() => setShowConfirmPassword(value => !value)}
                      edge='end'
                    >
                      {showConfirmPassword ? <VisibilityOff /> : <Visibility />}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />
          )}
        />
      </Modal>
    </>
  );
};

export default MemberUpdate;
