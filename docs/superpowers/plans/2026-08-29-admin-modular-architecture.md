# Admin 模块化架构重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkboxes (`- [ ]`/`- [x]`) for tracking.

**Goal:** 在不改变已验收 HTTP 行为和 Admin UI 体验的前提下，把 Admin 后端、前端、数据库和审计边界一次性收敛到按领域和表名定位的模块化基线。

**Architecture:** 数据库结构只由维护者手工执行的、可回滚且可重复执行的 PostgreSQL forward migration 改变；API 和 Worker 启动只连接并读取已有状态，不执行 DDL、seed、回填或 Redis 清理。后端固定使用“领域/表对应子模块”，请求继续沿“router -> middleware -> handler -> service -> repository -> model -> PostgreSQL”，前端把页面和 API 按同一领域树归位，保留现有 route path、route name、permission code、HTTP URL 和响应 envelope。

**Tech Stack:** Go 1.26.5、Gin、GORM、PostgreSQL、Redis、Asynq、Vue 3、TypeScript strict、Pinia、Vue Router、Element Plus、Vitest、Vite、SCSS

**Spec:** docs/superpowers/specs/2026-08-29-admin-modular-architecture-design.md

## Global Constraints

- 普通数据库请求固定为：router -> middleware -> handler -> service -> repository -> model -> PostgreSQL。
- 前端 HTTP 数据流固定为：view -> api/<module>.ts -> utils/request.ts -> Go API。
- API 和 Worker 启动期间禁止 AutoMigrate、PrepareSchema、EnsureSchema、seed、数据回填、菜单 foundation、平台预设、访问版本补建和 Redis 清理。
- 数据库变化只能由 docs/database/2026-08-29-admin-modular-architecture.sql 这份人工执行的 migration 完成；API、Worker 和测试运行时组合根不得调用它。
- user_account.phone 保留在账号主表并使用有效账号部分唯一索引；user_profile 只保存 birthday、gender 及显式时间字段。
- 数据库关系使用 PostgreSQL 外键和 ON DELETE RESTRICT；不添加 is_del、is_deleted 或 BaseModel，日志和不可变事件不添加 deleted_at。
- 数据库和 Go Model/Repository 使用 platform_id；HTTP DTO 只使用 platformId 或既有平台 code/name，不同时输出 platform 和 platformId。
- rbac_access_version 继续使用单列 user_id 主键；缓存键同时包含 user_id、platform_id 和用户级 version。
- 登录成功、登录失败和登出只写 user_login_log；刷新 Token、Token 轮换、Redis 状态更新、缓存维护不写登录日志或操作日志。
- 操作日志只覆盖 Admin 管理业务变更；密码、密码哈希、Token、Cookie、Authorization、Secret、密钥、DSN、SQL 和堆栈永不进入响应或日志。
- Admin 管理接口使用 /api/admin/v1；共享认证和当前平台 access 使用 /api/v1；Admin 前端不提供注册 UI，bootstrap-admin 仍是显式命令。
- 不创建 Canvas 业务、AI、Agent/Harness、支付、COS/OSS 上传、邮件发送、短信发送、地理位置、WebSocket 或其他未来业务占位模块。
- 不引入 BaseService、BaseRepository、BaseModel、Manager、Factory、通用 Adapter、infra 万能目录、DI 容器或运行时注册器；只在真实替换边界定义最小接口。
- 所有入口 context.Context 必须传到 Service、Repository、GORM、Redis、Queue 和第三方 I/O；不得把 *gin.Context 传入 Service 或 Repository，也不得中途换成 context.Background()。
- HTTP envelope 只接受 code、data、message；前端业务 TypeScript 禁止显式 any、@ts-ignore、宽泛 ambient declaration 和关闭 strict 选项。
- 直接在 main 分支施工；未得到新的明确授权前不自动 commit、push、fetch、pull、rebase 或 amend，不使用 Playwright。

---

## Final File Map

实施完成后，后端领域树固定为：

    server/internal/module/
      user/{account,profile,loginlog,session}
      rbac/{menu,role,userrole,rolemenu,accessversion,access,state}
      auth/{login,platform,client,state}
      audit/operationlog
      health

前端领域页面和 API 固定为：

    web/src/modules/{user/{account,profile,loginlog,session},rbac/{menu,role,access},auth/{login,platform},audit/operationlog,dashboard}
    web/src/api/{user/{account,profile,loginlog,session},rbac/{menu,role,access},auth/{login,platform},audit/operationlog,health}.ts

web/src/layout、web/src/components、web/src/store、web/src/utils、web/src/types 和 web/src/styles 继续作为共享基础设施；web/src/utils/request.ts 是唯一 Axios/envelope 基础，不放业务 API。

---

### Task 1: 落地显式数据库 Migration 与 PostgreSQL 验证

**Files:**
- Create: docs/database/2026-08-29-admin-modular-architecture.sql
- Create: docs/database/2026-08-29-admin-modular-architecture.md
- Create: server/internal/database/admin_modular_migration_test.go
- Verify without editing: server/internal/database/testschema/schema.go
- Preserve as historical record: docs/database/2026-08-28-user-profile.sql

**Interfaces:**
- Consumes: 现有数据库中的 user_account、auth_session、auth_platform、rbac_menu、rbac_role、rbac_user_role、rbac_role_menu、rbac_access_version、audit_operation_log 和可选的 foundation_task。
- Produces: user_profile、user_session、user_login_log 以及 platform_id 外键、固定约束和固定索引；migration 不提供 Go 运行时函数。

- [x] **Step 1: 写 migration 成功路径和失败回滚测试**

在 admin_modular_migration_test.go 中用 testschema.Open 创建隔离 schema，写入固定 ID 的 Admin/Canvas 平台、用户、角色、菜单、用户角色、角色菜单、旧 auth_session、旧 audit_operation_log 和一行 foundation_task。测试通过 runtime.Caller 定位仓库根目录并读取 migration 文件，然后断言迁移后保留这些 ID 和计数：

    func TestAdminModularMigrationPreservesIDsAndIsIdempotent(t *testing.T) {
        db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_admin_modular_migration")
        createLegacyAdminFixture(t, db, ctx)
        script := readAdminModularMigration(t)

        if err := db.WithContext(ctx).Exec(script).Error; err != nil {
            t.Fatalf("execute admin modular migration: %v", err)
        }
        assertTableExists(t, db, ctx, "user_profile")
        assertTableExists(t, db, ctx, "user_session")
        assertTableExists(t, db, ctx, "user_login_log")
        assertTableMissing(t, db, ctx, "auth_session")
        assertColumnMissing(t, db, ctx, "audit_operation_log", "platform")
        assertColumnExists(t, db, ctx, "audit_operation_log", "platform_id")
        assertCount(t, db, ctx, "user_account", 2)
        assertCount(t, db, ctx, "user_session", 1)
        assertIDExists(t, db, ctx, "user_session", 41)
        assertIDExists(t, db, ctx, "audit_operation_log", 51)
        assertForeignKey(t, db, ctx, "fk_user_session_platform")

        if err := db.WithContext(ctx).Exec(script).Error; err != nil {
            t.Fatalf("second migration execution: %v", err)
        }
        assertCount(t, db, ctx, "user_session", 1)
        assertIDExists(t, db, ctx, "user_session", 41)
    }

另外写 TestAdminModularMigrationRollsBackUnknownSessionPlatform：旧会话插入不存在的 platform 值，执行脚本必须失败；失败后 auth_session 仍存在、user_session 不存在、旧列和值未改变。再写 TestAdminModularMigrationRejectsOldAndNewObjectsTogether：预先创建 user_session 与 auth_session，脚本必须直接失败且不覆盖任一对象。

- [x] **Step 2: 运行测试确认 RED**

    cd D:\admin\server
    go test ./internal/database -run 'TestAdminModularMigration' -count=1

预期：失败，因为 migration 文件和新表结构尚未存在；失败原因不得是测试 schema、DSN 或断言代码本身的拼写错误。

- [x] **Step 3: 编写事务化、可重复执行的 SQL**

在 SQL 文件开头使用 BEGIN，最后使用 COMMIT；所有失败通过 RAISE EXCEPTION 触发 PostgreSQL 事务回滚。按以下顺序实现，任何前置冲突都必须在第一条 DDL 前失败：

