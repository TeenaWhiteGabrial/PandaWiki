import { getApiV1AppDetailCompat } from '@/request/appCompat';
import {
  DomainAppDetailResp,
  DomainKnowledgeBaseDetail,
} from '@/request/types';
import { useAppDispatch } from '@/store';
import { setAppBrand } from '@/store/slices/config';
import { applyAppBrand, getAppBrandFromDetail } from '@/utils/appBrand';
import { Box } from '@mui/material';
import { useEffect, useState } from 'react';
import CardBasicInfo from './CardBasicInfo';
import CardCatalog from './CardCatalog';
import CardCustom from './CardCustom';
import CardProxy from './CardProxy';
import CardStyle from './CardStyle';
import CardWebCustomCode from './CardWebCustomCode';
import CardWebSEO from './CardWebSEO';
import CardQaCopyright from './CardQaCopyright';
import CardWebStats from './CardWebStats';
import CardBrand from './CardBrand';

interface CardWebProps {
  kb: DomainKnowledgeBaseDetail;
  refresh: () => void;
}

const CardWeb = ({ kb, refresh }: CardWebProps) => {
  const dispatch = useAppDispatch();
  const [info, setInfo] = useState<DomainAppDetailResp | null>(null);

  const syncBrand = (nextInfo: DomainAppDetailResp) => {
    const brand = getAppBrandFromDetail(nextInfo);
    dispatch(setAppBrand(brand));
    applyAppBrand(brand);
  };

  const getInfo = async () => {
    const res = await getApiV1AppDetailCompat({ type: '1', kb_id: kb.id });
    syncBrand(res);
    setInfo(res);
  };

  useEffect(() => {
    getInfo();
  }, [kb]);

  if (!info?.id) return <></>;

  return (
    <Box
      sx={{
        width: 1000,
        margin: 'auto',
        pb: 4,
      }}
    >
      <CardBrand
        info={info}
        refresh={nextInfo => {
          syncBrand(nextInfo);
          setInfo(nextInfo);
        }}
      />
      <CardCustom
        refresh={value => {
          const nextInfo = {
            ...info,
            settings: {
              ...info.settings,
              ...value,
            },
          } as DomainAppDetailResp;
          syncBrand(nextInfo);
          setInfo(nextInfo);
        }}
        info={info}
      />
      <CardStyle
        id={info.id}
        data={info}
        refresh={value => {
          setInfo({
            ...info,
            settings: {
              ...info.settings,
              theme_mode: value.theme_mode,
              theme_and_style: {
                ...info.settings?.theme_and_style,
                doc_width: value.doc_width,
                bg_image: value.bg_image,
              },
            },
          });
        }}
      />
      <CardProxy kb={kb} refresh={refresh} />
      <CardBasicInfo kb={kb} refresh={refresh} />
      <CardQaCopyright
        data={info}
        refresh={value => {
          setInfo({
            ...info,
            settings: {
              ...info.settings,
              conversation_setting: value,
            },
          });
        }}
      />
      <CardCatalog
        id={info.id}
        data={info}
        refresh={value => {
          setInfo({
            ...info,
            settings: {
              ...info.settings,
              catalog_settings: {
                ...info.settings?.catalog_settings,
                ...value,
              },
            },
          });
        }}
      />

      <CardWebSEO
        id={info.id}
        data={info}
        refresh={value => {
          const nextInfo = {
            ...info,
            settings: {
              ...info.settings,
              ...value,
            },
          } as DomainAppDetailResp;
          syncBrand(nextInfo);
          setInfo(nextInfo);
        }}
      />

      <CardWebCustomCode
        id={info.id}
        data={info}
        refresh={value => {
          setInfo({
            ...info,
            settings: {
              ...info.settings,
              ...value,
            },
          });
        }}
      />
      <CardWebStats
        id={info.id}
        data={info}
        refresh={value => {
          setInfo({
            ...info,
            settings: {
              ...info.settings,
              stats_setting: {
                ...info.settings?.stats_setting,
                ...value,
              },
            },
          });
        }}
      />
    </Box>
  );
};
export default CardWeb;
