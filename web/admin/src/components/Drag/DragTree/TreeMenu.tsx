import { ITreeItem } from '@/api';
import Cascader from '@/components/Cascader';
import { addOpacityToColor } from '@/utils';
import { Box, IconButton, Stack, useTheme } from '@mui/material';
import { IconXiala, IconGengduo } from '@panda-wiki/icons';

export type TreeMenuItem = {
  key: string;
  label: string;
  onClick?: () => void;
  disabled?: boolean;
  disabledReason?: string;
  dividerBefore?: boolean;
  color?: 'error' | 'default';
  children?: TreeMenuItem[];
};

export type TreeMenuOptions = {
  item: ITreeItem;
  createItem: (type: 1 | 2, contentType?: string) => void;
  renameItem: () => void;
  isEditing: boolean;
  removeItem: (id: string) => void;
  openFolder?: () => void;
};

const TreeMenu = ({
  menu,
  context,
  anchorPosition,
  onClose,
}: {
  menu: TreeMenuItem[];
  anchorPosition?: { top: number; left: number } | null;
  onClose?: () => void;
  context?: React.ReactElement<{ onClick?: any; 'aria-describedby'?: any }>;
}) => {
  const theme = useTheme();

  const label = (item: TreeMenuItem, child = false) => (
    <Stack
      direction='row'
      alignItems='center'
      justifyContent='space-between'
      gap={1}
      title={item.disabled ? item.disabledReason : undefined}
      sx={{
        fontSize: 14,
        px: 2,
        minHeight: 40,
        width: 190,
        borderRadius: '5px',
        color: item.disabled
          ? 'text.disabled'
          : item.color === 'error'
            ? 'error.main'
            : 'text.primary',
        cursor: item.disabled ? 'not-allowed' : 'pointer',
        ':hover': {
          bgcolor: item.disabled
            ? 'transparent'
            : addOpacityToColor(
                item.color === 'error'
                  ? theme.palette.error.main
                  : theme.palette.primary.main,
                0.1,
              ),
        },
      }}
    >
      {item.label}
      {!child && item.children?.length ? (
        <IconXiala sx={{ fontSize: 20, transform: 'rotate(-90deg)' }} />
      ) : null}
    </Stack>
  );
  return (
    <Cascader
      anchorPosition={anchorPosition}
      onClose={onClose}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      childrenProps={{
        anchorOrigin: { vertical: 'top', horizontal: 'left' },
        transformOrigin: { vertical: 'top', horizontal: 'right' },
      }}
      list={menu.map(item => ({
        key: item.key,
        disabled: item.disabled,
        dividerBefore: item.dividerBefore,
        onClick: item.disabled ? undefined : item.onClick,
        label: (
          <Box>
            {label(item)}
            {item.key === 'next-line' && (
              <Box
                sx={{
                  borderBottom: '1px dashed',
                  borderColor: 'divider',
                  my: 0.5,
                }}
              />
            )}
          </Box>
        ),
        children: item.disabled
          ? undefined
          : item.children?.map(child => ({
              key: child.key,
              disabled: child.disabled,
              onClick: child.disabled ? undefined : child.onClick,
              label: label(child, true),
            })),
      }))}
      context={
        anchorPosition !== undefined
          ? undefined
          : context || (
              <IconButton size='small' aria-label='更多操作'>
                <IconGengduo sx={{ fontSize: '14px' }} />
              </IconButton>
            )
      }
    />
  );
};
export default TreeMenu;
