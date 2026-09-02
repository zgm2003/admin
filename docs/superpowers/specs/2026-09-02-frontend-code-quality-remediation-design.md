# 前端代码质量与组件结构整改设计

## 1. 状态

状态：设计已由用户确认，等待用户审阅本文档。

本文档定义 Admin 前端的全局代码质量整改基线。整改只重构前端工程结构、组件边界、布局写法、
API 契约校验和质量门禁，不改变现有业务功能、接口协议、权限语义、路由 URL 或页面操作习惯。

## 2. 背景与问题证据

当前前端已有 Vue 3、严格 TypeScript、Pinia、Vue Router、Element Plus、Vue I18n、Vitest、
`AppTable`、`AppSearch` 和 `AppDialog` 等基础能力，且基本遵守：

```text
view -> api/<module>.ts -> utils/request.ts -> Go API
```

现有基础应保留，但代码质量已经出现以下系统性问题：

1. 页面职责堆积：菜单、认证平台、角色和用户页面分别达到约 1277、882、877 和 713 行，
   单个 SFC 同时承担查询、权限、筛选、表单、状态机、确认、通知、模板和样式。
2. 目录形态不一致：页面使用 `目录/index.vue`，部分页面私有组件仍是单独的 `Foo.vue`，公共
   组件又使用 `Component/src/index.vue`，形成三套规则。
3. Element Plus 布局使用失控：简单表单也反复嵌套 `el-row/el-col`，或者用 `div + CSS` 实现
   `el-form`、`el-form-item`、`el-space` 和组件宽度已经能完成的布局。
4. 样式缺少必要性约束：页面为了少量宽度、间距和排列新增 class 与 scoped style；普通 CSS、
   SCSS、单双引号、分号和缩进风格并存。
5. API 契约不一致：部分模块从 `unknown` 严格解析 DTO，部分模块直接使用 `request<T>` 信任
   服务端，还有模块用 `?? []` 把必填集合的 `null` 静默修复为空数组。
6. 公共能力使用不一致：已有 `AppTable` 和 `AppSearch`，部分列表页仍重复维护原生表格、搜索栏、
   分页和刷新布局。
7. 工程门禁不足：`package.json` 只有 `dev`、`build`、`preview` 和 `test`，缺少统一 lint、format、
   typecheck、架构扫描和前端完整验证命令。
8. 测试开始锁定实现细节：个别测试读取 `.vue` 源码并断言字符串，而不是验证可观察行为或通过
   专用架构规则检查代码形态。
9. 动态路由仍有静态页面特例，并使用 `../views/**/index.vue` 广泛扫描。组件全面采用
   `目录/index.vue` 后，该 glob 会把页面私有组件也收进路由模块集合。

## 3. 文档优先级与被替代条款

本设计服从根 `AGENTS.md` 和 `docs/agent/README.md`。以下旧文档只在不冲突的部分继续有效：

- `2026-08-21-frontend-component-foundation-design.md` 与
  `2026-08-24-frontend-table-search-icon-baseline-design.md` 中的公共组件职责继续保留，但
  `Component/src/index.vue` 文件边界由本设计的 `Component/index.vue` 替代。
- `2026-08-28-frontend-foundation-remediation-design.md` 中“新增或实质重构的 Vue 文件统一使用
  SCSS”条款失效，改为本设计的“无必要样式则无 style block，普通 CSS 优先”。
- `2026-08-26-frontend-api-dto-consumption-design.md` 中“业务 API 直接信任 `request<T>`、删除
  业务运行时解析”条款失效。按照当前项目硬规则，Envelope 由请求层解析，业务 DTO 由对应 API
  模块从 `unknown` 严格解析。
- 旧文档定义的业务功能、权限码、页面信息结构、操作流程和错误语义不因本次整改改变。

出现其他冲突时，以用户当前指令、根 `AGENTS.md`、`docs/agent/README.md` 和本文档的顺序为准，
不保留双轨兼容实现。

## 4. 整改原则

### 4.1 渐进整改，不推倒重写