1. 检查重复的有效 lower(username)、有效 lower(email)、有效非空手机号、未知平台 code、重复默认角色、旧/新对象同时存在以及 foundation_task 的非 demo 外键引用。
2. CREATE TABLE user_profile，固定 user_id BIGINT PRIMARY KEY、birthday DATE、gender SMALLINT NOT NULL DEFAULT 0 CHECK (gender IN (0, 1, 2))、created_at/updated_at TIMESTAMPTZ NOT NULL 和 fk_user_profile_account。
3. 仅当 auth_session 存在且 user_session 不存在时执行 ALTER TABLE auth_session RENAME TO user_session；若两者同时存在直接失败。
4. 在 user_session 中增加 platform_id BIGINT，把旧字符串按 auth_platform.code 精确映射；未知、空值或重复映射直接失败。确认全部行映射后添加 fk_user_session_platform，再删除旧 platform 列和旧索引。保留 refresh_token_hash、版本、客户端摘要、时间和撤销数据。
5. 创建 user_login_log，严格使用 spec 的 event_type/login_type 联合检查、is_success SMALLINT CHECK (is_success IN (0, 1))、user_id/session_id/platform_id 外键、脱敏所需字段和显式时间戳；不把历史 audit_operation_log 行复制成登录事件。
6. 在 audit_operation_log 增加 platform_id BIGINT，旧非空平台 code 必须映射到唯一 auth_platform.id，空值保持空；未知值回滚。添加 fk_audit_operation_log_platform 后删除旧 platform 列。
7. 为每个现有用户补齐缺失的 rbac_access_version 行，保留已有 version；补齐只在该脚本中发生。验证 fk_rbac_access_version_user 和 ck_rbac_access_version_version。
8. 只有没有非 demo 外键引用时才 DROP TABLE foundation_task；发现业务引用立即失败，不删除引用方。
9. 创建并验证固定索引：ux_user_account_username_active、ux_user_account_email_active、ux_user_account_phone_active、ux_user_session_refresh_token_hash、ix_user_session_user_created_at、ix_user_session_user_platform_created_at、ix_user_login_log_created_at、ix_user_login_log_user_created_at、ix_user_login_log_platform_created_at、ix_user_login_log_account_created_at、ux_auth_platform_code_active、ux_rbac_role_code_active、ux_rbac_role_name_active、ux_rbac_role_default_active、ux_rbac_user_role_active、ux_rbac_menu_platform_code_active、ux_rbac_menu_platform_path_active、ix_rbac_menu_platform_parent_sort、ux_rbac_role_menu_active、ux_audit_operation_log_event_id、ix_audit_operation_log_request_id、ix_audit_operation_log_created_at、ix_audit_operation_log_user_created_at、ix_audit_operation_log_action_created_at 和实际有平台筛选页面需求时的 ix_audit_operation_log_platform_created_at。不为 is_enabled、is_success 或 deleted_at 单独建低选择性索引。

- [x] **Step 4: 增加 migration 文档中的执行、回滚和验证命令**

在 docs/database/2026-08-29-admin-modular-architecture.md 固定以下流程：先备份并在维护窗口停止 API/Worker，使用 psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f docs/database/2026-08-29-admin-modular-architecture.sql 执行；失败由事务自动回滚，脚本不提供猜测性 down migration，结构回退必须从维护者确认的备份恢复。验证 SQL 必须查询 to_regclass、information_schema.columns、pg_constraint、pg_indexes、各表计数和迁移前后 ID。

- [x] **Step 5: 运行成功、幂等和回滚测试**

    cd D:\admin\server
    go test ./internal/database -run 'TestAdminModularMigration' -count=1

预期：成功路径、第二次执行、未知平台回滚、旧新对象冲突全部 PASS；测试必须连接真实 PostgreSQL，不能换成内存数据库。

- [x] **Step 6: 检查 SQL 和文档差异**

    cd D:\admin
    git diff --check
    rg -n 'AutoMigrate|EnsureSchema|PrepareSchema|INSERT INTO.*foundation_task|CREATE TABLE.*auth_session' docs/database/2026-08-29-admin-modular-architecture.sql docs/database/2026-08-29-admin-modular-architecture.md

预期：第二条搜索无输出，migration 文件不包含运行时迁移调用、seed 或旧表重新创建。

- [x] **Step 7: 提交点（仅在维护者明确授权后执行）**

    git add docs/database/2026-08-29-admin-modular-architecture.sql docs/database/2026-08-29-admin-modular-architecture.md server/internal/database/admin_modular_migration_test.go
    git commit -m "feat!: 增加 Admin 模块化数据库迁移"

提交正文必须说明旧 auth_session/平台字符串迁移到 user_session.platform_id、新增 user_login_log 和删除 foundation_task 的受影响范围，并注明运行前必须备份、失败自动回滚且 API/Worker 不会执行 migration。

### Task 2: 拆分 User、Profile 与 Session 表模块

**Files:**
- Move: server/internal/module/user/errors.go -> server/internal/module/user/account/errors.go
- Move: server/internal/module/user/errors_test.go -> server/internal/module/user/account/errors_test.go
- Move: server/internal/module/user/handler.go -> server/internal/module/user/account/handler.go
- Move: server/internal/module/user/handler_test.go -> server/internal/module/user/account/handler_test.go
- Split: server/internal/module/user/model.go -> server/internal/module/user/account/model.go and server/internal/module/user/profile/model.go
- Move: server/internal/module/user/protocol.go -> server/internal/module/user/account/protocol.go
- Move: server/internal/module/user/protocol_test.go -> server/internal/module/user/account/protocol_test.go
- Split: server/internal/module/user/repository.go -> server/internal/module/user/account/repository.go and server/internal/module/user/profile/repository.go
- Move: server/internal/module/user/request.go -> server/internal/module/user/account/request.go
- Move: server/internal/module/user/response.go -> server/internal/module/user/account/response.go
- Move: server/internal/module/user/route.go -> server/internal/module/user/account/route.go
- Split: server/internal/module/user/service.go -> server/internal/module/user/account/service.go and server/internal/module/user/profile/service.go
- Move: server/internal/module/user/service_test.go -> server/internal/module/user/account/service_test.go
- Move: server/internal/module/user/repository_test.go -> server/internal/module/user/account/repository_test.go
- Move: server/internal/module/account/handler.go -> server/internal/module/user/profile/handler.go
- Move: server/internal/module/account/handler_test.go -> server/internal/module/user/profile/handler_test.go
- Move: server/internal/module/account/request.go -> server/internal/module/user/profile/request.go
- Move: server/internal/module/account/response.go -> server/internal/module/user/profile/response.go
- Move: server/internal/module/account/route.go -> server/internal/module/user/profile/route.go
- Move: server/internal/module/auth/session_model.go -> server/internal/module/user/session/model.go
- Move: server/internal/module/auth/session_repository.go -> server/internal/module/user/session/repository.go
- Move: server/internal/module/auth/session_repository_test.go -> server/internal/module/user/session/repository_test.go
- Move: server/internal/module/auth/session_admin.go -> server/internal/module/user/session/service.go
- Move: server/internal/module/auth/session_admin_errors.go -> server/internal/module/user/session/errors.go
- Move: server/internal/module/auth/session_admin_handler.go -> server/internal/module/user/session/handler.go
- Move: server/internal/module/auth/session_admin_handler_test.go -> server/internal/module/user/session/handler_test.go
- Move: server/internal/module/auth/session_admin_integration_test.go -> server/internal/module/user/session/integration_test.go
- Move: server/internal/module/auth/session_admin_request.go -> server/internal/module/user/session/request.go
- Move: server/internal/module/auth/session_admin_response.go -> server/internal/module/user/session/response.go
- Move: server/internal/module/auth/session_admin_route.go -> server/internal/module/user/session/route.go
- Move: server/internal/module/auth/session_admin_service_test.go -> server/internal/module/user/session/service_test.go
- Create: server/internal/module/user/session/protocol.go
- Modify: server/cmd/bootstrap-admin/main.go
- Modify: server/cmd/bootstrap-admin/main_test.go

**Interfaces:**
- Consumes: existing user CRUD, personal profile, password-independent session administration and user_session migration result.
- Produces: user/account owning user_account, user/profile owning user_profile, and user/session owning user_session; profile code must not import auth/login, account code must not import an HTTP package, and session code must not own token issuance.

- [x] **Step 1: 写目标包的表归属和编译护栏测试**

在新目录中先写 model_test.go，断言 Account.TableName() 为 user_account、Profile.TableName() 为 user_profile、Session.TableName() 为 user_session，并检查三个模型都直接声明 CreatedAt/UpdatedAt；同时写一个 source-level 测试，禁止目标包中的 SQL 出现 auth_session 或 session.platform =。

- [x] **Step 2: 运行目标包测试确认 RED**

    cd D:\admin\server
    go test ./internal/module/user/account ./internal/module/user/profile ./internal/module/user/session -run 'Test(Account|Profile|Session)' -count=1

