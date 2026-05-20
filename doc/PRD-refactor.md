# ChatGPT-WeChat 重构 PRD（v1.1 / v1.2）

> 版本：v1.1（兼容期）→ v1.2（清理期）
> 起草日期：2026-05-19
> 关联文档：[PRD.md](./PRD.md)（产品全貌）、[CHANGELOG.md](./CHANGELOG.md)

---

## 0. TL;DR

**本次重构的一句话目标**：把项目从"全栈 AI Bot 平台"重构为 **"企微 AI 网关 + 后端 AI 服务（Dify / OpenClaw / 其它）的高性能适配层"**。

- **保留护城河**：企微（应用 + 客服 + 多渠道）、状态化会话、流式分段、语音/图片消息适配、WebSocket 推送。
- **委托给后端**：Bot 编排、Prompt 工程、知识库、Agent / 工作流、工具调用、绘画。
- **演进策略**：v1.1 双轨并行 + 标记 Deprecated；v1.2 移除自研 AI 层，完成网关化转型。

---

## 1. 重构背景与动机

| 现状问题 | 影响 | 重构后预期 |
| --- | --- | --- |
| 功能太分散：Bot / Prompt 优化 / 知识库 / 插件 / 绘画 / 多模型适配全部自研 | 维护成本高，单个模块都做不深 | 聚焦企微链路，AI 大脑交给专业后端 |
| Dify / OpenClaw / Coze 等编排平台已成熟 | 自研知识库 / 工具调用 / 工作流被快速反超 | 借力生态，作为这些平台在企微侧的"最后一公里" |
| 全栈方案性能不可量化 | 没有清晰 SLO，无法对企业用户承诺 | 把网关定位下，性能就可以做指标化（QPS / P95 / 转发延迟） |
| 模型 Provider 维护负担重 | 每出一个新模型都要适配 | 由后端服务统一收口，本项目只对接"AI 网关协议" |

---

## 2. 重构原则

1. **企微侧做深，AI 侧做薄**：自己只做企微生态里别人做不了的部分；AI 推理 / 知识库 / 工具一律委托后端。
2. **状态化网关 + 无状态适配器**：会话状态、消息编排留在网关；具体模型调用走纯适配器。
3. **优先稳定，其次性能，再次特性**：v1.1/v1.2 不做新业务功能，只做"砍掉 + 加固 + 提速"。
4. **可观测优先 + 用框架自带**：trace / metrics / logs 一律走 go-zero 框架自带能力（OTel + Prometheus + logx），不自建 trace_id 字符串方案。
5. **后端可插拔**：Dify 是首选，但协议层抽象出 `AIBackend` interface，OpenClaw / 自研 / 直连 OpenAI 都可挂入。
6. **不破坏现网用户**：v1.1 保持现有 API 完全可用 + 标 Deprecated；v1.2 提供迁移工具与至少 1 个版本的过渡期。
7. **严格遵循 goctl 生成**：任何 API 接口/字段变更先改 `chat.api`，再 `goctl api go` 重新生成 routes / types / handler 骨架；**禁止手改** `routes.go` / `types.go`，否则下次 regen 被覆盖。
8. **基础设施版本同步**：v1.1 RC 阶段把 go-zero 从 v1.8.1 升到当前最新（v1.10.x），并同步升级 goctl 后做一次全量 regen，作为重构的起点。

---

## 3. 目标架构

### 3.1 角色重新定位