- 先建立格式、类型、目录和架构门禁，再逐页面迁移。
- 每次迁移一个完整页面或一个明确公共组件，保持可构建、可测试和可回滚。
- 先写或保留行为特征测试，再做结构重构；结构整改不得顺带改变业务规则。
- 不把所有页面一次性替换成新框架，不引入第二套 UI 库、状态管理或路由系统。
- 不创建 BasePage、BaseForm、BaseCRUD、Manager、Factory、通用 Schema Renderer 或万能
  `useCRUD`。

### 4.2 结构服务于职责

拆文件的目的不是追求更少行数，而是让每个单元只有一个清楚职责：

- 页面入口负责组装页面能力和协调顶层数据流；
- 页面私有组件负责一个可命名、可单测的业务区域；
- composable 负责一组真实内聚的响应式状态和操作；
- 纯函数文件负责树转换、Payload 映射、表单校验等无 UI 逻辑；
- API 文件负责 DTO、请求和响应解析；
- 公共组件不访问业务 API、权限 Store 或业务错误码。

禁止为满足行数指标把同一状态机随意切成多个互相穿透内部状态的文件。

## 5. Vue 组件目录规范

### 5.1 唯一允许的组件形态

除根入口 `web/src/App.vue` 外，所有 Vue 页面和组件必须拥有独立目录，组件实现文件固定命名为
`index.vue`。

页面：

```text
web/src/views/<domain>/<page>/index.vue
```

页面私有组件：

```text
web/src/views/<domain>/<page>/components/<ComponentName>/index.vue
```

共享组件：

```text
web/src/components/<ComponentName>/index.vue
```

布局组件：

```text
web/src/layout/components/<ComponentName>/index.vue
```

示例：

```text
web/src/views/message/mail/
|-- index.vue
`-- components/
    |-- ConfigPanel/
    |   `-- index.vue
    |-- LogTable/
    |   `-- index.vue
    `-- RuleTable/
        `-- index.vue
```

### 5.2 目录附属文件

组件目录可按真实需要放置：

- `types.ts`：只保存该组件公开或内部使用的明确类型；
- `logic.ts` 或具有业务含义的文件名：保存可独立测试的纯逻辑；
- `useXxx.ts`：保存真实拥有响应式状态的 composable；
- `index.ts`：仅在需要稳定导出组件和类型时作为单层 barrel。

禁止：

- `components/Foo.vue` 单文件组件；
- `components/Foo/src/index.vue` 的无意义中间 `src` 目录；
- `Foo/Foo.vue` 重复命名；
- 多层 `index.ts -> src/index.ts -> index.vue` barrel；
- 只包含一行转发且没有公共导出价值的 `index.ts`；
- 强迫普通 `.ts` 工具函数也使用目录/index 结构。

### 5.3 命名和 import

- 页面路由目录使用 lower kebab case，Vue 组件目录使用 PascalCase。
- 项目内业务 import 统一使用 `@/`；不再混用 `@src`、三级以上相对路径和显式 `.ts` 后缀。
- 同目录内部可使用 `./`；跨模块必须使用 `@/`，避免移动目录时产生大面积路径修改。
- 组件名、目录名和 `defineOptions({ name })` 在需要显式 name 时保持一致。

### 5.4 现有组件迁移

至少覆盖：

- `src/components/*/src/index.vue` -> `src/components/*/index.vue`；
- `layout/components/AppAside.vue`、`AppHeader.vue`、`AppFooter.vue`、`RouteTabs.vue`、
  `SettingDrawer.vue`、`PermissionMenuNode.vue`；
- `views/dashboard/components/ReadinessChart.vue`；
- `views/permission/roles/components/RolePermissionMatrix.vue` 与
  `RolePermissionDiffDialog.vue`；
- 后续发现的所有非 `App.vue` 独立命名 `.vue` 文件。

迁移只改变路径和边界，不改变 props、emits、slot、权限或业务行为。

## 6. Element Plus 布局规范

### 6.1 选择顺序

布局实现按以下顺序选择：

1. 先使用目标 Element Plus 组件自身的公开属性；
2. 表单使用 `el-form` 和 `el-form-item`；
3. 同行项目、按钮组和自然间距使用 `el-space`；
4. 确实需要 24 栅格和响应式列宽时使用 `el-row/el-col`；
5. 只有上述能力无法表达时才增加语义 HTML、普通容器或局部 CSS。

不得从“先写 div/class/style”开始，再考虑是否能由 Element Plus 代替。

### 6.2 el-form 是表单布局所有者

简单表单优先使用：

- `label-position`；
- `label-width`；
- `inline`；
- `size`；
- `status-icon`；
- `show-message` 和 `inline-message`；
- `el-form-item` 的 label、prop、error、required 等公开能力。

输入框、选择框、日期控件等宽度优先通过组件公开的 `width`、`style` 或宽度绑定直接设置；不为
单个控件的固定宽度新建 class 和 scoped style。Dialog 内普通单列表单不使用 `el-row/el-col`。

以下结构是默认形式：

```vue
<el-form :model="form" label-width="112px">
  <el-form-item :label="t('example.name')" prop="name">
    <el-input v-model="form.name" style="width: 320px" />
  </el-form-item>
