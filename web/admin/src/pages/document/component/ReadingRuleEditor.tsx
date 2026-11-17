import DepartmentTreeSelect from '@/components/DepartmentTreeSelect';
import type { Department, PermissionLevel } from '@/request/Enterprise';
import type { DomainReadingRule } from '@/request/types';
import { buildDepartmentTree } from '@/utils/enterprise';
import { Box, Button, Checkbox, Stack, Typography } from '@mui/material';
import { Select } from 'antd';
import { useMemo } from 'react';
import { incompleteReadingRule, readingRuleKind } from '../utils/readingRules';

export default function ReadingRuleEditor({
  value,
  onChange,
  onConfirm,
  onCancel,
  departments,
  levels,
  users,
  levelsDisabled,
  documentOnly,
  adding,
}: {
  value: DomainReadingRule;
  onChange: (rule: DomainReadingRule) => void;
  onConfirm: () => void;
  onCancel: () => void;
  departments: Department[];
  levels: PermissionLevel[];
  users: { value: string; label: string }[];
  levelsDisabled?: boolean;
  documentOnly: boolean;
  adding: boolean;
}) {
  const type = readingRuleKind(value),
    p = value.permissions;
  const tree = useMemo(() => buildDepartmentTree(departments), [departments]);
  const invalid = incompleteReadingRule(value);
  return (
    <Stack gap={1.5} sx={{ p: 2, bgcolor: 'background.paper2' }}>
      <Typography fontSize={13} fontWeight={600}>
        {adding ? '添加规则' : '编辑规则'}
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: '64px minmax(0, 1fr)',
          alignItems: 'start',
          gap: 1.5,
          '& > .field-label': { pt: 0.75 },
        }}
      >
        <Typography className='field-label' fontSize={13}>
          授权方式
        </Typography>
        <Select
          aria-label='规则授权方式'
          value={type}
          style={{ width: 190 }}
          options={[
            { value: 'condition', label: '部门与等级' },
            { value: 'users', label: '指定用户' },
            { value: 'all', label: '全员（需登录）' },
          ]}
          onChange={type =>
            onChange({
              ...value,
              permissions:
                type === 'all'
                  ? { scope_type: 'public', allow_all_users: true }
                  : type === 'users'
                    ? { scope_type: 'user', allowed_user_ids: [] }
                    : { scope_type: 'custom' },
            })
          }
        />
        {type === 'condition' && (
          <>
            <Typography className='field-label' fontSize={13}>
              部门
            </Typography>
            <DepartmentTreeSelect
              departments={tree}
              flatDepartments={departments}
              multiple
              cascadeDisplay
              size='small'
              placeholder='不限部门'
              value={p.allowed_department_ids || []}
              onChange={ids =>
                onChange({
                  ...value,
                  permissions: {
                    ...p,
                    scope_type: 'custom',
                    allowed_department_ids: ids,
                  },
                })
              }
            />
            {!levelsDisabled && (
              <>
                <Typography className='field-label' fontSize={13}>
                  等级
                </Typography>
                <Select
                  aria-label='规则等级'
                  mode='multiple'
                  placeholder='不限等级'
                  value={p.allowed_permission_level_ids || []}
                  options={levels
                    .filter(l => l.status === 'active')
                    .map(l => ({ value: l.id, label: l.name }))}
                  onChange={ids =>
                    onChange({
                      ...value,
                      permissions: {
                        ...p,
                        scope_type: 'custom',
                        allowed_permission_level_ids: ids,
                      },
                    })
                  }
                />
              </>
            )}
          </>
        )}
        {type === 'users' && (
          <>
            <Typography className='field-label' fontSize={13}>
              用户
            </Typography>
            <Select
              aria-label='规则用户'
              mode='multiple'
              showSearch
              optionFilterProp='label'
              placeholder='搜索姓名或账号'
              value={p.allowed_user_ids || []}
              options={users}
              onChange={ids =>
                onChange({
                  ...value,
                  permissions: { scope_type: 'user', allowed_user_ids: ids },
                })
              }
            />
          </>
        )}
        {!documentOnly && (
          <>
            <Typography className='field-label' fontSize={13}>
              范围
            </Typography>
            <Stack direction='row' gap={0.5} alignItems='center'>
              <Checkbox
                aria-label='包含下级目录及文件'
                slotProps={{ input: { 'aria-label': '包含下级目录及文件' } }}
                size='small'
                checked={value.include_descendants}
                onChange={(_, checked) =>
                  onChange({ ...value, include_descendants: checked })
                }
                sx={{ p: 0.5, ml: -0.5 }}
              />
              <Typography fontSize={13}>包含下级目录及文件</Typography>
            </Stack>
          </>
        )}
      </Box>
      <Stack direction='row' alignItems='center' gap={1} sx={{ pl: '76px' }}>
        <Typography fontSize={12} color='text.secondary' sx={{ flex: 1 }}>
          {invalid
            ? type === 'users'
              ? '请选择用户'
              : '部门、等级至少选择一项'
            : !documentOnly && !value.include_descendants
              ? '仅当前目录及其直接文件'
              : ''}
        </Typography>
        <Button size='small' color='inherit' onClick={onCancel}>
          取消
        </Button>
        <Button
          size='small'
          variant='outlined'
          disabled={invalid}
          onClick={onConfirm}
        >
          确认规则
        </Button>
      </Stack>
    </Stack>
  );
}