预期：失败，因为目标类型和包实现尚未迁入。

- [x] **Step 3: 移动并拆分账号与资料实现**

用 git mv 移动未拆分文件，手工把旧 user/model.go 的 User 模型放入 user/account/model.go，把 Profile 放入 user/profile/model.go；把 CurrentProfile、UpdatePersonalProfile、PersonalProfile 和 PersonalProfileInput 移入 profile Service/Repository。账号 Service 只保留列表、账号更新、状态、软删除和角色入口，phone 仍是账号字段，邮箱仍是全局登录标识。

profile Service 使用一个显式事务接口同时更新 user_account.username/phone 和 user_profile.birthday/gender；资料缺失只允许在明确的 profile 写入流程中插入，读取、API/Worker 启动和登录流程不得偷偷补行。保留 /account/profile、/account/password 的 HTTP path 和响应字段；密码请求 DTO 由 profile Handler 传入 login Service 的最小接口，不让 profile 包依赖 login 包。

- [x] **Step 4: 移动会话管理并切换数据库列**

把会话模型、Repository、管理员列表/统计/踢除 Handler 和测试移入 user/session。所有查询从 auth_session 改为 user_session，平台过滤改为 session.platform_id = ? 并通过 auth_platform join 得到既有响应中的平台 code；Session 模型字段命名为 PlatformID，对外 AdminSession.Platform 仍保留平台 code。会话 Repository 继续只负责 PostgreSQL，Redis 代际和 Token 逻辑留给后续 auth/state、auth/login。

在 user/session/protocol.go 固定 auth:session:list 和 auth:session:revoke 权限码，并让 RegisterRoutes 继续注册 /sessions、/sessions/stats、/sessions/:id 和 /sessions。

- [x] **Step 5: 修复跨包依赖和 bootstrap-admin**

将 cmd/bootstrap-admin 的 user.NewRepository、user.User、auth.BootstrapAdminInput 引用改为 user/account、rbac/role 和 auth/login 的明确导出类型。不得让 user/account 反向 import auth/login；需要的密码哈希和角色读写通过 login/role 暴露的当前最小接口完成。更新所有 Go 测试 import，禁止保留 internal/module/user 或 internal/module/account 的运行时包。

- [x] **Step 6: 运行 User/Profile/Session 定向测试**

    cd D:\admin\server
    go test ./internal/module/user/... ./internal/module/auth/... ./cmd/bootstrap-admin -count=1

预期：目标包全部通过；若仍有旧包引用，测试必须在本步骤暴露具体 import 路径并修复后再继续。

- [x] **Step 7: 提交点（仅在维护者明确授权后执行）**

    git add server/internal/module/user server/internal/module/account server/internal/module/auth server/cmd/bootstrap-admin
    git commit -m "refactor!: 按账号资料会话表拆分 User 模块"

提交正文必须说明 Go import 路径变化、auth_session 更名为 user_session、会话平台改为 platform_id，以及 HTTP path/response 未变化。

### Task 3: 拆分 RBAC 表模块并锁定跨平台权限算法

**Files:**
- Move: server/internal/module/menu/errors.go -> server/internal/module/rbac/menu/errors.go
- Move: server/internal/module/menu/errors_test.go -> server/internal/module/rbac/menu/errors_test.go
- Move: server/internal/module/menu/handler.go -> server/internal/module/rbac/menu/handler.go
- Move: server/internal/module/menu/handler_test.go -> server/internal/module/rbac/menu/handler_test.go
- Move: server/internal/module/menu/model.go -> server/internal/module/rbac/menu/model.go after removing RoleMenu
- Move: server/internal/module/menu/protocol.go -> server/internal/module/rbac/menu/protocol.go
- Move: server/internal/module/menu/protocol_v2_test.go -> server/internal/module/rbac/menu/protocol_v2_test.go
- Move: server/internal/module/menu/repository.go -> server/internal/module/rbac/menu/repository.go
- Move: server/internal/module/menu/repository_test.go -> server/internal/module/rbac/menu/repository_test.go
- Move: server/internal/module/menu/request.go -> server/internal/module/rbac/menu/request.go
- Move: server/internal/module/menu/response.go -> server/internal/module/rbac/menu/response.go
- Move: server/internal/module/menu/route.go -> server/internal/module/rbac/menu/route.go
- Move: server/internal/module/menu/service.go -> server/internal/module/rbac/menu/service.go
- Move: server/internal/module/menu/service_test.go -> server/internal/module/rbac/menu/service_test.go
- Move: server/internal/module/menu/tree.go -> server/internal/module/rbac/menu/tree.go
- Move: server/internal/module/menu/tree_test.go -> server/internal/module/rbac/menu/tree_test.go
- Move: server/internal/module/menu/test_database_test.go -> server/internal/module/rbac/menu/test_database_test.go
- Move: server/internal/module/menu/test_models_test.go -> server/internal/module/rbac/menu/test_models_test.go
- Move: server/internal/module/role/errors.go -> server/internal/module/rbac/role/errors.go
- Move: server/internal/module/role/errors_test.go -> server/internal/module/rbac/role/errors_test.go
- Move: server/internal/module/role/handler.go -> server/internal/module/rbac/role/handler.go
- Move: server/internal/module/role/handler_test.go -> server/internal/module/rbac/role/handler_test.go
- Move: server/internal/module/role/permission.go -> server/internal/module/rbac/role/permission.go
- Move: server/internal/module/role/permission_test.go -> server/internal/module/rbac/role/permission_test.go
- Move: server/internal/module/role/protocol.go -> server/internal/module/rbac/role/protocol.go
- Move: server/internal/module/role/request.go -> server/internal/module/rbac/role/request.go
- Move: server/internal/module/role/response.go -> server/internal/module/rbac/role/response.go
- Move: server/internal/module/role/route.go -> server/internal/module/rbac/role/route.go
- Move: server/internal/module/role/service.go -> server/internal/module/rbac/role/service.go after removing relation persistence
- Move: server/internal/module/role/service_test.go -> server/internal/module/rbac/role/service_test.go
- Move: server/internal/module/role/repository.go -> server/internal/module/rbac/role/repository.go after splitting relation queries
- Move: server/internal/module/role/repository_test.go -> server/internal/module/rbac/role/repository_test.go
- Split: server/internal/module/role/model.go into server/internal/module/rbac/role/model.go and server/internal/module/rbac/userrole/model.go
- Split: server/internal/module/menu/model.go into server/internal/module/rbac/menu/model.go and server/internal/module/rbac/rolemenu/model.go
- Create: server/internal/module/rbac/userrole/repository.go
- Create: server/internal/module/rbac/userrole/service.go
- Create: server/internal/module/rbac/userrole/service_test.go
- Create: server/internal/module/rbac/rolemenu/repository.go
- Create: server/internal/module/rbac/rolemenu/service.go
- Create: server/internal/module/rbac/rolemenu/service_test.go
- Move: server/internal/module/access/model.go -> server/internal/module/rbac/accessversion/model.go
- Move: server/internal/module/access/errors.go -> server/internal/module/rbac/access/errors.go
- Move: server/internal/module/access/errors_test.go -> server/internal/module/rbac/access/errors_test.go
- Move: server/internal/module/access/repository.go -> server/internal/module/rbac/access/repository.go after extracting version queries
- Move: server/internal/module/access/repository_test.go -> server/internal/module/rbac/access/repository_test.go
- Move: server/internal/module/access/handler.go -> server/internal/module/rbac/access/handler.go
- Move: server/internal/module/access/handler_test.go -> server/internal/module/rbac/access/handler_test.go
- Move: server/internal/module/access/middleware.go -> server/internal/module/rbac/access/middleware.go
- Move: server/internal/module/access/middleware_test.go -> server/internal/module/rbac/access/middleware_test.go
- Move: server/internal/module/access/response.go -> server/internal/module/rbac/access/response.go
- Move: server/internal/module/access/route.go -> server/internal/module/rbac/access/route.go
- Move: server/internal/module/access/service.go -> server/internal/module/rbac/access/service.go
- Move: server/internal/module/access/service_test.go -> server/internal/module/rbac/access/service_test.go
- Move: server/internal/module/access/redis.go -> server/internal/module/rbac/access/redis.go
- Move: server/internal/module/access/redis_test.go -> server/internal/module/rbac/access/redis_test.go
- Move: server/internal/module/accessstate/mutation.go -> server/internal/module/rbac/state/mutation.go
- Move: server/internal/module/accessstate/mutation_test.go -> server/internal/module/rbac/state/mutation_test.go
- Move: server/internal/module/accessstate/redis.go -> server/internal/module/rbac/state/redis.go
- Move: server/internal/module/accessstate/redis_test.go -> server/internal/module/rbac/state/redis_test.go
- Move: server/internal/module/accessstate/state.go -> server/internal/module/rbac/state/state.go
- Move: server/internal/module/accessstate/state_test.go -> server/internal/module/rbac/state/state_test.go
- Create: server/internal/module/rbac/accessversion/repository.go
- Create: server/internal/module/rbac/accessversion/service.go
- Create: server/internal/module/rbac/accessversion/service_test.go
- Delete after test replacement: server/internal/module/menu/schema.go, server/internal/module/menu/migration.go, server/internal/module/menu/foundation.go, server/internal/module/menu/foundation_test.go, server/internal/module/menu/schema_migration_test.go, server/internal/module/menu/domain_rekey_test.go, server/internal/module/menu/schema_test.go, server/internal/module/role/schema.go, server/internal/module/role/schema_test.go, server/internal/module/access/schema.go, server/internal/module/access/schema_test.go

