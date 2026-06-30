# PandaWiki 前端组件体系迁移到 Ant Design 设计说明

## 1. 文档目的

本文档用于评估 PandaWiki 前端从当前的 `MUI + @ctzhian/ui` 体系，逐步迁移到 `Ant Design` 的可行性，并给出可执行的迁移路径、风险说明与实施建议。

本文档关注的是：

- 保留现有网页端能力不受破坏
- 不直接修改第三方库源码
- 通过调整组件引用来源，逐步把新旧页面迁移到 `antd`
- 为后续大规模前端重构提供阶段性落地方案

本文档不包含：

- 一次性全量替换所有页面
- 直接废弃现有 `web/admin` 或 `web/app`
- 对后端接口协议进行改造

## 2. 当前结论

结论很明确：**可以迁移到 `Ant Design`，但不适合通过批量替换 import 的方式直接完成。**

原因不是技术上不能换，而是当前项目对 `MUI` 和 `@ctzhian/ui` 的依赖已经深入到以下层级：

- 全局主题创建
- 全局主题 Provider
- 颜色变量与组件样式覆盖
- 消息提醒、弹窗、表格、标签页等基础交互组件
- 页面中大量 `Box`、`Stack`、`TextField`、`Tabs`、`Drawer`、`Tooltip` 等基础布局与表单控件
- 部分页面直接依赖 `@ctzhian/ui` 提供的增强组件

因此，正确路线不是“全局搜索替换 import”，而是：

1. 先建立项目自己的 UI 兼容层
2. 兼容层内部接入 `antd`
3. 新代码优先使用兼容层
4. 老页面分模块逐步迁移
5. 最后再处理主题体系与剩余 MUI 布局能力

## 3. 当前依赖现状

基于当前仓库代码扫描，前端依赖范围如下：

- `web/admin/src` 中约 `182` 个文件直接引用 `@mui/*` 或 `@ctzhian/ui`
- `web/app/src` 中约 `59` 个文件直接引用 `@mui/*` 或 `@ctzhian/ui`
- 全仓库前端源码中约有 `132` 处直接引用 `@ctzhian/ui`
- 全仓库前端源码中约有 `276` 处直接引用 `@mui/material` 或 `@mui/icons-material`

说明：

- `admin` 是迁移主战场，复杂度明显高于 `app`
- `app` 虽然引用量更少，但它是 Next.js，接入方案不能完全照搬 `admin`
- 当前不是单一组件替换问题，而是“组件库 + 主题系统 + 页面实现方式”的组合迁移问题

## 4. 已识别的关键耦合点

### 4.1 全局主题入口

当前主题入口直接依赖 `MUI` 与 `@ctzhian/ui`：

- [web/admin/src/App.tsx](/Users/topazzz/unified-project/PandaWiki/web/admin/src/App.tsx:1)
- [web/admin/src/themes/index.ts](/Users/topazzz/unified-project/PandaWiki/web/admin/src/themes/index.ts:1)
- [web/app/src/provider/themeStore.tsx](/Users/topazzz/unified-project/PandaWiki/web/app/src/provider/themeStore.tsx:1)
- [web/app/src/theme.ts](/Users/topazzz/unified-project/PandaWiki/web/app/src/theme.ts:1)

当前模式是：

- 使用 `createTheme` 创建 MUI theme
- 通过 `@ctzhian/ui` 的 `ThemeProvider` 注入
- 同时叠加 `zhCN` 本地化
- 通过 `components` 大量覆写 `Mui*` 组件样式

这意味着：

- 如果切到 `antd`，不能只替换按钮、表单、弹窗
- 还要设计新的全局主题注入方案
- 旧页面在迁移期间仍需要继续依赖 MUI theme

### 4.2 `@ctzhian/ui` 的职责不只是“样式库”

当前 `@ctzhian/ui` 被直接用作以下能力来源：

- `ThemeProvider`
- `Table`
- `Modal`
- `message`
- `Ellipsis`
- `Empty`
- `Image`
- `CusTabs`

这说明 `@ctzhian/ui` 在本项目中承担了“基础组件库 + 若干业务增强组件”的职责，而不是纯视觉皮肤。

因此，如果要逐步转向 `antd`，不能简单理解成“把 MUI 替换为 antd”；实际上还需要决定：

- 哪些能力直接用 `antd`
- 哪些能力改为项目内自维护组件
- 哪些能力继续短期保留 `@ctzhian/ui`

### 4.3 页面对 MUI 基础控件依赖很深

当前页面对以下 MUI 控件使用频率非常高：

- 布局类：`Box`、`Stack`、`Paper`
- 表单类：`TextField`、`Checkbox`、`Radio`、`Select`
- 交互类：`Button`、`IconButton`、`Tooltip`、`Drawer`、`Dialog`
- 导航类：`Tabs`、`Tab`
- 状态类：`Skeleton`、`LinearProgress`、`CircularProgress`
- 样式类：`styled`、`useTheme`、`alpha`