```
┌──────────────────────────────────────────────────────────────┐
│                  企微 / 微信 / 客服渠道                       │
└─────────────────────────────┬────────────────────────────────┘
                              │ 应用消息 / 客服消息 / 回调
                              ▼
┌──────────────────────────────────────────────────────────────┐
│             ChatGPT-WeChat 网关（本项目，v1.2 形态）         │
│  ┌───────────────────────────────────────────────────────┐   │
│  │ 1. WeCom 协议层（多应用、多客服、签名、加解密）       │   │
│  │ 2. 消息适配层（文本/语音 AMR/图片/文件 ↔ 标准 Message）│   │
│  │ 3. 会话与状态（session、上下文窗口、用户路由）        │   │
│  │ 4. 流式分段编排（分段策略、首字加速、节流）           │   │
│  │ 5. 用户与租户（JWT、租户隔离、配额）                  │   │
│  │ 6. WebSocket / HTTP 推送通道                          │   │
│  │ 7. AIBackend 适配器接口（Dify / OpenClaw / OpenAI ...）│  │
│  │ 8. 可观测（trace、metrics、日志、慢请求采样）         │   │
│  └───────────────────────────────────────────────────────┘   │
└──────────────┬───────────────────────────┬───────────────────┘
               │ AIBackend 协议            │ AIBackend 协议
               ▼                           ▼
       ┌──────────────┐            ┌──────────────────┐
       │   Dify       │            │   OpenClaw       │  ... 其它
       │ App/Workflow │            │ App/Agent        │
       │ + 数据集     │            │ + 工具           │
       └──────────────┘            └──────────────────┘
```

### 3.2 网关核心职责（v1.2 终态）

| 能力 | 状态 |
| --- | --- |
| 企微应用消息回调（文本/语音/图片/事件） | **核心保留 + 加固** |
| 企微客服消息（多渠道：公众号/视频号/小程序/Web/App） | **核心保留 + 加固** |
| 多应用 / 多 AgentID / 多客服 KFID 路由 | **核心保留** |
| 语音 AMR ↔ MP3 ↔ 文本（多语言） | **核心保留** |
| 图片消息 → AIBackend 多模态调用 | **核心保留** |
| 会话状态（多会话切换、自适应上下文裁剪） | **核心保留 + 重构** |
| 流式分段写回（首字加速、防风控） | **核心保留 + 优化** |
| 用户身份 / JWT / 租户隔离 | **核心保留** |
| WebSocket / HTTP 推送 | **核心保留** |
| AIBackend 适配器接口 | **新增（v1.1）** |
| Trace / Metrics / 慢请求采样 | **新增（v1.1）** |

### 3.3 委托给后端（不再自己做）

| 能力 | 委托去向 | 备注 |
| --- | --- | --- |
| Bot / Prompt / 角色模板 | Dify App / OpenClaw App | 网关只透传 app_id 与会话 ID |
| 知识库（Knowledge / Unit / Segment） | Dify 数据集 / OpenClaw 知识库 | 本项目不再做向量库 |
| 工具调用（搜索 / Webhook / Shell） | Dify Tools / OpenClaw Tools | 插件机制完全下沉到后端 |
| 工作流 / Agent / 多步推理 | Dify Workflow / OpenClaw Agent | 网关不参与编排 |
| 绘画（SD / DALL-E / Midjourney） | 后端工作流内置或调用专门服务 | 网关只负责图片消息回写 |
| Prompt 优化 | 后端自带或第三方服务 | 不在网关层做 |
| 直连 OpenAI / DeepSeek / Gemini | 作为 AIBackend 的兜底实现（轻量） | 仅在后端不可用时作为 fallback |

---

## 4. v1.1（兼容期）

**主题**：**搭骨架、可量化、标 Deprecated**。在不破坏现有用户的前提下，建好新架构地基。

### 4.1 范围

#### 4.1.1 新增（必须）
- **AIBackend 适配器接口**：抽象出 `AIBackend` Go interface，提供 `OpenConversation / Chat / ChatStream / UploadFile / Health` 方法（详细契约见 §6.1）。Dify 实现作为 v1.1 的标杆实现。`EmbedFile`（数据集管理）与 `ListApps`（诊断）属于运维面，留待后续按需扩展，不进入 v1.1 核心契约。
- **Dify 集成升级为一等公民**：
  - 支持 Dify App ID 维度的会话映射（`bot_id ↔ dify_app_id ↔ conversation_id`）。
  - 支持 Dify Workflow 同步 / 异步两种模式。
  - 支持文件 / 图片上传到 Dify。