</el-form>
```

### 6.3 el-row/el-col 的正确边界

`el-row/el-col` 用于替代承担网格布局职责的普通 `div`，不是每个表单字段的默认包裹层。

允许使用的场景：

- 两个或更多字段必须按 24 栅格同行分栏；
- 桌面和移动端需要通过 `xs/sm/md/lg/xl` 明确改变列宽；
- 卡片、统计块或详情项确实构成规则网格；
- 使用 `gutter` 能直接替代重复 gap/margin CSS。

禁止使用的场景：

- 一个 `el-row` 只包一个 `span=24` 的 `el-col`；
- 每个 `el-form-item` 外都套一层 `el-row/el-col`；
- 为了垂直排列字段使用 row/col；
- `el-col` 内继续嵌套 `el-row/el-col`，但不存在独立子网格；
- row/col 只为增加 padding、margin 或固定宽度；
- row/col 与同职责的 flex/grid CSS 同时存在。

默认只允许一层 row/col。确有独立子网格时可以嵌套，但必须在组件边界或代码审查中说明其布局
语义，不能把嵌套当作通用响应式方案。

### 6.4 其他容器边界

- 应用壳使用 `el-container/el-aside/el-header/el-main/el-footer`；业务页不机械套 `el-container`。
- 操作区、按钮组和紧凑字段优先 `el-space`，不为 `display:flex + gap` 重复写 CSS。
- `section`、`header`、`nav`、`main` 等真实语义元素可以保留。
- `div` 允许用于滚动所有者、第三方挂载点、slot 包裹或 Element Plus 无对应能力的专用结构；
  不以“消灭 div 数量”为验收目标。

## 7. CSS 与 SCSS 规范

### 7.1 无必要样式即不写 style block

页面能通过 Element Plus 组件、公开属性、`AppTable`、`AppSearch`、`AppDialog` 和全局 token 完成
时，不得增加 `<style>`。以下需求通常不构成新增 style block 的理由：

- 单个输入控件宽度；
- 表单 label 宽度；
- 普通按钮间距；
- 表单纵向或 inline 排列；
- 两列 24 栅格布局；
- Element Plus 已提供的 loading、empty、alert、divider 和响应式显隐。

### 7.2 普通 CSS 优先

- 需要少量局部样式时使用 `<style scoped>`；
- 只有确实使用 Sass 变量、mixin、函数或能显著提升可读性的浅层嵌套时，才使用
  `<style scoped lang="scss">`；
- 不得只因为项目已安装 Sass 就给所有 Vue 文件添加 `lang="scss"`；
- 不得把普通 CSS 机械嵌套成 SCSS；嵌套深度默认不超过两层；
- 颜色、字体、边框、圆角和状态优先使用 Element Plus CSS 变量与现有项目 token；
- 页面样式不得写入全局 `styles/index.scss`，全局文件只保存应用壳、token、reset 和多个页面
  真实共享的稳定规则；
- `:deep()` 和 `!important` 只用于第三方组件无公开能力可解决的问题，并需在测试或注释中说明
  原因。

### 7.3 样式删除标准

迁移页面时逐条核对现有规则：

1. 能被 Element Plus prop 替代的删除；
2. 能被 `el-space` 或 `gutter` 替代的删除；
3. 只服务单个固定宽度的 class 改为组件局部 width/style；
4. 无 DOM 命中、重复或被覆盖的规则删除；
5. 仍承担业务布局、响应式、滚动或专用视觉状态的规则保留并简化。

不得为了“CSS 行数下降”删除可访问性焦点、移动端适配、滚动边界和必要状态反馈。

## 8. AppTable、AppSearch 与 AppDialog

### 8.1 AppTable 默认规则

所有普通数据列表默认使用 `AppTable`，包括分页、loading、empty、error、selection、刷新和列配置。
页面只负责查询条件、权限、业务操作和 API 调用。

允许暂不使用 `AppTable` 的情况：

- 菜单树表等依赖 AppTable 尚未暴露的 Element Plus 树形能力；
- 权限矩阵、虚拟列表或其他并非普通数据表格的专用控件；
- 接入 AppTable 会迫使公共组件理解页面业务语义。

遇到例外时按以下顺序处理：

1. 若能力对两个以上真实页面稳定通用，扩展 AppTable；
2. 若只属于一个页面，保留页面私有组件；
3. 在代码中或架构 allowlist 中记录例外原因，不允许无说明直接使用原生 `el-table`。

### 8.2 AppSearch 默认规则

所有列表筛选区默认使用 `AppSearch`：

- 字段配置使用严格联合类型；
- 查询和重置只通过 emit 通知页面；
- 业务 API、权限和结果状态仍由页面拥有；
- 新的通用筛选控件类型在至少两个真实页面需要时扩展；
- 单页专用复杂筛选通过明确 slot 或页面私有筛选组件实现，不把业务条件写进 AppSearch。

Dialog 业务表单、个人资料表单和单独输入项不使用 AppSearch。

### 8.3 AppDialog 默认规则

普通新增、编辑和详情弹窗默认使用 `AppDialog`。直接使用 `el-dialog` 必须依赖 AppDialog 无法提供
且不适合公共化的特殊行为。AppDialog 只统一尺寸、滚动、移动端、焦点恢复和结构 slot，不拥有
提交 API、表单状态、权限或业务错误。

## 9. 页面与组件职责拆分

### 9.1 页面入口职责

页面 `index.vue` 可以拥有：

- 当前页面权限计算；
- 顶层查询状态和调用顺序；
- 页面私有组件间的数据协调；
- 路由参数和页面生命周期；
- 顶层错误、空态和刷新语义。

页面入口不应继续内联：

- 数百行表单模板；
- 可独立命名的新增/编辑 Dialog；
- 树遍历、排序、Payload 映射等纯算法；
- 多个相互独立的 mutation 状态机；
- 大段只服务一个局部区域的样式。

### 9.2 私有组件拆分标准

满足任一条件即可拆为页面私有组件：

- 区域拥有独立 props/emits 边界；
- 区域拥有独立表单生命周期；
- 区域可单独描述为“配置表单”“权限矩阵”“日志详情”“规则列表”等；
- 区域模板和状态已经妨碍理解页面顶层数据流；
- 区域需要专门测试交互或错误状态。

私有组件默认不提升到 `src/components`。只有两个以上真实页面已经复用同一稳定交互时，才迁移
为共享组件。

### 9.3 文件规模质量信号

文件行数不是唯一验收指标，但作为强制审查信号：

- 页面 SFC 目标不超过 400 行，超过 500 行必须拆分或在架构 allowlist 记录具体理由；
- 页面私有/共享组件目标不超过 300 行，超过 400 行必须说明为何仍是单一职责；
- 单个纯逻辑文件目标不超过 250 行；
- 测试文件不以行数强拆，但应使用 fixture/builder 去掉重复样板。

禁止通过压成单行、删除可读空行或创建无职责薄文件规避阈值。

## 10. API DTO 与请求边界

### 10.1 固定数据流

请求方向固定为：

```text
View -> api/<module>.ts -> request<unknown>() -> Go API
```

响应方向固定为：

```text
Go API -> request.ts 校验并解包 Envelope -> api/<module>.ts DTO parser -> View
```

- `utils/request.ts` 只校验严格的 `code/data/message` Envelope、处理 HTTP 错误、认证刷新和重试；
- 业务 API 从请求层接收 `unknown`，在模块边界完成 DTO 解析；
- View、Store 和 Router 只能接收已经解析的明确 DTO；
- 页面不得通过断言、可选链或默认值修复服务端必填字段。

### 10.2 DTO parser 规则

每个业务 API 必须：

- 检查对象、数组、基础类型和必填字段；
- 对封闭 DTO 检查未知字段；
- 对有限状态使用联合类型或 `isYesNo` 等明确守卫；
- 对分页结果检查 `list/total/page/pageSize`；
- 对写操作结果和空对象同样定义明确返回类型；
- 解析成功后返回新建的明确对象或经过完整守卫证明的类型，不使用宽泛断言掩盖未校验字段。

禁止：

- 直接 `request<BusinessDTO>()`；
- `request<unknown>()` 后 `as BusinessDTO`；
- 必填数组使用 `value ?? []`；
- 接受旧字段别名、`msg` 或额外 Envelope 字段；
- 在页面重复解析 DTO；
- 同一模块部分接口严格解析、部分接口直接信任泛型。

### 10.3 解析基础函数

当前多个 API 已重复实现 record、exact keys、integer 和 text 判断。允许建立一个窄小的协议解析
工具文件，只提供无业务语义的基础守卫，例如：

- `expectRecord`；
- `expectExactKeys`；
- `expectString`；
- `expectInteger`；
- `expectArray`。

业务字段、状态关系和错误文案仍放在对应 API 模块。禁止创建运行时 DTO 注册器、通用 Schema DSL、
Adapter 或为所有接口生成动态对象的万能 parser。

### 10.4 错误展示所有权

`request.ts` 负责把协议和 HTTP 失败转换为明确错误并继续抛出，不直接调用 Element Plus notification
或决定业务页面文案。认证失效后的清理和跳转可以通过明确注入的认证回调协调，但传输层不得因此
成为全局 UI 控制器。

页面错误展示遵守：

- 列表加载失败进入 AppTable 或页面 inline error 状态；
- 表单和 mutation 失败进入对应表单/页面错误状态；
- 确需 toast 的动作由发起动作的页面统一展示；
- 同一个失败只能展示一次，不同时出现请求层 notification 和页面 inline error；
- 成功通知、确认弹窗和取消语义继续由页面拥有；
- catch 不得以“请求层已经提示”为由吞掉错误。

迁移请求层通知副作用前必须盘点所有调用方，保证每条失败路径仍有可见、可测试的错误反馈，不得
把“去重提示”实现成静默失败。

## 11. 路由与页面注册

组件统一采用 `index.vue` 后，动态路由不得继续使用会捕获私有组件的
`import.meta.glob('../views/**/index.vue')`。

页面注册应只覆盖规范页面深度：

```text
views/<page>/index.vue
views/<domain>/<page>/index.vue
```

页面私有组件路径必须排除在路由模块集合之外。可采用两个受限 glob 或构建时生成并校验的显式页面
注册表，但禁止运行时任意扫描并猜测页面。

同时处理当前菜单管理静态路由特例：

- 登录页、Dashboard 和应用壳继续是明确静态路由；
- 菜单、用户、角色、认证平台、存储、邮件等业务页统一由 Access 快照动态注册；
- 删除菜单页专用 `staticPageBinding` 兼容分支；
- `componentPath` 到源码页面的映射必须显式、唯一、可由架构检查验证；
- 不从 `:view` 自动派生任何 API action 权限。

路由整改不得改变现有 path、route tab、breadcrumb、权限码或未授权跳转行为。

## 12. 工程质量门禁

### 12.1 标准工具

前端增加并固定：

- ESLint：TypeScript、Vue SFC、Promise、未使用代码和项目禁止项；
- Prettier：统一 TypeScript、Vue、JSON、SCSS/CSS 的机械格式；
- Vue TypeScript：独立 `vue-tsc -b`；
- 架构检查脚本：检查目录、组件文件名、路由页面集合和渐进整改 allowlist。

Lint 负责代码正确性和架构禁止项，Prettier 只负责格式，不在两者之间配置互相冲突的风格规则。

### 12.2 package scripts

最终至少提供：

```json
{
  "scripts": {
    "typecheck": "vue-tsc -b",
    "lint": "eslint . --max-warnings 0",
    "format": "prettier --write .",
    "format:check": "prettier --check .",
    "check:architecture": "node scripts/check-frontend-architecture.mjs",
    "test": "vitest run",
    "verify:frontend": "pnpm format:check && pnpm lint && pnpm check:architecture && pnpm typecheck && pnpm vitest run --pool=threads --maxWorkers=1 && pnpm build"
  }
}
```

实际脚本可按 pnpm 参数规范微调，但 `verify:frontend` 必须成为本地和 CI 的单一完整入口。

### 12.3 架构检查内容

架构检查使用结构化 Vue SFC/模板解析能力，不用脆弱的整文件正则替代解析器。至少检查：

1. 除 `App.vue` 外不存在非 `index.vue` 的 Vue 组件；
2. 不存在 `Component/src/index.vue`；
3. 页面和组件目录命名符合规则；
4. 路由页面 glob 不包含 `components/**/index.vue`；
5. 新增页面没有未经批准的原生 `el-table`、`el-dialog` 或列表搜索表单；
6. 不存在单 row/单 span=24 col、无意义 row/col 嵌套等明确反模式；
7. 不存在业务 `any`、`as any`、`any[]`、`Record<string, any>`、`@ts-ignore`；
8. 不存在未批准的超大 SFC；
9. 渐进迁移 allowlist 只能减少，新增例外必须经用户确认并写明原因。
10. `utils/request.ts` 不导入 Element Plus 或直接产生 notification/message 等 UI 副作用。

“CSS 是否必要”需要结合页面语义判断，不用简单脚本禁止所有 style block；由迁移清单和代码审查
逐页验收。

## 13. 测试质量规范

- 结构重构前保留并补足关键行为特征测试；
- 页面测试验证用户可见行为、API 调用、权限、loading、empty、error、分页和确认流程；
- 纯树算法、Payload 映射和表单规则迁入 `.ts` 后使用纯单元测试；
- 组件测试验证 props、emits、slot 和交互，不读取内部 ref 或依赖文件行号；
- 禁止在 Vitest 中通过 `readFileSync` + `source.toContain(...)` 锁定源码写法；这类结构约束迁移到
  ESLint 或架构检查脚本；
- fixture 和 builder 必须返回明确类型，不用 `any`；
- 不以快照测试替代关键业务断言；
- 重构不降低现有业务覆盖，新增公共基础能力必须有定向测试。

## 14. 分阶段迁移

### 阶段一：建立不可回退门禁

1. 增加 ESLint、Prettier、typecheck、架构检查和 `verify:frontend`；
2. 记录当前遗留 allowlist，只允许后续减少；
3. 统一 `@/` alias 和格式，不在同一步改变业务；
4. 把源码字符串测试迁移为行为测试或架构检查。

### 阶段二：统一组件目录

1. 迁移公共组件，删除中间 `src` 目录；
2. 迁移 Layout 和页面私有命名 `.vue` 文件；
3. 收紧动态路由页面 glob；
4. 删除菜单页静态绑定特例；
5. 验证 lazy loading、RouteTabs、breadcrumb 和 Access 动态注册。

### 阶段三：统一 API 契约

1. 建立窄小的协议解析基础守卫；
2. 按模块把响应入口统一为 `unknown -> parser -> DTO`；
3. 优先整改当前明显不一致的 COS、上传规则、会话、操作日志和简单直返 API；
4. 删除 `?? []` 等必填字段静默修复；
5. 移除请求层 UI 通知副作用，保证每个错误显式到达且只由一个页面状态或提示展示。

### 阶段四：逐页面整改

推荐顺序：

1. 邮件页：以现有 page + 私有 components 结构作为首个标准样板，收紧表单和样式；
2. 对象存储：修复压缩式代码、复杂表单和 row/col 过度嵌套；
3. 认证平台：建立复杂 Dialog 表单样板；
4. 用户、会话、操作日志：统一 AppTable/AppSearch 和列表状态；
5. 角色：拆分列表、编辑、授权矩阵和 diff Dialog；
6. 菜单：最后处理树算法、平台切换、展开状态、表单和树表，是最高风险页面；
7. Layout、Dashboard 和设置组件按相同目录与样式规则收尾。

每个页面单独完成：特征测试 -> 目录/职责拆分 -> Element Plus 布局简化 -> AppTable/AppSearch 接入
-> CSS 清理 -> 定向测试 -> 完整门禁。不得积累多个半迁移页面后一次性修复。

## 15. 非目标

- 不修改 Go API、PostgreSQL、Redis、认证、权限码或菜单数据；
- 不改变页面字段、操作入口、分页规则、确认语义和成熟交互；
- 不重做视觉品牌，不以代码整改名义重新设计所有页面；
- 不删除移动端、暗色主题、国际化、可访问性和错误状态；
- 不引入 Tailwind、第二套组件库、CSS-in-JS 或运行时表单 Schema；
- 不照搬旧前端的重型 modules/adapters/managers 架构；
- 不把 AppTable/AppSearch/AppDialog 扩展为理解业务的万能组件；
- 不在 Spec 阶段修改运行时代码或自动创建 Git 提交。

## 16. 验证要求

每个迁移切片执行定向测试，跨公共组件、请求层、路由或工程配置的阶段执行完整验证：

```powershell
cd D:\admin\web
pnpm format:check
pnpm lint
pnpm check:architecture
pnpm typecheck
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

