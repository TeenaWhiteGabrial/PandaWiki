import { Segmented } from 'antd';
import { Box } from '@mui/material';
import type { SxProps } from '@mui/material';
import type React from 'react';

interface ListItem {
  label: React.ReactNode;
  value: string | number;
  disabled?: boolean;
}

interface CusTabsProps {
  list: ListItem[];
  defatValue?: ListItem['value'];
  onChange?(value: ListItem['value']): void;
  size?: 'small' | 'middle' | 'large';
  sx?: SxProps;
  change?(value: ListItem['value']): void;
  value?: ListItem['value'];
}

const CusTabs = ({
  list,
  defatValue,
  onChange,
  change,
  value,
  size = 'middle',
  sx,
}: CusTabsProps) => {
  const handleChange = (nextValue: string | number) => {
    onChange?.(nextValue);
    change?.(nextValue);
  };

  return (
    <Box sx={sx}>
      <Segmented
        block={false}
        size={size}
        options={list.map(item => ({
          label: item.label,
          value: item.value,
          disabled: item.disabled,
        }))}
        defaultValue={defatValue}
        value={value}
        onChange={handleChange}
      />
    </Box>
  );
};

export default CusTabs;
