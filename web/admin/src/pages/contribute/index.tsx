import NoData from '@/assets/images/nodata.png';
import Card from '@/components/Card';
import { tableSx } from '@/constant/styles';
import { useURLSearchParams } from '@/hooks';
import {
  auditContribute,
  getContributeDetail,
  getContributeList,
  getApiV1NavList,
} from '@/request';
import {
  ConstsContributeStatus,
  GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp,
  GithubComChaitinPandaWikiProApiContributeV1ContributeItem,
  V1NavListResp,
} from '@/request/pro/types';
import { useAppSelector } from '@/store';
import { Ellipsis, Table, message } from '@ctzhian/ui';
import { ColumnType } from '@ctzhian/ui/dist/Table';
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  Stack,
  TextField,
} from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';

const STATUS_OPTIONS = [
  { label: '全部', value: 'all' },
  { label: '待审核', value: ConstsContributeStatus.ContributeStatusPending },
  { label: '已通过', value: ConstsContributeStatus.ContributeStatusApproved },
  { label: '已拒绝', value: ConstsContributeStatus.ContributeStatusRejected },
] as const;

const statusMap = {
  [ConstsContributeStatus.ContributeStatusPending]: {
    label: '待审核',
    color: 'warning' as const,
  },
  [ConstsContributeStatus.ContributeStatusApproved]: {
    label: '已通过',
    color: 'success' as const,
  },
  [ConstsContributeStatus.ContributeStatusRejected]: {
    label: '已拒绝',
    color: 'error' as const,
  },
};

