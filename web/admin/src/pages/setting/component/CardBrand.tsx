import UploadFile from '@/components/UploadFile';
import { message } from '@/components/ui';
import { putApiV1AppCompat } from '@/request/appCompat';
import type { DomainAppDetailResp, DomainAppSettings } from '@/request/types';
import { Box, Stack, TextField, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import { FormItem, SettingCardItem } from './Common';

type BrandSettings = Pick<
  DomainAppSettings,
  'title' | 'icon' | 'favicon' | 'admin_title' | 'admin_logo' | 'admin_favicon'
>;

const emptyBrandSettings: BrandSettings = {
  title: '',
  icon: '',
  favicon: '',
  admin_title: '',
  admin_logo: '',
  admin_favicon: '',
};

const CardBrand = ({
  info,
  refresh,
}: {
  info: DomainAppDetailResp;
  refresh: (info: DomainAppDetailResp) => void;
}) => {
  const [brand, setBrand] = useState<BrandSettings>(emptyBrandSettings);
  const [isEdit, setIsEdit] = useState(false);

  useEffect(() => {
    setBrand({
      title: info.settings?.title || '',
      icon: info.settings?.icon || '',
      favicon: info.settings?.favicon || '',
      admin_title: info.settings?.admin_title || '',
      admin_logo: info.settings?.admin_logo || '',
      admin_favicon: info.settings?.admin_favicon || '',
    });
    setIsEdit(false);
  }, [
    info.id,
    info.settings?.admin_favicon,
    info.settings?.admin_logo,
    info.settings?.admin_title,
    info.settings?.favicon,
    info.settings?.icon,
    info.settings?.title,
  ]);

  const updateBrand = (key: keyof BrandSettings, value: string) => {
    setBrand(current => ({ ...current, [key]: value }));
    setIsEdit(true);
  };

  const handleSave = async () => {
    const nextInfo: DomainAppDetailResp = {
      ...info,
      settings: {
        ...info.settings,
        ...brand,
      },
    };

    await putApiV1AppCompat(
      { id: info.id! },
      {
        kb_id: info.kb_id,
        settings: nextInfo.settings,
      },
    );
    refresh(nextInfo);
    setIsEdit(false);
    message.success('保存成功');
  };

  return (
    <SettingCardItem title='网站品牌设置' isEdit={isEdit} onSubmit={handleSave}>
      <Typography sx={{ fontSize: 14, fontWeight: 600 }}>门户网站</Typography>
      <FormItem label='网站名称'>
        <TextField
          fullWidth
          value={brand.title}
          placeholder='知识库'
          onChange={event => updateBrand('title', event.target.value)}
        />
      </FormItem>
      <FormItem label='网站 Logo' sx={{ alignItems: 'flex-start' }}>
        <UploadFile
          id='app_site_logo'
          name='app_site_logo'
          type='url'
          accept='image/*'
          width={80}
          height={80}
          value={brand.icon || ''}
          onChange={value => updateBrand('icon', value)}
        />
      </FormItem>
      <FormItem
        label='浏览器图标'
        tooltip='显示在浏览器标签页，建议上传正方形 PNG、SVG 或 ICO 文件'
        sx={{ alignItems: 'flex-start' }}
      >
        <UploadFile
          id='app_site_favicon'
          name='app_site_favicon'
          type='url'
          accept='image/*,.ico'
          width={64}
          height={64}
          value={brand.favicon || ''}
          onChange={value => updateBrand('favicon', value)}
        />
      </FormItem>

      <Box sx={{ height: 1, bgcolor: 'divider', my: 1 }} />

      <Stack direction='row' alignItems='baseline' gap={1}>
        <Typography sx={{ fontSize: 14, fontWeight: 600 }}>管理后台</Typography>
        <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>
          未设置时沿用门户网站配置
        </Typography>
      </Stack>
      <FormItem label='网站名称'>
        <TextField
          fullWidth
          value={brand.admin_title}
          placeholder={brand.title || '知识库管理台'}
          onChange={event => updateBrand('admin_title', event.target.value)}
        />
      </FormItem>
      <FormItem label='网站 Logo' sx={{ alignItems: 'flex-start' }}>
        <UploadFile
          id='admin_site_logo'
          name='admin_site_logo'
          type='url'
          accept='image/*'
          width={80}
          height={80}
          value={brand.admin_logo || ''}
          onChange={value => updateBrand('admin_logo', value)}
        />
      </FormItem>
      <FormItem
        label='浏览器图标'
        tooltip='显示在管理后台浏览器标签页，建议上传正方形 PNG、SVG 或 ICO 文件'
        sx={{ alignItems: 'flex-start' }}
      >
        <UploadFile
          id='admin_site_favicon'
          name='admin_site_favicon'
          type='url'
          accept='image/*,.ico'
          width={64}
          height={64}
          value={brand.admin_favicon || ''}
          onChange={value => updateBrand('admin_favicon', value)}
        />
      </FormItem>
    </SettingCardItem>
  );
};

export default CardBrand;
