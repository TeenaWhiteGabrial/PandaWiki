import NoData from '@/assets/images/nodata.png';
import type { SxProps, Theme } from '@mui/material';
import { Box, Stack } from '@mui/material';
import { Table as AntdTableBase } from 'antd';
import type { TablePaginationConfig } from 'antd';
import type { ColumnsType as AntdColumnsType } from 'antd/es/table';

type TableAlign = 'left' | 'center' | 'right' | 'justify' | 'inherit';

type TableSortDirection = 'asc' | 'desc';

type TableRender<T> = (value: any, record: T, index: number) => React.ReactNode;

export type ColumnType<T> = {
  dataIndex?: string | number;
  title?: React.ReactNode;
  minWidth?: number | string;
  width?: number | string;
  align?: TableAlign;
  defaultSortOrder?: TableSortDirection;
  sorter?: boolean | ((a: T, b: T) => number);
  render?: TableRender<T>;
};

export type ColumnsType<T> = ColumnType<T>[];

type TablePagination = {
  page?: number;
  pageSize?: number;
  total?: number;
  pageSizeOptions?: number[];
  onChange?: (page: number, pageSize: number) => void;
};

export type TableProps<T> = {
  showHeader?: boolean;
  height?: number | string;
  virtual?: boolean;
  columns: ColumnsType<T>;
  dataSource: T[];
  loading?: boolean;
  pagination?: TablePagination | false;
  rowKey?: string | ((record: T) => string);
  sx?: SxProps<Theme>;
  defaultSort?: unknown;
  rowSelection?: unknown;
  onChange?: unknown;
  expandable?: unknown;
  renderEmpty?: React.ReactNode;
  size?: 'small' | 'medium' | 'large';
  updateScrollTop?: boolean;
};

type AntdTableProps<T> = Pick<
  TableProps<T>,
  | 'columns'
  | 'dataSource'
  | 'height'
  | 'rowKey'
  | 'size'
  | 'pagination'
  | 'renderEmpty'
  | 'loading'
  | 'sx'
>;

const sizeMap: Record<
  NonNullable<TableProps<unknown>['size']>,
  'small' | 'middle' | 'large'
> = {
  small: 'small',
  medium: 'middle',
  large: 'large',
};

const toAntdColumns = <T extends object>(
  columns: ColumnsType<T>,
): AntdColumnsType<T> =>
  columns.map(column => ({
    dataIndex: column.dataIndex,
    title: column.title,
    width: column.width ?? column.minWidth,
    align:
      column.align === 'inherit' || column.align === 'justify'
        ? 'left'
        : column.align,
    render: column.render
      ? (value, record, index) => column.render?.(value, record, index)
      : undefined,
  }));

const defaultEmpty = (
  <Stack alignItems={'center'} sx={{ mt: 20 }}>
    <img src={NoData} width={174} />
    <Box>暂无数据</Box>
  </Stack>
);

export const AntdTable = ({
  columns,
  dataSource,
  height,
  rowKey = 'id',
  size = 'medium',
  pagination,
  renderEmpty,
  loading,
  sx,
}: AntdTableProps<any>) => {
  const paginationConfig: TablePaginationConfig | false = pagination
    ? {
        current: pagination.page,
        pageSize: pagination.pageSize,
        total: pagination.total,
        onChange: (page, pageSize) => pagination.onChange?.(page, pageSize),
        pageSizeOptions: (pagination.pageSizeOptions || [10, 20, 50, 100]).map(
          value => String(value),
        ),
        showSizeChanger: true,
        showTotal: total => `共 ${total} 条`,
      }
    : false;

  return (
    <Box
      sx={{
        height,
        '& .ant-table-wrapper': {
          width: '100%',
        },
        '& .ant-table-container table > thead > tr > th:first-of-type': {
          paddingLeft: '24px',
        },
        '& .ant-table-container table > tbody > tr > td:first-of-type': {
          paddingLeft: '24px',
        },
        '& .ant-table-pagination': {
          mx: 0,
          px: 2,
          py: 2,
        },
        '& .ant-spin-nested-loading, & .ant-spin-container': {
          height: '100%',
        },
        ...(sx as SxProps<Theme>),
      }}
    >
      <AntdTableBase<any>
        columns={toAntdColumns(columns)}
        dataSource={dataSource}
        rowKey={rowKey as string | ((record: any) => string)}
        loading={loading}
        size={sizeMap[size]}
        pagination={paginationConfig}
        locale={{
          emptyText: renderEmpty || defaultEmpty,
        }}
      />
    </Box>
  );
};

export const Table = AntdTable;
