import type { DomainAppSettingsResp } from '@/request/types';
import { getBasePath } from './getBasePath';

export const APP_BRAND_STORAGE_KEY = 'panda_wiki_admin_brand';

export type AppBrand = {
  title: string;
  logo: string;
  favicon: string;
};

export const getAppBrandFromDetail = (
  detail?: { settings?: DomainAppSettingsResp } | null,
): AppBrand => {
  const settings = detail?.settings;
  const fallbackLogo = settings?.icon
    ? getBasePath(settings.icon)
    : `${window.__BASENAME__ || ''}/logo.png`;

  return {
    title:
      settings?.admin_title?.trim() ||
      settings?.title?.trim() ||
      '知识库管理台',
    logo: settings?.admin_logo
      ? getBasePath(settings.admin_logo)
      : fallbackLogo,
    favicon: settings?.admin_favicon
      ? getBasePath(settings.admin_favicon)
      : settings?.favicon
        ? getBasePath(settings.favicon)
        : fallbackLogo,
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
    const parsed = JSON.parse(raw) as Partial<AppBrand> & { icon?: string };
    if (!parsed || typeof parsed.title !== 'string') return null;
    return {
      title: parsed.title || '知识库管理台',
      logo:
        typeof parsed.logo === 'string'
          ? parsed.logo
          : typeof parsed.icon === 'string'
            ? parsed.icon
            : '',
      favicon:
        typeof parsed.favicon === 'string'
          ? parsed.favicon
          : typeof parsed.icon === 'string'
            ? parsed.icon
            : '',
    };
  } catch {
    return null;
  }
};

export const applyAppBrand = (brand: AppBrand) => {
  document.title = brand.title || '知识库管理台';
  const favicon = document.getElementById('favicon') as HTMLLinkElement | null;
  if (favicon) {
    favicon.href = brand.favicon || `${window.__BASENAME__ || ''}/logo.png`;
  }
  saveAppBrand(brand);
};
