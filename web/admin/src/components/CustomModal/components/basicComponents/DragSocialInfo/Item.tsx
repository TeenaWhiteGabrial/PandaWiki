import UploadFile from '@/components/UploadFile';
import { DomainSocialMediaAccount } from '@/request/types';
import { IconShanchu2, IconDrag } from '@panda-wiki/icons';
import {
  Box,
  IconButton,
  Popover,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
} from '@mui/material';
import {
  CSSProperties,
  Dispatch,
  forwardRef,
  HTMLAttributes,
  MouseEvent,
  SetStateAction,
  useState,
} from 'react';
import { Control, Controller } from 'react-hook-form';
import { options } from '../../config/FooterConfig';

export interface SocialInfoProps extends HTMLAttributes<HTMLDivElement> {
  item: DomainSocialMediaAccount;
  data: DomainSocialMediaAccount[];
  control: Control<any>;
  withOpacity?: boolean;
  isDragging?: boolean;
  dragHandleProps?: any;
  setIsEdit: Dispatch<SetStateAction<boolean>>;
  index: number;
}
const Item = forwardRef<HTMLDivElement, SocialInfoProps>(
  (
    {
      item,
      data = [],
      control,
      setIsEdit,
      index,
      style,
      withOpacity,
      isDragging,
      dragHandleProps,
      ...props
    },
    ref,
  ) => {
    const [channelAnchorEl, setChannelAnchorEl] = useState<HTMLElement | null>(
      null,
    );
    const channelPopoverOpen = Boolean(channelAnchorEl);
    const inlineStyles: CSSProperties = {
      opacity: withOpacity ? '0.5' : '1',
      borderRadius: '10px',
      cursor: isDragging ? 'grabbing' : 'grab',
      backgroundColor: '#ffffff',
      ...style,
    };
    const selectedOption = options.find(i => i.key === item.channel);
    const SelectedIcon =
      selectedOption?.config_type || selectedOption?.type || null;

    const handleOpenChannelPopover = (event: MouseEvent<HTMLElement>) => {
      setChannelAnchorEl(event.currentTarget);
    };

    const handleCloseChannelPopover = () => {
      setChannelAnchorEl(null);
    };

    return (
      <>
        {item && (
          <Box ref={ref} {...props} style={inlineStyles}>
            <Controller
              control={control}
              name='social_media_accounts'
              render={({ field }) => (
                <Stack
                  direction={'column'}
                  sx={{
                    p: 1,
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: '10px',
                    width: '346px',
                  }}
                  gap={1}
                >
                  <Stack
                    direction={'row'}
                    justifyContent={'flex-start'}
                    alignItems={'center'}
                  >
                    <IconButton
                      size='small'
                      sx={{
                        cursor: 'grab',
                        color: 'text.secondary',
                        '&:hover': { color: 'primary.main' },
                        flexShrink: 0,
                        width: '28px',
                        height: '28px',
                      }}
                      {...dragHandleProps}
                    >
                      <IconDrag sx={{ fontSize: '18px' }} />
                    </IconButton>
                    <Box
                      sx={{
                        fontSize: '12px',
                        lineHeight: '20px',
                        fontWeight: '600',
                      }}
                    >
                      社交信息{index + 1}
                    </Box>
                    <IconButton
                      size='small'
                      onClick={() => {
                        let newData = [...data];
                        newData = newData.filter((_, i) => i !== index);
                        field.onChange(newData);
                        setIsEdit(true);
                      }}
                      sx={{
                        color: 'text.tertiary',
                        ':hover': { color: 'error.main' },
                        flexShrink: 0,
                        width: '28px',
                        height: '28px',
                        ml: 'auto',
                      }}
                    >
                      <IconShanchu2 sx={{ fontSize: '12px' }} />
                    </IconButton>
                  </Stack>
                  <Stack direction={'row'} gap={1}>
                    <Box
                      onClick={handleOpenChannelPopover}
                      sx={{
                        bgcolor: '#fff',
                        width: '60px',
                        minWidth: '60px',
                        height: '36px',
                        border: '1px solid',
                        borderColor: 'divider',
                        borderRadius: '6px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        cursor: 'pointer',
                        '&:hover': {
                          borderColor: 'primary.main',
                        },
                      }}
                    >
                      <Stack justifyContent={'center'} sx={{ mt: '2px' }}>
                        {SelectedIcon && (
                          <SelectedIcon sx={{ fontSize: '14px' }} />
                        )}
                      </Stack>
                    </Box>
                    <Popover
                      open={channelPopoverOpen}
                      anchorEl={channelAnchorEl}
                      onClose={handleCloseChannelPopover}
                      anchorOrigin={{
                        vertical: 'bottom',
                        horizontal: 'left',
                      }}
                      transformOrigin={{
                        vertical: 'top',
                        horizontal: 'left',
                      }}
                    >
                      <Box sx={{ bgcolor: '#fff', p: 1 }}>
                        <ToggleButtonGroup
                          value={item?.channel}
                          onChange={(e, newValue) => {
                            if (!newValue) return;
                            const newData = [...data];
                            newData[index] = {
                              ...item,
                              channel: newValue,
                            };
                            field.onChange(newData);
                            setIsEdit(true);
                            handleCloseChannelPopover();
                          }}
                          exclusive
                          sx={{
                            display: 'flex',
                            flexWrap: 'wrap',
                            gap: 1,
                            backgroundColor: 'white',
                            p: 1,
                            borderRadius: 1,
                          }}
                        >
                          {options.map(item => {
                            const AppIcon = item?.config_type || item?.type;
                            return (
                              <ToggleButton
                                key={item.key}
                                value={item.key}
                                sx={{
                                  p: 1,
                                  height: 'auto',
                                  border: '1px solid #ddd !important',
                                  borderRadius: '0px',
                                }}
                              >
                                <Stack
                                  direction='row'
                                  gap={1}
                                  alignItems='center'
                                >
                                  {AppIcon && (
                                    <AppIcon sx={{ fontSize: '16px' }} />
                                  )}
                                </Stack>
                              </ToggleButton>
                            );
                          })}
                        </ToggleButtonGroup>
                      </Box>
                    </Popover>
                    <TextField
                      {...field}
                      slotProps={{
                        inputLabel: {
                          shrink: true,
                        },
                      }}
                      value={item?.text || ''}
                      sx={{
                        width: '100%',
                        height: '36px',
                        bgcolor: '#ffffff',
                        '& .MuiOutlinedInput-root': {
                          height: '36px',
                          padding: '0 12px',
                          '& .MuiOutlinedInput-input': {
                            padding: '8px 0',
                          },
                        },
                      }}
                      placeholder={
                        options.find(i => i.key === item.channel)
                          ?.text_placeholder || ''
                      }
                      label={
                        options.find(i => i.key === item.channel)?.text_label ||
                        ''
                      }
                      onChange={e => {
                        const newData = [...data];
                        newData[index] = {
                          ...item,
                          text: e.target.value,
                        };
                        field.onChange(newData);
                        setIsEdit(true);
                      }}
                    />
                  </Stack>
                  <Stack
                    sx={{ width: '100%', bgcolor: '#ECEEF1', height: '1px' }}
                  ></Stack>
                  {item.channel === 'wechat_oa' && (
                    <UploadFile
                      {...field}
                      id={`${item.link}-${Date.now()}`}
                      name={`${item.link}-${Date.now()}`}
                      type='url'
                      accept='image/*'
                      width={80}
                      value={item?.icon || ''}
                      onChange={(url: string) => {
                        const newData = [...data];
                        newData[index] = {
                          ...item,
                          icon: url,
                        };
                        field.onChange(newData);
                        setIsEdit(true);
                      }}
                    />
                  )}
                  {item.channel === 'phone' && (
                    <TextField
                      {...field}
                      slotProps={{
                        inputLabel: {
                          shrink: true,
                        },
                      }}
                      value={item?.phone || ''}
                      sx={{
                        mt: 1,
                        width: '100%',
                        height: '36px',
                        bgcolor: '#ffffff',
                        '& .MuiOutlinedInput-root': {
                          height: '36px',
                          padding: '0 12px',
                          '& .MuiOutlinedInput-input': {
                            padding: '8px 0',
                          },
                        },
                      }}
                      placeholder={'请输入电话号码'}
                      label={'电话'}
                      onChange={e => {
                        const newData = [...data];
                        newData[index] = {
                          ...item,
                          phone: e.target.value,
                        };
                        field.onChange(newData);
                        setIsEdit(true);
                      }}
                    />
                  )}
                </Stack>
              )}
            />
          </Box>
        )}
      </>
    );
  },
);
export default Item;
