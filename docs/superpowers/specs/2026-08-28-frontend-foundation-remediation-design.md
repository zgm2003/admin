# 前端基础整改设计

## 1. 状态与目标

状态：设计已确认，等待用户审阅。

本设计解决当前 Admin 前端的基础一致性问题，为后续菜单和业务页面开发固定一套不会反复
重构的运行时基础：

- 中英文语言由一个响应式来源驱动，并通过 `localStorage` 持久化；
- Element Plus 内置文案与业务文案始终同步切换；
- 登录页和已登录布局复用同一语言状态；
- 页面布局优先使用 Element Plus 组件能力，减少为响应式而堆叠的手写 `div` 和 CSS；
- 页面样式统一采用 SCSS 语法，保持线性、易读、低抽象；
- 保留 RouteTabs、内容全屏、Main 滚动所有者和现有权限/认证行为。

本设计只约束前端基础，不修改后端 API、数据库、认证协议、RBAC 算法、菜单数据或业务功能。

## 2. 当前现状与问题证据

### 2.1 语言状态

`web/src/i18n/index.ts` 已具备正确的基础能力：

- 支持 `zh-CN | en-US`；
- 使用 `admin:locale` 读取和写入 `localStorage`；
- 缺失或非法值默认 `zh-CN`；
- `setLocale` 同步更新 Vue I18n 和 `document.documentElement.lang`；
- 中英文 message key 形状已有测试约束。

但 `web/src/main.ts` 固定使用 `element-plus` 的中文 locale，导致 Element Plus 控件不会随业务
语言变化。`web/src/layout/index.vue` 又通过 `ref(readLocale())` 保存独立快照，形成第二个语言来源。

多个页面在模块初始化时执行 `t(...)`，把表格列、搜索字段、选项和校验提示固化为启动时语言，
切换语言后不会更新。这类代码包括用户、角色、认证平台、会话、操作日志和登录表单。

### 2.2 布局与样式

布局已使用 `el-container`、`el-header`、`el-main`、`el-footer` 和 `el-drawer`，但部分页面和
公共组件仍以原生 `div + flex/grid + media query` 承担本可由 Element Plus 表单、行列、间距和
显示工具完成的职责。当前项目也尚未全局引入 `element-plus/theme-chalk/display.css`。

不少 Vue 文件仍使用普通 `<style scoped>`，而项目已经具备 SCSS 入口和变量。后续页面若继续
混用两种样式语法，会让主题、密度和移动端修复难以持续。

### 2.3 旧项目参考结论

旧前端 `D:\github-project\admin_front_ts` 已验证以下可继承行为：

- 根 `App.vue` 用 `el-config-provider` 根据 i18n locale 计算 Element Plus locale；
- 布局使用 Element Plus 容器，并保留 RouteTabs 在内容全屏时作为退出入口；
- 登录页在未登录状态下也属于同一 i18n 运行时；
- 页面和组件样式使用 `lang="scss"`。

只迁移这些成熟行为和信息结构，不复制旧项目的 kernel、Store、Adapter、Manager 或其他重型
架构。

## 3. 已确认的持久化决策

### 3.1 语言

`admin:locale` 是全站唯一语言持久化键：

1. 启动时读取 `localStorage`；
2. 只接受 `zh-CN` 和 `en-US`；
3. 缺失、非法值或读取异常时使用中文作为冷启动默认值；
4. 通过语言切换控件改变语言时，立即更新 Vue I18n、Element Plus locale、HTML `lang` 和
   `localStorage`；
5. 切换不得刷新页面、重新挂载应用或创建第二套语言状态。

登录页和已登录布局都直接消费该状态。未来新增页面不得自行维护语言 ref、Store 或持久化键。

### 3.2 系统设置

现有 `admin:ui-preferences` 版本化记录继续作为界面设置的唯一存储位置，但按本设计调整持久化
边界。记录版本从 `1` 升为 `2`，v2 的 `preferences` 不再包含 `theme` 字段：

