# 系统字典管理设计

## 目标

在“系统设置”下增加全局字典管理能力，为可配置业务选项提供统一的类型、选项、多语言标签、启用状态和排序；前端通过接口生成 `el-select-v2` options，后端对业务输入执行字典项校验。

## 边界

字典管理的是“业务可配置选项”，不是动态数据库字段、低代码表单、任意 JSON Schema，也不是程序协议 enum。

保留在 Go/TypeScript 代码中的协议 enum 包括 `YesNo`、权限节点类型、登录方式、邮件发送状态、API 错误码等。它们参与程序分支、数据库约束或协议兼容，不能由管理员删除或改变。

字典是全局共享资源，不包含 `platform_id`。认证平台拆分只适用于 COS 上传策略、邮件限流策略和登录 Session；不扩展到字典、普通用户资料选项或邮件模板配置。

## 领域模型

`system_dictionary` 是字典类型，字段包括：

- `id`：内部 ID；
- `code`：全局唯一、稳定的 lower dot code，例如 `user.gender`；
- `name_zh`、`name_en`：管理展示名称；
- `description`：用途说明；
- `is_enabled`：是否允许业务查询；
- `is_builtin`：是否为系统内置字典；
- `created_at`、`updated_at`：非空 `TIMESTAMPTZ`。

`system_dictionary_item` 是字典项，字段包括：

- `id`：内部 ID；
- `dictionary_id`：所属字典；
- `value`：稳定业务值，按字符串传输，不能翻译；
- `label_zh`、`label_en`：多语言标签；
- `sort`：同一字典内的显示顺序；
- `is_enabled`：是否出现在业务 options；
- `is_builtin`：是否为系统内置项；
- `created_at`、`updated_at`：非空 `TIMESTAMPTZ`。

约束：启用字典只返回启用项；同一字典的有效项 `value` 唯一；字典 `code` 全局唯一；删除字典前必须没有字典项；内置字典和内置项不能删除；修改 `value` 或字典 `code` 必须禁止，避免破坏已有业务数据。

## 语言和查询协议

管理 API 返回 `nameZh/nameEn` 和 `labelZh/labelEn`，便于管理员维护。业务 options API 根据请求语言返回统一的 `{ label, value }`：

```http
GET /api/admin/v1/system/dictionary/options?codes=user.gender,notification.type
```

```json
{
  "code": 0,
  "data": {
    "user.gender": [
      { "label": "男", "value": "male" },
      { "label": "女", "value": "female" }
    ]
  },
  "message": "ok"
}
```

标签选择顺序：请求语言（`zh-CN`/`en-US` 归一化为 `zh`/`en`）-> 中文 -> 协议错误。不能以空标签、空数组或原始值掩盖缺失配置。未知 code、停用字典和无启用项按明确业务错误返回；批量查询不得静默丢掉其中一个 code。

## 管理 API

资源路径使用小写单数 HTTP 路径，代码模块使用 `system/dictionary`：

- `GET /api/admin/v1/system/dictionary`：分页查询字典类型；
- `GET /api/admin/v1/system/dictionary/:id`：字典详情及其项；
- `POST /api/admin/v1/system/dictionary`：创建字典；
- `PUT /api/admin/v1/system/dictionary/:id`：更新名称、描述；
- `PATCH /api/admin/v1/system/dictionary/:id/status`：启用/停用；
- `DELETE /api/admin/v1/system/dictionary/:id`：删除非内置且无项字典；
- `POST /api/admin/v1/system/dictionary/:id/item`：创建字典项；
- `PUT /api/admin/v1/system/dictionary/:id/item/:itemId`：更新标签、排序；
- `PATCH /api/admin/v1/system/dictionary/:id/item/:itemId/status`：启用/停用；
- `DELETE /api/admin/v1/system/dictionary/:id/item/:itemId`：删除非内置字典项；
- `GET /api/admin/v1/system/dictionary/options`：批量获取业务 options。

所有成功响应使用 `code/data/message` envelope；API 模块从 `unknown` 严格解析 DTO。

## 权限和菜单

新增页面节点：`system:dictionary:view`，菜单位于系统设置下，`is_hidden=0`。动作权限只声明真实能力：`list`、`detail`、`create`、`update`、`status`、`delete`。options 供已登录管理端页面消费，仍使用 `system:dictionary:list` 保护，不新增公开接口。每个 Handler 路由使用与动作相同的 `requirePermission` code；action 不进入 `menuTree`。

菜单、权限、i18n 和动态路由需保持 `system/dictionary` lower camel 模块身份。数据库菜单迁移保留现有 ID/授权并只对受影响平台递增 `menu_version`。

## 缓存与一致性

PostgreSQL 是字典事实来源。options 热路径使用 Redis 版本化快照，键至少隔离字典 code、语言和 schema/version；进程内缓存命中前必须确认 Redis 字典版本和 `ready` 状态。字典或字典项写事务提交前取得 tokenized mutation lease，提交后以 CAS 发布新版本；事务失败或发布失败必须显式报错，不返回旧快照或假成功。冷缓存重建必须有界，不能每请求回源 PostgreSQL。

第一期可以只缓存 options 查询，但必须在接口和 Service 中保留版本/失效边界，不能先做无失效的永久进程缓存。

## 前端消费

新增 `web/src/api/system/dictionary.ts` 严格解析管理 DTO 和 options envelope；新增受控的 dictionary Store/组合式函数负责按 code 加载、缓存、请求序列化和失败状态。页面只通过 Store 取得 options，再映射成明确类型的 `el-select-v2` options。基础管理页包含字典列表、字典详情/项列表、创建编辑、启停和删除确认，并区分 loading、空态、失败、提交中。

第一期不迁移现有业务页的硬编码 options；迁移作为后续独立任务，逐个确认哪些字段是业务字典、哪些仍是代码 enum。

## 验收重点

- 多语言标签和中文回退正确；
- `value/code` 稳定且不可被编辑破坏；
- 内置保护、有效项唯一性、删除前依赖检查正确；
- 页面/API 权限独立且后端路由受保护；
- Redis 故障、版本不一致、失效中、快照损坏均显式失败；
- 前端严格 DTO、批量 options 不丢项、架构门禁无问题；
- 管理页和 options API 可独立测试，现有页面行为不回归。
