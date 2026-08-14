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

## 实现阶段（2026-08-13，subagent-driven-development）

### [S4] 实现全流程总览
- **工作流**：using-git-worktrees（每模块一个 worktree 分支）+ subagent-driven-development（每 task 新鲜实现 subagent + 任务评审 + 修复轮次）+ TDD 强制 + SDD 台账（`.superpowers/sdd/PLAN.md/progress.md`）
- **最终状态**：17 task + F1 + 终审修复全部完成并合并 master；`go test ./...` 13 包全绿；全分支终审 R1 后 CLEAN
- **task→commit 映射** 见 PLAN.md「实现状态」表
- **修复轮次统计**：T3 R1（symlink 逃逸）、T4 R1（schema command）、T6 R1+R2（超时终态/panic）、T7 R1（pytest 回退）、T11 R1（nil fail-closed）、F1 R1（flaky 断言）、终审 R1（审批记忆双重追加）；每条均经定向复审 CLEAN
- **评审拦截的实质缺陷**（subagent 输出被评审修正的证据）：T3 symlink 逃逸、T6 Await 并发窄窗口+未知 id panic、T11 nil Guard fail-open、F1 首轮审批控制台失明、终审 C1/I1/I2、审批记忆双重追加

### [S5] 偏离记录（如实登记，课程 §3.6 允许在记录与解释前提下的偏离）
1. **subagent 网络中断 5 次**（T16 实现者断线 2 次、终审修复实现者断线 1 次、评审派发 ECONNRESET 1 次、空响应 1 次）：对策=断点核查 worktree 状态后派续作；T16 与终审 I1/I2 的收尾由**协调者直接补写代码并代提交**，随后均经独立 subagent 定向复审通过——违反 SDD「控制器不写代码」规范，但保证了交付完整性与复审监督。
2. **Open Design skill 未安装**：WebUI 采用 shadcn/ui 设计系统的手写等价组件（CSS 变量 + 组件结构遵循 shadcn 规范），SPEC §8 已声明 shadcn/ui 为所选设计系统；过程偏差为未走 Open Design skill 流程。
3. **GitHub PR 工作流本地化**：每模块 branch + merge commit 历史完整，但 PR 由本地 merge 代替（本会话无法创建 GitHub 平台 PR）；仓库历史中每个模块的 commits 与 merge 记录齐全，可作为 PR 等价证据。
4. **gofmt 全局收尾**由协调者执行（T1 登记的 deferred minor 闭环）。
5. **Git 提交身份**：孙其瑶（全局配置）；首个 commit 曾临时使用占位身份，随后已改用真实身份。

### [S6] 关键教训
- 写 PLAN 时把"授权点"（如"具体代码由执行者按此要点写出"）的验收标准写成可断言测试，能显著减少 subagent 的自由裁量与评审循环。
- 评审派发前必须把 diff 写入文件再交给评审 subagent——直接贴 diff 会污染上下文（本会话遵守）。
- 子代理中断恢复靠 worktree + git log 状态核查，台账是唯一可靠的事实源。
- 治理类机制（护栏/HITL）的"默认行为"必须 fail-closed 并配 nil 装配测试，两次评审都抓到真实漏洞。

### [S7] CI/CD 执行记录（交付清单第 7 条证据）
- **2026-08-14**：推送 NJU GitLab（`git@git.nju.edu.cn:166176/se-ai.git`，分支 `main`，commit `8cbb815`）后自动触发流水线。
- **流水线 #320986（最后一次执行）**：job `unit-test`（镜像 golang:1.24，`go test ./...`，GOPROXY=goproxy.cn）——**PASS（用户已确认）**。
- 说明：为防 runner 拉取依赖时 `proxy.golang.org` 不可达，`.gitlab-ci.yml` 已配置 `GOPROXY: https://goproxy.cn,direct`（本地冷启动验证时实证该域名不可达）。

### [S8] 云部署（2026-08-14，技能：verification-before-completion）
- **EnvProvider**：容器/云端无 keyring 与 `.env` 文件，新增 `internal/secret/env.go`（读 `GAVEL_API_KEY` 环境变量，只读），密钥链改为 keyring → `.env` → env（commit `91decad`，单测覆盖含 Chain 回退）。
- **GitHub CI 修复**：`.github/workflows/ci.yml` 触发分支原为 `master` 而分支是 `main` → CI 从未运行、GHCR 镜像从未构建；改为 `[main, master]`（commit `44f74e3`），推送后 CI 全绿：`unit-test` + `docker`（构建并推送 `ghcr.io/166176/harness:latest`）。
- **Dockerfile 大陆网络适配**：build 阶段加 `GOPROXY=https://goproxy.cn,direct`（默认 proxy.golang.org 不可达，冷启动已证）；运行基镜像由 `gcr.io/distroless` 改为 `alpine:3.20` + ca-certificates（gcr.io 大陆常被墙，Docker Hub 可镜像加速）。
- **部署路径决策**：GHCR 匿名查询返回 401 → 包默认私有；且大陆 ECS 直拉 ghcr.io 慢。选定「本地构建 → `docker save | gzip` → `scp` → `docker load`」离线传输，配套 `deploy/upload.ps1`（本机侧）+ `deploy/ecs-setup.sh`（ECS 侧：装 Docker → load → 校验 `/opt/gavel/.env`(0600) → `docker run -p 80:8080 --env-file --restart unless-stopped`）。
- **ECS**：`47.97.60.137`（阿里云，待用户输入 SSH 密码完成上传与启动）。
- **实例实况（2026-08-14 晚）**：实例 `i-bp1f9q3m4wrr7pi3jv51`，公网 IP `47.97.60.137` 确认无误，安全组 `sg-bp1f9q3m4wrr7phzssah`（cn-hangzhou）已放行 22/80 → `0.0.0.0/0`；但**操作系统为 Windows Server 2022**（非 Linux）。已确认本机出口 IP=`223.68.97.95` 且安全组含「所有流量 from 该 IP」规则，TCP 22/80 仍被拒 → 系 Windows 无 sshd 所致，非安全组问题。已构建 `gavel.exe`(windows/amd64) 与 `deploy/windows-setup.ps1`（计划任务 + `.env`），并补 `deploy/binary-setup.sh`(linux systemd)。README 云部署小节已改为「二进制优先 + 双系统脚本」。
- **本机 Docker 环境故障**：Docker Desktop 数据盘 vhdx 损坏（pull 全部 `short read EOF`），经 `wsl --unregister` + 删除 `%LOCALAPPDATA%\Docker\wsl` 重建后恢复；镜像拉取在后台验证中。ECS 部署已切二进制方案，Docker 仅保留为 CI/ghcr 容器分发产物。
- **部署成功（2026-08-14 晚，交付清单第 9 项达成）**：新实例 `i-bp1i8936zyi8gq862z74`（Alibaba Cloud Linux 3，公网 `47.97.30.54`）。SSH 密码认证被拒（实例为密钥对创建）→ 改用**阿里云 Workbench CLI**（`workbench` v1.0.0，经 OSS 内网中转，无需公网 SSH）：`workbench upload` 上传 `gavel-linux` + `deploy/binary-setup.sh`，`workbench exec`（`--output json` 规避 Windows 管道竞态 bug）执行 `binary-setup.sh`。服务 `gavel.service` Active(running)，`/opt/gavel/.env`(0600) 注入 `GAVEL_API_KEY`+`PORT=80`。公网验证：`http://47.97.30.54/` HTTP 200（WebUI）、`/api/key/status` 200（provider=chain, mask `sk-...7518`）、`/api/demo` 三场景全部 `pass:true`。
