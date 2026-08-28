# 前端基础整改实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立可持续的 Admin 前端基础，使 localStorage 持久化语言、Element Plus 内置文案、业务文案、布局组件和 SCSS 样式在登录前后保持一致，并为后续页面开发提供稳定边界。

**Architecture:** 以 web/src/i18n/index.ts 的 AppLocale 为唯一语言协议，通过根 App.vue 的响应式 el-config-provider 把 Element Plus locale 传给所有页面。UI 偏好继续由 uiPreferences Store 管理，但持久化记录升级为 v2，主题模式只留在运行时，其他设置继续落盘。页面采用现有“视图 -> API 模块 -> request”数据流，动态翻译配置使用 computed 或当前语言生成函数，公共组件只在真实复用成立时抽取。

**Tech Stack:** Vue 3 Composition API、TypeScript strict、Vue Router 4、Pinia、vue-i18n 11、Element Plus 2、Sass/SCSS、Vitest、Vite。

**Spec:** docs/superpowers/specs/2026-08-28-frontend-foundation-remediation-design.md

## Global Constraints

- 支持语言严格为 zh-CN | en-US，默认语言仅用于冷启动兜底。
- admin:locale 是唯一语言持久化键；页面不得创建第二套语言状态或第二个语言键。
- Element Plus locale 必须由根 el-config-provider 响应式驱动，不能把中文安装参数当成最终运行时方案。
- 主题模式不持久化；主题色、面包屑、菜单折叠按钮、RouteTabs、菜单单开、Footer、页面过渡和过渡类型必须持久化。
- 现有 admin:ui-preferences 从 v1 明确迁移到 v2；v2 preferences 不包含 theme。
- Main 继续是唯一业务纵向滚动所有者；内容全屏必须保留 RouteTabs 作为退出入口。
- 前端请求流固定为 view -> api/<module>.ts -> utils/request.ts -> Go API。
- 不修改后端、数据库、认证协议、RBAC 菜单数据、权限合并算法、COS、邮件、短信、支付或 AI 模块。
- 业务 TypeScript 禁止显式 any、any[]、as any、Record<string, any>、@ts-ignore 和关闭 strict。
- 新增或实质重构的 Vue 样式使用 style scoped lang="scss"；不做无关全仓格式化。
- 布局优先使用 el-container、el-row、el-col、el-space、el-form、el-divider、el-alert、el-empty 和 el-select-v2。
- 不创建 BaseLayout、FormLayout、PageShell、Manager、Factory、Adapter 或万能响应式组件。
- 除非用户明确要求，不使用 Playwright；使用 Vitest、vue-tsc 和生产构建验证。
- Git 提交只在维护者明确授权后执行；各任务中的提交命令是审批后的检查点，不构成自动提交授权。

## File Map

| 区域 | 文件 | 责任 |
| --- | --- | --- |
| 语言运行时 | web/src/main.ts、web/src/App.vue、web/src/i18n/index.ts | 初始化语言、Element Plus locale 映射和根配置提供器 |
| 公共文案 | web/src/i18n/messages/zh-CN.ts、web/src/i18n/messages/en-US.ts | AppTable/Search 默认状态、操作和可访问性文案 |
| UI 偏好 | web/src/utils/ui-preferences.ts、web/src/store/ui-preferences.ts | v1→v2 读取迁移、严格解析、运行时主题和持久化设置 |
| 登录与壳层 | web/src/views/auth/login/index.vue、web/src/layout/index.vue、web/src/layout/components/AppHeader.vue、web/src/layout/components/SettingDrawer.vue | 登录前后语言入口、布局语言来源、主题设置和全屏边界 |
| 公共组件 | web/src/components/AppTable/src/index.vue、web/src/components/Search/src/index.vue | 表格、搜索工具栏、分页和移动端结构 |
| 业务页面 | web/src/views/account/users/index.vue、web/src/views/access/roles/index.vue、web/src/views/access/auth-platforms/index.vue、web/src/views/account/sessions/index.vue、web/src/views/system/operation-logs/index.vue、web/src/views/access/menus/index.vue | 动态列、筛选项、选项、表单结构和 SCSS 迁移 |
| 测试 | web/tests/main.test.ts、web/tests/App.test.ts、web/tests/utils/ui-preferences.test.ts、web/tests/store/ui-preferences.test.ts、布局/登录/公共组件/业务页面测试 | 行为、协议、响应式语言和构建前静态约束 |

