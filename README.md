# gavel — AI 修复 agent 治理脚手架（Coding Agent Harness）

> AI4SE 期末项目 A。核心等式：**Agent = LLM + Harness**。本仓库自研 agent 主循环、Mock-LLM 抽象层、工具分发、治理护栏、人工审批（HITL）与反馈闭环，做到**移除真实 LLM 后机制仍可被确定性单测验证**（`go test ./...` 全程离线）。

## 简介

gavel（法槌）是一个“自主修测试”场景的编码 agent 脚手架：给定一个仓库、一条测试命令与一个任务描述，agent 循环执行「LLM 决策 → 工具调用 → 运行测试 → 结果回灌」直至测试全绿或预算耗尽；危险动作（如 `rm -rf`、强制 push）进入人工审批队列，由 WebUI 审批控制台实时批准/拒绝。

- **LLM 抽象**：`llm.Client` 统一接口；`MockLLM` 供确定性单测；`NewOpenAIClient` 直连任意 OpenAI 兼容 `/chat/completions`（默认 DeepSeek `deepseek-chat`），不依赖 SDK
- **工具分发**：`read_file` / `write_file` / `list_files`（路径围栏限定仓库内）+ `run_shell` + `run_test`
- **治理**：YAML 策略规则引擎（approval/deny 模式）+ HITL 审批状态机（pending/approved/denied/timeout，同会话单 pending，超时 fail-closed）+ 审批倒计时
- **反馈闭环**：测试输出回灌解析，会话状态 JSON 落盘（`internal/memory`）
- **凭据**：`go-keyring` 接系统凭据库（桌面）+ `.env` 回退（云端容器），仅掩码/指纹回显，绝不回显明文
- **WebUI**：React + Vite + Tailwind（shadcn/ui），构建产物 `go:embed` 内嵌，SSE 实时推送审批事件

## 安装

**工具链前置**

| 依赖 | 版本要求 | 说明 |
| --- | --- | --- |
| Go | 1.24+（测试用 `testing.T.Context`） | 官方安装包或 `winget install GoLang.Go` |
| Node.js | 18+（仅改前端时需要） | 前端产物已提交内嵌，日常开发无需 Node |
| GOPROXY | 建议国内设置 | `go env -w GOPROXY=https://goproxy.cn,direct` |

```bash
# 源码构建单文件二进制
go build -o gavel ./cmd/gavel

# 或直接安装
go install github.com/166176/harness/cmd/gavel@latest
```

前端产物位于 `internal/server/webui/dist`（git 跟踪，保证 `go:embed` 开箱可编译）。如需修改前端：

```bash
cd webui
npm ci
npm run build    # 产物输出到 internal/server/webui/dist
```

## 运行

```bash
# 1. 配置 LLM key（桌面：交互式隐藏录入，写入系统凭据库；容器：见下方安全边界）
gavel key set
gavel key status    # 只显示掩码与指纹

# 2. 自主修测试
gavel run --repo <仓库路径> --test "go test ./..." --task "修复失败的测试"

# 3. 审批控制台（REST + SSE + WebUI），打开 http://localhost:8080
gavel serve [--port 8080] [--host 地址]

# 4. 离线机制演示（纯 MockLLM，三场景：护栏拦截 / 反馈闭环 / HITL 超时）
gavel demo [--json]

# 5. 版本
gavel version
```

退出码：`0`=测试全绿，`1`=预算耗尽，`2`=人工终止，`3`=配置错误。

`gavel run` 可用 `--config <yaml>` 覆盖 base_url/model/max_turns/timeout_seconds/policy_path，用 `--policy <yaml>` 覆盖审批/拒绝正则（缺省 `approval_timeout_seconds` 会回退默认 300）。

## 分发

**Docker 镜像**（三阶段：node 构建前端 → golang 编译 → alpine 运行时）：

```bash
docker build -t ghcr.io/166176/harness .
docker run -p 8080:8080 -e GAVEL_API_KEY=sk-xxx ghcr.io/166176/harness
# 容器入口为 gavel serve，监听 8080；支持 $PORT 覆盖
```

**跨平台二进制**（goreleaser，windows/linux/darwin × amd64，免 tar 单文件）：

```bash
goreleaser release --clean
```

**托管 Release 链接（方案一）**：打 `v*` tag 自动触发 goreleaser 发布，二进制可直接下载：
<https://github.com/166176/harness/releases/tag/v0.1.0>（含 `gavel_*_windows_amd64.exe` / `_linux_amd64` / `_darwin_amd64` 与 `checksums.txt`）

**双形态架构（方案二，参照 llama.cpp 的 llama-cli + llama-server）**：

| 形态 | 命令 | 用途 |
| --- | --- | --- |
| CLI | `gavel run --repo <路径> --test <测试命令> --task <任务>` | 无头自主修复（批处理/CI） |
| WebUI | `gavel serve`（REST + SSE + 审批控制台） | 人工审批、会话监控、机制演示 |

