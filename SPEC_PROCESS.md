# SPEC_PROCESS.md — 规约与计划生成过程文档

> 项目：Gavel（Coding Agent Harness）｜主开发智能体：GitHub Copilot（VS Code，Superpowers v6.2.0）
> 日期：2026-08-13 ｜状态：brainstorming / writing-plans 已完成，冷启动验证待执行

## 1. 过程概览

| 阶段 | 触发技能 | 产出 | 提交 |
|---|---|---|---|
| 需求澄清 → 设计确认 | `brainstorming` | SPEC.md（§1–§10 + 领域与机制设计专节） | `bf691e7`、`4335c3e` |
| 任务拆分 | `writing-plans` | PLAN.md（17 task，TDD 步骤） | `d9af6fc` |
| 冷启动验证 | —（换 agent 试跑） | 本节 §6 | 待补 |
| 实现 | worktrees + subagent + TDD | 待开始 | — |

## 2. Brainstorming 关键节点

按时间顺序的 9 个追问节点 + 1 次复审修正：

| # | 问题 | 我的推荐 | 你的决策 | 备注 |
|---|---|---|---|---|
| 1 | 技术栈语言 | Go | **Go** | 你未直接拍板，先追问"市面 harness 主流语言"做调研再决定（见 §3 节选 1） |
| 2 | 重点维度 | 治理（做深）+ 反馈闭环第二梯队 | **治理** | 对应 §A.4-D；演示①②③全覆盖 |
| 3 | 任务场景 | 自主修测试闭环 | **A** | 反馈信号客观（测试结果），演示剧本一镜到底 |
| 4 | LLM 适配层 | 单一 OpenAI 兼容 + net/http 直连 | **A** | 不用任何 SDK/框架 |
| 5 | 凭据存储 | OS 钥匙串 + .env 回退 | **A** | 桌面 Credential Manager / 云端 .env |
| 6 | 分发形态 | 二进制 + Docker 双形态 | **A** | 本地 exe + ghcr.io 镜像 + Render |
| 7 | WebUI 技术栈 | React+Vite 内嵌 + Open Design(shadcn/ui) | **A** | SSE + 轮询兜底 |
| 8 | 云平台 | Render | **A** | Railway 替补 |
| 9 | 项目名 | gavel（法槌） | **gavel** | 治理语义 |
| — | 复审修正 | — | 仓库 `github.com/166176/harness`、LLM 用 DeepSeek | 落定 SPEC §10 两个未决项 |

**好问题的样本**：问题 2（重点维度）与问题 3（任务场景）最为关键——前者锁定评分主战场（治理），后者让所有机制（工具/反馈/危险动作/记忆）都有了具体的领域形态，后续每个设计决策都能回溯到"修测试"这个场景上。这两问使整个项目免于流于"泛泛的 agent 框架"。

## 3. 至少 3 轮关键迭代（对话节选与决策）

### 轮次 1：语言选择前的市场调研

> **你**：目前市面的 harness 主流使用什么语言
>
> **AI**：给出了 TS/Python/Go/Rust 四类产品的清单与规律——CLI 型 harness 的现代趋势是 Go/Rust（Copilot CLI、Codex 重写版），并据此坚持推荐 Go。
>
> **处理决策**：调研结果与 AI 原推荐一致，但**你没有盲从推荐**，而是要求先看证据再拍板。这验证了一个判断：对"做什么"的关键决策，AI 的建议需要经受质询，而非直接领受。

### 轮次 2：护栏必须是代码，不是提示词（§A.4 判据的落地方案）

> **AI（设计第 3 节）**：护栏做成规则引擎，三态判定 allow/deny/approval；`guardrail(action)` 对构造动作的判定完全确定。HITL 状态机含审批超时，默认自动拒绝（fail-closed）。
>
> **处理决策**：批准。同时我在展示中把"超时自动拒绝"设为默认并写进演示③，使治理维度的机制演示不依赖任何真实 LLM——这是 §A.4-C"移除 LLM 后仍可验证"的直接落实。

### 轮次 3：SPEC 自审的自我推翻