---

### Task 1: 建立响应式 Element Plus 国际化运行时

**Files:**
- Modify: web/src/i18n/index.ts
- Modify: web/src/main.ts
- Modify: web/src/App.vue
- Modify: web/tests/main.test.ts
- Create: web/tests/App.test.ts

**Interfaces:**
- elementPlusLocaleFor(locale: AppLocale): Language 严格映射 zh-CN 到 zhCn、en-US 到 enUs。
- App.vue 根模板用 el-config-provider 包住现有 RouterView 和 UI 偏好初始化。
- main.ts 安装 Element Plus 时只提供启动语言，并全局引入 element-plus/theme-chalk/display.css 一次。

- [ ] **Step 1: Write the failing tests**

在 web/tests/main.test.ts 中 mock readLocale 返回 en-US、mock elementPlusLocaleFor 返回可识别对象，并断言：

    expect(appHarness.app.use).toHaveBeenCalledWith(ElementPlus, {
      locale: { name: 'english-locale' },
    })

新增 web/tests/App.test.ts，用 pinia、ElementPlus、appI18n 挂载 App，stub RouterView 为包含 el-pagination 的组件。断言根组件存在 ElConfigProvider，初始 provider locale 为中文；调用 setLocale('en-US') 后 await nextTick()，断言 provider locale prop 为英文 locale 对象，并断言 pagination 子组件从注入的英文语言包读取 total 文案。测试不得只检查全局 i18n 文案，必须覆盖 Element Plus 注入链路。

- [ ] **Step 2: Run tests to verify they fail**

    cd D:\admin\web
    pnpm vitest run tests/main.test.ts tests/App.test.ts --pool=threads --maxWorkers=1

Expected: main test 因固定中文 locale 失败，App test 因没有 ElConfigProvider 失败。

- [ ] **Step 3: Implement the runtime boundary**

在 web/src/i18n/index.ts 使用 Element Plus 官方带类型入口：

    import type { Language } from 'element-plus/es/locale'
    import enUs from 'element-plus/es/locale/lang/en'
    import zhCn from 'element-plus/es/locale/lang/zh-cn'

    export function elementPlusLocaleFor(locale: AppLocale): Language {
      return locale === 'en-US' ? enUs : zhCn
    }

在 main.ts 调用 initializeLocale() 后，用 elementPlusLocaleFor(readLocale()) 作为 Element Plus 启动参数，并导入 display.css。在 App.vue 用 useI18n().locale 创建 computed provider locale；由于 Vue I18n 的 locale 类型可以宽于 AppLocale，显式分支为 locale.value === 'en-US' ? 'en-US' : 'zh-CN'，不使用类型断言。保持现有 useUIPreferencesStore().initializeSafely() 在根组件初始化。

- [ ] **Step 4: Run focused tests and type-check**

    cd D:\admin\web
    pnpm vitest run tests/main.test.ts tests/App.test.ts --pool=threads --maxWorkers=1
    pnpm exec vue-tsc --noEmit

Expected: 两个测试通过，vue-tsc 无新增错误；切换语言不需要 reload 或重新 mount。

- [ ] **Step 5: Commit**

    cd D:\admin
    git add web/src/main.ts web/src/App.vue web/src/i18n/index.ts web/tests/main.test.ts web/tests/App.test.ts
    git commit -m "fix: 建立 Element Plus 动态双语运行时"

