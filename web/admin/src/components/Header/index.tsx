import { getApiV1KnowledgeBaseDetail } from '@/request/KnowledgeBase';
import { useAppSelector, useAppDispatch } from '@/store';
import { setKbDetail } from '@/store/slices/config';
import { Box, IconButton, Stack, Tooltip } from '@mui/material';
import { message, Modal } from '@/components/ui';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import Logo from '@/assets/images/logo.png';
import Avatar from '../Avatar';
import { IconDengchu } from '@panda-wiki/icons';
import { useEffect, useState } from 'react';
import Bread from './Bread';

const Header = () => {
  const { kb_id } = useAppSelector(state => state.config);
  const { user, appBrand } = useAppSelector(state => state.config);
  const dispatch = useAppDispatch();
  const [logoutConfirmOpen, setLogoutConfirmOpen] = useState(false);

  useEffect(() => {
    if (kb_id) {
      getApiV1KnowledgeBaseDetail({ id: kb_id }).then(res => {
        dispatch(setKbDetail(res));
      });
    }
  }, [kb_id]);

  return (
    <Stack
      direction={'row'}
      alignItems={'center'}
      justifyContent={'space-between'}
      sx={{
        minWidth: '900px',
        position: 'fixed',
        pl: '170px',
        py: 2,
        pr: 2,
        zIndex: 998,
        width: '100%',
        bgcolor: 'background.paper2',
        'body.document-workbench-fullscreen &': {
          pl: 2,
        },
      }}
    >
      <Bread />
      <Stack direction='row' alignItems='center' gap={1.5}>
        <Avatar src={appBrand.logo || Logo} sx={{ width: 30, height: 30 }} />
        <Box
          sx={{
            maxWidth: 180,
            fontSize: 13,
            color: 'text.secondary',
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {user?.account || '当前用户'}
        </Box>
        <Tooltip arrow title='退出登录'>
          <IconButton size='small' onClick={() => setLogoutConfirmOpen(true)}>
            <IconDengchu sx={{ fontSize: 18 }} />
          </IconButton>
        </Tooltip>
      </Stack>
      <Modal
        open={logoutConfirmOpen}
        onCancel={() => setLogoutConfirmOpen(false)}
        onOk={() => {
          message.success('退出登录成功，请重新登录');
          setTimeout(() => {
            localStorage.removeItem('panda_wiki_token');
            window.location.href = '/login';
          }, 1500);
        }}
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <ErrorOutlineIcon sx={{ color: 'error.main' }} />
            确定要退出当前账号？
          </Stack>
        }
      />
    </Stack>
  );
};

export default Header;
