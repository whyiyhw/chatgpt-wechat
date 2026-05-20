# CLAUDE.md

本文件给 Claude Code 在本仓库工作时使用。**所有协作者（人/AI）都应遵守这里的约定**，与 `doc/PRD-refactor.md` 配合使用。

---

## 1. 项目定位（重要，影响所有判断）

chatgpt-wechat 正在从 **"全栈 AI Bot 平台"** 重构为 **"企微 AI 网关 + AIBackend 适配后端"**。

- **保留护城河**：企微（应用 + 客服 + 多渠道）、状态化会话、流式分段、语音/图片消息适配、WebSocket 推送。
- **委托给后端**（Dify / OpenClaw 等）：Bot 编排、Prompt 工程、知识库、Agent / 工作流、工具调用、绘画。
- **演进节奏**：v1.1 兼容期（双轨并行 + 标 Deprecated）→ v1.2 清理期（移除自研 AI 层）。

完整重构方案见 `doc/PRD-refactor.md`，产品全貌见 `doc/PRD.md`。

### 1.1 新增功能前先回答两个问题

1. **这是企微侧的还是 AI 大脑侧的？** AI 大脑侧默认**不在本项目做**，引导到 Dify / 后端。
2. **会不会增加要在 v1.2 删掉的代码？** 比如新增"模型 Provider 直连"，默认不收，让 AIBackend 接口收口。

---

## 2. 仓库结构

- `chat/` — 主服务（go-zero）
  - `chat/service/chat/api/` — HTTP API 服务，`.api` 文件定义路由与类型
  - `chat/service/websocket/` — WebSocket 推送服务
  - `chat/common/` — 公共能力（openai / gemini / deepseek / dify / draw / wecom / **aibackend**…）
  - `chat/go.mod` — `module chat`，go 1.24.1，go-zero v1.8.1（v1.1 RC 阶段需升至 v1.10.x）
- `plugins/` — 自研插件（v1.2 计划移除，由 Dify Tools 替代）
- `doc/` — 产品文档、PRD、架构图

---

## 3. go-zero 强制约定

### 3.1 改接口必须走 goctl 流程，**禁止手改生成产物**

修改 HTTP API 时：

1. **先改 `.api` 文件**（`chat/service/chat/api/*.api`）
2. **再 `goctl api go -api ... -dir ...` 重新生成**
3. 业务逻辑只写在手写区（`*logic.go`），生成区（`routes.go`、`types.go`、`handler/*.go` 的注册部分）一律不手改。

> **Why**：手改生成产物会在下一次 goctl regen 时被覆盖；本仓库已有过此类被覆盖的事故。
> **官方 skill 已装**：`~/.claude/skills/zero-skills/`，遇到 go-zero 相关任务优先调用。

### 3.2 v1.1 起 go-zero 升级

v1.1 RC 的 PR #0 = `go get -u github.com/zeromicro/go-zero@latest` + `goctl@latest` + **一次全量 regen**。重构所有其它工作都基于这个升级后的基线。

---

## 4. 可观测性

**统一使用 go-zero 自带的 OpenTelemetry + Prometheus**，**不要自建** trace_id 字符串方案或手写中间件。

- 配置：yaml 里加 `Telemetry` / `Prometheus` 块 + config struct 同步即可。
- WeCom 入口两端（HTTP / 客服回调）写好 propagator，让 trace 贯穿全链路。
- 自定义业务指标可加，但走 go-zero 的 metric API。

> **Why**：框架自带方案足够覆盖需求；自建会重复造轮子，且无法与 go-zero 生态（k8s probe、监控面板模板）联动。

---

## 5. AIBackend 接口（v1.1 核心新增）

位于 `chat/common/aibackend/`（PR #1 已搭骨架）：

- `backend.go` — `AIBackend` interface
- `types.go` — `ChatReq` / `ChatResp` / `StreamChunk` / `FileRef` / `Usage`
- `registry.go` — `Register(typeName, Factory)` + `New(typeName, cfg)`，各实现包在 `init()` 自注册
- `errors.go` — `ErrBackendUnavailable` / `ErrRateLimited` / `ErrBadRequest` / `ErrUnauthorized` / `ErrConversationNotFound` / `ErrNotImplemented` + `IsRetryable()`

### 编写适配器的硬性要求

1. **错误必须映射到上面 6 类标准错误之一**（用 `errors.Join` / `fmt.Errorf("%w", ...)` 包装），否则网关的主备切换、重试、限流逻辑会失效。
2. **`ChatStream` 返回的 channel 由实现方负责关闭**；调用方靠 `ctx.Done()` 中断。
3. **配置反序列化由实现方负责**：Factory 收到的是 `map[string]any`，自己断言/解析。
4. **不要在适配器里塞业务逻辑**（Prompt 拼装、知识库召回、会话状态等），这些是后端的事。

---

## 6. v1.1 PR 拆分（按顺序，互相依赖）

| PR | 范围 | 状态 |
| --- | --- | --- |
| #0 | go-zero / goctl 升级 + 全量 regen | 未开始 |
| #1 | `chat/common/aibackend/` 接口包 | 骨架已写，未提交 |
| #2 | Dify 适配器 `chat/common/aibackend/dify/` | 未开始 |
| #3 | OTel + Prometheus 接入 | 未开始 |
| #4 | 配置层加 `AIBackend.Default / Backends / Routes` | 未开始 |
| #5 | `chatlogic.go` 的 `case "dify":` 切到 `svc.AIBackend.ChatStream(...)` | 未开始 |
| #6 | 双轨开关 + `RolloutPercent` 灰度 | 未开始 |
| #7 | Deprecated Header + 启动日志告警 + CHANGELOG | 未开始 |
| #8 | 性能压测（k6/wrk）+ SLO 达标 | 未开始 |

每个 PR 都应单独提交、单独 review，**不要合并 PR**。

---

## 7. 不做清单（v1.1/v1.2 都不做）

- 复刻 Dify 的知识库 / 工作流 / Agent / Tools
- 新增 LLM Provider 直连（除非作为 fallback）
- 插件市场 / 应用商店
- 自研绘画引擎
- 语音 ASR/TTS 自研（保留现有 AMR ↔ 文本即可）
- 多语言 i18n

收到此类需求时，提醒用户它在 §1 边界外。

---

## 8. 编码风格

- Go 代码：遵循 go-zero 既有风格，错误处理用 `errors.Is/As`，避免裸 `panic`（包初始化的 `Register` 除外）。
- 注释：只在 **WHY 不明显**时写；不重复 WHAT；不要写"为 issue #xxx 加的"这种会随时间腐烂的描述。
- 测试：核心接口 + registry + 错误映射 必须有单元测试。集成测试用真实 Dify 测试实例，**不要 mock 后端 HTTP**。

---

## 9. 参考

- `doc/PRD.md` — 产品全貌
- `doc/PRD-refactor.md` — v1.1/v1.2 重构 PRD（含 SLO、迁移路径、PR 拆分详细说明）
- `~/.claude/skills/zero-skills/` — 官方 go-zero AI 知识库 skill