| 设置 | 是否持久化 | 冷启动行为 |
| --- | --- | --- |
| 主题模式（明亮/深色） | 否 | 每次应用冷启动使用默认明亮主题 |
| 主题色 | 是 | 读取有效值，否则使用默认主题色 |
| 显示面包屑 | 是 | 读取有效值，否则使用默认值 |
| 显示菜单折叠按钮 | 是 | 读取有效值，否则使用默认值 |
| 显示 RouteTabs | 是 | 读取有效值，否则使用默认值 |
| 菜单只展开一个目录 | 是 | 读取有效值，否则使用默认值 |
| 显示 Footer | 是 | 读取有效值，否则使用默认值 |
| 页面过渡开关 | 是 | 读取有效值，否则使用默认值 |
| 页面过渡类型 | 是 | 读取有效值，否则使用默认值 |

主题模式仍然可以在当前会话中即时切换，也可以由“重置配置”恢复默认，但不得写入持久化记录。
主题色不是主题模式，必须继续持久化。读取现有 v1 记录时执行一次明确的 v1→v2 迁移：严格
校验 v1 的所有字段，保留除 `theme` 外的持久化字段，丢弃旧主题模式，使用明亮主题作为本次
启动值，并成功写回 v2 记录。v2 记录出现未知字段、缺失字段、非法类型或不支持的版本时，
保持现有显式配置错误和内存默认值，不猜测修复。

## 4. 运行时架构

### 4.1 Element Plus 动态国际化

`web/src/main.ts` 只负责安装 Element Plus、业务 i18n、路由和 Pinia，并以当前持久化语言提供
启动默认 locale。运行时语言由 `web/src/App.vue` 的 `el-config-provider` 统一控制：

```text
appI18n.global.locale
        |
        v
computed Element Plus locale
        |
        v
el-config-provider -> RouterView / 登录页 / 已登录布局 / 所有 Element Plus 控件
```

实现要求：

- 使用 Element Plus 官方带类型的中英文 locale 模块；
- `elementLocale` 必须是响应式计算值，`en-US` 使用英文 locale，其他受支持值使用中文
  locale；
- `el-config-provider` 覆盖根下所有页面；
- 语言切换只调用现有 `setLocale`，不通过页面 reload 或重新 mount 让语言“看起来生效”；
- `display.css` 在全局入口只引入一次。

### 4.2 统一语言来源

- Header、SettingDrawer、Login 和所有业务页面使用 `useI18n()`；
- 布局不再维护 `ref(readLocale())` 形式的副本，显示值从 `appI18n.global.locale` 或
  `useI18n().locale` 派生；
- `setLocale` 是唯一写入语言和 HTML `lang` 的函数；
- API 请求继续从 `readLocale()` 读取当前语言发送 `Accept-Language`，不得新增第二个请求语言键。

### 4.3 动态文案规则

任何会显示在界面的翻译结果都不得在模块顶层固化。以下模式必须改为响应式计算或按当前语言
生成的函数：

- AppTable columns；
- Search fields、select options 和日期控件文案；
- Element Plus FormRules 的 message；
- 通知、确认框、错误提示和空态；
- SettingDrawer 的过渡选项和帮助文案。

允许在事件处理函数内调用 `t(...)`，因为事件发生时会读取当前语言；不允许用启动时生成的
静态数组掩盖必填文案缺失。中英文 key 仍由静态 message shape 测试保证一致，禁止静默 fallback。

## 5. 登录页与布局要求

### 5.1 登录页

`web/src/views/auth/login/index.vue` 必须：

- 在未登录状态下直接读取当前 `admin:locale`；
- 提供中英文切换入口，切换后业务文案、校验提示和 Element Plus 表单文案立即更新；
- 登录请求、认证错误、redirect 和 Auth Store 行为保持不变；
- 表单校验规则随 locale 响应式重建或通过校验时动态读取，不能保留旧语言消息；
- 使用 Element Plus 表单结构，布局优先采用 `el-container`、`el-row`、`el-col`、`el-space`
  等组件；必要的品牌装饰结构可以保留语义化元素和少量 `div`。