**Interfaces:**
- Consumes: Task 1 的最终表结构和 user/account、user/session 的明确读写方法。
- Produces: rbac/menu 只处理 rbac_menu，rbac/role 只处理 rbac_role，rbac/userrole 只处理 rbac_user_role，rbac/rolemenu 只处理 rbac_role_menu，rbac/accessversion 只处理 rbac_access_version，rbac/access 计算当前平台权限，rbac/state 只处理权限 Redis 状态。

- [x] **Step 1: 写表归属和权限隔离失败测试**

先在目标包测试中固定以下行为：角色模型 TableName 为 rbac_role，用户角色为 rbac_user_role，角色菜单为 rbac_role_menu，访问版本为 rbac_access_version；HTTP 集成测试创建 Admin 和 Canvas 菜单，使用同一用户的多个角色调用 /api/v1/access，断言 Admin 响应不含 Canvas 的 menu ID、path、permission code，Canvas 响应不含 Admin 节点；超级管理员也只获得请求平台的有效菜单。

再增加关系约束测试：尝试把 Canvas parent_id 指向 Admin 菜单，PostgreSQL 必须因 (parent_id, platform_id) 复合外键拒绝；尝试向 rbac_role_menu 写入不存在的 role/menu，必须因外键失败。

- [x] **Step 2: 运行 RBAC 定向测试确认 RED**

    cd D:\admin\server
    go test ./internal/module/rbac/... -run 'Test(Table|Access|Platform|Role|Menu)' -count=1

预期：目标包尚未完整存在或仍返回旧平铺包行为，跨平台隔离测试失败。

- [x] **Step 3: 移动菜单并抽离 RoleMenu**

将菜单 CRUD、树校验、请求/响应和路由移动到 rbac/menu；从菜单模型删除 RoleMenu，在 rbac/rolemenu/model.go 声明 RoleID、MenuID、显式时间戳和 deleted_at。菜单 Repository 的授权查询改为调用 rolemenu 的明确读方法，不在菜单包里塞入角色业务规则。删除生产 foundation.go、migration.go 和 schema mutation；菜单的数据库约束由 Task 1 SQL 提供。

- [x] **Step 4: 移动角色并拆分 UserRole/RoleMenu 变更**

rbac/role 保留角色生命周期、默认角色和系统角色保护；rbac/userrole 负责用户多角色的规范化、去重、增删和 rbac_access_version 递增；rbac/rolemenu 负责角色菜单授权的规范化、祖先目录校验和 version 递增。角色 Service 通过这两个包的最小接口编排事务，不直接访问对方 GORM 表。super_admin 不写入平台字段，也不通过角色表复制菜单。

- [x] **Step 5: 拆分 access version 与 Redis state**

把当前 access.Version 模型和版本锁/递增查询移入 rbac/accessversion；把 accessstate 全部移入 rbac/state；rbac/access 只保留当前平台菜单源查询、角色 code 去重、菜单 ID 去重、祖先补齐、树构建和 snapshot cache。缓存 key 必须保持 (userID, platformID, version)，任一角色、用户角色或角色菜单变更递增同一用户的单行 version。

- [x] **Step 6: 固定访问算法测试**

在真实 PostgreSQL/Redis 测试中覆盖：普通多角色合并去重、禁用角色不生效、超级管理员只取当前平台、Canvas 无目录的 root page 可作为 page、跨平台父子关系被复合外键阻断、version 改变导致所有平台旧快照失效。测试请求必须把平台从认证身份传入，禁止从请求 body、query 或菜单数据猜平台。

- [x] **Step 7: 运行 RBAC 全部定向测试**

    cd D:\admin\server
    go test ./internal/module/rbac/... -count=1

预期：目标模块的 Model、Repository、Service、Handler、路由和 PostgreSQL/Redis 集成测试全部通过，且没有 EnsureSchema、AutoMigrate 或 seed 调用。

- [x] **Step 8: 提交点（仅在维护者明确授权后执行）**

    git add server/internal/module/menu server/internal/module/role server/internal/module/access server/internal/module/accessstate server/internal/module/rbac
    git commit -m "refactor!: 按 RBAC 表拆分权限模块"

提交正文必须说明角色为全局资源、菜单拥有平台归属、rbac_access_version 保持用户级单行，以及 /api/v1/access 不返回其他平台权限。

### Task 4: 拆分 Auth、接入登录日志并固定会话事件语义

**Files:**
- Move: server/internal/module/auth/bootstrap_service.go -> server/internal/module/auth/login/bootstrap_service.go
- Move: server/internal/module/auth/bootstrap_service_test.go -> server/internal/module/auth/login/bootstrap_service_test.go
- Move: server/internal/module/auth/handler.go -> server/internal/module/auth/login/handler.go
- Move: server/internal/module/auth/handler_test.go -> server/internal/module/auth/login/handler_test.go
- Move: server/internal/module/auth/middleware.go -> server/internal/module/auth/login/middleware.go
- Move: server/internal/module/auth/middleware_test.go -> server/internal/module/auth/login/middleware_test.go
- Move: server/internal/module/auth/password.go -> server/internal/module/auth/login/password.go
- Move: server/internal/module/auth/password_test.go -> server/internal/module/auth/login/password_test.go
- Move: server/internal/module/auth/protocol.go -> server/internal/module/auth/login/protocol.go
- Move: server/internal/module/auth/request.go -> server/internal/module/auth/login/request.go
- Move: server/internal/module/auth/response.go -> server/internal/module/auth/login/response.go
- Move: server/internal/module/auth/route.go -> server/internal/module/auth/login/route.go
- Move: server/internal/module/auth/service.go -> server/internal/module/auth/login/service.go
- Move: server/internal/module/auth/service_test.go -> server/internal/module/auth/login/service_test.go
- Move: server/internal/module/auth/token.go -> server/internal/module/auth/login/token.go
- Move: server/internal/module/auth/token_test.go -> server/internal/module/auth/login/token_test.go
- Move: server/internal/module/auth/redis.go -> server/internal/module/auth/state/session_cache.go
- Move: server/internal/module/auth/redis_test.go -> server/internal/module/auth/state/session_cache_test.go
- Move: server/internal/module/authclient/client.go -> server/internal/module/auth/client/client.go
- Move: server/internal/module/authclient/middleware.go -> server/internal/module/auth/client/middleware.go
- Move: server/internal/module/authclient/middleware_test.go -> server/internal/module/auth/client/middleware_test.go
- Move: server/internal/module/authplatform/errors.go -> server/internal/module/auth/platform/errors.go
- Move: server/internal/module/authplatform/handler.go -> server/internal/module/auth/platform/handler.go
- Move: server/internal/module/authplatform/handler_test.go -> server/internal/module/auth/platform/handler_test.go
- Move: server/internal/module/authplatform/model.go -> server/internal/module/auth/platform/model.go
- Move: server/internal/module/authplatform/protocol.go -> server/internal/module/auth/platform/protocol.go
- Move: server/internal/module/authplatform/redis.go -> server/internal/module/auth/platform/redis.go after removing ClearBuiltinPolicies
- Move: server/internal/module/authplatform/repository.go -> server/internal/module/auth/platform/repository.go
- Move: server/internal/module/authplatform/repository_test.go -> server/internal/module/auth/platform/repository_test.go
- Move: server/internal/module/authplatform/request.go -> server/internal/module/auth/platform/request.go
- Move: server/internal/module/authplatform/response.go -> server/internal/module/auth/platform/response.go
- Move: server/internal/module/authplatform/route.go -> server/internal/module/auth/platform/route.go
- Move: server/internal/module/authplatform/service.go -> server/internal/module/auth/platform/service.go
- Move: server/internal/module/authplatform/service_test.go -> server/internal/module/auth/platform/service_test.go
- Move: server/internal/module/authstate/mutation.go -> server/internal/module/auth/state/mutation.go
- Move: server/internal/module/authstate/mutation_test.go -> server/internal/module/auth/state/mutation_test.go
- Move: server/internal/module/authstate/redis.go -> server/internal/module/auth/state/redis.go
- Move: server/internal/module/authstate/redis_test.go -> server/internal/module/auth/state/redis_test.go
- Move: server/internal/module/authstate/state.go -> server/internal/module/auth/state/state.go
- Move: server/internal/module/authstate/state_test.go -> server/internal/module/auth/state/state_test.go
- Create: server/internal/module/user/loginlog/model.go
- Create: server/internal/module/user/loginlog/repository.go
- Create: server/internal/module/user/loginlog/service.go
- Create: server/internal/module/user/loginlog/handler.go
- Create: server/internal/module/user/loginlog/request.go
- Create: server/internal/module/user/loginlog/response.go
- Create: server/internal/module/user/loginlog/route.go
- Create: server/internal/module/user/loginlog/model_test.go
- Create: server/internal/module/user/loginlog/repository_test.go
- Create: server/internal/module/user/loginlog/service_test.go
- Create: server/internal/module/user/loginlog/handler_test.go
- Modify: server/internal/middleware/access_log.go
- Delete after test replacement: server/internal/module/auth/schema.go, server/internal/module/auth/schema_test.go, server/internal/module/authplatform/schema.go, server/internal/module/authplatform/schema_test.go, server/internal/module/authplatform/redis_test.go cases for ClearBuiltinPolicies

