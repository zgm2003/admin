# 当前产品设计基线

本文只记录当前产品和交互中稳定、会影响实现选择的约定。具体字段、页面行为和接口契约以目标模块代码、
测试与 PostgreSQL 事实为准。

## 产品范围

Admin 是面向管理员的平台控制台。当前可见业务域：

| 域 | 当前入口 |
| --- | --- |
| 认证与账户 | 登录、个人中心、用户、会话、登录日志 |
| 权限管理 | 认证平台、菜单、角色、角色授权、Access 快照 |
| 系统运维 | 操作日志、健康/就绪检查 |
| 对象存储 | COS 配置、上传规则、凭证 |
| 消息 | 邮件与短信配置、站内通知、通知任务、模板、发送测试、发送日志、收件规则、限流策略 |
| 应用壳 | Dashboard、动态菜单、路由标签、语言切换、主题 |

新业务不因历史文档里出现过名称就视为已实现；先看 `server/internal/module`、`web/src/views` 和
`STATUS.md`。

代码中的复合业务模块名使用 lower camel case；页面模块、API 模块、后端模块、菜单 componentPath、权限码
和 i18n key 使用同一模块段。数据库表继续使用 snake_case，HTTP API 使用小写资源路径，Vue 组件使用
PascalCase，CSS class 与测试定位符使用 kebab-case。

## 体验基线

- 前端是 Vue 3 + TypeScript + Pinia + Vue Router + Element Plus；页面优先保持成熟旧项目的信息结构、工作密度
  和操作习惯，再接入当前 DTO、主题和公共组件。
- 页面负责编排状态、收集输入和展示 loading/empty/error/success；HTTP 调用只放在
  `web/src/api/<module>.ts`，经过 `web/src/utils/request.ts`。
- 优先使用已有 `AppDialog`、`AppTable`、`Search`、`DIcon`、`IconSelect`。只有多个真实页面已经复用且交互
  稳定时才新增公共组件。
- 所有下拉选择使用 `el-select-v2` 的 `options` 数据契约，不使用 `el-select`/`el-option` 子节点；选项文案走
  i18n，值保持业务 DTO 的精确类型，多选、可创建和禁用状态不得在迁移中降级。
- 所有可见文案走 i18n；JSON 和 TypeScript 字段使用 lower camel case。表格、树和分页保持可扫描的管理后台
  密度，并在窄屏下不产生遮挡或横向溢出。
- 系统设置按“站点信息 / 协议与隐私 / 高级设置”分区。用户协议与隐私政策各维护一份富文本正文，不随界面
  语言拆分；保存即发布。登录页只提供查看入口，不以勾选协议作为登录前置条件；未配置或加载失败时明确展示状态。
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
- 个人中心是隐藏页面：`user:profile:view`、`user:profile:detail`、
  `user:profile:update`、`user:password:update` 分别对应进入、读取、保存和改密。
- Page 和 action 完全独立；获得上传等 action 不会自动获得所属管理页面或目录。个人中心是隐藏根页面，
  不因个人能力暴露“用户与账号”目录。
- 前端按钮隐藏不是安全边界；后端 API 必须使用同一个 action code 再校验。

## 数据和状态

- 腾讯云 SES 配置、邮件模板和收件规则全系统共享，不随 Admin/Canvas 等认证平台复制；只有发送日志、验证码
  记录和额度 key 保留来源平台。COS 逻辑配置同样全局共享且允许维护多条，上传规则按认证平台隔离，并通过
  `cosConfigId` 选择配置。
- COS 配置的物理 version 只用于定位对象写入时所在的 Bucket/Region/Endpoint，不承担管理员编辑冲突控制；
  配置 generation 只用于 Redis state/snapshot/lease/outbox 一致性。普通管理 CRUD 采用 last-write-wins，DTO
  不暴露 generation、revision 或 expected version。
- 邮件只保留每分钟、每 10 分钟两条发送上限；同平台、同邮箱跨所有场景和管理测试共享，任一达到上限
  即拒绝，不再叠加场景/IP/管理员专属策略。配置的次数、窗口与验证码 TTL 都由邮件管理拥有。
- 找回密码成功不自动登录，并撤销全部既有会话；首次设置密码只允许无密码账号，保持会话且不强制拦路。
- Admin 允许邮箱验证码自注册。未知邮箱验证码验证成功后原子创建无密码账号并绑定唯一默认
  `registered_user`；不开放绕过邮箱验证的密码直注册入口。默认角色只有个人中心/设密与上传 action，
  额外管理权限由管理员显式分配其他角色。