### 5.2 已登录布局与 RouteTabs

布局固定为：

```text
el-container
|-- el-aside / mobile el-drawer
`-- el-container(direction=vertical)
    |-- el-header
    |-- RouteTabs
    |-- el-main
    `-- el-footer
```

要求：

- Main 是唯一业务纵向滚动所有者；全局可见滚动条保持隐藏但不能禁用滚轮、触摸、键盘和程序化滚动；
- 内容全屏隐藏 Aside、Header、Footer，但始终保留 RouteTabs；
- RouteTabs 仍可切换页面、刷新、关闭标签和退出全屏；
- 移动端继续复用同一个 `AppAside` 和 `el-drawer`，不复制菜单数据或权限逻辑；
- `div` 仅在滚动所有者、过渡包裹、组件插槽或无对应 Element Plus 语义容器时使用，不做机械删除。

### 5.3 Element Plus 组件优先级

页面结构按以下顺序选型：

1. 页面壳层使用 `el-container`、`el-header`、`el-main`、`el-footer`、`el-aside`；
2. 表单和筛选布局使用 `el-form`、`el-form-item`、`el-row`、`el-col`、`el-space`；
3. 选项、开关、状态和帮助使用 `el-select-v2`、`el-switch`、`el-tooltip`、`el-alert`、
   `el-empty`、`el-divider` 等已有组件；
4. 简单响应式显隐使用 `element-plus/theme-chalk/display.css` 的 display class；
5. 只有 Element Plus 无法表达的业务视觉和滚动行为才增加页面专属 CSS。

不创建 `BaseLayout`、`FormLayout`、`PageShell` 或万能响应式组件。公共组件只有在两个以上真实
页面共享稳定交互时才抽取，并保持当前 `AppDialog`、`AppTable`、`Search`、`DIcon` 和 `IconSelect`
的边界。

## 6. SCSS 与样式规则

- 新增或实质重构的 Vue 页面、组件统一使用 `<style scoped lang="scss">`；
- 旧页面不进行无关全量格式化，只有被迁移的文件顺带改为 SCSS；
- 颜色、间距、圆角和状态优先使用 Element Plus CSS 变量以及现有项目变量；
- SCSS 嵌套只表达真实组件层级，禁止深层嵌套和把普通 CSS 搬进更长的嵌套；
- 页面级 flex/grid 仅用于 Element Plus 无法合理表达的专用结构；
- 不引入新的全局样式框架，不复制旧项目的主题变量体系；
- 结构必须在桌面和移动断点下保持稳定尺寸，避免按钮、表格、标签或动态文案撑破布局。

## 7. 分阶段实施范围

### 阶段一：国际化运行时基线

涉及：

- `web/src/main.ts`；
- `web/src/App.vue`；
- `web/src/i18n/index.ts`（仅在需要时补充类型化 locale 映射）；
- `web/src/views/auth/login/index.vue`；
- `web/src/layout/index.vue`、`AppHeader.vue`、`SettingDrawer.vue`；
- `web/src/styles/index.scss`；
- 对应 i18n、main、layout、login、SettingDrawer 测试。

交付结果：Element Plus 与业务文案动态同步，登录页可切换语言，主题模式不落盘，其他设置仍
可恢复，display.css 已接入。

### 阶段二：组件化布局样板

按真实重复度迁移：

1. `AppTable` 和 `Search` 的工具栏、分页、空态和移动端布局；
2. `access/auth-platforms`，作为复杂表单和 TTL 帮助文案样板；
3. `access/menus`，作为树形管理和 `el-row/el-col` 表单样板；
4. 用户、角色、会话、操作日志页面。

每个迁移只处理当前页面真实需要的结构，不改变 API 请求、权限判断、分页语义、错误处理或业务
操作顺序。页面标题、RouteTabs、面包屑和主题设置继续遵循已确认的布局设计。

### 阶段三：持续渐进迁移

以后每新增或实质修改一个页面，都必须检查：

