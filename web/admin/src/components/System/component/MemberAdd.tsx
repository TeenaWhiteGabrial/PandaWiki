import { postApiV1UserCreate } from '@/request/User';
import Card from '@/components/Card';
import { FormItem } from '@/components/Form';
import { getApiV1EnterpriseDepartments } from '@/request/Enterprise';
import type { Department } from '@/request/Enterprise';
import { copyText, generatePassword } from '@/utils';
import {
  buildDepartmentTree,
  flattenDepartmentTree,
  formatDepartmentOptionLabel,
  normalizeSelectedDepartmentIDs,
} from '@/utils/enterprise';
import { CheckCircle } from '@mui/icons-material';
import { Box, Button, MenuItem, Stack, TextField } from '@mui/material';
import { Modal } from '@ctzhian/ui';
import { useEffect, useState } from 'react';
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
  const [departments, setDepartments] = useState<Department[]>([]);

  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
    watch,
  } = useForm({
    defaultValues: {
      account: '',
      name: '',
      role: 'user' as Role,
      department_ids: [] as string[],
      primary_department: '',
    },
  });

  const departmentIDs = watch('department_ids');
  const flatDepartments = flattenDepartmentTree(
    buildDepartmentTree(departments),
  );

  useEffect(() => {
    if (!addMember) return;
    getApiV1EnterpriseDepartments().then(res => {
      setDepartments(res.departments || []);
    });
  }, [addMember]);

  const copyUserInfo = ({
    account,
    password,
  }: {
    account: string;
    password: string;
  }) => {
    copyText(`账号: ${account}\n密码: ${password}`, () => {
      setCreatedUserInfo(null);
      reset();
    });
  };

  const onSubmit = handleSubmit(data => {
    setLoading(true);
    const password = generatePassword();
    const payload: V1CreateUserReq = {
      account: data.account,
      name: data.name,
      department_ids: data.department_ids,
      password,
      primary_department: data.primary_department,
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
              <TextField {...field} fullWidth select>
                <MenuItem value='manager'>管理员</MenuItem>
                <MenuItem value='user'>普通用户</MenuItem>
              </TextField>
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
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                select
                SelectProps={{ multiple: true }}
                value={field.value || []}
                onChange={event => {
                  const selectedValues = Array.isArray(event.target.value)
                    ? event.target.value
                    : [event.target.value];
                  field.onChange(
                    normalizeSelectedDepartmentIDs(selectedValues, departments),
                  );
                }}
                error={!!errors.department_ids}
                helperText={errors.department_ids?.message as string}
              >
                {flatDepartments.map(item => (
                  <MenuItem key={item.id} value={item.id}>
                    {formatDepartmentOptionLabel(item)}
                  </MenuItem>
                ))}
              </TextField>
            )}
          />
        </FormItem>
        <FormItem label='主部门' required sx={{ mt: 2 }}>
          <Controller
            control={control}
            name='primary_department'
            rules={{
              required: '请选择主部门',
            }}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                select
                error={!!errors.primary_department}
                helperText={errors.primary_department?.message}
              >
                {flatDepartments
                  .filter(item => departmentIDs?.includes(item.id))
                  .map(item => (
                    <MenuItem key={item.id} value={item.id}>
                      {formatDepartmentOptionLabel(item)}
                    </MenuItem>
                  ))}
              </TextField>
            )}
          />
        </FormItem>
      </Modal>
    </>
  );
};

export default MemberAdd;