- Email 和 Phone 是同级用户身份：用户可以只有其中一个或同时拥有两个。`user/account` 与普通
  `user/profile` 只读展示身份，绑定/换绑分别由 `user/email` 与 `user/phone` 负责；首次绑定只需新身份 proof，
  换绑必须当前身份与新身份两份 proof，并以一次 `ConsumeMany` 原子消费。身份更新和 append-only 变更审计在同一
  PostgreSQL 事务中完成，唯一冲突、陈旧 current 与并发失败返回冲突语义，并使 authority generation 失效。
- Mail 与 SMS 是发送渠道，不是身份模块。Mail 使用 `login`/`forget`/`bind_email`/`change_password`，SMS 只使用
  `login`/`forget`/`bind_phone`/`change_password` 四个固定场景；两种渠道的模板、Provider、Redis namespace 和
  加密/HMAC key 独立。验证码改密按 `loginType=email|phone` 选择渠道，保留当前 Session 并撤销其他 Session；找回密码
  撤销全部 Session。

- PostgreSQL 保存用户、权限、菜单、配置、日志等业务事实；Redis 只做会话、Access 版本/快照和队列存储等
  明确用途，不成为第二个权限来源。
- 配置缓存固定覆盖 `system.setting/global`、`system.dictionary/global`、`message.mail/global`、
  `message.sms/global` 和 `storage.cosconfig/<configId>`。权限的 menu/policy/access version、Session/authority
  generation、验证码、限流计数和队列数据保持各自业务语义，不迁入配置 generation。
- 系统字典全平台共享，负责可由后台维护的展示型选项及中英文标签；`code`/`value` 是稳定业务值，
  `isBuiltin` 只表示禁止删除。登录方式、Session/邮件状态、权限节点类型、Yes/No 等参与后端分支、协议或
  数据库约束的 enum 不迁入字典。消费页不得用硬编码选项掩盖字典加载失败或畸形值。
- Yes/No 统一为 `0 = No`、`1 = Yes`。有真实删除行为时使用 `deleted_at`，不并列维护另一套删除标记。
- 页面状态必须区分初始加载、成功空态、失败、提交中和提交失败；不以空数组、默认对象或旧快照掩盖依赖错误。
- Mail 发送日志与验证码记录是审计事实，采用 append-only 语义，不提供管理员单条、批量删除或恢复操作。
  认证平台软删只停止服务，不抹除历史来源；审计列表必须继续显示原平台 code。

## 实时通知

- 一个通知任务固定一个认证平台，受众为指定用户、提交时点冻结的指定角色或平台全体；平台广播不展开百万用户，发布后新建账号不可见，长期离线用户仍可在保留期内从 PostgreSQL 邮箱读取。
- 草稿采用 last-write-wins，只允许草稿编辑和删除；提交后标题、富文本、展示类型、优先级、链接、平台、受众和发送时间全部冻结。取消只停止后续批次，不撤回已经生成的通知；已提交任务通过复制产生新草稿。
- 正文使用模块内简化富文本编辑器，只开放粗体、斜体、下划线、H2/H3、列表、HTTPS 链接、撤销/重做和清除格式；不提供图片、视频、文件、表格、代码、字体、颜色或源码模式。内部链接必须是应用路由，外部链接只接受 HTTPS。
- 顶栏铃铛只在 Access 已就绪且具有 `message:notification:list` 时工作，展示未读数和最近 5 条；`message:notification:view` 独立控制进入隐藏通知中心，read/delete action 各自控制已读和删除。普通通知只刷新邮箱，紧急通知只由多标签 leader 弹一次。
- 通知中心提供 all/unread、展示类型和优先级筛选，使用 ID cursor 加载更多；没有撤回、归档或恢复。通知和 realtime event 保留天数由系统设置管理，缩短保留期必须二次确认，已清理数据不会因调大配置恢复。

## 旧项目迁移

迁移已有功能时只读取对应旧项目的直接页面/接口和调用方：成熟行为、字段语义、筛选分页、危险操作确认和
信息结构优先继承；重型分层、兼容协议、重复状态和无真实用途抽象按当前架构替换。未得到用户或当前任务
确认，不删减已存在的成熟功能。

项目尚未上线期间，经维护者确认的破坏性重构直接切换到新表、新 DTO、新权限和新 Redis 协议；正常运行时不保留
旧字段、旧 key、旧对象格式的双读、双写或回退分支。需要清理的旧 Redis key 只由固定 pattern 的一次性 migration
命令处理。