- 是否存在模块级 `t(...)` 固化文案；
- 是否可以用 Element Plus 容器、行列、间距和显示组件替代手写布局；
- 是否使用 `style scoped lang="scss"`；
- 是否保留 loading、空态、错误态、权限、分页、移动端和主题行为；
- 是否需要新增公共组件，且已有至少两个真实复用场景。

## 8. 错误与边界

- `localStorage` 语言值非法时只使用中文冷启动默认，并立即写回规范值；
- UI 偏好 JSON 损坏、版本不支持、字段缺失或字段类型错误时保持现有显式错误提示和内存默认值，
  不伪造持久化成功；
- 主题模式不持久化不是错误，也不应在控制台报告写入失败；
- Element Plus locale 映射缺失属于编程错误，测试必须覆盖两种受支持语言；
- 页面翻译 key 缺失不得显示另一语言的静默 fallback；
- 语言切换失败、存储写入失败和 API 协议错误继续沿现有错误路径展示，不吞错；
- 不以 CSS 隐藏代替权限控制，不修改后端平台过滤和 access 契约。

## 9. 测试与验证

### 9.1 行为测试

至少覆盖：

- 无语言记录时默认中文，非法记录被规范化；
- 切换英文后 `localStorage`、HTML `lang`、业务文案和 Element Plus 内置文案同时变更；
- 刷新应用后恢复上次语言；
- 登录页未登录即可切换语言，校验提示不会停留在旧语言；
- Header、SettingDrawer 和布局不维护独立 locale 副本；
- 主题模式切换只影响当前会话，冷启动恢复默认明亮；
- 主题色、面包屑、菜单按钮、RouteTabs、菜单单开、Footer、页面过渡和过渡类型可持久化恢复；
- 配置重置只重置界面设置，不清除认证、权限或业务状态；
- 内容全屏仍保留 RouteTabs，Main 仍是唯一业务滚动所有者；
- 表格列、搜索字段、选项和校验提示在语言切换后重新渲染。

### 9.2 静态与构建验证

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

补充检查：

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src tests -g "*.ts" -g "*.vue"
rg -n "<style scoped>" src -g "*.vue"
git diff --check
```

第一条搜索不得产生业务代码结果；第二条只允许尚未迁移且本次未修改的旧文件，新增和实质重构
文件不得使用普通 scoped style。除非用户明确要求，不使用 Playwright。

## 10. 非目标与后续边界

- 不在本设计中重写全部页面；
- 不删除所有 `div`，不以标签数量作为组件化指标；
- 不引入新的状态管理、通用布局框架、Adapter、Manager、Factory 或运行时注册器；
- 不修改后端、数据库、菜单平台归属、权限合并、COS、邮件、短信、支付或 AI 模块；
- 不为未来业务端创建占位路由、接口或菜单；
- 不使用主题模式持久化替代真正的主题初始化策略；
- 不把主题色误归类为主题模式，主题色必须继续持久化。

阶段一完成并通过验证后，再为阶段二编写独立 implementation plan。每个业务页面迁移前继续
按当前项目规则核对旧前端对应实现，保留成熟交互，适配当前 DTO、权限、路由、i18n 和组件边界。

## 11. 验收标准

1. `admin:locale` 是唯一语言持久化键，登录前后语言一致；
2. Element Plus 内置文案随业务语言即时切换，不固定为中文；
3. 表格、搜索、表单校验、通知和确认文案不会因模块初始化而冻结旧语言；
4. 主题模式不持久化，其他系统设置均可持久化恢复和重置；
5. 登录页支持中英文切换，不增加第二套语言状态；
6. 页面布局优先使用 Element Plus 组件，复杂布局不依赖大量自定义 flex/grid CSS；
7. 新增和实质重构样式均使用 SCSS，且不引入深层嵌套或全局污染；
8. RouteTabs 全屏、移动 Drawer、Main 滚动、主题色、权限和现有业务行为保持不变；
9. 严格 TypeScript、Vitest 和生产构建验证通过，未使用 Playwright；
10. 方案没有提前引入 AI、支付、COS、邮件、短信或业务端功能。
