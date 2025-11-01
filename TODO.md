# TODO 列表

## Prompt 工作台生成配置

- [ ] 前端：在 Prompt 工作台新增“生成配置”弹窗，允许用户开关逐步推理、调节温度、Top P、最大输出长度等参数，默认继承账号偏好，仅对当前 Prompt 生效。
- [ ] 后端：`prompts` 表新增 `generation_profile` TEXT 字段，以 JSON 形式记录生成参数并在生成流程中透传给模型 SDK，自适应逐步推理指令。
- [ ] 文档：更新 PRD、前端 README 与后端 README，说明新增生成配置项、默认值以及环境变量覆盖策略。