**Interfaces:**
- Consumes: user/account account lookup/password hash, user/session session persistence, rbac/access current permission snapshot, auth/platform policy, auth/client request platform/device data and auth/state Redis generation/cache.
- Produces: auth/login for login/register/password/token/logout orchestration, auth/platform for auth_platform, auth/client for client parsing, auth/state for authentication Redis state, and user/loginlog for synchronous security-event persistence and Admin read API.

- [x] **Step 1: 写登录日志模型和认证落点失败测试**

先创建 user/loginlog 的测试假实现和表模型测试。固定事件输入结构：

    type Event struct {
        UserID       *int64
        SessionID    *int64
        PlatformID   int64
        LoginAccount string
        EventType    string
        LoginType    *string
        IsSuccess    yesno.Value
        ReasonCode   string
        ClientIP     string
        UserAgent    string
    }

服务测试必须先失败地声明以下期望：成功 password login 写一行 event_type=login,is_success=1；密码错误写一行 event_type=login,is_success=0 且 user_id/session_id 为空；logout 撤销会话后写一行 event_type=logout,login_type IS NULL；refresh 只轮换 hash 和 session version，登录日志仓储调用次数为零；登录日志落库失败时登录请求返回依赖错误且不返回凭据。

- [x] **Step 2: 运行认证和 loginlog 测试确认 RED**

    cd D:\admin\server
    go test ./internal/module/user/loginlog ./internal/module/auth/login -run 'Test(Login|Logout|Refresh|LoginLog)' -count=1

预期：失败，因为 loginlog 仓储、事件调用和目标 auth 包尚未存在。

- [x] **Step 3: 实现 user_login_log 线性链路**

model.go 严格映射 spec 的字段和 user_login_log 表；repository.go 提供 Insert(ctx, Event) 与按 created_at DESC, id DESC 的分页查询，用户和平台展示通过 LEFT JOIN user_account、JOIN auth_platform 得到 username/platform code。service.go 只做事件类型、login type、reason code、账号长度、IP/User-Agent 长度和 yes/no 校验，不接受密码、验证码、Token 或原始异常。

route.go 注册成熟旧项目的两个只读入口：GET /users/login-logs/page-init 和 GET /users/login-logs，挂在 /api/admin/v1 下并要求 account:user:loginlog:list。响应使用当前项目的 code/data/message 和 lower camel case；保留平台、登录类型、成功状态、账号前缀、日期范围和分页筛选，不复制旧项目的 is_del 或 snake_case envelope。

- [x] **Step 4: 移动 auth 并接入显式事件调用**

将 Token、密码、注册和认证 Handler 放入 auth/login；会话模型和管理员会话服务使用 Task 2 的 user/session；SessionCache 和 generation 状态使用 auth/state。登录成功事务按以下顺序执行：规范化邮箱 -> 读取启用账号 -> 校验密码 -> 创建/轮换 user_session(platform_id) -> 插入成功 login log -> 发布 Redis 状态 -> 返回凭据。登录失败只插入失败事件，不创建会话；失败日志插入错误必须沿统一 envelope 返回。

logout 事务撤销当前会话并插入一条 logout 事件；重复 logout 不重复改变会话状态，但每次有效身份请求记录一次成功事件。refresh 只做 refresh hash 原子轮换、session version/Redis 更新和返回新凭据，不触发任何 loginlog 或 operationlog。

- [x] **Step 5: 补齐认证平台 ID 边界**

认证请求继续用平台 code 作为外部识别，auth/client 解析后必须查出 auth_platform.id；auth.Identity 同时携带 PlatformID 和既有 Platform code。JWT 中可保留现有 platform claim 以维持已验收 Token 协议，但所有持久化 session/login log 查询和写入只使用 PlatformID。

- [x] **Step 6: 运行认证事件和登录日志集成测试**

    cd D:\admin\server
    go test ./internal/module/auth/... ./internal/module/user/loginlog ./internal/module/user/session -count=1

预期：登录成功/失败、logout 幂等、refresh 无日志、平台 ID 外键、Admin 登录日志列表和错误透传全部 PASS。

- [x] **Step 7: 提交点（仅在维护者明确授权后执行）**

    git add server/internal/module/auth server/internal/module/authclient server/internal/module/authplatform server/internal/module/authstate server/internal/module/user/loginlog server/internal/module/user/session server/internal/middleware/access_log.go
    git commit -m "feat: 增加登录日志并拆分认证模块"

提交正文必须明确 refresh 不写登录/操作日志、logout 写 login log、认证持久化使用 platform_id，以及 Admin 注册 UI 仍不存在。

### Task 5: 收敛 Operation Log、Worker 和 API 启动组合根

