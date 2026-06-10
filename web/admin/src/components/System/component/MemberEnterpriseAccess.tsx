import { FormItem } from '@/components/Form';
import {
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
  putApiV1EnterpriseUsersAccess,
} from '@/request/Enterprise';
import type { Department, PermissionLevel } from '@/request/Enterprise';
import type { UserListItemCompat } from '@/request/userCompat';
import { ConstsUserRole } from '@/request/types';
import { useAppSelector } from '@/store';
import { Modal } from '@ctzhian/ui';
import { MenuItem, TextField } from '@mui/material';
import { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import {
  buildDepartmentTree,
  flattenDepartmentTree,
  formatDepartmentOptionLabel,
  normalizeSelectedDepartmentIDs,
} from '@/utils/enterprise';

type MemberEnterpriseAccessProps = {
  open: boolean;
  user: UserListItemCompat | null;
  onClose: () => void;
  refresh: () => void;
};

type FormValues = {
  permission_level_id: string;
  primary_department: string;
  department_ids: string[];
};

const MemberEnterpriseAccess = ({
  open,
  user,
  onClose,
  refresh,
}: MemberEnterpriseAccessProps) => {
  const currentUser = useAppSelector(state => state.config.user);
  const [loading, setLoading] = useState(false);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [levels, setLevels] = useState<PermissionLevel[]>([]);
  const { control, handleSubmit, reset, watch } = useForm<FormValues>({
    defaultValues: {
      permission_level_id: '',
      primary_department: '',
      department_ids: [],
    },
  });

  const departmentIDs = watch('department_ids');
  const departmentTree = buildDepartmentTree(departments);
  const flatDepartments = flattenDepartmentTree(departmentTree);

  useEffect(() => {
    if (!open || !user) return;
    Promise.all([
      getApiV1EnterpriseDepartments(),
      getApiV1EnterprisePermissionLevels(),
    ]).then(([departmentResp, levelResp]) => {
      setDepartments(departmentResp.departments || []);
      setLevels(levelResp.levels || []);
      reset({
        permission_level_id: user.permission_level_id || '',
        primary_department: user.primary_department || '',
        department_ids: user.department_ids || [],
      });
    });
  }, [open, user, reset]);

  if (!user) return null;
  const canEdit =
    currentUser?.role === ConstsUserRole.UserRoleAdmin ||
    user.role === ConstsUserRole.UserRoleUser;

  return (
    <Modal
      title='编辑组织权限'
      open={open}
      onCancel={onClose}
      okButtonProps={{ loading, disabled: !canEdit }}
      onOk={handleSubmit(values => {
        if (!canEdit) return;
        setLoading(true);
        putApiV1EnterpriseUsersAccess({
          user_id: user.id || '',
          permission_level_id: values.permission_level_id || undefined,
          department_ids: values.department_ids || [],
          primary_department: values.primary_department || undefined,
        })
          .then(() => {
            refresh();
            onClose();
          })
          .finally(() => setLoading(false));
      })}
    >
      <FormItem label='权限等级'>
        <Controller
          name='permission_level_id'
          control={control}
          render={({ field }) => (
            <TextField {...field} fullWidth select>
              <MenuItem value=''>默认</MenuItem>
              {levels.map(item => (
                <MenuItem key={item.id} value={item.id}>
                  {item.name}（{item.rank}）
                </MenuItem>
              ))}
            </TextField>
          )}
        />
      </FormItem>
      <FormItem label='所属部门' sx={{ mt: 2 }}>
        <Controller
          name='department_ids'
          control={control}
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
      <FormItem label='主部门' sx={{ mt: 2 }}>
        <Controller
          name='primary_department'
          control={control}
          render={({ field }) => (
            <TextField {...field} fullWidth select>
              <MenuItem value=''>无</MenuItem>
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
  );
};

export default MemberEnterpriseAccess;