线上 WebUI（交付清单第 9 项）：<http://47.97.30.54/>；本地启动：`gavel serve` 后打开 <http://localhost:8080>。

**CI**：`.github/workflows/ci.yml`（job `unit-test` = `go test ./...` + webui vitest；job `docker` = 构建并推送 `ghcr.io/166176/harness`；job `release` = 打 tag `v*` 时 goreleaser 发版）；`.gitlab-ci.yml`（job `unit-test` = golang:1.24 镜像跑 `go test ./...`）。

## 云部署（阿里云 ECS）

部署形态：**单文件二进制**（Linux systemd / Windows 计划任务），公网入口 `http://<ECS公网IP>/` → 安全组 80 → `gavel serve`（WebUI / REST / SSE，监听 `$PORT`）。镜像（`ghcr.io/166176/harness`，CI 自动构建）作为容器分发产物保留。

**CI/CD 策略**：GitHub Actions 自动构建镜像并推送 `ghcr.io/166176/harness`（发布流）；ECS 采用「本机交叉编译 → `scp` → 目标机运行」离线传输（大陆 ECS 直拉 ghcr.io 慢，二进制最稳、无需装 Docker）。

**Linux 系统**（推荐）：

```bash
# 本机：构建 linux/amd64 二进制并上传（SSH 密码在终端输入）
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -o gavel-linux ./cmd/gavel
scp gavel-linux deploy/binary-setup.sh root@<ECS公网IP>:/tmp/
ssh root@<ECS公网IP> "bash /tmp/binary-setup.sh"
# 服务器侧：校验二进制 → 校验 /opt/gavel/.env(0600, 含 GAVEL_API_KEY) → 写入 systemd 服务(PORT=80, 开机自启)
```

**Windows Server 系统**：

```powershell
# 本机：构建 windows/amd64 二进制并上传
$env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -o gavel.exe ./cmd/gavel
scp gavel.exe deploy/windows-setup.ps1 Administrator@<ECS公网IP>:C:/gavel/
# 服务器侧（管理员）：创建 C:\gavel\.env（GAVEL_API_KEY + PORT=80）后运行 windows-setup.ps1
# 脚本注册开机自启计划任务 "gavel"，本机自检 /api/key/status
```

- 部署脚本：`deploy/binary-setup.sh`（Linux systemd）、`deploy/windows-setup.ps1`（Windows 计划任务）、`deploy/upload.ps1` / `deploy/ecs-setup.sh`（Docker 方案备用）
- 密钥：目标机 `.env`（Linux `/opt/gavel/.env` 0600；Windows `C:\gavel\.env`）经环境注入，进程环境可见（明文风险见 SPEC §4.2）
- 备案：按 IP 访问无需 ICP 备案；若绑定域名则需备案
- 线上地址：**`http://47.97.30.54/`**（交付清单第 9 项；阿里云 ECS `i-bp1i8936zyi8gq862z74`，Alibaba Cloud Linux 3，systemd 服务 `gavel.service`，2026-08-14 部署验证通过）

## 目录结构

```text
.
├── cmd/gavel/                  CLI 入口（main → cli.Run）
├── internal/
│   ├── cli/                    子命令装配（version/key/demo/serve/run）
│   ├── config/                 YAML 配置加载 + 内置默认值
│   ├── core/                   agent 主循环与会话状态机
│   ├── demo/                   三场景机制演示（MockLLM，离线确定性）
│   ├── feedback/               测试输出回灌解析
│   ├── govern/                 护栏规则引擎 + HITL 审批状态机 + 默认策略
│   ├── llm/                    LLM 抽象（MockLLM + OpenAI 兼容适配器）
│   ├── memory/                 会话 JSON 存储
│   ├── secret/                 凭据（keyring / .env 回退，掩码回显）
│   ├── server/                 REST + SSE + 内嵌 WebUI（go:embed）
│   ├── tools/                  工具分发（文件三件套/run_shell/run_test，路径围栏）
│   └── version/                版本号（-ldflags 可覆盖）
├── webui/                      React + Vite + Tailwind（shadcn/ui）审批控制台
├── Dockerfile / .dockerignore  三阶段镜像构建
├── .goreleaser.yaml            三平台二进制发布
├── .github/workflows/ci.yml    GitHub Actions
├── .gitlab-ci.yml              GitLab CI
├── Makefile                    test / build / webui / docker
└── SPEC.md / PLAN.md / AGENT_LOG.md / SPEC_PROCESS.md   项目过程文档
```

## 安全边界