const ContributePage = () => {
  const { kb_id = '' } = useAppSelector(state => state.config);
  const [searchParams, setSearchParams] = useURLSearchParams();
  const page = Number(searchParams.get('page') || '1');
  const pageSize = Number(searchParams.get('pageSize') || '20');
  const keyword = searchParams.get('keyword') || '';
  const statusValue = searchParams.get('status') || 'all';

  const [data, setData] = useState<
    GithubComChaitinPandaWikiProApiContributeV1ContributeItem[]
  >([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [keywordInput, setKeywordInput] = useState(keyword);
  const [navList, setNavList] = useState<V1NavListResp[]>([]);
  const [detail, setDetail] =
    useState<GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp | null>(
      null,
    );
  const [detailOpen, setDetailOpen] = useState(false);
  const [auditLoading, setAuditLoading] = useState(false);
  const [selectedNavId, setSelectedNavId] = useState('');

  useEffect(() => {
    setKeywordInput(keyword);
  }, [keyword]);

  useEffect(() => {
    if (!kb_id) return;
    getApiV1NavList({ kb_id }).then(res => setNavList(res || []));
  }, [kb_id]);

  const getData = () => {
    if (!kb_id) return;
    setLoading(true);
    getContributeList({
      kb_id,
      page,
      per_page: pageSize,
      status:
        statusValue === 'all'
          ? undefined
          : (statusValue as 'pending' | 'approved' | 'rejected'),
    })
      .then(res => {
        const list = (res.list || []).filter(item => {
          if (!keyword.trim()) return true;
          return (
            item.contribute_name?.includes(keyword.trim()) ||
            item.node_name?.includes(keyword.trim()) ||
            item.auth_name?.includes(keyword.trim()) ||
            item.reason?.includes(keyword.trim())
          );
        });
        setData(list);
        setTotal(keyword.trim() ? list.length : res.total || 0);
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    getData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kb_id, page, pageSize, statusValue, keyword]);

  const columns: ColumnType<GithubComChaitinPandaWikiProApiContributeV1ContributeItem>[] =
    [
      {
        dataIndex: 'node_name',
        title: '目标文档',
        width: 220,
        render: (_, record) => (
          <Stack>
            <Ellipsis>
              {record.node_name || record.contribute_name || '-'}
            </Ellipsis>
            <Box sx={{ fontSize: 12, color: 'text.tertiary' }}>
              {record.type === 'add' ? '新增内容' : '修改文档'}
            </Box>
          </Stack>
        ),
      },
      {
        dataIndex: 'reason',
        title: '贡献说明',
        render: (text?: string) => <Ellipsis lines={2}>{text || '-'}</Ellipsis>,
      },
      {
        dataIndex: 'auth_name',
        title: '提交人',
        width: 160,
      },
      {
        dataIndex: 'status',
        title: '状态',
        width: 120,
        render: (status?: ConstsContributeStatus) => {
          const current =
            statusMap[status || ConstsContributeStatus.ContributeStatusPending];
          return (
            <Chip
              size='small'
              label={current.label}
              color={current.color}
              variant='outlined'
            />
          );
        },
      },
      {
        dataIndex: 'created_at',
        title: '提交时间',
        width: 180,
        render: (text?: string) => (
          <Stack>
            <Box>{text ? dayjs(text).fromNow() : '-'}</Box>
            <Box sx={{ fontSize: 12, color: 'text.tertiary' }}>
              {text ? dayjs(text).format('YYYY-MM-DD HH:mm:ss') : '-'}
            </Box>
          </Stack>
        ),
      },
      {
        dataIndex: 'id',
        title: '操作',
        width: 220,
        render: (_, record) => (
          <Stack direction='row' gap={1}>
            <Button
              size='small'
              variant='outlined'
              onClick={() => handleOpenDetail(record.id || '')}
            >
              查看
            </Button>
            {record.status ===
              ConstsContributeStatus.ContributeStatusPending && (
              <>
                <Button
                  size='small'
                  variant='contained'
                  onClick={() => handleQuickAudit(record.id || '', 'approved')}
                >
                  通过
                </Button>
                <Button
                  size='small'
                  color='error'
                  variant='outlined'
                  onClick={() => handleQuickAudit(record.id || '', 'rejected')}
                >
                  拒绝
                </Button>
              </>
            )}
          </Stack>
        ),
      },
    ];

  const handleOpenDetail = async (id: string) => {
    if (!kb_id || !id) return;
    const res = await getContributeDetail({ id, kb_id });
    setDetail(res);
    setSelectedNavId(navList[0]?.id || '');
    setDetailOpen(true);
  };

  const handleQuickAudit = async (
    id: string,
    status: 'approved' | 'rejected',
  ) => {
    if (!kb_id || !id) return;
    setAuditLoading(true);
    try {
      const record =
        detail?.id === id ? detail : await getContributeDetail({ id, kb_id });
      const navID = selectedNavId || navList[0]?.id || '';
      if (status === 'approved' && !navID) {
        message.error('请先创建栏目');
        return;
      }
      await auditContribute({
        id,
        kb_id,
        nav_id: navID,
        parent_id: record.node_id || undefined,
        status,
      });
      message.success(
        status === 'approved'
          ? '已通过贡献，文档已自动发布并进入学习'
          : '已拒绝贡献',
      );
      setDetailOpen(false);
      getData();
    } finally {
      setAuditLoading(false);
    }
  };

  return (
    <Card sx={{ height: '100%' }}>
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        sx={{ mb: 2 }}
      >
        <Stack direction='row' gap={1}>
          {STATUS_OPTIONS.map(item => (
            <Button
              key={item.value}
              variant={statusValue === item.value ? 'contained' : 'outlined'}
              onClick={() =>
                setSearchParams({
                  page: '1',
                  pageSize: String(pageSize),
                  status: item.value,
                  keyword,
                })
              }
            >
              {item.label}
            </Button>
          ))}
        </Stack>
        <Stack direction='row' gap={1}>
          <TextField
            size='small'
            placeholder='搜索文档/提交人/说明'
            value={keywordInput}
            onChange={event => setKeywordInput(event.target.value)}
            onKeyDown={event => {
              if (event.key === 'Enter') {
                setSearchParams({
                  page: '1',
                  pageSize: String(pageSize),
                  status: statusValue,
                  keyword: keywordInput.trim(),
                });
              }
            }}
          />
          <Button
            variant='contained'
            onClick={() =>
              setSearchParams({
                page: '1',
                pageSize: String(pageSize),
                status: statusValue,
                keyword: keywordInput.trim(),
              })
            }
          >
            搜索
          </Button>
        </Stack>
      </Stack>

      <Table
        rowKey='id'
        loading={loading}
        columns={columns}
        dataSource={data}
        pagination={{
          current: page,
          pageSize,
          total,
          onChange: (nextPage, nextPageSize) =>
            setSearchParams({
              page: String(nextPage),
              pageSize: String(nextPageSize),
              status: statusValue,
              keyword,
            }),
        }}
        locale={{
          emptyText: (
            <Stack alignItems='center' sx={{ py: 6 }}>
              <Box
                component='img'
                src={NoData.src}
                sx={{ width: 140, mb: 2 }}
              />
              <Box sx={{ color: 'text.tertiary' }}>暂无贡献记录</Box>
            </Stack>
          ),
        }}
        sx={tableSx}
      />

      <Dialog
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        maxWidth='md'
        fullWidth
      >
        <DialogTitle>贡献详情</DialogTitle>
        <DialogContent>
          <Stack gap={2} sx={{ pt: 1 }}>
            <Box>
              <Box sx={{ fontWeight: 700, mb: 1 }}>贡献说明</Box>
              <Box sx={{ whiteSpace: 'pre-wrap' }}>{detail?.reason || '-'}</Box>
            </Box>
            <Box>
              <Box sx={{ fontWeight: 700, mb: 1 }}>贡献内容</Box>
              <Box sx={{ whiteSpace: 'pre-wrap' }}>
                {detail?.content || '-'}
              </Box>
            </Box>
            {detail?.original_node && (
              <Box>
                <Box sx={{ fontWeight: 700, mb: 1 }}>原文档内容</Box>
                <Box sx={{ whiteSpace: 'pre-wrap', color: 'text.secondary' }}>
                  {detail.original_node.content || '-'}
                </Box>
              </Box>
            )}
            {detail?.type === 'add' && (
              <TextField
                select
                fullWidth
                size='small'
                label='发布到栏目'
                value={selectedNavId}
                onChange={event => setSelectedNavId(event.target.value)}
              >
                {navList.map(item => (
                  <MenuItem key={item.id} value={item.id}>
                    {item.name}
                  </MenuItem>
                ))}
              </TextField>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDetailOpen(false)}>关闭</Button>
          {detail?.status ===
            ConstsContributeStatus.ContributeStatusPending && (
            <>
              <Button
                color='error'
                variant='outlined'
                disabled={auditLoading}
                onClick={() => handleQuickAudit(detail.id || '', 'rejected')}
              >
                拒绝
              </Button>
              <Button
                variant='contained'
                disabled={auditLoading}
                onClick={() => handleQuickAudit(detail.id || '', 'approved')}
              >
                通过
              </Button>
            </>
          )}
        </DialogActions>
      </Dialog>
    </Card>
  );
};

export default ContributePage;