**Files:**
- Move: server/internal/module/operationlog/handler.go -> server/internal/module/audit/operationlog/handler.go
- Move: server/internal/module/operationlog/handler_test.go -> server/internal/module/audit/operationlog/handler_test.go
- Move: server/internal/module/operationlog/integration_test.go -> server/internal/module/audit/operationlog/integration_test.go
- Move: server/internal/module/operationlog/middleware.go -> server/internal/module/audit/operationlog/middleware.go
- Move: server/internal/module/operationlog/middleware_test.go -> server/internal/module/audit/operationlog/middleware_test.go
- Move: server/internal/module/operationlog/model.go -> server/internal/module/audit/operationlog/model.go
- Move: server/internal/module/operationlog/protocol.go -> server/internal/module/audit/operationlog/protocol.go
- Move: server/internal/module/operationlog/repository.go -> server/internal/module/audit/operationlog/repository.go
- Move: server/internal/module/operationlog/request.go -> server/internal/module/audit/operationlog/request.go
- Move: server/internal/module/operationlog/response.go -> server/internal/module/audit/operationlog/response.go
- Move: server/internal/module/operationlog/route.go -> server/internal/module/audit/operationlog/route.go
- Move: server/internal/module/operationlog/rules.go -> server/internal/module/audit/operationlog/rules.go
- Move: server/internal/module/operationlog/service.go -> server/internal/module/audit/operationlog/service.go
- Move: server/internal/module/operationlog/task.go -> server/internal/module/audit/operationlog/task.go
- Move: server/internal/module/operationlog/task_test.go -> server/internal/module/audit/operationlog/task_test.go
- Delete after moving test-only builders: server/internal/module/operationlog/schema.go, server/internal/module/operationlog/schema_test.go
- Delete after SQL migration coverage: server/internal/database/domain_names.go, server/internal/database/domain_names_test.go
- Modify: server/internal/module/audit/operationlog/model.go
- Modify: server/internal/module/audit/operationlog/protocol.go
- Modify: server/internal/module/audit/operationlog/middleware.go
- Modify: server/internal/module/audit/operationlog/rules.go
- Modify: server/internal/module/audit/operationlog/repository.go
- Modify: server/internal/module/audit/operationlog/handler.go
- Modify: server/internal/module/audit/operationlog/route.go
- Modify: server/internal/module/audit/operationlog/*_test.go
- Modify: server/cmd/api/main.go
- Modify: server/cmd/api/main_test.go
- Modify: server/cmd/worker/main.go
- Modify: server/cmd/worker/main_test.go
- Modify: server/cmd/bootstrap-admin/main.go

**Interfaces:**
- Consumes: auth/login, auth/platform, auth/client, rbac/*, user/* and the existing queue client.
- Produces: operation log tasks for Admin management mutations only; API/Worker startup that only loads config, opens dependencies, composes services and registers routes.

- [x] **Step 1: 写启动禁用 DDL/seed/Redis 清理测试**

在 server/cmd/api/main_test.go 和 server/cmd/worker/main_test.go 中固定禁止字符串集合：

    forbidden := []string{
        "database.AutoMigrate(", "PrepareDomainNames(", "PrepareSchema(", "EnsureSchema(",
        "EnsureCanvasPreset(", "EnsureFoundation(", "EnsurePlatformFoundation(",
        "EnsureSystemRoles(", "ClearBuiltinPolicies(", "CleanupLegacySessionPointers(",
    }

测试同时断言 database.Open、Redis open、queue client、buildRouter 的组合顺序存在，并断言 Worker 只注册 operation log task handler。

- [x] **Step 2: 写 operation log 规则失败测试**

把 route rule 断言改为只接受以下 Admin 管理变更：认证平台创建/修改/状态/删除、菜单创建/修改/状态/删除、角色创建/修改/状态/default/删除/权限、用户修改/状态/删除/角色、profile 修改、password 修改、session revoke。以下四个共享认证路由必须没有 operation rule：

    /api/v1/auth/register
    /api/v1/auth/login
    /api/v1/auth/refresh
    /api/v1/auth/logout

密码变更 action 固定为 account.password.change，资料变更固定为 account.profile.update；login、refresh、logout 不得因为 Handler 设置 access-log operation 而进入 operation log 队列。

- [x] **Step 3: 运行目标测试确认 RED**

    cd D:\admin\server
    go test ./cmd/api ./cmd/worker ./internal/module/audit/operationlog -count=1

预期：旧平铺 import、旧认证规则和启动期 mutation 断言失败。

- [x] **Step 4: 完成 operation log 的 platform_id 迁移**

将 Model 字段改为 PlatformID *int64；TaskPayload 使用 PlatformID *int64 和 JSON 字段 platformId，由认证上下文写入；Repository 插入 platform_id 并在列表查询 join auth_platform 返回既有 platform code 字段，避免前端因内部外键改名而改变协议。保留 event ID 幂等、request ID、脱敏 JSON、状态码、耗时和分页索引。

- [x] **Step 5: 重写组合根但不引入万能注册器**

buildRouter 固定注册：

    sharedRoutes := router.Group("/api/v1")
    sharedRoutes.Use(authclient.Require())
    auth.RegisterRoutes(sharedRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
    authplatform.RegisterPublicRoutes(sharedRoutes, dependencies.AuthPlatform)
    access.RegisterRoutes(sharedRoutes, dependencies.Access, dependencies.Authenticate)

    adminRoutes := router.Group("/api/admin/v1")
    adminRoutes.Use(authclient.Require(), authclient.RequireAdminPlatform())
    authplatform.RegisterManagementRoutes(adminRoutes, dependencies.AuthPlatform, dependencies.Authenticate, dependencies.RequirePermission)
    menu.RegisterRoutes(adminRoutes, dependencies.Menu, dependencies.Authenticate, dependencies.RequirePermission)
    role.RegisterRoutes(adminRoutes, dependencies.Role, dependencies.Authenticate, dependencies.RequirePermission)
    user.RegisterRoutes(adminRoutes, dependencies.User, dependencies.Authenticate, dependencies.RequirePermission)
    profile.RegisterRoutes(adminRoutes, dependencies.Profile, dependencies.Authenticate)
    loginlog.RegisterRoutes(adminRoutes, dependencies.LoginLog, dependencies.Authenticate, dependencies.RequirePermission)
    session.RegisterRoutes(adminRoutes, dependencies.Session, dependencies.Authenticate, dependencies.RequirePermission)
    operationlog.RegisterRoutes(adminRoutes, dependencies.OperationLog, dependencies.Authenticate, dependencies.RequirePermission)

保留全局 Middleware 顺序和健康检查 /health、/ready；不注册旧管理路径别名，不把 Auth/Access 搬到 Admin namespace。

- [x] **Step 6: 让 Worker 只消费操作日志**

删除 taskdemo Processor、Repository、Service 和注册调用；buildWorkerMux 只接收 operationlog.Processor 并注册 operation log task。操作日志 Task Handler 继续严格 decode schemaVersion/eventId/requestId/platformId，从 context 传入 Repository，不直接访问 GORM。

- [x] **Step 7: 运行 API/Worker/审计定向测试**

    cd D:\admin\server
    go test ./cmd/api ./cmd/worker ./internal/module/audit/operationlog ./internal/module/auth ./internal/module/rbac ./internal/module/user -count=1

预期：路由表、Admin 平台隔离、operation rule、脱敏、幂等任务、无启动 DDL/seed/Redis 清理全部 PASS。

- [x] **Step 8: 提交点（仅在维护者明确授权后执行）**

    git add server/cmd/api server/cmd/worker server/cmd/bootstrap-admin server/internal/module/audit server/internal/module/operationlog server/internal/database
    git commit -m "refactor!: 收敛审计模块与运行时启动边界"

提交正文必须说明旧管理路径不再注册、共享 /api/v1 保持不变、API/Worker 不执行任何 schema/data mutation，以及 Worker 仅保留操作日志消费。

### Task 6: 删除 taskdemo、foundation_task 和首页示例任务

**Files:**
- Delete: server/internal/module/taskdemo/handler.go
- Delete: server/internal/module/taskdemo/handler_test.go
- Delete: server/internal/module/taskdemo/model.go
- Delete: server/internal/module/taskdemo/model_test.go
- Delete: server/internal/module/taskdemo/repository.go
- Delete: server/internal/module/taskdemo/repository_test.go
- Delete: server/internal/module/taskdemo/request.go
- Delete: server/internal/module/taskdemo/response.go
- Delete: server/internal/module/taskdemo/route.go
- Delete: server/internal/module/taskdemo/service.go
- Delete: server/internal/module/taskdemo/service_test.go
- Delete: server/internal/module/taskdemo/task.go
- Delete: server/internal/module/taskdemo/task_test.go
- Delete: web/src/api/taskDemo.ts
- Delete: web/tests/api/taskDemo.test.ts
- Modify: web/src/views/dashboard/index.vue (moved to web/src/modules/dashboard in Task 7)
- Modify: web/tests/views/dashboard/index.test.ts (moved to web/tests/modules/dashboard in Task 7)
- Modify: web/src/i18n/messages/zh-CN.ts
- Modify: web/src/i18n/messages/en-US.ts
- Modify: README.md
- Modify: docs/learning/admin-architecture-and-crud.md
- Modify: docs/database/2026-08-29-admin-modular-architecture.sql

**Interfaces:**
- Consumes: Task 1 migration's foundation_task preflight/drop and Task 5 operation log Worker.
- Produces: 无示例任务 API、无示例任务首页 UI、无 taskdemo 包；操作日志队列和 Worker 仍可独立运行。

- [x] **Step 1: 写删除后失败测试**

在 Dashboard 测试中删除示例任务 mock 后增加断言：页面不存在 data-testid=task-submit、task-panel 和示例任务标题，只保留健康状态、PostgreSQL/Redis readiness 和刷新行为；在 API composition 测试中请求 /api/admin/v1/example-tasks 必须返回 404。

- [x] **Step 2: 运行测试确认 RED**

    cd D:\admin\web
    pnpm vitest run tests/views/dashboard/index.test.ts --pool=threads --maxWorkers=1

    cd D:\admin\server
    go test ./cmd/api ./cmd/worker -run 'Test.*Example|Test.*Task' -count=1

预期：当前 Dashboard 和 API 仍渲染/注册示例任务，测试失败。

- [x] **Step 3: 删除后端和 Worker wiring**

从 API routerDependencies、run、buildRouter 删除 Task 字段、Repository、Service、Handler 和 route 注册；从 Worker 删除 taskdemo import、Processor 参数和 taskdemo.Register。operation log rules 删除 /api/admin/v1/example-tasks，不删除 queue client/server 或 operation log task。

- [x] **Step 4: 删除首页示例任务和文档示例**

从 Dashboard 删除 createExampleTask import、表单、task ID/error 状态和提交函数；保留真实依赖状态与 readiness chart。删除中英文 dashboard.exampleTask、dashboard.message、dashboard.submitTask、dashboard.taskId 和仅服务示例任务的文案。README 和学习文档删除 foundation-check、/example-tasks、foundation_task 以及“启动自动迁移/seed”说明，改为指向人工 SQL migration。

- [x] **Step 5: 运行删除范围测试与残留扫描**

    cd D:\admin\web
    pnpm vitest run tests/views/dashboard/index.test.ts tests/App.test.ts --pool=threads --maxWorkers=1

    cd D:\admin\server
    go test ./cmd/api ./cmd/worker -count=1

    cd D:\admin
    rg -n 'taskdemo|foundation_task|example-tasks|foundation-check' server web README.md docs/learning

预期：运行时代码、当前测试和当前文档无匹配；历史已完成 spec/plan 中的决策记录可保留，不得被机械删除。

- [x] **Step 6: 提交点（仅在维护者明确授权后执行）**

    git add server web README.md docs/learning/admin-architecture-and-crud.md docs/database/2026-08-29-admin-modular-architecture.sql
    git commit -m "refactor!: 删除示例任务并保留操作日志 Worker"

提交正文必须说明 /api/admin/v1/example-tasks 被移除、foundation_task 由人工 migration 删除、queue 基础设施未删除、Worker 仍消费 operation log task。

### Task 7: 按领域树迁移前端 API、页面、路由和测试

**Files:**
- Move: web/src/api/user.ts -> web/src/api/user/account.ts
- Move: web/src/api/account.ts -> web/src/api/user/profile.ts
- Create: web/src/api/user/loginlog.ts
- Move: web/src/api/session.ts -> web/src/api/user/session.ts
- Move: web/src/api/menu.ts -> web/src/api/rbac/menu.ts
- Move: web/src/api/role.ts -> web/src/api/rbac/role.ts
- Move: web/src/api/access.ts -> web/src/api/rbac/access.ts
- Move: web/src/api/auth.ts -> web/src/api/auth/login.ts
- Move: web/src/api/auth-platform.ts -> web/src/api/auth/platform.ts
- Move: web/src/api/operation-log.ts -> web/src/api/audit/operationlog.ts
- Preserve: web/src/api/health.ts
- Move: web/src/views/account/users/index.vue -> web/src/modules/user/account/index.vue
- Move: web/src/views/account/profile/index.vue -> web/src/modules/user/profile/index.vue
- Move: web/src/views/account/sessions/index.vue -> web/src/modules/user/session/index.vue
- Create: web/src/modules/user/loginlog/index.vue
- Move: web/src/views/access/menus/index.vue -> web/src/modules/rbac/menu/index.vue
- Move: web/src/views/access/menus/filter-menu-tree.ts -> web/src/modules/rbac/menu/filter-menu-tree.ts
- Move: web/src/views/access/roles/index.vue -> web/src/modules/rbac/role/index.vue
- Move: web/src/views/access/roles/role-permission-matrix.ts -> web/src/modules/rbac/role/role-permission-matrix.ts
- Move: web/src/views/access/roles/components/RolePermissionDiffDialog.vue -> web/src/modules/rbac/role/components/RolePermissionDiffDialog.vue
- Move: web/src/views/access/roles/components/RolePermissionMatrix.css -> web/src/modules/rbac/role/components/RolePermissionMatrix.css
- Move: web/src/views/access/roles/components/RolePermissionMatrix.vue -> web/src/modules/rbac/role/components/RolePermissionMatrix.vue
- Move: web/src/views/access/auth-platforms/index.vue -> web/src/modules/auth/platform/index.vue
- Move: web/src/views/system/operation-logs/index.vue -> web/src/modules/audit/operationlog/index.vue
- Move: web/src/views/auth/login/index.vue -> web/src/modules/auth/login/index.vue
- Move: web/src/views/dashboard/index.vue -> web/src/modules/dashboard/index.vue
- Move: web/src/views/dashboard/components/ReadinessChart.vue -> web/src/modules/dashboard/components/ReadinessChart.vue
- Modify: web/src/router/index.ts
- Modify: web/src/router/access-routes.ts
- Modify: web/src/layout/components/AccessMenuNode.vue
- Modify: web/src/layout/breadcrumbs.ts
- Modify: web/src/permission.ts
- Modify: web/src/store/access.ts
- Modify: web/src/store/auth.ts
- Modify: web/src/i18n/messages/zh-CN.ts
- Modify: web/src/i18n/messages/en-US.ts
- Move tests to mirror target paths: web/tests/api/{user/account,user/profile,user/loginlog,user/session,rbac/menu,rbac/role,rbac/access,auth/login,auth/platform,audit/operationlog}.test.ts
- Move: web/tests/api/menu-fields.test.ts -> web/tests/api/rbac/menu-fields.test.ts
- Move view tests to mirror target paths: web/tests/modules/{user/{account,profile,loginlog,session},rbac/{menu,role},auth/{login,platform},audit/operationlog,dashboard}/index.test.ts
- Modify: web/tests/router/index.test.ts
- Modify: web/tests/router/access-routes.test.ts
- Modify: web/tests/layout/components/AccessMenuNode.test.ts
- Modify: web/tests/layout/breadcrumbs.test.ts
- Delete after import scan is clean: old files under web/src/api/{user.ts,account.ts,session.ts,menu.ts,role.ts,access.ts,auth.ts,auth-platform.ts,operation-log.ts}, web/src/views/{account,access,auth,system,dashboard} and their old mirrored tests

**Interfaces:**
- Consumes: Task 3/4/5 的稳定 HTTP DTO、/api/admin/v1 管理 URL、/api/v1/auth 和 /api/v1/access 共享 URL。
- Produces: web/src/modules 与后端领域同名的页面树，web/src/api 与表/领域同名的业务 API 文件，view -> api -> utils/request.ts 的单向数据流。

- [x] **Step 1: 写目标目录和 componentPath 映射测试**

在 web/tests/router/access-routes.test.ts 先增加映射断言：旧数据库中的 account/users、account/profile、account/sessions、access/menus、access/roles、access/auth-platforms、system/operation-logs 必须解析到新的 modules/user/account、modules/user/profile、modules/user/session、modules/rbac/menu、modules/rbac/role、modules/auth/platform、modules/audit/operationlog 文件；未知 componentPath 仍抛 ProtocolError。这是显式映射表，不创建动态目录 Factory，也不修改已持久化菜单的 componentPath。

- [x] **Step 2: 运行前端定向测试确认 RED**

    cd D:\admin\web
    pnpm vitest run tests/router/access-routes.test.ts tests/router/index.test.ts --pool=threads --maxWorkers=1

预期：当前 glob 仍读取 src/views，新目录不存在，映射测试失败。

- [x] **Step 3: 移动现有 API 和页面并修复相对 import**

按 Files 清单执行 git mv，将 API 类型和函数随领域文件移动；所有 API 仍直接调用正确层级的 utils/request.ts，管理请求继续使用 /api/admin/v1，Auth/Access/Refresh 继续使用 /api/v1。页面、Pinia、permission、layout 和测试只更新 import 路径，不增加字段猜测、默认值或兼容 envelope。

更新 access-routes.ts 的 import.meta.glob 为 modules/**/index.vue，并实现以下显式 componentPath 映射：

    const componentPathMap: Readonly<Record<string, string>> = {
      "account/users": "user/account",
      "account/profile": "user/profile",
      "account/sessions": "user/session",
      "user/login-logs": "user/loginlog",
      "access/menus": "rbac/menu",
      "access/roles": "rbac/role",
      "access/auth-platforms": "auth/platform",
      "system/operation-logs": "audit/operationlog",
    }

