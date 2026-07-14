import { getApiV1KnowledgeBaseDetail } from '@/request/KnowledgeBase';
import { useAppSelector, useAppDispatch } from '@/store';
import { setKbDetail } from '@/store/slices/config';
import { Stack } from '@mui/material';
import { useEffect } from 'react';
import Bread from './Bread';

const Header = () => {
  const { kb_id } = useAppSelector(state => state.config);
  const dispatch = useAppDispatch();

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
      }}
    >
      <Bread />
      <Stack direction={'row'} alignItems={'center'} gap={2} />
    </Stack>
  );
};

export default Header;