- **OpenClaw / 自定义 HTTP 后端协议**：定义一套与 Dify 兼容、可对接 OpenClaw 或自研服务的最小契约（见 §6）。
- **可观测基线（接入 go-zero 自带，不自建）**：
  - **Trace**：`etc/chat-api.yaml` 配置 `Telemetry` 块（Endpoint / Sampler / Batcher），业务侧用 `trace.TraceIDFromContext(ctx)` 取 ID。仅需手写两端的 propagator：企微回调入口把消息 ID / 时间戳映射为新 span；回写企微时把 trace_id 写入日志 / `ChatReq.Metadata`。
  - **Metrics**：`etc/chat-api.yaml` 配置 `Prometheus` 块；自定义指标用 `github.com/zeromicro/go-zero/core/metric` 的 `NewCounterVec` / `NewHistogramVec`：`ai_backend_request_total{backend,status}`、`ai_backend_duration_seconds{backend,phase}`（phase: decode / route / call / writeback）、`ai_backend_first_token_ms{backend}`。
  - **Logs**：统一 `logx.WithContext(ctx).Infow(...)`，trace_id 自动随日志输出。
  - 慢请求采样日志（>3s 全量记录请求/响应摘要），用 logx + sampler 实现。
- **配置层重构**：
  - 引入 `AIBackend.Default` 与 `AIBackend.Routes`（按 bot_id 路由到不同后端）。
  - 保留旧的 `ModelProvider` / `OpenAi` / `Gemini` 配置块，但标 Deprecated。

#### 4.1.2 加固（必须）
- **会话存储**：抽象 `SessionStore` 接口，默认实现走 Redis，去除散落各处的会话状态访问。
- **流式分段重构**：抽出 `StreamSegmenter` 模块，分段策略（按句号 / 字数 / 时间窗）可配置。
- **多应用路由**：把 `MultipleApplication` 的路由表统一收口到一个 router；当前代码里多处分散判断需要合并。

#### 4.1.3 标 Deprecated（不删，仅警告）
| 模块 | API / 文件 | 替代方案 | 移除版本 |
| --- | --- | --- | --- |
| 自研知识库 | `/api/knowledge/**`、`/api/knowledge/unit/**`、`/api/knowledge/segments/**` | Dify 数据集 | v1.2 |
| Prompt 优化 | `/api/bot/prompt/optimize` | 后端自带 | v1.2 |
| Bot 模板发现 | `/api/bot/explore/list` | 后端市场 | v1.2 |
| 自建插件协议 | `plugins/search`、`plugins/webhook` | Dify Tools | v1.2 |
| Stable Diffusion / DALL-E 直连 | `chat/common/draw` | 后端工作流 | v1.2 |
| OpenAI / Gemini / DeepSeek 直连 SDK | `chat/common/{openai,gemini,deepseek}` | 由 AIBackend 接口收口，旧实现降为 fallback | v1.2 |

**Deprecated 实施方式**：
- 在响应 Header 加 `Deprecation: true` 与 `Sunset: 2026-xx-xx`。
- 启动日志打印警告。
- README 与 CHANGELOG 显著提示。

#### 4.1.4 不做（明确划清边界）
- 不在 v1.1 移除任何已上线 API。
- 不重写前端 agent-web（仅在 v1.2 同步调整需要废弃的页面）。
- 不引入新的 LLM Provider 适配（新模型一律走 Dify / OpenClaw）。
- 不重构数据库结构（仅新增 trace / metrics 相关字段）。

### 4.2 v1.1 验收标准（DoD）

- [ ] `AIBackend` 接口落地，Dify 实现可承接 ≥95% 现有对话场景。
- [ ] 同一份配置可同时跑"旧 ModelProvider 直连"和"新 AIBackend 转发"两种模式，可热切换。
- [ ] Prometheus metrics 暴露 + Grafana 示例面板。
- [ ] 全链路 trace_id：日志可串起一条消息从企微回调到企微回写的全过程。
- [ ] 至少 1 个 Dify Workflow 样例 + 文档（含数据集、工具、工作流配置示例）。
- [ ] 单元测试：`AIBackend` 接口与 Dify 实现的核心路径覆盖率 ≥70%。
- [ ] 压测报告：单实例 ≥ 200 并发对话时 P95 < 800ms（不含后端推理）。
- [ ] CHANGELOG 与 Deprecation 公告齐全。