moduleKey 先读取该表再拼接 modules/<mappedPath>/index.vue；静态 Dashboard 和菜单管理 route 的 path/name/meta 保持当前值。

- [x] **Step 4: 添加登录日志页面并接入既有 Admin 交互**

web/src/api/user/loginlog.ts 定义 LoginLogListQuery、LoginLogItem、LoginLogPage 和 getLoginLogPageInit/getLoginLogs，请求 /api/admin/v1/users/login-logs/page-init 与 /api/admin/v1/users/login-logs。页面使用现有 AppTable、Search、Element Plus 日期范围和空态，默认按 createdAt DESC 展示；不提供写按钮，不在前端补平台权限，不把 userAgent 或 reason code 当成可执行内容。只在 /access 返回含该 page 的菜单时由动态路由挂载。

- [x] **Step 5: 更新所有测试 import 和 fixture**

API 测试断言新文件仍发出原 HTTP URL；登录、refresh、access 测试继续断言 /api/v1；管理 API 继续断言 /api/admin/v1。所有页面测试移动到 tests/modules，mock 路径与生产 import 一致；删除 taskDemo.test.ts，Dashboard 测试只覆盖真实健康状态。

- [x] **Step 6: 运行前端模块定向测试**

    cd D:\admin\web
    pnpm vitest run tests/api tests/modules tests/router tests/layout tests/store --pool=threads --maxWorkers=1

