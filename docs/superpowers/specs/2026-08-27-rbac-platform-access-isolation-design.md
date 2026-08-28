# RBAC 多平台菜单归属与 Access 隔离设计

状态：已实施并完成验证

## 1. 背景

当前 `rbac_menu` 没有平台归属，`GET /api/v1/access` 的权限源查询会读取全部有效菜单。认证身份虽然已经携带当前平台，但菜单树没有按平台过滤，因此共享后端未来接入多个前端时存在跨平台权限泄露风险。

本切片为 Admin 后端建立多平台 RBAC 基线，并在本地启动时预设 `canvas` 业务平台。Canvas 预设只有一个根级 `test` 页面和一个页面下的 `test` 按钮权限，不创建目录，不写 Canvas 业务前端。

## 2. 目标

1. 每条有效菜单明确归属一个认证平台。
2. 当前已有菜单全部归属 `admin`，既有 Admin 行为和菜单 ID 保持不变。
3. `GET /api/v1/access` 只返回当前认证平台的菜单、角色授权和权限码。
4. 用户可以拥有多个全局角色；多个角色的当前平台菜单合并并去重。
5. 超级管理员也只能获得当前平台菜单，不能通过超级管理员身份跨平台读取菜单。
6. 菜单树允许根级页面，以支持 Canvas 的 `test` 页面不经过目录。
7. 菜单管理、角色授权和相关 DTO 能表达平台维度。
8. 通过 PostgreSQL、Redis 和 Access Service 测试证明平台隔离和缓存一致性。

## 3. 非目标

- 不创建或迁移 Canvas 业务前端项目。
- 不创建 Canvas 业务前端；当前启动预设只建立认证平台及用于验证权限链路的 test 页面和按钮。
- 不改变用户、角色和角色绑定表的全局语义；角色不增加平台字段。
- 不实现 COS、邮件、短信、支付、AI、WebSocket 或业务模块。
- 不重构现有 Access 缓存协议，不引入通用 Adapter、Manager、Factory 或权限注册器。
- 不改变 Admin API 的 `/api/admin/v1` 命名空间和共享 Access API 的 `/api/v1` 命名空间。

## 4. 核心模型

### 4.1 菜单平台归属

`rbac_menu` 增加非空 `platform_id` 字段，类型为 `BIGINT`，外键引用 `auth_platform(id)` 并使用 `ON DELETE RESTRICT`。平台 code 不在菜单表中冗余存储；现有数据迁移时通过内置 `admin` 平台 ID 回填全部历史菜单。

每条菜单的 `platform_id` 必须与其父菜单相同；根菜单不需要父菜单。菜单删除仍使用现有 `deleted_at` 语义。

菜单 code 和页面 path 在 `platform_id` 范围内唯一，允许不同平台各自拥有同名 code 或 path。菜单 ID 仍然全局唯一，角色授权继续使用菜单 ID。

菜单的 `platform_id` 在创建后不可修改。平台迁移不是普通编辑动作；错误平台的菜单需要按现有删除约束删除后在目标平台重新创建，避免在线迁移整棵子树、角色授权和 Access 快照。

### 4.2 菜单树形状

保留现有目录、页面、操作三种类型和父子约束：

- 根节点可以是目录或页面，根操作仍然禁止；
- 目录可以包含目录或页面；
- 页面可以包含操作；
- 操作不能包含子节点；
- 父子节点必须使用同一 `platform_id`。

Canvas 本地预设的结构固定为：

```text
canvas:test       page   parent = null   path = /test
canvas:test:button action parent = canvas:test
```

API 启动时幂等确保内置、启用且默认允许注册的 `canvas` 认证平台，并通过平台维度的 foundation 幂等创建上述两条菜单。已有有效 Canvas 策略值不被启动流程重置；测试仍在独立事务或测试 schema 内验证数据库行为。

### 4.3 角色与用户

角色和用户仍是全局资源。一个用户可绑定多个角色，一个角色可绑定不同平台的菜单。平台隔离发生在 Access 权限源查询阶段，而不是通过复制角色或用户。

## 5. Access 数据流

请求继续遵循：

