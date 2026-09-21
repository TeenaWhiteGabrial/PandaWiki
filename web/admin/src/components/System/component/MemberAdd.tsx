import { postApiV1UserCreate } from '@/request/User';
import Card from '@/components/Card';
import { message, Modal } from '@/components/ui';
import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import { FormItem } from '@/components/Form';
import { getApiV1EnterpriseDepartments } from '@/request/Enterprise';
import type { Department } from '@/request/Enterprise';
import { generatePassword } from '@/utils';
import {
  buildDepartmentTree,
  flattenDepartmentTree,
  getAncestorIds,
} from '@/utils/enterprise';
import { CheckCircle } from '@mui/icons-material';
import { Select } from 'antd';
import { Box, Button, Stack, TextField } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import type { V1CreateUserReq } from '@/request/types';

type Role = 'manager' | 'user';

const MemberAdd = ({ refresh }: { refresh: () => void }) => {
  const [addMember, setAddMember] = useState(false);
  const [loading, setLoading] = useState(false);
  const [createdUserInfo, setCreatedUserInfo] = useState<{
    account: string;
    password: string;
  } | null>(null);
  const [copying, setCopying] = useState(false);
  const [departments, setDepartments] = useState<Department[]>([]);

  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm({
    defaultValues: {
      account: '',
      name: '',
      role: 'user' as Role,
      department_ids: [] as string[],
    },
  });

  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );

  useEffect(() => {
    if (!addMember) return;
    getApiV1EnterpriseDepartments().then(res => {
      setDepartments(res.departments || []);
    });
  }, [addMember]);

  const fallbackCopy = (text: string) => {
    const textArea = document.createElement('textarea');
    textArea.style.position = 'fixed';
    textArea.style.opacity = '0';
    textArea.style.left = '-9999px';
    textArea.style.top = '-9999px';
    textArea.value = text;
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
      return document.execCommand('copy');
    } finally {
      document.body.removeChild(textArea);
    }
  };

  const copyUserInfo = async ({
    account,
    password,
  }: {
    account: string;
    password: string;
  }) => {
    const text = `账号: ${account}\n密码: ${password}`;
    setCopying(true);
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
      } else if (!fallbackCopy(text)) {
        throw new Error('copy failed');
      }
      message.success('复制成功', 1.5);
      setCreatedUserInfo(null);
      reset();
    } catch {
      message.error('复制失败，请手动复制', 1.5);
    } finally {
      setCopying(false);
    }
  };

  const onSubmit = handleSubmit(data => {
    setLoading(true);
    const password = generatePassword();
    const payload: V1CreateUserReq = {
      account: data.account,
      name: data.name,
      department_ids: [
        ...new Set([
          ...getAncestorIds(data.department_ids, departments),
          ...data.department_ids,
        ]),
      ],
      password,
      role: data.role,
    };
    const onSuccess = () => {
      setCreatedUserInfo({
        account: data.account,
        password,
      });
      setAddMember(false);
      refresh();
    };
    postApiV1UserCreate(payload)
      .then(() => {
        onSuccess();
      })
      .finally(() => {
        setLoading(false);
      });
  });

  return (
    <>
      <Button
        size='small'
        variant='outlined'
        onClick={() => setAddMember(true)}
      >
        添加用户
      </Button>
      <Modal
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <CheckCircle sx={{ color: 'success.main' }} />
            新用户创建成功
          </Stack>
        }
        open={!!createdUserInfo}
        closable={false}
        cancelText='关闭'
        onCancel={() => {
          setCreatedUserInfo(null);
          reset();
        }}
        okText='复制用户信息'
        okButtonProps={{
          sx: { minWidth: '120px' },
          loading: copying,
        }}
        onOk={() => createdUserInfo && copyUserInfo(createdUserInfo)}
      >
        <Card sx={{ p: 2, fontSize: 14, bgcolor: 'background.paper3' }}>
          <Stack direction={'row'}>
            <Box sx={{ width: 80 }}>账号</Box>
            <Box sx={{ fontWeight: 700 }}>{createdUserInfo?.account}</Box>
          </Stack>
          <Stack direction={'row'} sx={{ mt: 1 }}>
            <Box sx={{ width: 80 }}>密码</Box>
            <Box sx={{ fontWeight: 700 }}>{createdUserInfo?.password}</Box>
          </Stack>
        </Card>
      </Modal>
      <Modal
        title='添加用户'
        open={addMember}
        onCancel={() => {
          setAddMember(false);
          reset();
        }}
        onOk={onSubmit}
        okButtonProps={{
          loading,
        }}
      >
        <FormItem label='账号' required>
          <Controller
            control={control}
            name='account'
            rules={{
              required: {
                value: true,
                message: '账号不能为空',
              },
              pattern: {
                value: /^[A-Za-z0-9]+$/,
                message: '账号只允许英文和数字',
              },
            }}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                autoFocus
                placeholder='输入账号，仅支持英文和数字'
                error={!!errors.account}
                helperText={errors.account?.message}
              />
            )}
          />
        </FormItem>
        <FormItem label='姓名' required sx={{ mt: 2 }}>
          <Controller
            control={control}
            name='name'
            rules={{
              required: {
                value: true,
                message: '姓名不能为空',
              },
            }}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                placeholder='输入姓名'
                error={!!errors.name}
                helperText={errors.name?.message}
              />
            )}
          />
        </FormItem>

        <FormItem label='角色' required sx={{ mt: 2 }}>
          <Controller
            control={control}
            name='role'
            render={({ field }) => (
              <Select
                value={field.value}
                onChange={field.onChange}
                style={{ width: '100%' }}
                options={[
                  { value: 'manager', label: '管理员' },
                  { value: 'user', label: '普通用户' },
                ]}
              />
            )}
          />
        </FormItem>
        <FormItem label='所属部门' required sx={{ mt: 2 }}>
          <Controller
            control={control}
            name='department_ids'
            rules={{
              validate: value => (value?.length ? true : '请选择所属部门'),
            }}
            render={({ field, fieldState }) => (
              <DepartmentTreeSelect
                departments={departmentTree}
                flatDepartments={flatDepartments}
                value={field.value || []}
                onChange={field.onChange}
                multiple={false}
                label='所属部门'
                error={!!fieldState.error}
                helperText={fieldState.error?.message}
              />
            )}
          />
        </FormItem>
      </Modal>
    </>
  );
};

export default MemberAdd;
