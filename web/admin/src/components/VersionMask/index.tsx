import { ConstsLicenseEdition } from '@/request/types';
import { Box, SxProps } from '@mui/material';
import React from 'react';

const VersionMask = ({
  children,
}: {
  permission?: ConstsLicenseEdition[];
  children?: React.ReactNode;
  wrapperSx?: SxProps;
  sx?: SxProps;
}) => {
  return <>{children}</>;
};

export const VersionCanUse = ({
  sx,
}: {
  permission?: ConstsLicenseEdition[];
  sx?: SxProps;
  mode?: 'icon' | 'text';
}) => {
  return (
    <Box
      sx={{
        width: 'auto',
        ml: 1,
        pointerEvents: 'auto',
        ...sx,
      }}
      onClick={e => {
        e.stopPropagation();
        e.preventDefault();
      }}
    />
  );
};

export default VersionMask;
