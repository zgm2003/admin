# 前端质量与职责审计

审计范围：`web/src/api`、`web/src/views`、`web/src/components`、`web/src/store`、`web/src/router`。

## 已确认问题

### P1：公共 request 泛型是 DTO 绕过口

- 证据：`web/src/utils/request.ts:34-44` 的 `unwrapEnvelope<T>` 使用 `envelope.data as T`。
- 影响：调用方误用 `request<T>` 时，未经解析的 payload 会伪装为 DTO。
- 修复：请求层返回 `unknown`；API 模块必须显式 parser，或 request 接收 parser 函数。

### P1：通知 HTML 渲染边界缺少前端契约保障

- 证据：`NotificationCenterList/index.vue:89`、`NotificationTaskDetail/index.vue:121` 使用 `v-html`；API parser 只验证字符串。
- 后端当前有 `bluemonday` 内容净化和测试，因此暂定为契约回归风险，不直接认定为已发生 XSS。
- 修复：增加恶意标签/事件属性/危险 URL 的前后端契约测试，并明确 `contentHtml` 是已净化字段。

### P1：状态选项页面重复硬编码

- 证据：通知、通知任务、SMS、Scheduler 页面分别定义状态/场景字面量。
- 修复：固定状态由 domain enum metadata + i18n 生成 `{label,value}`；可运营选项走字典 options API。

### P2：AppSearch 类型边界松动

- 证据：`web/src/components/AppSearch/index.vue:48-51,72,122-139` 使用 `Record` 与多处强制断言。
- 影响：SearchField 的 key/value 关联在编译期丢失。
- 修复：建立字段 key 到模型值类型的映射，减少 `as TModel`。

### P2：Mail/SMS 聚合页重复编排

- 证据：`web/src/views/message/mail/index.vue` 与 `web/src/views/message/sms/index.vue` 重复维护 tab、loading、error、权限和请求序列。
- 修复：只抽取稳定、强类型的聚合 tab composable；不创建万能 Manager。

### P2：watch 主要是重复请求/副作用治理，不是自动泄漏

- 统计观察：约 33 个源文件使用 watch/effect；layout/profile/聚合页最集中。
- Vue setup 中同步创建的 watcher 会随组件卸载停止；需重点修复快速筛选并发、layout 身份变化重连和重复 immediate load。

### P2：icon catalog 前后端重复维护

- 证据：前端 `web/src/icons/menuIcons.ts` 与后端 `permission/menu/tree.go` 各维护 38 项，目前集合一致。
- 修复：建立单一 catalog 生成/契约校验，消除新增 icon 只改一侧导致整树拒绝。

