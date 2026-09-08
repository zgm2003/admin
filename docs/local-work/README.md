# 本地 Spec / Plan 工作区

此目录用于维护者明确要求的临时 Spec、Plan、审查清单和跨会话施工记录。

规则：

- 除本 README 外，目录内容由根 `.gitignore` 忽略，不提交、不进入发布产物。
- Agent 默认不扫描、不读取、不引用本目录；只有维护者明确指定文件，或明确要求继续当前本地计划时读取。
- 稳定产品规则写入 `docs/agent/design.md`，稳定架构规则写入 `docs/agent/architecture.md`，实际进度与执行结果
  写入 `docs/agent/STATUS.md`；本目录不能替代这些事实入口。
- 临时计划完成并完成必要事实沉淀后，由维护者删除。不得让旧 Spec/Plan 指导未来任务。
- 文件建议命名：`YYYY-MM-DD-<topic>-spec.md`、`YYYY-MM-DD-<topic>-plan.md`。
