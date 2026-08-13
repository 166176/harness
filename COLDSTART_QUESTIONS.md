# COLDSTART_QUESTIONS.md

本文件由冷启动智能体在实现 PLAN **Task 2（internal/llm）** 与 **Task 5（internal/govern）** 期间创建，用于记录文档未明确、需由用户/维护者确认的不确定点。每个问题均注明：出处、我的解读、对验收的影响。

---

## Q1. `Check()` 中「越界围栏」的具体判定规则（PLAN Task 5，实现要点 ②）

- **出处**：PLAN.md Task 5 步骤 3 实现要点 ② 仅写「文件类工具的 path 经 `path.Clean` 判断越界 → `Deny{rule:"fence"}`」；SPEC §6.2 规则 4 提到「路径归一化 + symlink 解析后越出 `--repo` 根」。但任务 5 的测试 `TestEscapingWriteIsDenied` 传入的是空的 `GuardContext{}`（`RepoRoot` 为空），无法用「相对 `RepoRoot` 判断越界」的方式判定。
- **我的解读**：govern 层做的是**仅基于路径自身**的简化围栏：对 `Args["path"]` 取 `path.Clean`，若结果为绝对路径、或等于 `..`、或以 `../` 开头，则判 `Deny{rule:"fence"}`。真正的「repo 根 + symlink 解析」完整围栏位于 tools 层 `ResolveInside`（Task 3），不在本任务范围内。
- **影响**：满足测试；完整围栏的正确性依赖 Task 3，需用户确认这种「govern 层仅做路径冒烟判断、tools 层做完整围栏」的拆分是否符合文档意图。

## Q2. shell 命令命中危险规则时 `Verdict.Rule` 的取值约定

- **出处**：PLAN.md Task 5 实现要点只在 ①（`"secret-leak"`）和 ②（`"fence"`）给出了明确 rule 名称；对 shell 的 deny/approval 命中，未规定 `Rule` 字段应填什么。测试仅断言 `Decision` 语义，未断言 `Rule` 的取值。
- **我的解读**：为便于审计追溯，shell 命中断言/批准规则时，将 `Rule` 设为**命中的那条正则模式字符串**（deny 命中 deny_patterns 的文本；approval 命中 approval_patterns 的文本）；`Allow` 时 `Rule` 为空字符串。
- **影响**：符合文档且不违背任何测试；但 rule 命名约定是执行者主观选择，请确认是否需要统一的短命名（如 `dangerous-command` / `network`）。

## Q3. `go.mod` 中 `gopkg.in/yaml.v3` 被标记为 `// indirect`

- **出处**：PLAN.md 外部依赖声明该库用于策略解析。用 `go get gopkg.in/yaml.v3@v3.0.1` 安装后，`go.mod` 首次写为 `require gopkg.in/yaml.v3 v3.0.1 // indirect`（因为安装时 go.mod 尚未进入代码引用阶段）。
- **我的解读**：`go mod tidy` 在代码正式 import 该包后会把它转为直接依赖、去掉 `// indirect` 标记。我未在本次提交中执行 `go mod tidy`（避免改动超出本任务范围的文件行）。
- **影响**：无功能影响，构建/测试均通过；建议后续任务（或由维护者）运行一次 `go mod tidy` 清理该标记。

## Q4. 网络代理可用性差异（非代码问题，仅记录）

- **出处**：执行 `go get` 时默认代理 `proxy.golang.org` 连接超时，改用 `goproxy.cn` 成功拉取 `yaml.v3 v3.0.1`。
- **记录**：已通过 `go env -w GOPROXY=...` 修改全局 Go 配置。请确认该修改是否应保留，或恢复为官方默认代理（`go env -w GOPROXY=https://proxy.golang.org,direct`）。

---

> 注：以上 Q1–Q3 我均按「不阻塞任务完成、且不改变任何测试断言结果」的原则实现并做了透明记录；请维护者复核后告知是否需要调整。Q4 为环境侧记录。
