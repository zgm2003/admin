# Admin 布局 UI 基线设计

## 1. 状态

状态：设计已确认，可以进入实施。

## 2. 背景

当前后台布局已经具备侧栏、Header、RouteTabs、Main 和 Footer，但视觉职责仍然重复且不统一：

- Header 固定显示“工作台”，不能表达当前动态 RBAC 路由；
- RouteTabs 已经显示当前页面名称，业务页面又重复渲染 `<h1>`；
- `admin-layout__main` 使用 `padding: 0`，各业务页面分别维护 toolbar/content 间距；
- 侧栏菜单缺少稳定的外层 padding、层级密度和内容滚动边界；
- 主题切换散落在 Header，尚无统一的系统设置入口；
- 页签、Footer、面包屑、菜单展开和页面过渡没有统一的 UI 偏好状态。

参考项目 `D:\github-project\admin_front_ts` 已形成较清晰的后台壳层。本切片复用其布局职责、交互密度和 SettingDrawer 形式，但适配当前项目的 RBAC 菜单、Pinia、i18n、Element Plus 主题和严格 TypeScript，不复制旧项目的 kernel、用户存储、搜索、钱包或个人中心。

## 3. 目标

1. 建立统一、紧凑、带明确 padding 的后台布局基线。
2. 删除业务页面与 Header/RouteTabs 重复的页面标题。
3. 让 Header 使用当前 RBAC 菜单树生成真实面包屑。
4. 在 Header 增加系统设置入口，并通过抽屉管理真实生效的 UI 偏好。
5. 保持全局无可见滚动条，Main 继续作为业务内容滚动所有者。
6. 不改变 RBAC、API、认证、数据库或业务页面请求流。

## 4. 布局职责

后台壳层固定为：

```text
Aside | Header
      | RouteTabs
      | Main -> RouteView
      | Footer
```

各区域职责唯一：

- Aside：品牌、Dashboard 和后端返回的 RBAC 菜单；
- Header：菜单折叠、当前路由面包屑、语言、系统设置和账户操作；
- RouteTabs：已访问页面切换、刷新、内容全屏和页签关闭；
- Main：统一外层间距、业务页面滚动和路由过渡；
- Footer：版权信息，可由 UI 偏好控制显示。

页面名称只出现在 Header 面包屑和 RouteTabs。普通后台业务页面不再显示独立 `<h1>`。

## 5. 视觉基线

### 5.1 间距与内容面

- Main 桌面端统一使用 `8px` 外层 padding，移动端使用 `10px`；
- 普通业务页占满可用高度，使用一个内容面承载筛选、工具栏、表格和分页；
- 内容面内部使用 `16px` 基础 padding，筛选区和表格区间距为 `12px`；
- 不在内容面内部继续嵌套装饰卡片；
- 树形菜单管理页可以保持专用表格结构，但使用相同的外层内容面和间距；
- 页面背景、边框、文字、悬停和选中色只使用当前 Element Plus 变量及项目 shell token；
- 明暗主题使用同一套结构，不为暗色模式维护另一套页面布局。

### 5.2 业务页面标题

以下页面删除内部 `<h1>`、标题图标和只用于展示标题的 toolbar 区域：

- 菜单管理；
- 角色管理；
- 用户管理；
- 认证平台；
- 会话管理；
- 操作日志。

标题区域内已有的刷新、新增等真实操作迁入筛选区或 AppTable 工具栏，不能随标题一起删除。

Dashboard 保留欢迎、状态和任务区域；登录和注册页面保留其表单标题。

## 6. Aside

- 展开宽度对齐旧项目的工作密度，收起后只显示图标；
- 外层提供稳定 padding，菜单本身拥有独立纵向滚动；
- 品牌区、菜单区和底部边界互不挤压；
- Dashboard 仍为前端固定菜单，动态业务菜单继续读取 `access.menuTree`；
- 菜单项、子菜单和折叠态统一高度、圆角、层级缩进和悬停状态；
- `uniqueOpened` 只控制 Element Plus 菜单是否同时展开多个目录，不改变后端菜单树；
- 移动端 Drawer 复用同一个 AppAside，不复制第二套菜单数据。

数据流保持：

```text
GET /api/v1/access -> access store -> AppAside -> AccessMenuNode
```

## 7. Header 与面包屑

Header 左侧按顺序显示：

```text
菜单折叠按钮 -> 当前路由面包屑
```

Header 右侧按顺序显示：

```text
语言 -> 系统设置 -> 当前用户 -> 退出登录
```

面包屑规则：

- Dashboard 只显示 Dashboard；
- 动态页面从 `access.menuTree` 中按 `route.path` 查找目录到页面的路径；
- 面包屑节点使用菜单节点 `titleKey` 和当前 i18n 翻译；
- 找不到已认证动态路由对应菜单节点时不猜测标题，显示明确的布局错误状态；
- 目录面包屑不提供虚构路由，只有带真实 `path` 的页面节点可导航；
- Header 不再固定显示“工作台”。