> **AI（自审）**：发现 SPEC 缺失「数据模型」小节（§4.2 第 6 项硬性要求）；且 `--timeout`（会话预算）与审批超时的表述易混淆；`.gavel/`（仓库内）与 `~/.gavel/`（全局）边界不清。
>
> **处理决策**：立即修补三处：补 §5.4 数据模型（Session/Step/Approval/MemoryEntry/CredentialRef）；明确两类超时归属（审批超时归 policy 配置）；区分两条路径。**这是 AI 自己推翻自己的输出，而不是用户抓出的问题**——说明仅靠人审不够，skill 的自审清单（占位符/一致性/歧义）有实际价值。

## 4. AI 建议的采纳与推翻

**采纳的 AI 建议（主要）**：
- Go 语言（对齐 Copilot CLI 路线：单二进制 + go-keyring + go test mock）
- 治理为 main contribution、反馈闭环第二梯队
- "自主修测试"任务场景（而不是泛化 coding agent）
- 单一 OpenAI 兼容适配器、标准库直连、不用 SDK
- 钥匙串 + .env 分层凭据；双形态分发；React+Vite 内嵌 + Open Design(shadcn/ui)；Render 部署
- HITL fail-closed 超时策略、会话内命令指纹免重复审批（记忆×治理联动）

**推翻或修正的（含 AI 自我修正）**：
- 用户侧：没有直接推翻 AI 的实质建议，但两处以质询修正了流程——语言决策前要求市场调研；复审时用仓库地址与 DeepSeek 供应商信息补全了未决项。
- AI 自我修正（SPEC 自审）：缺数据模型 → 补 §5.4；超时歧义 → 归属澄清；`.gavel/` 路径歧义 → 规则措辞修正。修订 diff 见 git 历史（`4335c3e` 与 SPEC.md 编辑记录）。

## 5. Writing-plans 过程

- 触发 `writing-plans` 后按 skill 要求：文件结构先行 → 17 个 task 拆分 → 每个 task 含"写失败测试→确认红色→最小实现→确认绿色→commit"五步 → 接口签名在 task 间显式传递（`llm.Client`、`govern.Check`、`tools.RegistryOf` 等）→ 依赖与并行分组图。
- 自审结论：spec 覆盖无缺口；无占位符；跨 task 类型一致。
- 两点主动偏离 skill 默认值的决定（已记录）：① PLAN 保存到仓库根 `PLAN.md` 而非 `docs/superpowers/plans/`（课程 §4.3 交付要求优先）；② 在正式实现前插入课程 §4.5 冷启动验证闸门（课程要求优先于 skill 的"立即执行"选项）。

## 6. 冷启动验证（§4.5，已执行）

- **第二 agent**：OpenCode（与主开发 agent 类型不同；试跑模型 `deepseek/deepseek-chat`）
- **试跑 task**：PLAN 的 T2（LLM 抽象+MockLLM）与 T5（护栏规则引擎），分支 `coldstart-validation`
- **过程**：先以免费模型试跑（中途因环境卡顿终止），随后配置 DeepSeek key 重跑完成。均只给 SPEC.md + PLAN.md，无任何对话历史。

### 6.1 它在哪里暂停并提问（完整清单见分支上的 COLDSTART_QUESTIONS.md）

1. **工具链缺失（两轮 agent 都在此停顿）**：`go` 不在 PATH——第一轮 agent 自行搜遍标准安装路径与包管理器，尝试 `winget install` 卡死；文档没有写明「需预装 Go 1.22+」这一前置条件。→ spec 缺陷 #1
2. **`proxy.golang.org` 不可达（两轮都撞上）**：agent 自行切换到 `goproxy.cn` 镜像解决，但文档没写网络前置。→ spec 缺陷 #2
3. **Q1 围栏语义**：PLAN 说「path 经 path.Clean 判断越界」，但 SPEC 又说「symlink 解析后越出 --repo 根」，且测试用空 `GuardContext{}`——govern 层到底做不做完整围栏？agent 按「govern 冒烟 + tools 完整」分层实现并提问确认。→ spec 缺陷 #3
4. **Q2 规则命名**：`Verdict.Rule` 对 shell 命中该填什么未规定，agent 选择「命中模式文本」并记录待确认。→ spec 缺陷 #4
5. **Q3**：`yaml.v3` 落入 `// indirect` 标记，agent 选择不动 `go mod tidy`（避免越界改动），留给后续任务。