- **密钥绝不落日志、绝不回显明文**：`/api/key/status` 只返回 provider/mask/fingerprint；日志与错误信息不含 key；`.env` 以 0600 权限写入且被 `.gitignore` 排除
- **目标机密钥配置**：桌面端用 `gavel key set`（TTY 隐藏录入 → 系统凭据库）；容器/云端无 TTY，注入环境变量 `GAVEL_API_KEY`（如 `docker run -e GAVEL_API_KEY=...` 或 Render 的 Environment 变量注入）。运行时读取优先级：系统凭据库 → `.env` 文件 → `GAVEL_API_KEY` 环境变量（明文风险见 SPEC §4.2）
- **工具围栏**：文件工具路径解析后必须落在仓库根内（越界拒绝）；`run_shell`/`run_test` 在仓库根执行且有超时
- **护栏 fail-closed**：策略解析失败、未知错误一律拦截进入审批或拒绝；高危命令（`rm -rf`、`git push --force`、`curl|sh` 等）需人工批准；deny 模式（如 `chmod 777 /etc`）直接拒绝
- **审批接口无鉴权**：控制台定位为可信内网/本机；公网部署请在反向代理层加认证
- **监听范围**：`gavel serve` 默认监听全接口，请仅在可信网络使用，或 `--host 127.0.0.1`

### 已知限制

- **terminate 通道未实现**：会话级终止尚无独立 REST/CLI 通道
- **WebUI「拒绝并终止」仅拒绝当前动作**：后端 API 只支持 approved|denied，按钮语义与后端能力不符（见任务报告 concern）
- **Render 免费版冷启动**：约 15 分钟无流量会休眠，首次请求有启动延迟
- **二进制未签名**：goreleaser 产物未做代码签名，Windows SmartScreen 会拦截提示，需手动“仍要运行”

### 第三方依赖与许可证

| 依赖 | 许可证 | 用途 |
| --- | --- | --- |
| [zalando/go-keyring](https://github.com/zalando/go-keyring) | MIT | 系统凭据库存储 API key |
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | Apache-2.0 | 策略/配置 YAML 解析 |
| [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) | BSD-3-Clause | key 隐藏录入（TTY 无回显） |
| [shadcn/ui](https://ui.shadcn.com) | MIT | WebUI 组件（Open Design） |

### DeepSeek 真实 LLM 验收步骤

1. 配置 key：`gavel key set` 录入 DeepSeek API key（或容器 `.env` 注入 `GAVEL_API_KEY`）
2. 启动控制台：`gavel serve`，浏览器打开 `http://localhost:8080` 查看审批队列
3. 准备样例仓库：一个含失败测试的 Go 仓库
4. 发起修复任务：`gavel run --repo <样例仓库> --test "go test ./..." --task "修复失败的测试"`（默认 base_url=`https://api.deepseek.com`、model=`deepseek-chat`，可用 `--config` 覆盖）
5. 观察闭环：工具调用与测试输出回灌持续进行；危险动作经 CLI 交互 y/n（终端内）或 WebUI 控制台审批，批准/拒绝后流程继续
6. 终态校验：测试全绿（退出码 0）或预算耗尽（退出码 1）
7. 离线机制复核：`gavel demo` 三场景全 PASS（不依赖真实 LLM，回归机制本身）

#### 快速验收：testdata/sample-repo（真实 LLM，人工步骤，不进 CI）

仓库内置种子 bug 样例 `testdata/sample-repo/`（`calc.go` 的 `Add` 故意返回 2，`calc_test.go` 断言 `Add(1,2)==3`），可一键验证端到端修复闭环。先完成公共准备：

```bash
# 0. 确认种子 bug 存在（应 FAIL）
cd testdata/sample-repo && go test ./...    # 预期：--- FAIL: TestAdd

# 1. 录入 DeepSeek key（隐藏输入；容器则注入 .env / GAVEL_API_KEY）
gavel key set
```

审批通道二选一（§6.3 CLI 交互 或 WebUI 控制台）：

**路径 A：CLI 交互式 y/n 审批**

```bash
# 2. 在交互式终端（TTY）发起修复任务；命中危险动作时按提示输入 y/n
gavel run --repo testdata/sample-repo --test "go test ./..." --task "修复失败测试"
```

**路径 B：WebUI 审批控制台**

```bash
# 2. 启动控制台，浏览器打开 http://localhost:8080 查看审批队列
gavel serve

# 3. 经 REST API 发起会话（返回 202 + 会话 id；也可在控制台会话页发起）
curl -X POST http://localhost:8080/api/sessions -H "Content-Type: application/json" \
  -d '{"repo":"testdata/sample-repo","test":"go test ./...","task":"修复失败测试"}'

# 4. 在控制台批准/拒绝危险动作（SSE 实时推送审批），观察会话继续或终止
```

预期：会话从失败测试出发，经「LLM 决策 → 工具调用 → 测试回灌」闭环，最终修复 `Add` 使 `go test ./...` 全绿，会话状态走到 `completed`（退出码 0）；期间危险动作经所选通道审批（CLI y/n 或 WebUI 控制台）。此步骤需要真实 DeepSeek API key，属人工验收，不纳入 CI。