### Task 2: 将 UI 偏好持久化升级为 v2

**Files:**
- Modify: web/src/utils/ui-preferences.ts
- Modify: web/src/store/ui-preferences.ts
- Modify: web/tests/utils/ui-preferences.test.ts
- Modify: web/tests/store/ui-preferences.test.ts

**Interfaces:**
- Runtime UIPreferences 保留 theme: ThemeMode，供当前会话切换；持久化 v2 不包含 theme。
- parseStoredUIPreferences(value: unknown) 严格解析 v2，返回 theme: 'light' 的运行时对象。
- readUIPreferences() 对有效 v1 执行一次迁移、写回 v2；v2 直接读取；非法版本/字段/存储失败抛出 UIPreferencesError。
- writeUIPreferences(preferences: UIPreferences) 只序列化 v2 持久化字段。

- [ ] **Step 1: Write the failing tests**

在 web/tests/utils/ui-preferences.test.ts 增加：

    it('writes v2 without the runtime-only theme', () => {
      writeUIPreferences({ ...defaultUIPreferences, theme: 'dark', primaryColor: '#059669' })
      const stored = JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '') as {
        version: number
        preferences: Record<string, unknown>
      }
      expect(stored.version).toBe(2)
      expect(stored.preferences.theme).toBeUndefined()
      expect(stored.preferences.primaryColor).toBe('#059669')
    })

    it('migrates a valid v1 record once and uses light for the cold start theme', () => {
      localStorage.setItem(uiPreferencesStorageKey, JSON.stringify({
        version: 1,
        preferences: { ...defaultUIPreferences, theme: 'dark', showFooter: false },
      }))
      expect(readUIPreferences()).toEqual({ ...defaultUIPreferences, theme: 'light', showFooter: false })
      expect(JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '').version).toBe(2)
    })

增加 v2 带 theme、缺失字段、未知字段和不支持版本的拒绝测试；在 Store 测试中断言纯 update({ theme: 'dark' }) 改变 DOM 主题、不会调用 Storage.setItem，并且不会修改已有 v2 记录；混合更新只序列化非 theme 字段。

- [ ] **Step 2: Run tests to verify they fail**

    cd D:\admin\web
    pnpm vitest run tests/utils/ui-preferences.test.ts tests/store/ui-preferences.test.ts --pool=threads --maxWorkers=1

Expected: 当前 parser 只接受 v1 且会序列化 theme，新增测试失败。

- [ ] **Step 3: Implement strict v1/v2 separation**

在 ui-preferences.ts：

1. 新增 PersistedUIPreferencesV2 和 StoredUIPreferencesV2，字段为 primaryColor、6 个布尔开关和 transitionName。
2. 保留 v1 专用严格解析器，先验证全部旧字段，再丢弃 theme。
3. v2 解析器拒绝 theme、未知字段和缺失字段，并返回 theme: 'light'。
4. readUIPreferences 按 version 分支：无记录返回默认运行时对象；v1 解析后调用 writeUIPreferences 写回 v2；v2 直接返回；其他版本抛出明确错误。
5. writeUIPreferences 验证运行时对象、移除 theme、写入 { version: 2, preferences }。
6. 为 UIPreferencesError 增加 operation: 'read' | 'write'，Store 根据 operation 将迁移写失败识别为 persistenceError='write'，其他解析失败识别为 invalid。

在 Store 中把 theme 从持久化 patch 中分离：纯 update({ theme }) 只调用 applyTheme 并更新内存 preferences.theme，不得调用 writeUIPreferences 或 Storage.setItem；同时包含其他字段的 patch 只把非 theme 字段写入 v2，再应用 theme。reset() 写 v2 的非 theme 字段，同时把运行时主题恢复为明亮。主题模式变化不得触发 localStorage 写入。

- [ ] **Step 4: Run focused tests**

    cd D:\admin\web
    pnpm vitest run tests/utils/ui-preferences.test.ts tests/store/ui-preferences.test.ts --pool=threads --maxWorkers=1