### 4.3 v1.1 性能 SLO

| 指标 | 目标 |
| --- | --- |
| 企微回调 → 后端首字（不含后端推理时间） | P95 < 300ms |
| 流式分段首段写回企微 | P95 < 1.5s（含后端首字） |
| WebSocket 推送时延 | P95 < 200ms |
| 单实例并发会话 | ≥ 200 |
| 错误率（5xx / 后端不可用） | < 0.5% |

---

## 5. v1.2（清理期）

**主题**：**完成网关化转型**。移除所有 Deprecated 模块，瘦身代码库，性能与可维护性双双提一个台阶。

### 5.1 范围

#### 5.1.1 移除（必须）
- 删除 §4.1.3 全部 Deprecated 模块（API + 代码 + 表结构 + 前端页面）。
- 移除自研向量库相关：
  - `chat/common/milvus` 整目录。
  - PostgreSQL 的 `pgvector` 字段（若仅用于自研知识库）。
  - 知识库 3 张表 + 关联表。
- 移除自研插件协议：`plugins/` 整目录或归档为示例。
- 移除自研绘画：`chat/common/draw` 与 `#draw:` 指令处理（绘画统一由后端 Workflow 完成）。
- 移除多 Provider 直连：`chat/common/{openai,gemini,deepseek}` 整合为最小 fallback 实现或彻底删除。

#### 5.1.2 重构（必须）
- **指令系统瘦身**：保留与"会话 / 系统 / 帮助"相关的指令，砍掉与 Bot / Knowledge / Plugin / Draw 相关的指令。
  - **保留**：`#help`、`#system`、`#clear`、`#session:*`、`#config_model:*`（仅切换后端 app_id）、`#exchange:*`。
  - **移除**：`#config_prompt:*`、`#prompt:list / set:*`、`#plugin:list`、`#draw:*`。
- **数据模型瘦身**：
  - `bot` 表只保留：`id / tenant_id / name / backend_type / backend_app_id / created_at`。
  - 移除 `knowledge_*`、`bot_prompt_*`、`bot_plugin_*` 等关联。
- **配置文件清理**：移除 `OpenAi / Gemini / DeepSeek / StableDiffusion` 等独立配置块；统一为 `AIBackend.*`。
- **前端 agent-web 同步**：删除知识库管理 / Prompt 优化 / 插件配置 / 绘画配置等页面，主页面改为"Bot ↔ 后端 App 绑定 + 客服渠道发布 + 监控"。

#### 5.1.3 新增（可选 / 增强）
- **限流 / 配额**：按租户 / Bot / 用户三级限流（QPS、日 token、并发数）。
- **多后端容灾**：单个 Bot 可配置主备后端（Dify 主 + OpenClaw 备 + 直连 OpenAI 兜底），按健康度自动切换。
- **冷启动加速**：后端会话 ID 预创建池；首条消息延迟下降。
- **回放与调试**：保存最近 N 条完整消息（含 trace），管理端可一键回放定位问题。

### 5.2 v1.2 验收标准（DoD）

- [ ] Deprecated 模块全部移除，二进制大小、镜像大小、go.mod 依赖均显著下降（目标各 ≥ 30%）。
- [ ] 数据库迁移脚本提供：旧表 → 新表，含数据清理（自研知识库数据无可迁移目标，导出为 JSON 归档后清理）。
- [ ] 单实例并发 ≥ 500 对话，P95 < 600ms（不含后端推理）。
- [ ] 主备后端自动切换演练通过：模拟 Dify 5xx 时自动降级到 OpenClaw / OpenAI。
- [ ] 全部接口的 OpenAPI 文档 / Postman 集合更新完毕。
- [ ] agent-web 同步发布对应版本。
- [ ] 旧版（v1.1）用户的升级文档 + 一键迁移脚本可用。

### 5.3 v1.2 性能 SLO

