import { ConstsHomePageSetting } from '@/request/types';
import { getBasePath } from '@/utils/getBasePath';

export const INIT_DOC_DATA = [
  {
    type: 2,
    emoji: '📘',
    name: '使用说明',
    summary:
      '本文档用于说明内部知识库的基本使用方式，包括登录、文档维护、知识问答和权限管理等常见操作。',
    content:
      '<h1>欢迎使用内部知识库</h1><p>本知识库用于沉淀公司内部的产品资料、技术文档、运维手册、制度规范和常见问题，帮助团队统一信息入口、提升检索效率，并支持基于知识库内容的智能问答。</p><h2>你可以在这里做什么</h2><ul><li><p>维护产品、研发、测试、运维等各类知识文档</p></li><li><p>通过目录和搜索快速查找资料</p></li><li><p>基于已发布内容进行智能问答</p></li><li><p>持续更新规范、流程和经验沉淀</p></li></ul><h2>建议使用方式</h2><ul><li><p>将稳定、可复用的信息整理为正式文档</p></li><li><p>将常见问题归纳为 FAQ，方便快速检索</p></li><li><p>发布前检查内容准确性，避免过期资料误导使用者</p></li><li><p>为重要文档补充清晰标题、摘要和目录结构</p></li></ul><h2>维护建议</h2><ul><li><p>明确文档负责人，定期更新</p></li><li><p>统一命名规范，减少重复内容</p></li><li><p>将历史经验、复盘结论和最佳实践持续沉淀</p></li></ul>',
  },
  {
    type: 2,
    emoji: '🧠',
    name: '智能问答说明',
    summary: '介绍知识库问答能力的工作方式，以及如何提升问答效果和内容命中率。',
    content:
      '<h1>智能问答说明</h1><p>问答结果基于当前知识库中已发布并已完成学习的文档生成。为了获得更好的回答质量，建议优先完善文档结构、标题、段落和关键术语。</p><h2>提升问答效果的建议</h2><ul><li><p>保持文档内容准确、完整、最新</p></li><li><p>为复杂主题增加章节和小标题，便于系统理解结构</p></li><li><p>将术语、流程、规则写清楚，减少模糊描述</p></li><li><p>文档更新后及时重新学习，确保问答内容同步</p></li></ul><h2>常见排查方向</h2><ul><li><p>确认文档是否已发布</p></li><li><p>确认文档是否已完成学习</p></li><li><p>确认问题表述是否与文档术语相近</p></li><li><p>确认知识库中是否确实存在相关内容</p></li></ul>',
  },
  {
    type: 2,
    emoji: '🗂️',
    name: '文档编写建议',
    summary:
      '提供适合内部知识库的文档编写规范，帮助团队产出更利于检索和复用的内容。',
    content:
      '<h1>文档编写建议</h1><h2>标题和结构</h2><ul><li><p>标题尽量简洁明确，直接体现主题</p></li><li><p>多使用二级、三级标题组织层次</p></li><li><p>长文档建议增加目录和摘要</p></li></ul><h2>内容表达</h2><ul><li><p>多使用列表、表格和步骤说明复杂流程</p></li><li><p>避免仅在截图中承载关键信息</p></li><li><p>对易混淆概念给出定义和边界</p></li></ul><h2>维护要求</h2><ul><li><p>标注适用范围、前置条件和注意事项</p></li><li><p>重要流程文档建议附上更新时间</p></li><li><p>当业务变化时及时修订，避免形成失效知识</p></li></ul>',
  },
] as const;

export const INIT_LADING_DATA = {
  title: '内部知识库',
  theme_mode: 'light',
  home_page_setting:
    ConstsHomePageSetting.HomePageSettingCustom as ConstsHomePageSetting,
  icon: getBasePath('/images/init/icon.png'),
  btns: [],
  web_app_custom_style: {
    allow_theme_switching: false,
    header_search_placeholder: '输入问题，开始检索知识库',
    show_brand_info: false,
    footer_show_intro: true,
    social_media_accounts: [],
  },
  footer_settings: {
    footer_style: 'complex',
    corp_name: '',
    icp: '',
    brand_name: '内部知识库',
    brand_desc: '用于沉淀和共享公司内部知识资产。',
    brand_logo: '',
    brand_groups: [],
  },
  web_app_landing_configs: [
    {
      type: 'banner',
      banner_config: {
        title: '欢迎使用内部知识库',
        title_color: '#6E73FE',
        title_font_size: 60,
        subtitle:
          '统一沉淀产品、技术、运维与业务知识，支持检索、阅读与智能问答。',
        placeholder: '输入你的问题',
        subtitle_color: '#ffffff80',
        subtitle_font_size: 16,
        bg_url: '',
        hot_search: [
          '如何录入文档？',
          '如何重新学习文档？',
          '如何提升问答效果？',
        ],
        btns: [
          {
            id: 'init-btn-doc',
            text: '开始浏览',
            type: 'contained',
            href: '',
          },
        ],
      },
      node_ids: [],
      nodes: null,
    },
    {
      type: 'basic_doc',
      basic_doc_config: {
        title: '推荐阅读',
        title_color: '#000000',
        bg_color: '#ffffff00',
      },
      node_ids: [],
    },
    {
      type: 'faq',
      faq_config: {
        title: '常见问题',
        title_color: '#000000',
        bg_color: '#ffffff00',
        list: [
          {
            id: 'faq-1',
            question: '如何录入并发布文档？',
            link: '',
          },
          {
            id: 'faq-2',
            question: '文档为什么提示未学习？',
            link: '',
          },
          {
            id: 'faq-3',
            question: '如何提高问答命中率？',
            link: '',
          },
        ],
      },
      node_ids: [],
      nodes: null,
    },
  ],
};
