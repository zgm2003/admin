# Asynqmon 中文 UI

本目录固定来自 `github.com/hibiken/asynqmon` v0.7.2，用于本项目的任务队列只读页面。

- 上游版本：v0.7.2
- 上游仓库：https://github.com/hibiken/asynqmon
- 本地改动：源码构建入口增加固定界面文案的中文本地化脚本；不翻译队列名、任务类型、Payload 和服务端数据
- 运行时：dist 通过 Go `embed` 提供，不依赖工作目录或 Go module cache
- 只读：后端 `ReadOnly=true`，外层 Gateway 另行拒绝非 GET/HEAD/OPTIONS

升级 Asynqmon 时必须重新同步对应 tag，重新检查文案、接口路径和许可证，不得直接替换生成后的压缩 JS。

构建使用上游 UI 自带的 Yarn lock：

```powershell
cd server\internal\module\system\queueMonitor\ui\upstream
yarn install --frozen-lockfile
yarn build
```

构建后删除 source map，并将 build 复制到本模块 dist。`node_modules`、缓存和 source map 不得提交。