## 8. 系统设置抽屉

Header 设置图标打开右侧 `320px` 抽屉。抽屉分为主题、界面显示、页面过渡和重置操作四组。

### 8.1 主题

- 明亮 / 深色分段选择；
- 预设主题色 swatch；
- Element Plus `el-color-picker` 自定义主题色；
- 主题色写入 `--el-color-primary`，并同步生成 Element Plus 当前需要的浅色和深色派生变量，不能只改主色导致组件状态不一致。

### 8.2 界面显示

- 显示面包屑；
- 显示菜单折叠按钮；
- 显示 RouteTabs；
- 菜单只展开一个目录；
- 显示 Footer。

隐藏 RouteTabs 后，Header 与 Main 直接相邻；内容全屏仍可通过 RouteTabs 设置菜单进入，因此进入内容全屏期间不允许关闭 RouteTabs，避免丢失退出入口。

隐藏菜单折叠按钮时，如果侧栏处于收起状态，先展开侧栏再隐藏按钮，避免用户无法恢复。

### 8.3 页面过渡

- 开启 / 关闭路由页面过渡；
- 过渡类型只提供当前实际实现的有限集合；
- 遵守 `prefers-reduced-motion`，系统要求减少动态效果时强制使用近乎零时长过渡。

### 8.4 重置

- “重置配置”只重置本地 UI 偏好，不清除认证 Cookie、用户身份、RBAC 权限或业务缓存；
- 重置后立即应用默认主题、默认主题色和默认布局开关；
- 本切片不提供含义模糊的“清理全部缓存”操作。

## 9. UI 偏好状态

新增具体的 `uiPreferences` Pinia Store，职责仅限当前布局需要的状态：

- `theme`；
- `primaryColor`；
- `showBreadcrumb`；
- `showMenuToggle`；
- `showRouteTabs`；
- `uniqueOpened`；
- `showFooter`；
- `pageTransition`；
- `transitionName`。

状态保存到单一版本化 localStorage 记录，不散落多个无版本键。要求：

- storage 不存在时使用明确的项目默认值；
- JSON 损坏、字段缺失、未知字段或非法枚举时返回显式错误；
- App 根组件在任何路由页面渲染前调用 Store 的安全初始化；Store 捕获偏好读取错误后使用内存默认值，Layout 持续展示用户可见错误和明确的重置操作；该恢复行为必须可见，不能静默吞掉；
- Store 不读取 Access、Auth 或业务 API；
- 当前会话的侧栏折叠、移动端 Drawer 和内容全屏仍属于瞬时布局状态，不写入持久化偏好。

## 10. 页面迁移范围

本切片迁移：

1. `system/roles`；
2. `system/users`；
3. `system/menus`；
4. `system/auth-platforms`；
5. `system/sessions`；
6. `system/operation-logs`。

已有 AppTable 和 AppDialog 继续使用。页面只调整结构、标题和间距，不改变：

- 请求参数；
- API 调用顺序；
- 权限码；
- 分页语义；
- 确认框和错误信息；
- 角色权限矩阵、菜单树和会话操作的业务规则。

## 11. i18n

新增的抽屉标题、分组、主题模式、显示开关、过渡选项、重置按钮和偏好错误信息同时加入 `zh-CN` 与 `en-US`。

页面删除标题后，不删除仍由菜单、面包屑或 RouteTabs 使用的既有翻译键。

## 12. 非目标

- 不复制旧项目 SearchDialog；
- 不增加个人中心、钱包、头像上传或角色名称展示；
- 不修改后端 RBAC 菜单协议；
- 不增加服务端 UI 偏好表；
- 不做列配置持久化；
- 不引入通用 Layout Manager、运行时主题注册器或 Platform Adapter；
- 不重做登录页、注册页和 Dashboard 的业务内容。

## 13. 测试与验收

至少覆盖：

- Header 根据 Dashboard 和动态 RBAC 路由生成正确面包屑；
- 设置按钮打开 SettingDrawer；
- 每个设置项立即改变真实布局；
- UI 偏好严格解析、保存、恢复和重置；
- 非法持久化数据产生明确错误；
- 隐藏菜单折叠按钮不会留下无法展开的侧栏；
- 内容全屏期间不能隐藏退出入口；
- 业务页面不再渲染重复 `<h1>`；
- Dashboard、登录和注册标题仍保留；
- Main 是唯一业务滚动所有者，页面没有全局可见滚动条；
- 明暗主题和主题色在 Element Plus 控件中真实生效；
- 前端业务 TypeScript 不出现显式 `any`、`as any`、`any[]` 或 `Record<string, any>`。

验证命令：

```powershell
cd D:\admin\web
pnpm vitest run
pnpm build
```

本切片不启动或停止用户服务，不创建 Git 提交。
