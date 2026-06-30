import type { DomainAppDetailResp } from '@/request/types';
import { getBasePath } from './getBasePath';

export const APP_BRAND_STORAGE_KEY = 'panda_wiki_admin_brand';

export type AppBrand = {
  title: string;
  icon: string;
};

export const getAppBrandFromDetail = (
  detail?: DomainAppDetailResp | null,
): AppBrand => {
  return {
    title: detail?.settings?.title?.trim() || '知识库管理台',
    icon: detail?.settings?.icon
      ? getBasePath(detail.settings.icon)
      : `${window.__BASENAME__ || ''}/logo.png`,
  };
};

export const saveAppBrand = (brand: AppBrand) => {
  try {
    localStorage.setItem(APP_BRAND_STORAGE_KEY, JSON.stringify(brand));
  } catch {
    // ignore
  }
};

export const loadAppBrand = (): AppBrand | null => {
  try {
    const raw = localStorage.getItem(APP_BRAND_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<AppBrand>;
    if (!parsed || typeof parsed.title !== 'string') return null;
    return {
      title: parsed.title || '知识库管理台',
      icon: typeof parsed.icon === 'string' ? parsed.icon : '',
    };
  } catch {
    return null;
  }
};

export const applyAppBrand = (brand: AppBrand) => {
  document.title = brand.title || '知识库管理台';
  const favicon = document.getElementById('favicon') as HTMLLinkElement | null;
  if (favicon) {
    favicon.href = brand.icon || `${window.__BASENAME__ || ''}/logo.png`;
  }
  saveAppBrand(brand);
};
