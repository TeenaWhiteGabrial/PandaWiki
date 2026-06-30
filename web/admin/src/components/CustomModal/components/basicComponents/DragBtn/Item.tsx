import { CardWebHeaderBtn } from '@/api';
import UploadFile from '@/components/UploadFile';
import { useAppDispatch, useAppSelector } from '@/store';
import { Select } from 'antd';

import { Box, Checkbox, IconButton, Stack, TextField } from '@mui/material';
import { IconShanchu2, IconDrag } from '@panda-wiki/icons';
import {
  CSSProperties,
  Dispatch,
  forwardRef,
  HTMLAttributes,
  SetStateAction,
} from 'react';
import { Control, Controller } from 'react-hook-form';

export type ItemProps = Omit<HTMLAttributes<HTMLDivElement>, 'onChange'> & {
  item: CardWebHeaderBtn;
  withOpacity?: boolean;
  isDragging?: boolean;
  dragHandleProps?: any;
  handleRemove?: (id: string) => void;
  setIsEdit: Dispatch<SetStateAction<boolean>>;
  data: CardWebHeaderBtn[];
  control: Control<any>;
};

const Item = forwardRef<HTMLDivElement, ItemProps>(
  (
    {
      item,
      withOpacity,
      isDragging,
      style,
      dragHandleProps,
      handleRemove,
      setIsEdit,
      data: btns,
      control,
      ...props
    },
    ref,
  ) => {
    const dispatch = useAppDispatch();
    const { appPreviewData } = useAppSelector(state => state.config);
    const inlineStyles: CSSProperties = {
      opacity: withOpacity ? '0.5' : '1',
      borderRadius: '10px',
      cursor: isDragging ? 'grabbing' : 'grab',
      backgroundColor: '#ffffff',
      ...style,
    };
    return (
      <Box ref={ref} style={inlineStyles} {...props}>
        <Stack
          direction={'row'}
          alignItems={'center'}
          justifyContent={'space-between'}
          gap={0.5}
          sx={{
            px: 1,
            py: '20px',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: '10px',
            height: '234px',
            width: '346px',
          }}
        >
          <Controller
            control={control}
            name='btns'
            render={({ field }) => {
              const curBtn = btns.find(btn => btn.id === item.id);
              if (!curBtn) return <></>;
              return (
                <Stack direction={'column'} gap={'20px'}>
                  <Stack direction={'row'} gap={1}>
                    <Box>
                      <Box sx={{ fontSize: 12, mb: 0.5 }}>按钮样式</Box>
                      <Select
                        value={curBtn.variant}
                        onChange={value => {
                          const newBtns = [
                            ...(appPreviewData?.settings?.btns || []),
                          ];
                          const index = newBtns.findIndex(
                            (btn: any) => btn.id === curBtn.id,
                          );
                          newBtns[index] = {
                            ...curBtn,
                            variant: value as 'contained' | 'outlined' | 'text',
                          };
                          field.onChange(newBtns);
                          setIsEdit(true);
                        }}
                        style={{ width: 144 }}
                        options={[
                          { value: 'contained', label: '实心' },
                          { value: 'outlined', label: '描边' },
                          { value: 'text', label: '文字' },
                        ]}
                      />
                    </Box>
                    <Box>
                      <Box sx={{ fontSize: 12, mb: 0.5 }}>打开方式</Box>
                      <Select
                        value={curBtn.target}
                        onChange={value => {
                          const newBtns = [
                            ...(appPreviewData?.settings?.btns || []),
                          ];
                          const index = newBtns.findIndex(
                            (btn: any) => btn.id === curBtn.id,
                          );
                          newBtns[index] = {
                            ...curBtn,
                            target: value as '_blank' | '_self',
                          };
                          field.onChange(newBtns);
                          setIsEdit(true);
                        }}
                        style={{ width: 144 }}
                        options={[
                          { value: '_self', label: '当前窗口' },
                          { value: '_blank', label: '新窗口' },
                        ]}
                      />
                    </Box>
                  </Stack>
                  <Stack direction={'row'} gap={2}>
                    <Stack direction={'row'} alignItems={'center'} gap={1}>
                      <Checkbox
                        size='small'
                        sx={{ p: 0, m: 0 }}
                        checked={curBtn.showIcon}
                        onChange={e => {
                          const newBtns = [
                            ...(appPreviewData?.settings?.btns || []),
                          ];
                          const index = newBtns.findIndex(
                            (btn: any) => btn.id === curBtn.id,
                          );
                          newBtns[index] = {
                            ...curBtn,
                            showIcon: e.target.checked,
                          };
                          field.onChange(newBtns);
                          setIsEdit(true);
                        }}
                      />
                      <Box sx={{ fontSize: 14, lineHeight: '32px' }}>
                        展示图标
                      </Box>
                    </Stack>
                    <UploadFile
                      name='icon'
                      id={`${curBtn.id}_icon`}
                      type='url'
                      disabled={false}
                      accept='image/*'
                      width={36}
                      value={curBtn.icon}
                      onChange={(url: string) => {
                        const newBtns = [
                          ...(appPreviewData?.settings?.btns || []),
                        ];
                        const index = newBtns.findIndex(
                          (btn: any) => btn.id === curBtn.id,
                        );
                        newBtns[index] = { ...curBtn, icon: url };
                        field.onChange(newBtns);
                        setIsEdit(true);
                      }}
                    />
                  </Stack>
                  <TextField
                    label='按钮文本'
                    slotProps={{
                      inputLabel: {
                        shrink: true,
                      },
                    }}
                    sx={{
                      height: '36px',
                      '& .MuiOutlinedInput-root': {
                        height: '36px',
                        padding: '0 12px',
                        '& .MuiOutlinedInput-input': {
                          padding: '8px 0',
                        },
                      },
                    }}
                    fullWidth
                    placeholder='请输入文本'
                    variant='outlined'
                    value={curBtn.text}
                    onChange={e => {
                      const newBtns = [
                        ...(appPreviewData?.settings?.btns || []),
                      ];
                      const index = newBtns.findIndex(
                        (btn: any) => btn.id === curBtn.id,
                      );
                      newBtns[index] = { ...curBtn, text: e.target.value };
                      field.onChange(newBtns);
                      setIsEdit(true);
                    }}
                  />
                  <TextField
                    label='按钮链接'
                    slotProps={{
                      inputLabel: {
                        shrink: true,
                      },
                    }}
                    sx={{
                      height: '36px',
                      '& .MuiOutlinedInput-root': {
                        height: '36px',
                        padding: '0 12px',
                        '& .MuiOutlinedInput-input': {
                          padding: '8px 0',
                        },
                      },
                    }}
                    fullWidth
                    placeholder='请输入链接'
                    value={curBtn.url}
                    variant='outlined'
                    onChange={e => {
                      const newBtns = [
                        ...(appPreviewData?.settings?.btns || []),
                      ];
                      const index = newBtns.findIndex(
                        (btn: any) => btn.id === curBtn.id,
                      );
                      newBtns[index] = { ...curBtn, url: e.target.value };
                      field.onChange(newBtns);
                      setIsEdit(true);
                    }}
                  />
                </Stack>
              );
            }}
          />

          <Stack
            direction={'column'}
            sx={{ justifyContent: 'space-between', height: '100%' }}
          >
            <IconButton
              size='small'
              onClick={e => {
                e.stopPropagation();
                handleRemove?.(item.id);
              }}
              sx={{
                color: 'text.tertiary',
                ':hover': { color: 'error.main' },
                width: '28px',
                height: '28px',
              }}
            >
              <IconShanchu2 sx={{ fontSize: '12px' }} />
            </IconButton>
            <IconButton
              size='small'
              sx={{
                cursor: 'grab',
                color: 'text.secondary',
                '&:hover': { color: 'primary.main' },
              }}
              {...dragHandleProps}
            >
              <IconDrag sx={{ fontSize: '18px' }} />
            </IconButton>
          </Stack>
        </Stack>
      </Box>
    );
  },
);

export default Item;
