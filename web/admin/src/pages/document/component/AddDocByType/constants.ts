import { ConstsCrawlerSource } from '@/request';

const INTERNAL_USAGE = '';

// 文档状态常量
export const DOCUMENT_STATUS = {
  DEFAULT: 'default',
  WAITING: 'waiting',
  UPLOADING: 'uploading',
  UPLOAD_DONE: 'upload-done',
  UPLOAD_ERROR: 'upload-error',
  PULLING: 'pulling',
  PULL_DONE: 'pull-done',
  PULL_ERROR: 'pull-error',
  CREATING: 'creating',
  SUCCESS: 'success',
  ERROR: 'error',
} as const;

// 项目类型常量
export const ITEM_TYPE = {
  FILE: 'file',
  OTHER: 'other',
  FOLDER: 'folder',
} as const;

export const NoParseTypes: readonly ConstsCrawlerSource[] = [
  ConstsCrawlerSource.CrawlerSourceFile,
  ConstsCrawlerSource.CrawlerSourceEpub,
] as const;

// 需要上传文件的导入类型
export const UPLOAD_FILE_TYPES: readonly ConstsCrawlerSource[] = [
  ConstsCrawlerSource.CrawlerSourceFile,
  ConstsCrawlerSource.CrawlerSourceEpub,
  ConstsCrawlerSource.CrawlerSourceWikijs,
  ConstsCrawlerSource.CrawlerSourceYuque,
  ConstsCrawlerSource.CrawlerSourceSiyuan,
  ConstsCrawlerSource.CrawlerSourceMindoc,
  ConstsCrawlerSource.CrawlerSourceConfluence,
] as const;

// 需要解析的导入类型
export const PARSE_TYPES: readonly ConstsCrawlerSource[] = [
  ConstsCrawlerSource.CrawlerSourceConfluence,
  ConstsCrawlerSource.CrawlerSourceWikijs,
  ConstsCrawlerSource.CrawlerSourceSiyuan,
  ConstsCrawlerSource.CrawlerSourceMindoc,
  ConstsCrawlerSource.CrawlerSourceNotion,
] as const;

// 需要抓取的导入类型
export const SCRAPE_TYPES: readonly ConstsCrawlerSource[] = [
  ConstsCrawlerSource.CrawlerSourceRSS,
  ConstsCrawlerSource.CrawlerSourceSitemap,
] as const;

// 类型配置
export const TYPE_CONFIG: Record<
  ConstsCrawlerSource,
  {
    label: string;
    okText?: string;
    accept?: string;
    usage?: string;
  }
> = {
  [ConstsCrawlerSource.CrawlerSourceFile]: {
    label: '通过离线文件导入',
    okText: '导入文件',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceUrl]: {
    label: '通过 URL 导入',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceRSS]: {
    label: '通过 RSS 导入',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceSitemap]: {
    label: '通过 Sitemap 导入',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceNotion]: {
    label: '通过 Notion 导入',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceEpub]: {
    label: '通过 Epub 导入',
    accept: '.epub',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceWikijs]: {
    label: '通过 Wiki.js 导入',
    accept: '.zip',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceYuque]: {
    label: '通过语雀导入',
    accept: '.lakebook',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceSiyuan]: {
    label: '通过思源笔记导入',
    accept: '.zip',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceMindoc]: {
    label: '通过 MinDoc 导入',
    accept: '.zip',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceFeishu]: {
    label: '通过飞书文档导入',
    okText: '拉取知识库',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceDingtalk]: {
    label: '通过钉钉文档导入',
    okText: '拉取知识库',
    usage: INTERNAL_USAGE,
  },
  [ConstsCrawlerSource.CrawlerSourceConfluence]: {
    label: '通过 Confluence 导入',
    accept: '.zip',
    usage: INTERNAL_USAGE,
  },
};