| 指标 | 目标 |
| --- | --- |
| 企微回调 → 后端首字 | P95 < 200ms |
| 流式分段首段写回企微 | P95 < 1.2s |
| WebSocket 推送时延 | P95 < 150ms |
| 单实例并发会话 | ≥ 500 |
| 错误率 | < 0.2% |
| 故障切换时延（主→备） | < 3s |

---

## 6. AIBackend 协议契约

### 6.1 接口定义（Go）

```go
type AIBackend interface {
    // 创建/恢复会话；返回后端侧的 conversation_id
    OpenConversation(ctx context.Context, req OpenConversationReq) (OpenConversationResp, error)

    // 非流式对话
    Chat(ctx context.Context, req ChatReq) (ChatResp, error)

    // 流式对话；通过 channel 返回分段
    ChatStream(ctx context.Context, req ChatReq) (<-chan StreamChunk, error)

    // 上传文件（图片 / 文档），返回后端文件 ID
    UploadFile(ctx context.Context, req UploadFileReq) (UploadFileResp, error)

    // 健康检查
    Health(ctx context.Context) error
}

type ChatReq struct {
    AppID          string            // 后端 App / Workflow 标识
    ConversationID string            // 后端会话 ID（可空，首次由 OpenConversation 创建）
    User           string            // 终端用户标识（透传给后端用于配额 / 标识）
    Inputs         map[string]any    // 工作流变量
    Query          string            // 文本内容
    Files          []FileRef         // 文件 / 图片引用
    Metadata       map[string]string // trace_id / channel / kf_id 等
}

type StreamChunk struct {
    Type           ChunkType      // text / image / tool_call / done / error
                                  // workflow_started / node_started / node_finished
                                  // workflow_finished / message / message_end
    Text           string
    ImageURL       string
    AudioURL       string
    ToolCallName   string
    ToolCallArgs   string
    MessageID      string
    ConversationID string
    TaskID         string
    Err            error
    Usage          *Usage         // 仅 done / workflow_finished 时携带
    Workflow       *WorkflowEvent // 仅 workflow_* / node_* 类型时非空
}

// WorkflowEvent 承载 Dify Chatflow / Workflow 类后端的节点/流程事件。
type WorkflowEvent struct {
    RunID       string
    NodeID      string
    NodeType    string
    Title       string
    Status      string         // succeeded / failed / running
    Outputs     map[string]any // 通常 Outputs["answer"] 是文本回复
    Inputs      map[string]any
    Error       string
    ElapsedTime float64
    TotalTokens int
}
```

### 6.2 配置示例

```yaml
AIBackend:
  Default: dify-main
  Backends:
    - Name: dify-main
      Type: dify              # dify | openclaw | openai | http
      BaseURL: https://dify.example.com/v1
      APIKey: ${DIFY_API_KEY}
      Timeout: 60s
      StreamTimeout: 120s
    - Name: openclaw-backup
      Type: openclaw
      BaseURL: https://openclaw.example.com/api
      APIKey: ${OPENCLAW_API_KEY}
    - Name: openai-fallback
      Type: openai
      BaseURL: https://api.openai.com/v1
      APIKey: ${OPENAI_API_KEY}
      Model: gpt-4o-mini
  Routes:
    # 按 Bot / 租户 / 渠道路由
    - Match: { bot_id: 1001 }
      Primary: dify-main
      Fallback: [openclaw-backup, openai-fallback]
    - Match: { tenant_id: 7 }
      Primary: openclaw-backup
```

### 6.3 Dify 实现要点
- `OpenConversation` → 调 Dify `POST /chat-messages` 时不带 conversation_id，首条响应返回的 `conversation_id` 入缓存。
- `ChatStream` → Dify `stream=true`，SSE 解析为 `StreamChunk`。
- `UploadFile` → Dify `POST /files/upload`，仅支持 Dify App 配置内允许的 mime。
- 错误码映射：Dify 4xx → 网关 4xx；Dify 5xx / 超时 → 触发 fallback。

