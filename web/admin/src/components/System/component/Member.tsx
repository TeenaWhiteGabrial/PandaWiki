import NoData from '@/assets/images/nodata.png';
import Card from '@/components/Card';
import { AntdTable } from '@/components/ui';
import type { ColumnType } from '@/components/ui';
import {
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
} from '@/request/Enterprise';
import {
  getApiV1UserListCompat,
  UserListItemCompat,
} from '@/request/userCompat';
import { ConstsUserRole } from '@/request/types';
import { useAppSelector } from '@/store';
import { Select } from 'antd';
import { Box, Button, Stack, TextField, Tooltip } from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useState } from 'react';
import MemberAdd from './MemberAdd';
import MemberDelete from './MemberDelete';
import MemberEnterpriseAccess from './MemberEnterpriseAccess';
import MemberUpdate from './MemberUpdate';

const ConstsUserRoleMap = {
  [ConstsUserRole.UserRoleAdmin]: '超级管理员',
  [ConstsUserRole.UserRoleManager]: '管理员',
  [ConstsUserRole.UserRoleUser]: '普通用户',
};

const Member = () => {
  const { user } = useAppSelector(state => state.config);
  const [loading, setLoading] = useState(false);
  const [userList, setUserList] = useState<UserListItemCompat[]>([]);
  const [curUser, setCurUser] = useState<UserListItemCompat | null>(null);
  const [curType, setCurType] = useState<'delete' | 'reset-password' | null>(
    null,
  );
  const [enterpriseOpen, setEnterpriseOpen] = useState(false);
  const [departmentNameMap, setDepartmentNameMap] = useState<
    Record<string, string>
  >({});
  const [levelNameMap, setLevelNameMap] = useState<Record<string, string>>({});
  const [filters, setFilters] = useState<{
    keyword: string;
    role: ConstsUserRole | '';
    department_id: string;
  }>({
    keyword: '',
    role: '',
    department_id: '',
  });

  const columns: ColumnType<UserListItemCompat>[] = [
    {
      title: '账号',
      dataIndex: 'account',
      render: (text: string, record) => (
        <Stack direction={'row'} alignItems={'center'} gap={2}>
          {text}
          {user?.id === record.id ? (
            <Box
              sx={{
                borderColor: 'text.primary',
                border: '1px solid',
                p: '0 8px',
                borderRadius: '10px',
                fontSize: '12px',
                lineHeight: '18px',
              }}
            >
              我
            </Box>
          ) : null}
        </Stack>
      ),
    },
    {
      title: '姓名',
      dataIndex: 'name',
      render: (text?: string) => <Box>{text || '-'}</Box>,
    },
    {
      title: '身份',
      dataIndex: 'role',
      render: (text: ConstsUserRole) => <Box>{ConstsUserRoleMap[text]}</Box>,
    },
    {
      title: '权限等级',
      dataIndex: 'permission_level_id',
      render: (value?: string) => (
        <Box>{(value && levelNameMap[value]) || '默认'}</Box>
      ),
    },
    {
      title: '所属部门',
      dataIndex: 'department_ids',
      render: (value?: string[]) => (
        <Box>
          {(value || []).map(id => departmentNameMap[id] || id).join('、') ||
            '-'}
        </Box>
      ),
    },
    {
      title: '上次使用时间',
      dataIndex: 'last_access',
      render: (text: string) => (
        <Box>{text ? dayjs(text).format('YYYY-MM-DD HH:mm:ss') : '-'}</Box>
      ),
    },
    {
      title: '',
      dataIndex: 'action',
      width: 140,
      render: (_, record) => (
        <Stack direction={'row'} gap={2}>
          <Button
            size='small'
            sx={{ p: 0, minWidth: 'auto' }}
            disabled={
              user?.role === ConstsUserRole.UserRoleManager &&
              record.role !== ConstsUserRole.UserRoleUser
            }
            onClick={() => {
              setCurUser(record);
              setEnterpriseOpen(true);
            }}
          >
            组织和权限
          </Button>
          {record.account === 'admin' ? (
            <Tooltip
              arrow
              slotProps={{
                tooltip: {
                  sx: {
                    maxWidth: 'none',
                  },
                },
              }}
              title={
                <Box>
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{ mb: 1, whiteSpace: 'nowrap' }}
                  >
                    修改安装目录下
                    <Box
                      sx={{
                        fontWeight: 700,
                        bgcolor: 'background.paper',
                        px: 1.5,
                        py: 0.25,
                        borderRadius: '4px',
                        color: 'text.primary',
                      }}
                    >
                      .env
                    </Box>
                    文件中的
                    <Box
                      sx={{
                        fontWeight: 700,
                        bgcolor: 'background.paper',
                        px: 1.5,
                        py: 0.25,
                        borderRadius: '4px',
                        color: 'text.primary',
                      }}
                    >
                      ADMIN_PASSWORD
                    </Box>
                    后，
                  </Stack>
                  <Stack direction={'row'} alignItems={'center'} gap={1}>
                    执行
                    <Box
                      sx={{
                        fontWeight: 700,
                        bgcolor: 'background.paper',
                        px: 1.5,
                        py: 0.25,
                        borderRadius: '4px',
                        color: 'text.primary',
                      }}
                    >
                      docker compose up -d
                    </Box>
                    即可生效。
                  </Stack>
                </Box>
              }
            >
              <Button
                size='small'
                sx={{
                  color: 'var(--mui-palette-action-disabled)',
                  cursor: 'not-allowed',
                  p: 0,
                  minWidth: 'auto',
                }}
              >
                修改密码
              </Button>
            </Tooltip>
          ) : (
            <Button
              size='small'
              sx={{ p: 0, minWidth: 'auto' }}
              color='primary'
              disabled={
                (record.role === ConstsUserRole.UserRoleAdmin &&
                  user.account !== 'admin' &&
                  user.id !== record.id) ||
                (user?.role === ConstsUserRole.UserRoleManager &&
                  record.role !== ConstsUserRole.UserRoleUser &&
                  user?.id !== record.id)
              }
              onClick={() => {
                setCurUser(record);
                setCurType('reset-password');
              }}
            >
              {user?.id === record.id ? '修改密码' : '重置密码'}
            </Button>
          )}
          {user?.id !== record.id &&
            (user.account === 'admin' ||
              (user.role === ConstsUserRole.UserRoleAdmin &&
                record.role !== ConstsUserRole.UserRoleAdmin) ||
              (user.role === ConstsUserRole.UserRoleManager &&
                record.role === ConstsUserRole.UserRoleUser)) && (
              <Button
                size='small'
                color='error'
                sx={{ p: 0, minWidth: 'auto' }}
                onClick={() => {
                  setCurUser(record);
                  setCurType('delete');
                }}
              >
                删除
              </Button>
            )}
        </Stack>
      ),
    },
  ];

  const getData = () => {
    setLoading(true);
    Promise.all([
      getApiV1UserListCompat(filters),
      getApiV1EnterpriseDepartments(),
      getApiV1EnterprisePermissionLevels(),
    ])
      .then(([data, departments, levels]) => {
        const res = data.users || [];
        setDepartmentNameMap(
          (departments.departments || []).reduce(
            (acc, item) => {
              acc[item.id] = item.name;
              return acc;
            },
            {} as Record<string, string>,
          ),
        );
        setLevelNameMap(
          (levels.levels || []).reduce(
            (acc, item) => {
              acc[item.id] = item.name;
              return acc;
            },
            {} as Record<string, string>,
          ),
        );
        const idx = res.findIndex(item => item.id === user?.id);
        if (idx !== -1) {
          setUserList([res[idx], ...res.slice(0, idx), ...res.slice(idx + 1)]);
        } else {
          setUserList(res);
        }
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    getData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters]);

  return (
    <Card
      sx={{
        flex: 1,
        py: 2,
        overflow: 'hidden',
        overflowY: 'auto',
        border: '1px solid',
        borderColor: 'divider',
      }}
    >
      <Stack
        direction={'row'}
        justifyContent={'space-between'}
        alignItems={'center'}
        sx={{ mb: 2, px: 2 }}
      >
        <Box sx={{ fontSize: 14, lineHeight: '24px', fontWeight: 'bold' }}>
          用户管理
        </Box>
        <Stack direction='row' gap={2} sx={{ mb: 2 }}>
          <TextField
            size='small'
            placeholder='按账号或姓名搜索'
            value={filters.keyword}
            onChange={event =>
              setFilters(prev => ({ ...prev, keyword: event.target.value }))
            }
            sx={{ width: 220 }}
          />
          <Select
            size='middle'
            value={filters.role}
            onChange={value =>
              setFilters(prev => ({
                ...prev,
                role: value as ConstsUserRole | '',
              }))
            }
            style={{ width: 160 }}
            options={[
              { value: '', label: '全部角色' },
              {
                value: ConstsUserRole.UserRoleAdmin,
                label: '超级管理员',
              },
              { value: ConstsUserRole.UserRoleManager, label: '管理员' },
              { value: ConstsUserRole.UserRoleUser, label: '普通用户' },
            ]}
          />
          <Select
            size='middle'
            value={filters.department_id}
            onChange={value =>
              setFilters(prev => ({
                ...prev,
                department_id: value,
              }))
            }
            style={{ width: 200 }}
            options={[
              { value: '', label: '全部部门' },
              ...Object.entries(departmentNameMap).map(([id, name]) => ({
                value: id,
                label: name,
              })),
            ]}
          />
        </Stack>
        <MemberAdd refresh={getData} />
      </Stack>
      <AntdTable
        columns={columns}
        dataSource={userList}
        rowKey='id'
        size='small'
        height='338px'
        sx={{ overflow: 'hidden' }}
        pagination={false}
        renderEmpty={
          loading ? (
            <Box></Box>
          ) : (
            <Stack alignItems={'center'}>
              <img src={NoData} width={150} />
              <Box
                sx={{
                  fontSize: 12,
                  lineHeight: '20px',
                  color: 'text.tertiary',
                }}
              >
                暂无数据
              </Box>
            </Stack>
          )
        }
      />
      {curUser && curType === 'reset-password' && (
        <MemberUpdate
          user={curUser}
          refresh={getData}
          type={user?.id === curUser.id ? 'update' : 'reset'}
        />
      )}
      <MemberEnterpriseAccess
        open={enterpriseOpen}
        user={curUser}
        refresh={getData}
        onClose={() => {
          setEnterpriseOpen(false);
          setCurUser(null);
        }}
      />
      <MemberDelete
        open={!!curUser && curType === 'delete'}
        onClose={() => {
          setCurType(null);
          setCurUser(null);
        }}
        user={curUser}
        refresh={getData}
      />
    </Card>
  );
};

export default Member;
