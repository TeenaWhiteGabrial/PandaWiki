import { VersionInfoMap } from '@/constant/version';
import { useVersionInfo } from '@/hooks';
import { ConstsLicenseEdition } from '@/request/types';
import { Box, styled, SxProps } from '@mui/material';
import React from 'react';

const StyledMaskWrapper = styled('div')(({ theme }) => ({
  position: 'relative',
  width: '100%',
  height: '100%',
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(2),
}));

const StyledMask = styled('div')(({ theme }) => ({
  position: 'absolute',
  inset: -8,
  zIndex: 99,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  flex: 1,
  borderRadius: '10px',
  border: `1px solid ${theme.palette.divider}`,
  background: 'rgba(241,242,248,0.8)',
  backdropFilter: 'blur(0.5px)',
}));

const StyledMaskContent = styled('div')(({ theme }) => ({
  width: '100%',
  height: '100%',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
}));

const StyledMaskVersion = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(0.5),
  padding: theme.spacing(0.5, 1),
  backgroundColor: theme.palette.background.paper3,
  borderRadius: '10px',
  fontSize: 12,
  lineHeight: 1,
  color: theme.palette.light.main,
}));

const VersionMask = ({
  permission = [
    ConstsLicenseEdition.LicenseEditionFree,
    ConstsLicenseEdition.LicenseEditionProfession,
    ConstsLicenseEdition.LicenseEditionBusiness,
    ConstsLicenseEdition.LicenseEditionEnterprise,
  ],
  children,
  wrapperSx,
  sx,
}: {
  permission?: ConstsLicenseEdition[];
  children?: React.ReactNode;
  wrapperSx?: SxProps;
  sx?: SxProps;
}) => {
  const versionInfo = useVersionInfo();
  const hasPermission = permission.includes(versionInfo.permission);
  if (hasPermission) return children;

  return (
    <StyledMaskWrapper sx={wrapperSx}>
      {children}
      <StyledMask sx={sx}>
        <StyledMaskContent>
          <StyledMaskVersion>当前环境未启用</StyledMaskVersion>
        </StyledMaskContent>
      </StyledMask>
    </StyledMaskWrapper>
  );
};

export const VersionCanUse = ({
  permission = [
    ConstsLicenseEdition.LicenseEditionFree,
    ConstsLicenseEdition.LicenseEditionProfession,
    ConstsLicenseEdition.LicenseEditionBusiness,
    ConstsLicenseEdition.LicenseEditionEnterprise,
  ],
  sx,
  mode = 'text',
}: {
  permission?: ConstsLicenseEdition[];
  sx?: SxProps;
  mode?: 'icon' | 'text';
}) => {
  const versionInfo = useVersionInfo();
  const hasPermission = permission.includes(versionInfo.permission);
  if (hasPermission) return null;
  return (
    <Box
      sx={{
        width: 'auto',
        ml: mode === 'icon' ? 0.5 : 1,
        // 允许 Tooltip 在 disabled 的父元素中正常工作
        pointerEvents: 'auto',
        ...sx,
      }}
      onClick={e => {
        e.stopPropagation();
        e.preventDefault();
      }}
    >
      {mode === 'icon' ? (
        '未启用'
      ) : (
        <StyledMaskVersion>当前环境未启用</StyledMaskVersion>
      )}
    </Box>
  );
};

export default VersionMask;