Expected: 所有偏好测试通过，存储记录 version=2 且不存在 preferences.theme。

- [ ] **Step 5: Commit**

    cd D:\admin
    git add web/src/utils/ui-preferences.ts web/src/store/ui-preferences.ts web/tests/utils/ui-preferences.test.ts web/tests/store/ui-preferences.test.ts
    git commit -m "fix: 调整界面偏好持久化边界"

### Task 3: 统一登录页和后台布局的语言来源

**Files:**
- Create: web/src/components/LocaleSwitch/src/index.vue
- Create: web/src/components/LocaleSwitch/index.ts
- Modify: web/src/layout/index.vue
- Modify: web/src/layout/components/AppHeader.vue
- Modify: web/src/views/auth/login/index.vue
- Modify: web/tests/layout/index.test.ts
- Modify: web/tests/views/auth/login/index.test.ts
- Create: web/tests/components/LocaleSwitch/src/index.test.ts

**Interfaces:**
- LocaleSwitch 只接受可选 testId: string（默认 locale-switch），无业务状态或本地持久化；读取 useI18n().locale，并在校验下拉 command 后调用 setLocale。菜单项测试标识为 `${testId}-zh` 与 `${testId}-en`。
- AppHeader 不再接收 locale prop，不再发出 changeLocale；它渲染 LocaleSwitch，只保留 toggleMenu 事件。
- layout/index.vue 不再导入 readLocale、拥有 locale ref 或把 locale 复制给子组件。
- Login 校验规则是 computed<FormRules<LoginForm>>，语言变化会重建校验消息。

- [ ] **Step 1: Write the failing tests**

新增 LocaleSwitch 测试：使用 ElementPlus 和 appI18n 挂载，触发英文 command，断言 localStorage 和 document.documentElement.lang。

在 web/tests/views/auth/login/index.test.ts 增加：

    it('updates validation messages when the locale changes', async () => {
      const wrapper = mountLogin()
      await wrapper.get('[data-testid="login-email"]').trigger('blur')
      expect(document.body.textContent).toContain('邮箱为必填项')
      setLocale('en-US')
      await wrapper.vm.$nextTick()
      await wrapper.get('[data-testid="login-email"]').trigger('blur')
      expect(document.body.textContent).toContain('Email is required')
    })

    it('allows language switching before authentication', async () => {
      const wrapper = mountLogin()
      await wrapper.get('[data-testid="login-locale-switch"]').trigger('click')
      getPopupItem('login-locale-switch-en').click()
      await wrapper.vm.$nextTick()
      expect(localStorage.getItem('admin:locale')).toBe('en-US')
      expect(wrapper.text()).toContain('Welcome back')
    })

更新 layout/index.test.ts，使语言测试通过 LocaleSwitch 输出完成，并确认 Layout 不再接收 locale prop。

- [ ] **Step 2: Run tests to verify they fail**

    cd D:\admin\web
    pnpm vitest run tests/components/LocaleSwitch/src/index.test.ts tests/views/auth/login/index.test.ts tests/layout/index.test.ts --pool=threads --maxWorkers=1

Expected: 新 control test 无法解析组件，login 校验消息保持启动语言，因为 rules 当前是普通对象。

- [ ] **Step 3: Implement LocaleSwitch and remove the layout locale snapshot**

实现 LocaleSwitch：使用 el-dropdown、Connection 图标、typed command 校验和 setLocale。testId 默认 locale-switch，页面传 login-locale-switch 时 dropdown、中文菜单项和英文菜单项依次为 login-locale-switch、login-locale-switch-zh、login-locale-switch-en。

在 AppHeader.vue 中移除内联 dropdown、locale prop、changeLocale emit 和 handleLocaleCommand，替换为 LocaleSwitch；设置按钮行为保持不变。

