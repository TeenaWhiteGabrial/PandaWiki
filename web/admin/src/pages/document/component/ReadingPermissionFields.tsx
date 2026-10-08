import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import { FormItem } from '@/pages/setting/component/Common';
import type { Department } from '@/request/Enterprise';
import { buildDepartmentTree, flattenDepartmentTree } from '@/utils/enterprise';
import { Select } from 'antd';
import { useMemo } from 'react';

export type ReadingScope =
  | 'public'
  | 'custom'
  | 'permission_level'
  | 'department'
  | 'user';
export interface ReadingPermissionValue {
  scope_type: ReadingScope;
  min_permission_level_id: string;
  allowed_permission_level_ids: string[];
  allowed_department_ids: string[];
  allowed_user_ids: string[];
}

export default function ReadingPermissionFields({
  value,
  onChange,
  levelsDisabled,
  levels,
  departments,
  users,
  disabled = false,
}: {
  value: ReadingPermissionValue;
  onChange: (patch: Partial<ReadingPermissionValue>) => void;
  levelsDisabled?: boolean;
  levels: { id: string; name: string; rank: number }[];
  departments: Department[];
  users: { id: string; account: string }[];
  disabled?: boolean;
}) {
  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  );
  const flatDepartments = useMemo(
    () => flattenDepartmentTree(departmentTree),
    [departmentTree],
  );
  const scope = value.scope_type;
  return (
    <>
      <FormItem label='查阅对象'>
        <Select
          disabled={disabled}
          style={{ width: '100%' }}
          value={scope}
          options={[
            { value: 'public', label: '所有可访问知识库的用户' },
            ...(!levelsDisabled
              ? [{ value: 'permission_level', label: '按用户等级' }]
              : []),
            { value: 'department', label: '按部门' },
            { value: 'user', label: '指定用户' },
            { value: 'custom', label: '部门 / 用户 + 等级要求' },
          ]}
          onChange={scope_type =>
            onChange({
              scope_type,
              min_permission_level_id: '',
              allowed_permission_level_ids: [],
              allowed_department_ids: [],
              allowed_user_ids: [],
            })
          }
        />
      </FormItem>
      {!levelsDisabled && (scope === 'public' || scope === 'custom') && (
        <FormItem label='等级要求'>
          <Select
            disabled={disabled}
            style={{ width: '100%' }}
            value={value.min_permission_level_id}
            options={[
              { value: '', label: '不限用户等级' },
              ...levels.map(level => ({
                value: level.id,
                label: `${level.name}及以上`,
              })),
            ]}
            onChange={min_permission_level_id =>
              onChange({ min_permission_level_id })
            }
          />
        </FormItem>
      )}
      {!levelsDisabled && scope === 'permission_level' && (
        <FormItem label='可查阅等级'>
          <Select
            disabled={disabled}
            mode='multiple'
            style={{ width: '100%' }}
            value={value.allowed_permission_level_ids}
            options={levels.map(level => ({
              value: level.id,
              label: level.name,
            }))}
            placeholder='选择允许查阅的用户等级'
            onChange={allowed_permission_level_ids =>
              onChange({ allowed_permission_level_ids })
            }
          />
        </FormItem>
      )}
      {(scope === 'department' || scope === 'custom') && (
        <FormItem label='可查阅部门'>
          <DepartmentTreeSelect
            disabled={disabled}
            departments={departmentTree}
            flatDepartments={flatDepartments}
            value={value.allowed_department_ids}
            onChange={allowed_department_ids =>
              onChange({ allowed_department_ids })
            }
            multiple
            label='选择可以查阅的部门'
            size='small'
          />
        </FormItem>
      )}
      {(scope === 'user' || scope === 'custom') && (
        <FormItem label='可查阅用户'>
          <Select
            disabled={disabled}
            mode='multiple'
            showSearch
            optionFilterProp='label'
            style={{ width: '100%' }}
            value={value.allowed_user_ids}
            placeholder='选择可以查阅的用户'
            options={users.map(user => ({
              value: user.id,
              label: user.account,
            }))}
            onChange={allowed_user_ids => onChange({ allowed_user_ids })}
          />
        </FormItem>
      )}
    </>
  );
}