这类依赖分布在几乎所有后台页面中，特别集中于：

- 文档管理
- 文档编辑
- 系统管理
- 设置页
- 自定义装修页
- 统计页

所以迁移不只是“替换弹窗和表格”。

## 5. 迁移可行性判断

### 5.1 可行

迁移到 `antd` 是可行的，原因如下：

- 当前项目是 React 技术栈，`antd` 适配没有框架层面的阻碍
- 页面大多数是业务表单、表格、弹窗、设置面板，属于 `antd` 的常规强项
- `admin` 后台界面与 `antd` 的适配度高
- 现有代码已经具备一定组件封装基础，可以继续向“项目自有 UI 层”演进

### 5.2 不适合一次性全量迁移

不建议一次性全量迁移，原因如下：

- 当前页面量和直接依赖数量都较大
- `admin` 与 `app` 构建方式不同，迁移细节不同
- 编辑器页、装修页、自定义拖拽页复杂度高
- 现有主题体系完全基于 MUI，强行硬切会带来大量样式回归
- `@ctzhian/ui` 中存在项目运行时直接依赖的能力，贸然替换风险高

### 5.3 最合理方案是“双轨期共存”

建议在较长一段时间内保持：

- 老页面继续运行在 `MUI + @ctzhian/ui`
- 新能力和已迁移页面运行在 `antd + 项目兼容层`

这类“双轨并行”是本项目最现实、风险最低的路线。

## 6. 推荐迁移架构

### 6.1 建立项目自己的 UI 兼容层

建议新增：

- `web/admin/src/components/ui`
- `web/app/src/components/ui`

如果后续需要跨端统一，也可以进一步抽成 workspace 包，例如：

- `web/packages/admin-ui`
- `web/packages/app-ui`

第一阶段不建议一开始就过度抽象到 workspace 包。原因是 `admin` 和 `app` 在运行环境、样式接入、SSR 约束上不同，先分别建立兼容层更稳。

### 6.2 兼容层职责

兼容层对外只暴露项目自己的接口，例如：

- `Button`
- `Input`
- `Select`
- `Checkbox`
- `Tabs`
- `Modal`
- `Drawer`
- `Table`
- `Tooltip`
- `EmptyState`
- `ImagePreview`
- `notify`

兼容层内部实现：

- 第一阶段接 `antd`
- 对需要保留原行为的部分做轻量封装
- 屏蔽页面对 `antd` 原始 API 的直接耦合

这样可以确保未来即使再换库，页面层改动也会小很多。

### 6.3 保留 MUI 仅用于布局与遗留主题

在过渡期内，建议保留 MUI 的以下职责：

- `ThemeProvider` 与现有 theme
- `Box` / `Stack` / `sx` 等布局能力
- 编辑器和历史页面中已经稳定运行的样式逻辑

同时，将 `antd` 优先用于：

- 消息提醒
- 弹窗
- 标签页
- 表格
- 表单控件

这样可以明显降低第一阶段改造成本。

## 7. 分阶段实施方案

### 7.1 第一阶段：接入 `antd`，不改业务页面

目标：

- 让项目具备使用 `antd` 的基础能力
- 不影响现有页面运行

实施内容：

1. 在 `web` workspace 中安装 `antd`
2. 为 `admin` 和 `app` 设计各自的 `antd` Provider 接入方式
3. 新建项目自有兼容层目录
4. 先封装最小公共能力，不替换已有页面

建议首批封装组件：

- `notify`，替代 `message`
- `Modal`
- `Tabs`
- `Table`

原因：

- 这四类组件收益最高
- 对业务页面渗透面大
- 更容易形成统一入口

### 7.2 第二阶段：先迁移 `admin` 公共组件

目标：

- 优先把后台的全局交互入口切到兼容层

建议优先迁移：

- [web/admin/src/App.tsx](/Users/topazzz/unified-project/PandaWiki/web/admin/src/App.tsx:1) 附近的 Provider 结构
- 公共提示能力，例如 `api/request`、`utils`、`hooks`
- 高频公共组件，例如 Header、通用弹窗、列表页基础表格

优先级高的原因：

- 可以先减少新代码继续增加对 `@ctzhian/ui` 的依赖
- 可以尽早建立统一入口
- 不需要一开始就碰最复杂的编辑器页

### 7.3 第三阶段：迁移后台业务页面

建议按复杂度与收益排序：

1. 登录页、基础设置页、系统设置页
2. 列表页、审核页、统计页
3. 文档管理页
4. 装修页、自定义拖拽页
5. 文档编辑页

这样排序的原因：

