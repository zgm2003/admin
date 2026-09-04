# 当前产品设计基线

本文只记录当前产品和交互中稳定、会影响实现选择的约定。具体字段、页面行为和接口契约以目标模块代码、
测试与 PostgreSQL 事实为准。

## 产品范围

Admin 是面向管理员的平台控制台。当前可见业务域：

| 域 | 当前入口 |
| --- | --- |
| 认证与账户 | 登录、个人资料、用户、会话、登录日志 |
| 权限管理 | 认证平台、菜单、角色、角色授权、Access 快照 |
| 系统运维 | 操作日志、健康/就绪检查 |
| 对象存储 | COS 配置、上传规则、凭证 |
| 消息 | 邮件配置、模板、发送测试、发送日志、收件规则 |
| 应用壳 | Dashboard、动态菜单、路由标签、语言切换、主题 |

新业务不因历史文档里出现过名称就视为已实现；先看 `server/internal/module`、`web/src/views` 和
`STATUS.md`。

## 体验基线

- 前端是 Vue 3 + TypeScript + Pinia + Vue Router + Element Plus；页面优先保持成熟旧项目的信息结构、工作密度
  和操作习惯，再接入当前 DTO、主题和公共组件。
- 页面负责编排状态、收集输入和展示 loading/empty/error/success；HTTP 调用只放在
  `web/src/api/<module>.ts`，经过 `web/src/utils/request.ts`。
- 优先使用已有 `AppDialog`、`AppTable`、`Search`、`DIcon`、`IconSelect`。只有多个真实页面已经复用且交互
  稳定时才新增公共组件。
- 所有可见文案走 i18n；JSON 和 TypeScript 字段使用 lower camel case。表格、树和分页保持可扫描的管理后台
  密度，并在窄屏下不产生遮挡或横向溢出。
- API 失败由请求层统一通知一次。自动刷新期间的中间 401 不通知；登录接口的凭据错误由登录页以内联状态展示，
  请求层不重复通知。页面只负责失败后的状态恢复、重试和业务动作，不重复弹同一错误。

## HTTP 契约

成功和失败都使用精确 envelope：

```json
{
  "code": 0,
  "data": {},
  "message": "ok"
}
```

不接受 `msg` 或静默兼容字段。请求层先校验 envelope，业务 API 再把 `unknown` 缩小成明确 DTO；必填字段缺失、
协议错误和依赖不可用都应显式失败。

## 权限设计

页面、读取和动作是三种不同能力。新页面在设计阶段先列清映射：

| 节点 | 进入页面 | 列表/详情 | 写操作 | 隐藏 |
| --- | --- | --- | --- | --- |
| TARGET | `TARGET:view` | `TARGET:list` / `TARGET:detail` | `TARGET:create` / `TARGET:update` / `TARGET:status` / `TARGET:delete` / `TARGET:authorize` | `0` 或 `1` |

规则：

- 页面 `:view` 只控制能否进入；不得自动推导读取或写入权限。
- `is_hidden=1` 只是不出现在侧边菜单，仍需动态路由、页面权限和后端 Middleware。
- Access 的 `menuTree` 只放 directory/page，action 只放入 `permissionCodes`。
- 个人资料是隐藏页面：`account:profile:view`、`account:profile:detail`、
  `account:profile:update`、`account:password:update` 分别对应进入、读取、保存和改密。
- 前端按钮隐藏不是安全边界；后端 API 必须使用同一个 action code 再校验。

## 数据和状态

- PostgreSQL 保存用户、权限、菜单、配置、日志等业务事实；Redis 只做会话、Access 版本/快照和队列存储等
  明确用途，不成为第二个权限来源。
- Yes/No 统一为 `0 = No`、`1 = Yes`。有真实删除行为时使用 `deleted_at`，不并列维护另一套删除标记。
- 页面状态必须区分初始加载、成功空态、失败、提交中和提交失败；不以空数组、默认对象或旧快照掩盖依赖错误。

## 旧项目迁移

迁移已有功能时只读取对应旧项目的直接页面/接口和调用方：成熟行为、字段语义、筛选分页、危险操作确认和
信息结构优先继承；重型分层、兼容协议、重复状态和无真实用途抽象按当前架构替换。未得到用户或当前任务
确认，不删减已存在的成熟功能。