```text
router -> auth middleware -> access service -> access repository -> PostgreSQL
                                             -> Redis snapshot
```

认证 Service 已经通过 `CurrentPolicy` 取得包含平台 ID 的认证 Policy。运行时 `auth.Identity` 增加 `PlatformID`，由认证成功后的 Policy ID 填充；JWT 继续只携带平台 code，不改变 Token 协议。Access Repository 直接接收 `identity.PlatformID`，不为菜单过滤额外查询认证平台。

1. 查询用户有效角色，保留角色 code；
2. 查询 `rbac_menu` 时限定 `platform_id = currentPlatformID`、启用且未删除；
3. 非超级管理员查询角色授权时，仅保留关联菜单的 `platform_id = currentPlatformID`；
4. 超级管理员从当前平台有效页面和操作菜单建立起点，不读取其他平台；
5. Service 继续补齐祖先节点、构建树、排序并对角色菜单和权限 code 去重；
6. 快照中的 `platform` 继续使用身份平台 code，必须与身份平台一致；Redis key 继续包含平台 code；
7. 缓存命中、重建、发布和失效失败继续显式记录并按现有错误策略处理。

Access Version 当前按用户维护，不拆成平台级版本。任一菜单或角色授权变更仍使受影响用户的全局版本递增，从而清理旧平台快照；这样实现简单且不会留下旧权限。后续有真实性能需求时再单独设计平台级版本。

## 6. 管理 API 与前端

### 6.1 菜单管理

Admin 菜单 API 继续使用 `/api/admin/v1/menus`。菜单列表和创建 DTO 增加必填 `platformId`；列表支持按平台筛选，编辑 DTO 不接受 `platformId`。创建时由 Service 校验平台 ID 对应的平台存在且未删除；创建或编辑父级时校验父菜单 `platform_id` 相同，Handler 不访问数据库。停用平台的历史菜单仍可由 Admin 查看和整理，但不能进入 Access 快照。

菜单管理页面最上方使用 `el-tabs` 展示平台 code/name，当前 Tab 是页面唯一的平台上下文；切换 Tab 后加载对应平台菜单树。新增菜单自动使用当前 Tab 的平台 ID，创建弹窗不重复提供平台下拉；编辑时平台只读，不提供跨平台移动。Admin 前端默认选中内置 `admin`，但页面可以查看和维护已经存在的其他平台测试菜单，不会因为平台不是 `admin` 就把数据伪装成 Admin 菜单。

### 6.2 角色权限

角色 API 继续使用 `/api/admin/v1/roles`。权限树和授权请求携带平台维度；角色授权弹窗顶部使用 `el-tabs` 切换平台，下面只呈现当前平台权限矩阵，“全选”和“清空”只作用于当前平台。保存时合并所有平台的菜单 ID 并提交一个集合，后端验证菜单存在、启用状态和平台归属，不接受跨平台越权授权。

角色本身不绑定单个平台，因此同一角色可以同时拥有 Admin 和 Canvas 的菜单。Admin 管理员只能通过 Admin Token 访问管理 API，但可以配置其他平台的角色菜单。

### 6.3 Access 响应

`GET /api/v1/access` 的 envelope 和快照字段保持现有协议。返回的 `menuTree` 与 `permissionCodes` 只包含当前平台数据，不增加跨平台菜单列表，也不通过前端隐藏来实现隔离。

## 7. 数据迁移与约束

迁移必须显式、幂等、可审计，不依赖 AutoMigrate 完成既有表的字段回填或约束替换：

1. 为 `rbac_menu` 增加可回滚的 `platform_id` 列；
2. 在事务内锁定并读取内置 `admin` 平台 ID，将历史有效和已删除菜单回填为该 ID；
3. 检查所有父子菜单 `platform_id` 一致；
4. 将列设置为 `NOT NULL`，创建指向 `auth_platform(id)` 的 `ON DELETE RESTRICT` 外键；
5. 创建 `(id, platform_id)` 唯一约束，并以 `(parent_id, platform_id)` 组合外键替换单列父级外键，从数据库层保证父子同平台；
6. 将 code 唯一索引调整为有效数据的 `(platform_id, code)`；
7. 将页面 path 唯一索引调整为有效页面的 `(platform_id, path)`；
8. 将树查询索引调整为 `(platform_id, parent_id, sort_order, id)`，保留角色授权真实查询索引；
9. 迁移任一步骤失败时回滚，不留下半迁移状态。

