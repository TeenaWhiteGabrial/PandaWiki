import NoData from '@/assets/images/nodata.png';
import Card from '@/components/Card';
import { tableSx } from '@/constant/styles';
import { useURLSearchParams } from '@/hooks';
import {
  deleteCommentList,
  getCommentList,
  moderateCommentList,
} from '@/request';
import {
  DomainCommentListItem,
  DomainCommentStatus,
  GetApiV1CommentParams,
} from '@/request/types';
import { useAppSelector } from '@/store';
import { Ellipsis, Table, message } from '@ctzhian/ui';
import { ColumnType } from '@ctzhian/ui/dist/Table';
import { Box, Button, Checkbox, Chip, Stack, TextField } from '@mui/material';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';

const STATUS_OPTIONS: Array<{
  label: string;
  value: 'all' | DomainCommentStatus;
}> = [
  { label: '全部', value: 'all' },
  { label: '待审核', value: DomainCommentStatus.CommentStatusPending },
  { label: '已通过', value: DomainCommentStatus.CommentStatusAccepted },
  { label: '已拒绝', value: DomainCommentStatus.CommentStatusReject },
];

const statusMap: Record<
  DomainCommentStatus,
  { label: string; color: 'default' | 'success' | 'warning' | 'error' }
> = {
  [DomainCommentStatus.CommentStatusPending]: {
    label: '待审核',
    color: 'warning',
  },
  [DomainCommentStatus.CommentStatusAccepted]: {
    label: '已通过',
    color: 'success',
  },
  [DomainCommentStatus.CommentStatusReject]: {
    label: '已拒绝',
    color: 'error',
  },
};