### 6.4 OpenClaw / 自定义 HTTP 实现要点
- 在没有官方明确协议前，按上述 interface 提供"标准 HTTP / SSE"协议作为通用对接方式，OpenClaw 侧只需实现 4 个端点：`/v1/conversations`、`/v1/chat`、`/v1/chat/stream`、`/v1/files`。
- 鉴权：Bearer Token；可选 mTLS。
- 流式协议：SSE，事件名固定为 `delta` / `done` / `error`。

---

## 7. 迁移路径

### 7.1 时间线（建议）

| 阶段 | 时间窗 | 主要交付 |
| --- | --- | --- |
| v1.1 RC | 起始 + 4 周 | AIBackend 接口、Dify 一等公民、可观测基线、Deprecated 公告 |
| v1.1 GA | 起始 + 8 周 | 性能 SLO 达标、文档齐全 |
| v1.2 RC | 起始 + 14 周 | 移除 Deprecated、数据库迁移脚本、前端同步 |
| v1.2 GA | 起始 + 18 周 | 主备切换、限流配额、回放调试 |

### 7.1.1 v1.1 PR 拆分路线（实施层）

| PR | 范围 | 与 go-zero 关系 |
| --- | --- | --- |
| **#0 升级基础设施** | `go get -u github.com/zeromicro/go-zero@latest` + `go install ...tools/goctl@latest` + 一次全量 regen | 全量 regen，必经 |
| **#1 aibackend 接口包** | 新增 `chat/common/aibackend/`（types / backend / registry / errors） | 与 go-zero 解耦的纯 Go 包 |
| **#2 Dify 适配器** | `chat/common/aibackend/dify/`，包装现有 `chat/common/dify` | 不涉及 go-zero |
| **#3 OTel + Prometheus 配置接入** | yaml 加 `Telemetry` / `Prometheus` 块；config struct 同步；wecom 两端写 propagator | 使用 go-zero 自带，**不写自定义中间件**；config 改完 yaml + struct 即可，不需要 regen |
| **#4 配置层加 AIBackend** | `internal/config.Config` 新增 `AIBackend.Default / Backends / Routes` 字段；ServiceContext 实例化 | 仅改 config struct，不需要 regen |
| **#5 chatlogic.go Dify 分支切到接口** | 把 `chatlogic.go` 中 `case "dify":` 调用改成 `svc.AIBackend.ChatStream(...)`；其它 Provider 暂不动 | logic 是手写区，不影响 .api / goctl |
| **#6 双轨开关 + 灰度** | 配置 `AIBackend.RolloutPercent`；按百分比走新接口或旧分支 | 不涉及 .api |
| **#7 Deprecated 公告** | 在响应 Header 加 `Deprecation: true`；启动日志告警；CHANGELOG | 不涉及 .api |
| **#8 性能压测 + SLO 达标** | k6 / wrk 脚本 + 调优；补齐 Prometheus 自定义指标 | 不涉及 .api |

### 7.2 现网用户迁移步骤

1. **升级到 v1.1**：配置文件维持原样可直接运行；按需新增 `AIBackend` 块开始双轨。
2. **建立 Dify / OpenClaw 资产**：把现有知识库导出 → 上传到 Dify 数据集；把 Prompt 整理成 Dify App。
3. **Bot 切换**：在 agent-web 把 Bot 绑定到对应的后端 App ID。
4. **观察期**：至少 2 周双轨运行，比对回复质量与延迟。
5. **升级到 v1.2**：执行迁移脚本 → 旧字段 / 旧表清理 → 前端升级 → 删除 Deprecated API 调用。

### 7.3 兼容策略

- v1.1 期间所有 Deprecated API 仍可用，仅返回 `Deprecation` Header。
- v1.2 启动时如检测到旧配置块（`ModelProvider` 等），打印迁移指引并以兼容模式启动，但日志中持续告警。
- 提供 `chat migrate` 子命令：
  - `chat migrate config` → 旧 yaml 转新 yaml
  - `chat migrate knowledge --target dify` → 导出知识库为 Dify 数据集可导入格式

---

## 8. 非目标 / 明确不做