在 layout/index.vue 中移除 locale ref、handleLocaleChange、readLocale/setLocale 引用，不再给 AppHeader 传 locale 或 changeLocale。

在 login/index.vue 中把 rules 改为 computed<FormRules<LoginForm>>，每次计算使用当前 t；插入 LocaleSwitch，并保持登录 API、认证错误和安全 redirect 不变。

- [ ] **Step 4: Run focused tests and type-check**

    cd D:\admin\web
    pnpm vitest run tests/components/LocaleSwitch/src/index.test.ts tests/views/auth/login/index.test.ts tests/layout/index.test.ts --pool=threads --maxWorkers=1
    pnpm exec vue-tsc --noEmit

Expected: 登录前后切换语言、校验消息更新和布局测试全部通过。

- [ ] **Step 5: Commit**

    cd D:\admin
    git add web/src/components/LocaleSwitch web/src/layout/index.vue web/src/layout/components/AppHeader.vue web/src/views/auth/login/index.vue web/tests/components/LocaleSwitch/src/index.test.ts web/tests/layout/index.test.ts web/tests/views/auth/login/index.test.ts
    git commit -m "fix: 统一登录页与后台布局语言状态"

### Task 4: 修复业务页面的动态翻译配置

**Files:**
- Modify: web/src/views/account/users/index.vue
- Modify: web/src/views/access/roles/index.vue
- Modify: web/src/views/access/auth-platforms/index.vue
- Modify: web/src/views/account/sessions/index.vue
- Modify: web/src/views/system/operation-logs/index.vue
- Modify: web/src/views/access/menus/index.vue
- Modify: 对应六个页面的 Vitest 测试

**Interfaces:**
- 每个包含 t(...) 的 TableColumn<Row>[] 都是 computed<TableColumn<Row>[]>(...)。
- 每个包含 t(...) 的 SearchField[] 都是 computed<SearchField[]>(...)。
- menuTypeOptions 是 computed<Array<{ label: string; value: ManagedMenuType }>>。
- 事件处理函数在事件发生时调用 t(...)；API payload、DTO 和 permission checks 不变。

- [ ] **Step 1: Write the failing tests**

在 users、roles、auth-platforms、sessions 和 operation-logs 页面测试中，使用现有 mount helper 和 API mock，挂载一行真实类型数据，调用 setLocale('en-US')、await nextTick()，断言表头和至少一个搜索标签变为英文。

在 menus 页面测试中增加：

    setLocale('en-US')
    await wrapper.vm.$nextTick()
    expect(document.body.textContent).toContain('Directory')

同时增加源结构断言或针对选项生成函数的测试，确保 menuTypeOptions 不是模块初始化时由 t(...) 生成的静态数组。

- [ ] **Step 2: Run the six page tests and confirm stale-label failures**

    cd D:\admin\web
    pnpm vitest run tests/views/account/users/index.test.ts tests/views/access/roles/index.test.ts tests/views/access/auth-platforms/index.test.ts tests/views/account/sessions/index.test.ts tests/views/system/operation-logs/index.test.ts tests/views/access/menus/index.test.ts --pool=threads --maxWorkers=1

Expected: 新断言会定位仍冻结在模块创建时的配置；已经使用 computed 的页面继续通过。

- [ ] **Step 3: Convert remaining static translated definitions**

在 access/menus/index.vue，把静态 menuTypeOptions 改为：

    const menuTypeOptions = computed<Array<{ label: string; value: ManagedMenuType }>>(() => [
      { label: t('menu.type.directory'), value: 'directory' },
      { label: t('menu.type.page'), value: 'page' },
      { label: t('menu.type.action'), value: 'action' },
    ])

逐页检查 tableColumns、searchFields 在模板中以 computed ref 传给 AppTable/Search，不把翻译结果移动到 API 模块、Pinia 或 localStorage。时间、TTL 和状态显示仍在视图/computed 层完成，不修改后端时间和查询参数。

