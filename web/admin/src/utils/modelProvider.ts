import { ModelProvider } from '@/constant/enums';
import type { GithubComChaitinPandaWikiDomainModelProvider } from '@/request/types';

export const normalizeModelProviderForApi = (
  provider?: string | null,
): GithubComChaitinPandaWikiDomainModelProvider => {
  const normalized = provider === 'Other' ? 'CompatiblePlatform' : provider;
  return normalized as GithubComChaitinPandaWikiDomainModelProvider;
};

export const normalizeModelProviderForDisplay = (
  provider?: string | null,
): keyof typeof ModelProvider => {
  const normalized = provider === 'Other' ? 'CompatiblePlatform' : provider;
  if (normalized && normalized in ModelProvider) {
    return normalized as keyof typeof ModelProvider;
  }
  return 'CompatiblePlatform';
};

export const AUTO_MODE_PROVIDER_KEYS: Array<keyof typeof ModelProvider> = [
  'BaiZhiCloud',
  'CompatiblePlatform',
  'Xinference',
  'gpustack',
];