预期：所有迁移后的 API、页面、动态路由、权限和布局测试通过，严格 TypeScript 不产生 any 或路径解析错误。

- [x] **Step 7: 提交点（仅在维护者明确授权后执行）**

    git add web/src web/tests
    git commit -m "refactor!: 按领域目录迁移 Admin 前端"

提交正文必须说明 API URL、route path/name、permission code、Element Plus locale、主题持久化和 RouterTabs 行为保持不变，只有文件归属和明确的登录日志页面新增发生变化。

### Task 8: 更新当前文档和人工验收手册

**Files:**
- Modify: README.md
- Modify: docs/learning/admin-architecture-and-crud.md
- Modify: docs/database/2026-08-29-admin-modular-architecture.md
- Verify only: docs/superpowers/specs/2026-08-29-admin-modular-architecture-design.md (status update is deferred to Task 9)
- Verify: docs/superpowers/plans/2026-08-29-admin-modular-architecture.md

**Interfaces:**
- Consumes: Tasks 1-7 的最终目录、SQL、路由和启动边界。
- Produces: 当前开发文档只描述新架构；历史已完成 spec/plan 作为决策记录保留，不被机械重写。

- [x] **Step 1: 更新 README 的启动与数据库说明**

删除示例任务 curl、foundation_task 介绍和“启动 API 自动迁移/seed”的表述；写明 API/Worker 启动前数据库必须已由维护者执行 migration，首次管理员由 go run ./cmd/bootstrap-admin 显式创建，健康检查仍为 /health 和 /ready，共享 Auth/Access 与 Admin 管理 URL 分属 /api/v1、/api/admin/v1。

- [x] **Step 2: 更新学习文档的模块链路**

删除 taskdemo 实体状态任务章节和示例 payload；用真实文件路径说明 user/account、user/profile、user/loginlog、user/session、rbac/*、auth/*、audit/operationlog 的线性链路、外键和日志规则。明确 COS、邮件、短信、AI、支付和 WebSocket 只在另开 spec 后实现，不在 Admin 创建空目录。

- [x] **Step 3: 完成文档关键词和链接检查**

    cd D:\admin
    rg -n 'server/internal/module/(user|account|auth|authplatform|authclient|authstate|menu|role|access|accessstate|operationlog|taskdemo)|src/views/(account|access|auth|system)|api/taskDemo|foundation_task|AutoMigrate|EnsureSchema|PrepareSchema' README.md docs/learning docs/database/2026-08-29-admin-modular-architecture.md
    git diff --check

预期：当前文档只保留新路径和“禁止运行时迁移”的说明；历史 spec/plan 不在本次搜索范围内，不因历史记录产生误报。

- [x] **Step 4: 保持 spec 状态未变**

Task 8 不修改 spec 顶部状态；只有 Task 9 的后端、前端、残留扫描和人工验收全部通过后，才执行 Task 9 的状态更新步骤。

- [x] **Step 5: 提交点（仅在维护者明确授权后执行）**

    git add README.md docs/learning/admin-architecture-and-crud.md docs/database/2026-08-29-admin-modular-architecture.md
    git commit -m "docs: 更新 Admin 模块化架构与迁移手册"

### Task 9: 全量验证、残留扫描和人工验收门禁

**Files:**
- Verify: server/**
- Verify: web/**
- Verify: docs/database/2026-08-29-admin-modular-architecture.sql
- Verify: README.md, docs/learning/admin-architecture-and-crud.md, docs/superpowers/specs/2026-08-29-admin-modular-architecture-design.md

**Interfaces:**
- Consumes: Tasks 1-8 的所有实现和测试。
- Produces: 可审查的验证证据、干净的当前路径扫描和人工验收清单；不创建空提交、不启动或停止用户服务。

- [x] **Step 1: 运行完整后端验证**

    cd D:\admin\server
    go fmt ./...
    go vet ./...
    go test ./...
    go build ./...

预期：四条命令全部退出码 0；go fmt 只改变本计划涉及的 Go 文件。

- [x] **Step 2: 运行完整前端验证**

    cd D:\admin\web
    pnpm vitest run --pool=threads --maxWorkers=1
    pnpm build

预期：所有 Vitest 通过，vue-tsc strict 和 Vite production build 通过。

- [x] **Step 3: 执行当前代码残留扫描**

    cd D:\admin
    rg -n 'taskdemo|foundation_task|example-tasks|auth_session|authplatform|authclient|authstate|module/(menu|role|access|accessstate|operationlog)|src/views/(account|access|auth|system)|api/taskDemo' server/cmd server/internal web/src web/tests README.md docs/learning
    rg -n 'AutoMigrate|PrepareSchema|EnsureSchema|EnsureCanvasPreset|EnsureFoundation|EnsurePlatformFoundation|EnsureSystemRoles|ClearBuiltinPolicies|CleanupLegacySessionPointers' server/cmd/api server/cmd/worker
    rg -n '/api/admin/v1/(auth|access)|/api/v1/(auth-platforms|menus|roles|users|sessions|operation-logs|example-tasks)' server web/src web/tests README.md docs/learning
    rg -n 'platform_id|PlatformID|platformId' server/internal/module web/src/api web/src/modules
    git diff --check
    git status --short

预期：第一条无当前运行时代码残留；第二条无 API/Worker 生产入口残留；第三条只保留共享 Auth/Access 的正确 /api/v1 和 Admin 管理资源的正确 /api/admin/v1，不注册错误 namespace；第四条显示所有数据库/Go/HTTP 边界的 platform_id 语义；diff 检查通过，工作区只包含本计划和维护者明确保留的改动。

- [x] **Step 4: 执行人工验收清单**

1. 在备份后的现有 PostgreSQL 上手工执行 migration，确认用户、角色、菜单、授权、平台、会话和历史操作日志 ID/计数保留；未知平台映射失败时整批回滚。
2. 重启 API 和 Worker，确认不会产生 DDL、seed、回填、foundation 删除或 Redis 清理；启动只在依赖可用时成功。
3. Admin 邮箱密码登录、失败登录、refresh、logout、个人资料和密码修改可用；user_login_log 能区分 login success、login failure、logout，refresh 不产生任何日志。
4. 同一账号拥有多个全局角色时，/api/v1/access 按当前平台合并去重；超级管理员获得当前 Admin 全部有效菜单，但看不到 Canvas 或其他平台权限。
5. 菜单、角色、用户、平台、个人资料、密码和会话踢除等 Admin 管理变更进入 audit_operation_log；注册、登录、refresh、logout 不进入 operation log。
6. 首页没有示例任务，Worker 仍能消费 operation log task；前端中英文、主题持久化、RouterTabs 全屏后仍可见且可退出全屏。

- [x] **Step 5: 验证通过后更新 spec 状态**

确认 Steps 1-4 的命令和人工验收都有证据后，把 docs/superpowers/specs/2026-08-29-admin-modular-architecture-design.md 顶部状态从“设计已获业务确认，等待实现计划”改为“已实施并完成验证”，并在文档末尾记录 migration 文件、后端/前端目标树、taskdemo 删除和全量验证命令。若任一验证失败，保留原状态并记录失败原因，不提前宣称完成。

- [x] **Step 6: 最终差异审查和交付**

    cd D:\admin
    git status --short
    git diff --stat
    git diff --check

审查清单：没有密钥、.env、构建产物、临时 SQL 输出或无关格式化；没有回退其他开发者改动；没有修改历史提交；所有破坏性 API/目录/表名变化都在对应提交正文和 spec 中写清迁移方式。只有在维护者明确授权后，才按单一目的拆分提交并使用 <type>: <中文摘要> 标题。

---

## Execution Handoff

Plan complete and saved to docs/superpowers/plans/2026-08-29-admin-modular-architecture.md. Two execution options:

1. **Subagent-Driven (recommended)** - 每个任务派发新的 subagent，并在任务之间进行两阶段 review。
2. **Inline Execution** - 在本会话使用 superpowers:executing-plans，按任务批次执行并在检查点停下复核。

本计划已按 Inline Execution 执行完成。数据库 migration 已在真实本地 PostgreSQL 备份后幂等执行，API/Worker 不包含运行时 schema/data mutation；后端、前端和残留扫描均已完成验证。后续业务模块（COS、邮件、短信、AI、支付、WebSocket 等）另开 spec，不在本计划范围内。
