# 菜单 Icon 契约审计

- 当前前端 `web/src/icons/menuIcons.ts` 与后端 `server/internal/module/permission/menu/tree.go` 各维护一份 38 项白名单，集合目前一致。
- 数据库现有菜单中允许 `icon IS NULL`，目录/页面是否必须 icon 需要按节点类型明确，而不是对所有菜单强制。
- Action 节点不应携带展示 icon；Directory/Page 的 icon 由 catalog 校验。
- 风险：新增图标需要人工同时改 Go/TS 两处，容易重现“后端拒绝整棵菜单树”。

## 目标

建立一个可生成/可校验的 icon catalog：

```text
catalog source -> Go allowlist + TypeScript render map + contract test
```

菜单写入、Access 快照和 IconSelect 必须共享同一份可验证集合；不做运行时动态 import 或前后端各自猜测。