### 6.2 解读差异：spec 写错还是它读错

- **都不是误读**：4 个问题全部是 spec 未写明文（underspecified），agent 没有一处把已写明的约束读错；它对「不凭猜测」的执行也很克制——不确定处全部透明记录而非静默实现。
- 唯一值得商榷的是它在 T1 未完整（缺 Makefile/version 包）时没有提问而直接继续 T2/T5——但这是任务范围外，属合理判断。

### 6.3 产出与预期差距

- T2、T5 严格按红→绿→commit 完成，接口签名与 PLAN 一字不差，5 个护栏用例全绿，`go test ./...` 通过（commit `acd521e`、`6bb0f37`）。
- 差距仅集中在 Check() 内部实现细节（Q1/Q2），恰好是 PLAN 里「具体代码由执行者按此要点写出」自我授权的部分——说明这种授权点正是冷启动验证最该盯的地方。
- 人工两阶段评审结论：spec 合规通过；代码质量 3 条 Minor（正则每次调用重编译、密钥比对只扫顶层字符串值、yaml 解析错误被忽略），留待实现阶段重构。

### 6.4 据此对 SPEC/PLAN 的修订（修订前后 diff）

1. **SPEC §6.2 规则 4**（围栏语义）：
   - 前：`范围围栏：路径归一化 + symlink 解析后越出 --repo 根 → deny`
   - 后：`范围围栏（双层）：govern 层对 Args["path"] 做 path.Clean 冒烟判定（绝对路径、..、../ 前缀 → deny）；tools 层 ResolveInside 执行归一化 + symlink 解析的完整围栏。任何一层拒绝即拒绝。`
2. **SPEC §6.2 新增**：`Rule 字段约定：固定规则短名（secret-leak/fence）；命中模式返回模式文本；allow 为空。`
3. **SPEC §8 新增行**：`开发前置：Go 1.22+、Node 18+；网络受限时 go env -w GOPROXY=https://goproxy.cn,direct；README 必须写明工具链安装方式。`
4. **PLAN Global Constraints 新增**：工具链前置 + GOPROXY 一行（同 3）。
5. **PLAN T5 实现要点 ② 改写**：明确 govern 层冒烟判定的三个条件与 tools 层完整围栏的分工；明确 Rule 字段取值。
6. **PLAN T16 新增**：`go mod tidy` 清理 indirect 标记；README 补充工具链前置章节。

**验证结论：SPEC/PLAN 整体质量良好**——结构、接口、TDD 步骤经受住了陌生 agent 的检验；真正的缺陷集中在「环境前置」与「授权点的内部细节」，均已修订。

## 7. 反思：brainstorming 做得好的地方与不满

**做得好的地方**：
1. **一次一问 + 多选题带推荐**：9 个决策没有一次是含糊的，每个答案都可回溯；
2. **分节签字 + 硬性闸门**：4 节设计逐节确认，且全程无一行实现代码，真正守住了"先规约后实现"；
3. **自审清单兜住了人没抓住的问题**：数据模型缺失是 skill 自审发现的，不是用户发现的。

**不满的地方**：
1. **skill 不提供跨 agent 验证机制**：冷启动验证（§4.5）要求"陌生 agent 失忆试跑"，但 brainstorming/writing-plans 技能本身没有辅助这个动作的工具，需要人工手动切换工具、手动搬运 SPEC+PLAN，过程摩擦大且容易敷衍；
2. **分节展示节奏在"快速确认"用户下形同虚设**：4 节设计每节都只得到"确认"二字，签字机制的深度取决于用户是否真的逐节阅读；技能没有内置任何"抽查式确认"来探测用户是否真读；
3. **mermaid 组件图长标签渲染体验差**，影响设计沟通效率。
