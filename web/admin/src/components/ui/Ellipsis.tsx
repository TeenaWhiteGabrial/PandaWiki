import KeyboardArrowRightIcon from '@mui/icons-material/KeyboardArrowRight';
import { Box, type BoxProps, type SxProps } from '@mui/material';
import type { ReactNode } from 'react';

interface EllipsisProps extends Omit<BoxProps, 'children'> {
  children?: ReactNode;
  arrow?: boolean;
  sx?: SxProps;
}

const Ellipsis = ({ children, arrow = false, sx, ...rest }: EllipsisProps) => {
  return (
    <Box
      {...rest}
      sx={[
        {
          minWidth: 0,
          display: 'inline-flex',
          alignItems: 'center',
          verticalAlign: 'bottom',
          maxWidth: '100%',
        },
        ...(Array.isArray(sx) ? sx : [sx]),
      ]}
    >
      <Box
        component='span'
        sx={{
          minWidth: 0,
          flex: 1,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}
      >
        {children}
      </Box>
      {arrow && (
        <KeyboardArrowRightIcon
          sx={{ flexShrink: 0, fontSize: '1em', color: 'text.disabled' }}
        />
      )}
    </Box>
  );
};

export default Ellipsis;