- [ ] **Step 4: Run page tests after implementation**

    cd D:\admin\web
    pnpm vitest run tests/views/account/users/index.test.ts tests/views/access/roles/index.test.ts tests/views/access/auth-platforms/index.test.ts tests/views/account/sessions/index.test.ts tests/views/system/operation-logs/index.test.ts tests/views/access/menus/index.test.ts --pool=threads --maxWorkers=1

Expected: 中文启动断言和英文切换断言都通过，页面不重复请求、不改变权限。

- [ ] **Step 5: Commit**

    cd D:\admin
    git add web/src/views/account/users/index.vue web/src/views/access/roles/index.vue web/src/views/access/auth-platforms/index.vue web/src/views/account/sessions/index.vue web/src/views/system/operation-logs/index.vue web/src/views/access/menus/index.vue web/tests/views
    git commit -m "fix: 修复业务页面语言切换后的动态文案"

### Task 5: 迁移公共布局与页面样式到 Element Plus 和 SCSS

该任务分为两个可独立提交的子阶段，先稳定公共组件，再迁移页面结构。

#### Task 5A: AppTable 与 Search 组件

**Files:**
- Modify: web/src/components/AppTable/src/index.vue
- Modify: web/src/components/Search/src/index.vue
- Modify: web/src/i18n/messages/zh-CN.ts
- Modify: web/src/i18n/messages/en-US.ts
- Modify: web/tests/components/AppTable/src/index.test.ts
- Modify: web/tests/components/Search/src/index.test.ts

- [ ] **Step 1: Write failing component layout tests**

为 AppTable 增加断言：工具栏使用 Element Plus spacing 容器，刷新仍只 emit 一次：

    it('keeps toolbar actions in an Element Plus spacing container', () => {
      const wrapper = mount(AppTable<UserRow>, { props: baseProps })
      expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
    })

为 Search 增加断言：仍由 el-form 提交、字段值在语言变化后保留、选择字段仍渲染 el-select-v2；不依赖实现细节 class。为两个组件增加默认文案测试：不传 refreshLabel/queryLabel 等 props 时，setLocale('en-US') 后按钮、空态、错误态和 aria label 使用英文。

- [ ] **Step 2: Run tests to verify they fail**

    cd D:\admin\web
    pnpm vitest run tests/components/AppTable/src/index.test.ts tests/components/Search/src/index.test.ts --pool=threads --maxWorkers=1

Expected: ElSpace 断言因当前 toolbar 使用 raw div 失败；刷新、分页和 selection 的既有行为测试仍是基线。

- [ ] **Step 3: Replace only direct layout equivalents**

在两份 i18n message 文件新增完全对称的 key：appTable.ariaLabel、appTable.refresh、appTable.requestFailed、appTable.empty、search.query、search.reset、search.expand、search.collapse。

在 AppTable 中使用 useI18n() 和 computed resolvedAriaLabel、resolvedRefreshLabel、resolvedEmptyMessage、resolvedErrorMessage；调用方传入非空 prop 时优先传入值，否则使用新增 key。保留外层 app-table div，因为它拥有 ARIA region、固定 footer 高度和滚动边界；用 el-space wrap size="small" 替换 toolbar-left/right 的间距包裹，保留两个业务 slot；保留 el-table、el-table-column、el-pagination、el-empty、el-alert 作为行为组件；样式块改为 style scoped lang="scss"，保留 768px 移动端规则。

在 Search 中移除中文 default props，使用 useI18n() 和 computed resolvedQueryLabel、resolvedResetLabel、resolvedExpandLabel、resolvedCollapseLabel；调用方显式 prop 优先，未传时读取新增 search key。保留 el-form 和类型收窄逻辑；用 el-space wrap 处理查询/重置/展开操作间距；样式块改为 SCSS，只保留 Element Plus form item 所需间距；不创建通用响应式 grid composable。

