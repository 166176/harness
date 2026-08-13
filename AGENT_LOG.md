# AGENT_LOG.md — 智能体协作过程日志

> 项目：Gavel（Coding Agent Harness）
> 说明：按时间顺序记录关键节点。主开发 agent = GitHub Copilot（VS Code）。

## 2026-08-13

### [S0] 项目启动与 brainstorming（技能：brainstorming）
- **时间戳**：2026-08-13（会话内）
- **关键 prompt / context**：提供作业文件 `AI4SE_Final_Project_A_Coding_Agent_Harness.md`（项目 A），要求"根据流程启动项目"。主 agent 先探索工作区（无 git、仅作业文件），随后按一次一问推进。
- **关键节点**：9 个决策问题全部由用户确认：Go / 治理做深 / 自主修测试 / OpenAI 兼容适配器 / keyring+.env / 二进制+Docker / React+Vite+shadcn / Render / gavel。
- **人工干预**：用户要求先调研"市面 harness 主流语言"再决定语言；复审时提供仓库 `github.com/166176/harness` 与 DeepSeek 供应商。
- **产出**：`SPEC.md`（commit `bf691e7`，复审修订 `4335c3e`）。
- **教训**：用户对关键决策会先质询后拍板——推荐必须附带证据，不能只给结论。

### [S1] 实现计划（技能：writing-plans）
- **时间戳**：2026-08-13
- **关键 prompt / context**：以 SPEC 为唯一输入，按 skill 的 task 模板（文件/接口/TDD 五步/commit）生成。
- **产出**：`PLAN.md`（17 task，commit `d9af6fc`）。
- **人工干预**：偏离 skill 默认两点——PLAN 存仓库根（课程要求）；实现前插入 §4.5 冷启动验证闸门。
- **教训**：接口签名（`llm.Client`/`govern.Check`/`tools.RegistryOf`）在 task 间显式传递，是防 subagent 跑偏的关键。

### [S2] 过程文档
- **时间戳**：2026-08-13
- **产出**：`SPEC_PROCESS.md`、本文件。
- **备注**：冷启动验证部分待 OpenCode 试跑后补全。

### [S3] 环境配置 + 冷启动验证（§4.5）
- **时间戳**：2026-08-13
- **人工干预**：安装 Go 1.26.5 到 `D:\tools\go`（winget 网络失败，改用 golang.google.cn 镜像 zip）并写入用户 PATH；持久化 `GOPROXY=https://goproxy.cn,direct`；创建 `coldstart-validation` 分支；将用户提供的 DeepSeek key 写入 `~/.config/opencode/opencode.json`（直接改文件，未进终端历史）。
- **第二 agent**：OpenCode（deepseek-chat），全新会话，仅凭 SPEC+PLAN 完成 T2/T5：commit `acd521e`、`6bb0f37`、提问清单 `36ebfed`。
- **验证发现**：4 个 spec 缺口（工具链前置 / GOPROXY / 围栏语义 / 规则命名），已修订 SPEC §6.2/§8 与 PLAN（详见 SPEC_PROCESS §6.4）。
- **教训**：冷启动暴露的全是「文档没写明的环境前置」与「授权点内部细节」——写 PLAN 时把"具体代码由执行者按此要点写出"这类授权点的验收标准写得更死，能显著减少陌生 agent 的停顿。

## 待办日志槽位（实现阶段将逐 task 填充）
- 每个 task：时间戳、task 编号、触发技能、subagent 输出片段/commit hash、人工修改内容与理由、教训。
