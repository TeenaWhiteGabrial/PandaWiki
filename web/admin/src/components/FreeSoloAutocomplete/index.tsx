import { useCommitPendingInput } from '@/hooks';
import { Select } from 'antd';
import type { DefaultOptionType } from 'antd/es/select';
import { Box, type TextFieldProps } from '@mui/material';
import type { ReactNode } from 'react';

export type FreeSoloAutocompleteProps<T> = {
  width?: number;
  placeholder?: string;
  inputProps?: TextFieldProps;
  options?: T[];
} & ReturnType<typeof useCommitPendingInput<T>> &
  Omit<
    React.ComponentProps<typeof Select>,
    'mode' | 'value' | 'onChange' | 'options'
  >;

export function FreeSoloAutocomplete<T>({
  width,
  placeholder,
  value,
  setValue,
  inputValue,
  setInputValue,
  commit,
  inputProps,
  options = [],
  ...selectProps
}: FreeSoloAutocompleteProps<T>) {
  const selectOptions: DefaultOptionType[] = options.map(option => ({
    value: option as string | number,
    label: option as ReactNode,
  }));

  return (
    <Select
      mode='tags'
      value={value}
      onChange={newValue => setValue(newValue as T[])}
      searchValue={inputValue}
      onSearch={setInputValue}
      onBlur={commit}
      options={selectOptions}
      placeholder={placeholder || inputProps?.placeholder}
      tokenSeparators={[',']}
      optionFilterProp='label'
      style={width ? { width } : { width: '100%' }}
      tagRender={({ label, closable, onClose }) => (
        <Box
          component='span'
          onMouseDown={event => {
            event.preventDefault();
            event.stopPropagation();
          }}
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 0.5,
            px: 1,
            py: 0.25,
            mr: 0.5,
            mb: 0.5,
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: '16px',
            fontSize: '12px',
            lineHeight: '20px',
            bgcolor: 'background.paper',
          }}
        >
          <Box component='span'>{label as ReactNode}</Box>
          {closable ? (
            <Box
              component='button'
              type='button'
              onClick={onClose}
              sx={{
                border: 0,
                background: 'transparent',
                cursor: 'pointer',
                fontSize: '12px',
                lineHeight: 1,
                p: 0,
              }}
            >
              x
            </Box>
          ) : null}
        </Box>
      )}
      {...selectProps}
    />
  );
}
