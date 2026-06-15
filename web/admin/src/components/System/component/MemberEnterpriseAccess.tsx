import { FormItem } from '@/components/Form';
import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
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
import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import {
  buildDepartmentTree,
  flattenDepartmentTree,
  getAncestorIds,
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
  const { control, handleSubmit, reset, watch, setValue } = useForm<FormValues>(
    {
      defaultValues: {
        permission_level_id: '',
        primary_department: '',
        department_ids: [],
      },
    },
  );

  const departmentIDs = watch('department_ids');
  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );

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
        // 补全祖先部门 ID
        const ancestorIds = getAncestorIds(values.department_ids, departments);
        const fullDepartmentIds = [
          ...new Set([...ancestorIds, ...values.department_ids]),
        ];
        putApiV1EnterpriseUsersAccess({
          user_id: user.id || '',
          permission_level_id: values.permission_level_id || undefined,
          department_ids: fullDepartmentIds,
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
            <DepartmentTreeSelect
              departments={departmentTree}
              flatDepartments={flatDepartments}
              value={field.value || []}
              onChange={newValue => {
                field.onChange(newValue);
                // 如果主部门不在新选中列表中，清空主部门
                const currentPrimary = watch('primary_department');
                if (currentPrimary && !newValue.includes(currentPrimary)) {
                  setValue('primary_department', '');
                }
              }}
              multiple
              label='所属部门'
              disabled={!canEdit}
            />
          )}
        />
      </FormItem>
      <FormItem label='主部门' sx={{ mt: 2 }}>
        <Controller
          name='primary_department'
          control={control}
          render={({ field }) => (
            <DepartmentTreeSelect
              departments={departmentTree}
              flatDepartments={flatDepartments}
              value={field.value ? [field.value] : []}
              onChange={newValue => {
                field.onChange(newValue[0] || '');
              }}
              multiple={false}
              label='主部门'
              disabled={!canEdit}
              excludeIds={flatDepartments
                .filter(d => !departmentIDs?.includes(d.id))
                .map(d => d.id)}
            />
          )}
        />
      </FormItem>
    </Modal>
  );
};

export default MemberEnterpriseAccess;