- 前两类页面大量是表单、表格、弹窗，和 `antd` 更契合
- 后两类页面包含自定义拖拽、复杂样式结构、编辑器相关逻辑，风险更高

### 7.4 第四阶段：迁移 `app`

`app` 建议晚于 `admin`。原因：

- `app` 是 Next.js，需要处理样式注入和客户端/服务端渲染配合
- 用户侧页面更敏感，视觉回归更容易影响线上体验
- 很多 `app` 页面本身 UI 复杂度不高，收益不如先做 `admin`

`app` 的优先替换目标建议是：

- `message`
- `Modal`
- 少量表单控件
- 反馈页、登录页、问答弹层

## 8. 不建议的迁移方式

以下方式不建议采用：

### 8.1 全局替换 import

不建议做类似：

- 把 `@mui/material` 直接全量替换成 `antd`
- 把 `@ctzhian/ui` 全量替换成 `antd`

原因：

- API 完全不同
- 样式语义不同
- 主题机制不同
- 很多组件没有一一对应关系

### 8.2 直接在页面里混用大量 `antd` 原始 API

如果直接在业务页面里到处写 `antd`，短期看似快，长期会有两个问题：

- 项目将从“依赖 MUI”变成“同时深度依赖 MUI 与 antd”
- 后续再次收敛成本更高

### 8.3 一开始就把全部布局从 `Box` / `Stack` 改掉

这会让改造范围失控。

第一阶段应当接受“表单和交互走 antd，布局和主题暂时保留 MUI”的过渡状态。

## 9. 主要风险

### 9.1 主题双轨并存

迁移期内会同时存在：

- MUI theme
- antd token/theme

风险：

- 设计语言不统一
- 颜色、圆角、字号、间距不完全一致

解决策略：

- 明确一套项目设计 token
- 在兼容层内做 token 映射
- 首阶段接受小范围差异，不追求一次性完全统一

### 9.2 `@ctzhian/ui` 的增强组件缺少平替

例如：

- `Ellipsis`
- `CusTabs`
- `Empty`
- `Image`

这些不一定能直接被 `antd` 原生组件替代。

解决策略：

- 项目内自己封装
- 能复用旧实现的先复用旧实现
- 只替换它们的调用入口，不强求立刻换掉内部实现

### 9.3 文档编辑器页面不适合作为第一批迁移对象

编辑器页已经有：

- 协同编辑
- markdown 与富文本双模式
- TipTap 扩展
- 复杂工具栏与状态同步

如果在编辑器页早期同时引入组件库迁移，排障成本会明显升高。

建议：

- 编辑器相关页面最后迁移
- 只先替换外围非核心组件，例如简单弹窗和提示

## 10. 推荐的第一步实施清单

如果开始正式做，我建议按以下顺序推进：

### 10.1 第一步

在 `web` 安装 `antd`，并建立兼容层目录。

建议目录：

- `web/admin/src/components/ui`
- `web/app/src/components/ui`

### 10.2 第二步

先做以下四个统一入口：

- `notify`
- `Modal`
- `Tabs`
- `Table`

这一步的目标不是全站替换，而是提供以后统一迁移的基础。

### 10.3 第三步

把以下“公共调用点”优先改为走兼容层：

- 请求错误提示
- 通用确认弹窗
- 后台高频表格页
- 后台标签页组件

### 10.4 第四步

选一个低风险页面做完整试点，建议从后台设置页开始，而不是编辑器页。

试点通过标准：

- 页面功能不回退
- 样式可接受
- 交互一致性可接受
- 构建通过

## 11. 建议的实施优先级

建议按以下优先级推进：

1. `admin` 先接入 `antd` 基础能力
2. 建立兼容层
3. 替换 `message` / `Modal` / `Tabs` / `Table`
4. 迁移后台设置类页面
5. 迁移后台列表类页面
6. 最后处理编辑器与复杂自定义页面
7. `app` 在后台方案稳定后再迁移

## 12. 最终建议

对于 PandaWiki，这次迁移最稳妥的目标不应定义为“马上把当前前端改成 antd”，而应定义为：

**把项目从“直接依赖第三方组件库”改造成“依赖项目自有 UI 层”，并让这层 UI 的默认实现逐步转向 Ant Design。**

这样做的价值是：

- 风险更可控
- 页面不需要一次性大规模回归
- 后续替换组件库的主动权回到项目自己手里
- 能兼容当前 `admin` 和 `app` 的不同运行方式

## 13. 下一步建议

如果继续执行，推荐直接进入实施阶段，而不是继续停留在评估层。

建议下一项实际任务是：

**在 `web/admin` 中接入 `antd` 并建立第一版兼容层，先完成 `notify`、`Modal`、`Tabs`、`Table` 四个统一入口。**

这会是最小但真实有效的第一步。