同时执行定向残留扫描：

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src tests -g "*.ts" -g "*.vue"
rg -n "request<[^>]*DTO|\?\? \[\]" src/api -g "*.ts"
rg -n "readFileSync|source\.toContain" tests -g "*.test.ts"
git diff --check
```

扫描规则最终由 lint/架构脚本结构化承担，命令行搜索只作为迁移期辅助证据。

## 17. 完成标准

全部满足后，前端代码质量整改才算完成：

1. 除 `App.vue` 外，所有 Vue 页面和组件均为“目录/index.vue”；
2. 不再存在 `Component/src/index.vue` 和独立命名 `Foo.vue`；
3. 页面优先使用 Element Plus 原生布局能力，简单表单不使用 row/col；
4. `el-row/el-col` 只承担真实 24 栅格布局，并替代相同职责的普通 div；
5. 能由 form 属性、组件宽度、el-space 或 gutter 完成的布局不新增 CSS；
6. Vue 文件无必要 style block；需要样式时普通 scoped CSS 优先，SCSS 有明确使用理由；
7. 普通列表默认使用 AppTable，列表筛选默认使用 AppSearch，例外均有明确依据；
8. 页面入口恢复为可理解的编排层，现有超大页面完成职责拆分并退出 allowlist；
9. 所有业务 API 执行 `unknown -> 严格 parser -> DTO`，不存在必填字段静默默认；
10. 动态路由只扫描页面，不扫描页面私有组件，菜单页不再拥有静态绑定特例；
11. 不存在业务 any、源码字符串形态测试和压缩式单行源码；
12. `format:check`、lint、架构检查、typecheck、Vitest 和生产构建全部通过；
13. 现有功能、权限、错误、国际化、移动端和主题行为保持不变；
14. 本次整改没有自动提交、推送或重写 Git 历史。

## 18. 后续流程

用户审阅并批准本文档后，再使用独立 implementation plan 拆成可顺序执行、逐步验证的迁移切片。
实施计划必须显式记录每一阶段的文件移动、行为保护测试、allowlist 收缩和验证命令，不能把全前端
整改压成一个不可审查的大提交。