- **不做 Dify 的功能复刻**：知识库、工作流、Agent、工具不在本项目内开发。
- **不再追加新的 LLM Provider 直连**（除非作为 fallback）。
- **不引入插件市场 / 应用商店**。
- **不做绘画引擎自研**。
- **不在本期重构语音 ASR / TTS**（保留现有 AMR ↔ 文本能力即可）。
- **不做多语言界面国际化**（暂仍中文为主）。

---

## 9. 风险与缓解

| 风险 | 等级 | 缓解 |
| --- | --- | --- |
| Dify / OpenClaw 协议变化 | 中 | AIBackend 抽象层 + 集成测试矩阵 |
| 后端服务可用性影响整体 SLA | 高 | 主备 + 兜底直连 OpenAI；超时 / 熔断；优雅降级提示 |
| 老用户对自研知识库 / 插件 / 绘画的依赖 | 高 | v1.1 不删；提供迁移工具；FAQ 与示例 |
| 数据库迁移风险 | 中 | 迁移脚本提供 dry-run；先归档后清理；可回滚 |
| 性能未达预期 | 中 | v1.1 引入 metrics 后基于真实数据迭代；v1.2 设定明确 SLO |
| 社区叙事变化（从"全栈"转"网关"） | 中 | README / 官网 / CHANGELOG 重新定位；强调"安全、稳定、易接 Dify" |

---

## 10. 度量与回顾

- **代码规模**：v1.2 GA 时 LOC、go.mod 依赖、Docker 镜像大小较 v1.0 各下降 ≥ 30%。
- **可维护性**：单测覆盖率 ≥ 60%；CI 全绿；issue 平均关闭时间下降。
- **性能**：v1.2 SLO 达标率 ≥ 99%。
- **使用情况**：Dify 后端占总流量 ≥ 80%；fallback 触发率 < 1%。
- **用户反馈**：至少 5 个真实部署用户完成 v1.0 → v1.2 升级且无回退。

---

## 11. 附录

### 11.1 与现有文档的关系
- 本文档关注**做减法 + 加网关**；产品全貌请仍参考 [PRD.md](./PRD.md)。
- v1.1 GA 后，[PRD.md](./PRD.md) 将整体改写为"网关版"。
- go-zero 开发规约：本机已安装官方 `zero-skills`（`~/.claude/skills/zero-skills/`），写 `.api` / 跑 goctl / 配 OTel 时优先 Read 该目录的 `references/` 与 `getting-started/claude-code-guide.md`。

### 11.2 关键代码路径（重构清单）

**保留 + 加固**
- `chat/common/wecom/**`
- `chat/service/chat/api/internal/handler/chathandler.go`、`customerchathandler.go`
- `chat/service/websocket/**`
- `chat/common/response/**`（流式分段）
- `chat/common/accesslog/**`（接入 trace）

**新增**
- `chat/common/aibackend/`（接口 + 实现：dify / openclaw / openai-fallback）
- `chat/common/aibackend/dify/`、`chat/common/aibackend/openclaw/`、`chat/common/aibackend/openai/`（具体适配器，包装现有 SDK）
- 可观测**不新增独立目录**：trace / metrics / log 全用 go-zero 自带（Telemetry / Prometheus / logx），仅在企微回调和回写两端补一段 propagator 代码（建议放在 `chat/common/wecom/trace.go` 或 `chat/common/accesslog/`）。

**v1.1 标 Deprecated / v1.2 删除**
- `chat/common/milvus/`
- `chat/common/openai/`（降级为 fallback 后保留极简实现）
- `chat/common/gemini/`、`chat/common/deepseek/`
- `chat/common/draw/`
- `chat/common/plugin/`、`plugins/`
- `chat/service/chat/api/internal/handler/knowledge*.go`
- `chat/service/chat/api/internal/handler/botoptimizeprompthandler.go`、`botexplorelisthandler.go`

### 11.3 与上游项目协作
- 与 Dify 上游：跟踪其 API 兼容性，必要时贡献修复。
- 与 OpenClaw 上游（如官方协议尚未稳定）：以本项目的 AIBackend 协议作为草案提交沟通。
- 前端 `agent-web` 仓库需要同步重构，作为本重构 PRD 的第二个工作流。
