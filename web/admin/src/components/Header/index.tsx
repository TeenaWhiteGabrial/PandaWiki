import { getApiV1KnowledgeBaseDetail } from '@/request/KnowledgeBase';
import { useAppSelector, useAppDispatch } from '@/store';
import { setKbDetail } from '@/store/slices/config';
import { Button, Stack } from '@mui/material';
import { useEffect, useState } from 'react';
import Bread from './Bread';

const Header = () => {
  const { kb_id } = useAppSelector(state => state.config);
  const dispatch = useAppDispatch();
  const [wikiUrl, setWikiUrl] = useState<string>('');

  useEffect(() => {
    if (kb_id) {
      getApiV1KnowledgeBaseDetail({ id: kb_id }).then(res => {
        dispatch(setKbDetail(res));
        if (res.access_settings?.base_url) {
          setWikiUrl(res.access_settings.base_url);
        } else {
          let defaultUrl: string = '';
          const host = res.access_settings?.hosts?.[0] || '';
          if (!host) return;

          if (
            res.access_settings?.ssl_ports &&
            res.access_settings?.ssl_ports.length > 0
          ) {
            defaultUrl = res.access_settings.ssl_ports.includes(443)
              ? `https://${host}`
              : `https://${host}:${res.access_settings.ssl_ports[0]}`;
          } else if (
            res.access_settings?.ports &&
            res.access_settings?.ports.length > 0
          ) {
            defaultUrl = res.access_settings.ports.includes(80)
              ? `http://${host}`
              : `http://${host}:${res.access_settings.ports[0]}`;
          }
          setWikiUrl(defaultUrl);
        }
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
      <Stack direction={'row'} alignItems={'center'} gap={2}>
        <Button
          size='small'
          variant='contained'
          color='dark'
          onClick={() => {
            if (wikiUrl) {
              window.open(wikiUrl, '_blank');
            }
          }}
        >
          访问 Wiki 网站
        </Button>
      </Stack>
    </Stack>
  );
};

export default Header;