const CommentPage = () => {
  const { kb_id = '' } = useAppSelector(state => state.config);
  const [searchParams, setSearchParams] = useURLSearchParams();
  const page = Number(searchParams.get('page') || '1');
  const pageSize = Number(searchParams.get('pageSize') || '20');
  const keyword = searchParams.get('keyword') || '';
  const statusValue = searchParams.get('status') || 'all';
  const [data, setData] = useState<DomainCommentListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [keywordInput, setKeywordInput] = useState(keyword);
  const [moderating, setModerating] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    setKeywordInput(keyword);
  }, [keyword]);

  const query = useMemo(() => {
    const req: GetApiV1CommentParams = {
      kb_id,
      page,
      per_page: pageSize,
    };
    if (statusValue !== 'all') {
      req.status = Number(statusValue) as DomainCommentStatus;
    }
    return req;
  }, [kb_id, page, pageSize, statusValue]);

  const getData = () => {
    if (!kb_id) return;
    setLoading(true);
    getCommentList(query)
      .then(res => {
        const list = (res.data || []) as DomainCommentListItem[];
        const filtered = keyword.trim()
          ? list.filter(item => {
              const userName = item.info?.user_name || '';
              return (
                item.content?.includes(keyword.trim()) ||
                item.node_name?.includes(keyword.trim()) ||
                userName.includes(keyword.trim())
              );
            })
          : list;
        setData(filtered);
        setTotal(keyword.trim() ? filtered.length : res.total || 0);
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    setSelectedRowKeys([]);
  }, [page, pageSize, statusValue, keyword, kb_id]);

  useEffect(() => {
    getData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kb_id, page, pageSize, statusValue, keyword]);

  const pendingSelectedCount = useMemo(
    () =>
      data.filter(
        item =>
          selectedRowKeys.includes(item.id || '') &&
          item.status === DomainCommentStatus.CommentStatusPending,
      ).length,
    [data, selectedRowKeys],
  );

  const columns: ColumnType<DomainCommentListItem>[] = [
    {
      dataIndex: 'id',
      title: (
        <Checkbox
          checked={data.length > 0 && selectedRowKeys.length === data.length}
          indeterminate={
            selectedRowKeys.length > 0 && selectedRowKeys.length < data.length
          }
          onChange={event => {
            setSelectedRowKeys(
              event.target.checked
                ? data.map(item => item.id!).filter(Boolean)
                : [],
            );
          }}
        />
      ),
      width: 70,
      render: (_: string, record) => (
        <Checkbox
          checked={selectedRowKeys.includes(record.id || '')}
          onChange={event => {
            setSelectedRowKeys(prev =>
              event.target.checked
                ? [...prev, record.id!]
                : prev.filter(id => id !== record.id),
            );
          }}
        />
      ),
    },
    {
      dataIndex: 'node_name',
      title: '文档',
      width: 220,
      render: (text?: string) => <Ellipsis>{text || '-'}</Ellipsis>,
    },
    {
      dataIndex: 'content',
      title: '评论内容',
      render: (text?: string) => <Ellipsis lines={2}>{text || '-'}</Ellipsis>,
    },
    {
      dataIndex: 'info',
      title: '评论人',
      width: 180,
      render: (info?: DomainCommentListItem['info']) => (
        <Stack>
          <Box>{info?.user_name || '访客'}</Box>
          <Box sx={{ fontSize: 12, color: 'text.tertiary' }}>
            {info?.remote_ip || '-'}
          </Box>
        </Stack>
      ),
    },
    {
      dataIndex: 'status',
      title: '状态',
      width: 120,
      render: (status?: DomainCommentStatus) => {
        const current =
          statusMap[status || DomainCommentStatus.CommentStatusPending];
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
  ];

  const handleModerate = (status: DomainCommentStatus) => {
    if (selectedRowKeys.length === 0) {
      message.error('请先选择评论');
      return;
    }
    const ids = data
      .filter(
        item =>
          selectedRowKeys.includes(item.id || '') &&
          item.status === DomainCommentStatus.CommentStatusPending,
      )
      .map(item => item.id!)
      .filter(Boolean);
    if (ids.length === 0) {
      message.error('仅待审核评论支持审核操作');
      return;
    }

    setModerating(true);
    moderateCommentList({ ids, status })
      .then(() => {
        message.success(
          status === DomainCommentStatus.CommentStatusAccepted
            ? '已通过'
            : '已拒绝',
        );
        setSelectedRowKeys([]);
        getData();
      })
      .finally(() => {
        setModerating(false);
      });
  };

  const handleDelete = () => {
    if (selectedRowKeys.length === 0) {
      message.error('请先选择评论');
      return;
    }
    setDeleting(true);
    deleteCommentList({ ids: selectedRowKeys })
      .then(() => {
        message.success('删除成功');
        setSelectedRowKeys([]);
        getData();
      })
      .finally(() => {
        setDeleting(false);
      });
  };

  return (
    <Card>
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        sx={{ p: 2 }}
      >
        <Stack direction='row' gap={1} alignItems='center' flexWrap='wrap'>
          {STATUS_OPTIONS.map(item => {
            const active = String(item.value) === statusValue;
            return (
              <Button
                key={String(item.value)}
                size='small'
                variant={active ? 'contained' : 'outlined'}
                color='dark'
                onClick={() =>
                  setSearchParams({
                    status: String(item.value),
                    page: '1',
                    pageSize: String(pageSize),
                    keyword,
                  })
                }
              >
                {item.label}
              </Button>
            );
          })}
        </Stack>
        <Stack direction='row' gap={1} alignItems='center'>
          <TextField
            size='small'
            placeholder='搜索评论内容/文档名/评论人'
            value={keywordInput}
            onChange={event => setKeywordInput(event.target.value)}
            onKeyDown={event => {
              if (event.key === 'Enter') {
                setSearchParams({
                  status: statusValue,
                  page: '1',
                  pageSize: String(pageSize),
                  keyword: keywordInput.trim(),
                });
              }
            }}
          />
          <Button
            size='small'
            variant='outlined'
            color='dark'
            onClick={() =>
              setSearchParams({
                status: statusValue,
                page: '1',
                pageSize: String(pageSize),
                keyword: keywordInput.trim(),
              })
            }
          >
            搜索
          </Button>
        </Stack>
      </Stack>
      <Stack
        direction='row'
        alignItems='center'
        justifyContent='space-between'
        sx={{ px: 2, pb: 2 }}
      >
        <Box sx={{ color: 'text.tertiary', fontSize: 12 }}>
          已选 {selectedRowKeys.length} 条，待审核 {pendingSelectedCount} 条
        </Box>
        <Stack direction='row' gap={1}>
          <Button
            size='small'
            variant='contained'
            color='success'
            disabled={moderating || pendingSelectedCount === 0}
            onClick={() =>
              handleModerate(DomainCommentStatus.CommentStatusAccepted)
            }
          >
            通过
          </Button>
          <Button
            size='small'
            variant='contained'
            color='error'
            disabled={moderating || pendingSelectedCount === 0}
            onClick={() =>
              handleModerate(DomainCommentStatus.CommentStatusReject)
            }
          >
            拒绝
          </Button>
          <Button
            size='small'
            variant='outlined'
            color='dark'
            disabled={deleting || selectedRowKeys.length === 0}
            onClick={handleDelete}
          >
            删除
          </Button>
        </Stack>
      </Stack>
      <Table
        columns={columns}
        dataSource={data}
        rowKey='id'
        height='calc(100vh - 148px)'
        size='small'
        sx={{
          overflow: 'hidden',
          ...tableSx,
          '.MuiTableContainer-root': {
            height: 'calc(100vh - 148px - 120px)',
          },
        }}
        pagination={{
          total,
          page,
          pageSize,
          onChange: (nextPage, nextPageSize) => {
            setSearchParams({
              status: statusValue,
              page: String(nextPage),
              pageSize: String(nextPageSize),
              keyword,
            });
          },
        }}
        PaginationProps={{
          sx: {
            borderTop: '1px solid',
            borderColor: 'divider',
            p: 2,
            '.MuiSelect-root': {
              width: 100,
            },
          },
        }}
        renderEmpty={
          loading ? (
            <Box></Box>
          ) : (
            <Stack alignItems='center' sx={{ mt: 20 }}>
              <img src={NoData} width={174} />
              <Box>暂无评论</Box>
            </Stack>
          )
        }
      />
    </Card>
  );
};

export default CommentPage;