认证平台被停用时不自动删除菜单，该平台不能建立新会话，其菜单也不能进入 Access 快照。认证平台仍有有效菜单时，Service 拒绝软删除；管理员必须先删除有效菜单。已软删除的历史菜单继续引用平台记录，数据库禁止硬删除平台，不通过级联静默清理。

菜单创建与平台软删除必须在同一 `auth_platform` 行锁上串行化：菜单创建事务锁定目标平台并确认未删除后再插入；平台删除事务锁定平台、复查有效菜单数量后再软删除。这样无论哪一方先取得锁，另一方都必须基于提交后的最新事实重新判断，不允许产生“已删除平台新增菜单”的竞态。

## 8. 错误边界

- 缺少或非法 `platformId`：返回现有 invalid request 错误；
- 创建菜单时平台不存在或已删除：返回明确业务错误；平台停用不妨碍 Admin 整理已有菜单；
- 父菜单不存在、已删除、`platform_id` 不同或类型不允许：返回菜单结构错误；
- Access 数据源出现跨平台父子关系、重复 code/path 或快照平台不一致：返回 Access snapshot invalid，不返回部分数据；
- PostgreSQL、Redis 或缓存发布失败：沿现有依赖错误路径返回，不使用内存、旧缓存或空数组兜底。

## 9. 测试策略

### 9.1 PostgreSQL Schema 与迁移

- `platform_id` 非空且存在有效外键，`(parent_id, platform_id)` 组合外键拒绝跨平台父子关系；
- 历史菜单全部回填为 `admin`，菜单 ID 和角色授权关系保持；
- `(platform_id, code)` 和 `(platform_id, path)` 只对有效记录唯一；
- 父子平台不一致或迁移发现非法历史数据时事务回滚；
- 根级页面合法，根级操作非法。

### 9.2 Access Repository 与 Service

- Admin 身份只读取 Admin 菜单；
- Canvas 身份只读取 Canvas 预设菜单；
- 认证成功后 `Identity.PlatformID` 精确来自当前 Policy，Access 重建不额外查询平台 ID；
- 同一用户多角色在当前平台合并菜单并去重；
- 超级管理员只能获得当前平台页面和操作权限；
- Canvas 的根级 `test` 页面和 `canvas:test:button` 能形成有效快照；
- 跨平台角色授权不会进入当前平台的 `grantedMenuIDs`；
- Redis 快照命中、重建和平台 key 隔离保持有效。

### 9.3 管理 Handler 与前端

- 菜单和角色请求必须携带平台，未知字段和缺失字段被拒绝；
- 跨平台父菜单和非法平台请求返回 4xx，Service 不写入数据库；
- 菜单列表和角色权限树正确显示平台筛选；
- Admin 前端继续请求 `/api/admin/v1`，Access 继续请求 `/api/v1`；
- TypeScript DTO 闭合，不使用显式 `any` 或类型断言兜底。

### 9.4 完整验证

后端运行 `go fmt ./...`、`go vet ./...`、`go test -count=1 -p=1 ./...`、`go build ./...`；前端运行 `pnpm vitest run --pool=threads --maxWorkers=1` 和 `pnpm build`；最后运行定向路径、平台泄露和 `git diff --check` 审计。不使用 Playwright，界面由用户人工验收。

## 10. 完成标准

1. 生产菜单全部具有明确平台且历史 Admin 数据不丢失；
2. Access 从数据库源到 Redis 快照全链路按身份平台隔离；
3. 根级页面可用，Canvas 启动预设能稳定生成 test 页面和 test 按钮权限；
4. 多角色合并、去重、超级管理员和缓存失效行为有自动化证据；
5. 管理 API、前端 DTO 和权限树没有跨平台兼容分支或静默过滤；
6. 完整后端、前端和静态审计通过；
7. 本切片不引入 Canvas 业务页面或其他未确认业务模块。