- [ ] **Step 4: Run component tests and type-check**

    cd D:\admin\web
    pnpm vitest run tests/components/AppTable/src/index.test.ts tests/components/Search/src/index.test.ts --pool=threads --maxWorkers=1
    pnpm exec vue-tsc --noEmit

Expected: 组件行为全部通过，refresh 仍只 emit、不调用 API，strict 类型无新增错误。

- [ ] **Step 5: Commit**

    cd D:\admin
    git add web/src/components/AppTable/src/index.vue web/src/components/Search/src/index.vue web/src/i18n/messages/zh-CN.ts web/src/i18n/messages/en-US.ts web/tests/components/AppTable/src/index.test.ts web/tests/components/Search/src/index.test.ts
    git commit -m "refactor: 收敛表格与搜索布局组件"

#### Task 5B: 登录、设置和业务页面布局迁移

**Files:**
- Modify: web/src/views/auth/login/index.vue
- Modify: web/src/layout/index.vue
- Modify: web/src/layout/components/SettingDrawer.vue
- Modify: web/src/views/access/auth-platforms/index.vue
- Modify: web/src/views/access/menus/index.vue
- Modify: web/src/views/account/users/index.vue
- Modify: web/src/views/access/roles/index.vue
- Modify: web/src/views/account/sessions/index.vue
- Modify: web/src/views/system/operation-logs/index.vue
- Modify: related tests

- [ ] **Step 1: Record structure and add responsive regression tests**

编辑每个文件前执行：

    cd D:\admin\web
    rg -n "<div|display:|grid-template|flex-direction|<el-row|<el-col|<el-space|<style scoped>" src/views/auth/login/index.vue src/layout src/views/access src/views/account src/views/system

测试必须覆盖：登录在 1200px 和 390px 保持邮箱/密码表单、语言入口和提交 loading；SettingDrawer 主题只影响当前会话、其他开关和过渡选择器可持久化；认证平台四个 TTL 帮助和恢复默认动作仍在；菜单父级/类型/权限字段和 protected 行为不变；用户、角色、会话、操作日志原有 loading/empty/error/pagination/permission 断言不变。

- [ ] **Step 2: Migrate login shell to Element Plus row/column**

在 login/index.vue 保留 main、section、header 和 form 语义元素；用 el-row/el-col 替换两列 CSS grid，桌面品牌区和表单各 span 12，移动端各 span 24；简单品牌详情使用 Element Plus display class，复杂品牌装饰保留页面专属样式；样式改为 SCSS，删除被组件结构替换的 grid 规则，保留键盘焦点、8px 圆角、主题变量和 reduced-motion。

- [ ] **Step 3: Migrate settings and complex forms**

在 SettingDrawer.vue 保留 el-drawer 和语义分组；用 el-space 处理主题按钮、重置动作和过渡选项间距；标签/开关确需两列时用 el-row/el-col，不创建 settings-row 组件；过渡 options 使用 computed；全屏时 RouteTabs 开关继续 disabled。

在 auth-platforms/index.vue 和 menus/index.vue，把已有 form grid 改为 el-row :gutter="16" + el-col，使用 xs=24、sm=12 和对应 lg span 表达原断点；保留 el-form-item、TTL tooltip、认证平台身份、菜单图标选择和 protected 行为；两页样式改为 SCSS。

在 users、roles、sessions、operation-logs 中，AppTable/Search 继续是边界；操作簇用 el-space，多字段表单/统计摘要用 el-row/el-col；API、权限、矩阵、会话操作和日志详情留在页面；修改的样式块统一改为 SCSS。

- [ ] **Step 4: Run migrated page tests and build checks**

    cd D:\admin\web
    pnpm vitest run tests/layout/index.test.ts tests/layout/components/SettingDrawer.test.ts tests/views/auth/login/index.test.ts tests/views/access/auth-platforms/index.test.ts tests/views/access/menus/index.test.ts tests/views/account/users/index.test.ts tests/views/access/roles/index.test.ts tests/views/account/sessions/index.test.ts tests/views/system/operation-logs/index.test.ts --pool=threads --maxWorkers=1
    pnpm exec vue-tsc --noEmit
    pnpm build

