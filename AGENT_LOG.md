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

## 待办日志槽位（实现阶段将逐 task 填充）
- 每个 task：时间戳、task 编号、触发技能、subagent 输出片段/commit hash、人工修改内容与理由、教训。
