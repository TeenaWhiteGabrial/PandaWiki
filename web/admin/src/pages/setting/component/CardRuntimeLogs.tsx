import { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Input,
  Segmented,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import dayjs, { Dayjs } from 'dayjs';
import httpClient from '@/request/httpClient';

type StructuredLog = {
  id: number;
  event_time: string;
  level: string;
  service: string;
  module: string;
  source: string;
  stage: string;
  event_code: string;
  safe_message: string;
  safe_fields: Record<string, unknown>;
  kb_id?: string;
  document_id?: string;
  job_id?: string;
  run_id?: string;
};

type RawLog = {
  event_time: string;
  level: string;
  service: string;
  container: string;
  stream: string;
  location: string;
  message: string;
  labels: Record<string, string>;
};

type StructuredResp = { items: StructuredLog[]; total: number };
type RawResp = { items: RawLog[]; available: boolean; has_more: boolean };
type Scope = 'system' | 'business';

const serviceOptions = [
  'api',
  'consumer',
  'crawler',
  'raglite',
  'postgres',
  'redis',
  'nats',
  'qdrant',
  'preview',
  'admin',
  'app',
  'admin-build',
  'app-build',
  'web-dev',
  'caddy',
  'minio',
].map(value => ({ value, label: value }));

const levelColor = (level: string) =>
  level === 'fatal' || level === 'panic' || level === 'error'
    ? 'red'
    : level === 'warn'
      ? 'orange'
      : level === 'debug' || level === 'trace'
        ? 'default'
        : 'blue';

export default function CardRuntimeLogs() {
  const [scope, setScope] = useState<Scope>('system');
  const [structuredItems, setStructuredItems] = useState<StructuredLog[]>([]);
  const [rawItems, setRawItems] = useState<RawLog[]>([]);
  const [total, setTotal] = useState(0);
  const [available, setAvailable] = useState(true);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [level, setLevel] = useState('');
  const [service, setService] = useState('');
  const [location, setLocation] = useState('');
  const [keyword, setKeyword] = useState('');
  const [queryKeyword, setQueryKeyword] = useState('');
  const [page, setPage] = useState(1);
  // 默认关闭，避免“查询日志”请求自身每 5 秒生成一条新日志；需要实时观察时可手动开启。
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [timeRange, setTimeRange] = useState<[Dayjs, Dayjs]>([
    dayjs().subtract(1, 'hour'),
    dayjs(),
  ]);
  const pageSize = 50;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const commonQuery = {
        scope,
        service: service || undefined,
        location: location || undefined,
        keyword: queryKeyword || undefined,
        from: (autoRefresh
          ? dayjs().subtract(timeRange[1].diff(timeRange[0]), 'millisecond')
          : timeRange[0]
        ).toISOString(),
        to: (autoRefresh ? dayjs() : timeRange[1]).toISOString(),
      };
      if (scope === 'system') {
        const result = await httpClient<RawResp>({
          path: '/api/v1/system/runtime-logs',
          method: 'GET',
          query: { ...commonQuery, page_size: 1000 },
          format: 'json',
        });
        setRawItems(result.items || []);
        setAvailable(result.available);
        setHasMore(result.has_more);
      } else {
        const result = await httpClient<StructuredResp>({
          path: '/api/v1/system/runtime-logs',
          method: 'GET',
          query: {
            ...commonQuery,
            level: level || undefined,
            page,
            page_size: pageSize,
          },
          format: 'json',
        });
        setStructuredItems(result.items || []);
        setTotal(result.total || 0);
      }
    } finally {
      setLoading(false);
    }
  }, [
    autoRefresh,
    level,
    location,
    page,
    queryKeyword,
    scope,
    service,
    timeRange,
  ]);

  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (!autoRefresh) return;
    const timer = window.setInterval(() => void load(), 5000);
    return () => window.clearInterval(timer);
  }, [autoRefresh, load]);

  return (
    <Card
      title='运行日志'
      extra={
        <Space>
          <Typography.Text type='secondary'>自动刷新</Typography.Text>
          <Switch
            size='small'
            checked={autoRefresh}
            onChange={setAutoRefresh}
          />
        </Space>
      }
    >
      <Space direction='vertical' size={12} style={{ width: '100%' }}>
        <Segmented
          value={scope}
          options={[
            { value: 'system', label: '全部运行日志' },
            { value: 'business', label: '业务事件日志' },
          ]}
          onChange={value => {
            setScope(value as Scope);
            setPage(1);
          }}
        />
        <Space wrap>
          <DatePicker.RangePicker
            showTime
            allowClear={false}
            value={timeRange}
            onChange={value => {
              if (value?.[0] && value[1]) {
                setTimeRange([value[0], value[1]]);
                setPage(1);
              }
            }}
          />
          <Select
            allowClear
            value={service || undefined}
            placeholder='全部服务'
            style={{ width: 150 }}
            options={serviceOptions}
            onChange={value => {
              setService(value || '');
              setPage(1);
            }}
          />
          {scope === 'business' && (
            <Select
              value={level}
              style={{ width: 120 }}
              onChange={value => {
                setLevel(value);
                setPage(1);
              }}
              options={[
                { value: '', label: '全部级别' },
                { value: 'error', label: '错误' },
                { value: 'warn', label: '警告' },
                { value: 'info', label: '信息' },
                { value: 'debug', label: '调试' },
              ]}
            />
          )}
          <Input
            value={location}
            allowClear
            placeholder={
              scope === 'system'
                ? '容器位置，如 raglite'
                : '代码位置、模块或服务'
            }
            style={{ width: 220 }}
            onChange={event => setLocation(event.target.value)}
            onPressEnter={() => {
              setPage(1);
              void load();
            }}
          />
          <Input.Search
            value={keyword}
            allowClear
            placeholder='搜索完整日志内容'
            style={{ width: 280 }}
            onChange={event => setKeyword(event.target.value)}
            onSearch={value => {
              setQueryKeyword(value);
              setPage(1);
            }}
          />
          <Button onClick={() => void load()}>刷新</Button>
        </Space>

        {scope === 'system' && !available && (
          <Alert
            type='warning'
            showIcon
            message='集中日志服务未启用'
            description='当前环境未配置 LOG_STORE_URL；生产部署会自动启用全服务日志采集。'
          />
        )}
        {scope === 'system' && hasMore && (
          <Alert
            type='info'
            showIcon
            message='当前筛选结果超过 1000 条，请缩小时间范围或增加筛选条件。'
          />
        )}

        {scope === 'system' ? (
          <Table<RawLog>
            rowKey={record =>
              `${record.event_time}-${record.container}-${record.message}`
            }
            size='small'
            loading={loading}
            dataSource={rawItems}
            pagination={false}
            scroll={{ x: 1100 }}
            expandable={{
              expandedRowRender: record => (
                <pre
                  style={{
                    margin: 0,
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                  }}
                >
                  {record.message}
                  {'\n\n'}
                  {JSON.stringify(record.labels, null, 2)}
                </pre>
              ),
            }}
            columns={[
              {
                title: '时间',
                dataIndex: 'event_time',
                width: 185,
                render: value => dayjs(value).format('YYYY-MM-DD HH:mm:ss.SSS'),
              },
              {
                title: '级别',
                dataIndex: 'level',
                width: 80,
                render: value => (
                  <Tag color={levelColor(value)}>
                    {String(value).toUpperCase()}
                  </Tag>
                ),
              },
              { title: '服务', dataIndex: 'service', width: 110 },
              {
                title: '日志位置',
                dataIndex: 'location',
                width: 270,
                ellipsis: true,
              },
              { title: '完整日志', dataIndex: 'message', ellipsis: true },
            ]}
          />
        ) : (
          <Table<StructuredLog>
            rowKey='id'
            size='small'
            loading={loading}
            dataSource={structuredItems}
            pagination={{
              current: page,
              pageSize,
              total,
              showSizeChanger: false,
              onChange: setPage,
              showTotal: value => `共 ${value} 条`,
            }}
            scroll={{ x: 1100 }}
            expandable={{
              expandedRowRender: record => (
                <pre
                  style={{
                    margin: 0,
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                  }}
                >
                  {JSON.stringify(
                    {
                      source: record.source,
                      event_code: record.event_code,
                      stage: record.stage,
                      kb_id: record.kb_id,
                      document_id: record.document_id,
                      job_id: record.job_id,
                      run_id: record.run_id,
                      fields: record.safe_fields,
                    },
                    null,
                    2,
                  )}
                </pre>
              ),
            }}
            columns={[
              {
                title: '时间',
                dataIndex: 'event_time',
                width: 185,
                render: value => dayjs(value).format('YYYY-MM-DD HH:mm:ss.SSS'),
              },
              {
                title: '级别',
                dataIndex: 'level',
                width: 80,
                render: value => (
                  <Tag color={levelColor(value)}>
                    {String(value).toUpperCase()}
                  </Tag>
                ),
              },
              { title: '服务', dataIndex: 'service', width: 100 },
              {
                title: '日志位置',
                dataIndex: 'source',
                width: 280,
                ellipsis: true,
                render: (value, record) => value || record.module,
              },
              { title: '消息', dataIndex: 'safe_message', ellipsis: true },
            ]}
          />
        )}
      </Space>
    </Card>
  );
}