Expected: 所有行为测试通过，生产构建完成，没有新增可见 body scrollbar 或横向溢出。

- [ ] **Step 5: Commit**

    cd D:\admin
    git add web/src/views/auth/login/index.vue web/src/layout/index.vue web/src/layout/components/SettingDrawer.vue web/src/views/access/auth-platforms/index.vue web/src/views/access/menus/index.vue web/src/views/account/users/index.vue web/src/views/access/roles/index.vue web/src/views/account/sessions/index.vue web/src/views/system/operation-logs/index.vue web/tests/views/auth/login/index.test.ts web/tests/layout/index.test.ts web/tests/layout/components/SettingDrawer.test.ts web/tests/views/access/auth-platforms/index.test.ts web/tests/views/access/menus/index.test.ts web/tests/views/account/users/index.test.ts web/tests/views/access/roles/index.test.ts web/tests/views/account/sessions/index.test.ts web/tests/views/system/operation-logs/index.test.ts
    git commit -m "refactor: 迁移后台布局到 Element Plus 与 SCSS"

### Task 6: 完整验收、静态清理与交付检查

**Files:**
- Modify: only files identified by verification failures; do not add unrelated refactors.
- Test: complete web/tests suite.

- [ ] **Step 1: Run complete frontend verification**

    cd D:\admin\web
    pnpm vitest run --pool=threads --maxWorkers=1
    pnpm build

Expected: all Vitest tests pass and vue-tsc plus Vite production build complete successfully.

- [ ] **Step 2: Run static contract scans**

    cd D:\admin\web
    rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src tests -g "*.ts" -g "*.vue"
    rg -n "<style scoped>" src -g "*.vue"
    rg -n "const [A-Za-z0-9_]+.*t\(" src/views src/components -g "*.ts" -g "*.vue"
    git diff --check

Expected: AnyScript scan无输出；普通 scoped style 扫描只列出未修改的旧文件；module-level t(...) 扫描无被冻结的翻译数组或规则；git diff --check clean。

- [ ] **Step 3: Verify persistence and fullscreen contracts**

确认测试覆盖：admin:locale 可在 reload simulation 后恢复并同时驱动登录/后台；v1 UI preferences 转为 v2 且不含 theme；主题模式冷启动为 light、其他偏好恢复；英文 Element Plus 分页/表单文案无需 remount 即更新；RouteTabs 在 content fullscreen 中保持挂载并能退出；Main 是唯一业务滚动所有者且 display.css 只引入一次。

- [ ] **Step 4: Review staged diff against the spec**

    cd D:\admin
    git status --short
    git diff --stat
    git diff --cached --check

确认只修改计划中的前端文件，没有后端/数据库/菜单数据变更，没有密钥、环境文件或构建产物进入暂存区，每个页面仍遵守 API、权限和错误边界。

- [ ] **Step 5: Commit verification-only fixes when required**

如果验证发现缺陷，先补一条针对性回归测试，再做最小修复并重复失败命令；提交标题使用具体中文摘要，例如：

    git commit -m "fix: 修复语言切换后的表格列标题刷新"

不得把无关清理混入验证修复。

## Execution Order

1. Task 1: runtime Element Plus locale。
2. Task 2: UI preference v2 and non-persistent theme mode。
3. Task 3: shared language control, login validation and layout locale source。
4. Task 4: dynamic business translation configurations。
5. Task 5A: AppTable/Search component layout。
6. Task 5B: login, settings and business page layout/SCSS migration。
7. Task 6: full verification and diff review。

每个任务结束后再开始下一个任务；后续任务不得在前一任务的聚焦测试或类型检查失败时启动。实现会话必须同时阅读本计划和链接 spec，保留其他开发者的现有改动，并如实报告无法运行的命令。
