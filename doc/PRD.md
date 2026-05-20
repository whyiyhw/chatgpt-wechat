# ChatGPT-WeChat 产品需求文档（PRD）

> 版本：v1.0.0（开发中 / 稳定版 v0.6.6）
> 更新日期：2026-05-19
> 适用仓库：[whyiyhw/chatgpt-wechat](https://github.com/whyiyhw/chatgpt-wechat)
> 配套前端：[whyiyhw/agent-web](https://github.com/whyiyhw/agent-web)

---

## 1. 产品概述

### 1.1 一句话定位
**一个可在微信内安全使用的多模型 AI 助手与企微客服平台**：通过企业微信作为安全中转层，将 OpenAI / DeepSeek / Gemini / Dify 等大语言模型能力，以及绘画、语音、知识库、插件等增强能力，安全地交付给微信生态内的终端用户与企业客服场景，零封号风险。

### 1.2 解决的问题
- **合规与封号风险**：直接在微信内对接 ChatGPT 类服务存在账号风险，本项目通过"微信 → 企业微信 → 后端服务"链路规避。
- **多模型割裂**：单一模型供应商存在限流、地域、计费等问题；本项目对 OpenAI、Azure OpenAI、DeepSeek、Gemini、Dify 做了统一抽象。
- **企业知识私有化**：通用大模型无法直接回答企业内部资料的问题；本项目提供 Knowledge / Unit / Segment 三层向量知识库。
- **客服场景适配**：传统 Bot 服务不直接支持企微客服多渠道（公众号、视频号、小程序、Web），本项目内置客服消息协议适配。

### 1.3 产品愿景
让个人开发者与中小企业可以用一套自托管的开源服务，安全地把多模型 AI 能力接入微信生态，同时提供 Bot、知识库、插件等可二次开发的工程化能力。

---

## 2. 目标用户与角色

| 角色 | 描述 | 主要场景 |
| --- | --- | --- |
| **终端用户（微信用户）** | 通过微信/企微/客服渠道接收 AI 服务的 C 端用户 | 聊天问答、绘画、语音、答题（图片识别） |
| **Bot 创建者 / 租户** | 在管理端创建并配置 Bot、Prompt、知识库、模型 | 搭建专属助手、调整参数、复制模板、发布到客服 |
| **企微客服坐席** | 在企微客服中辅助回复客户的人工坐席 | 借助 Bot 自动回复、人工接管、知识库辅助 |
| **系统管理员** | 部署运维 / 多应用配置 / 企微对接 | Docker 部署、配置文件维护、密钥与代理管理 |
| **API 调用方** | 通过 HTTP / WebSocket 接入外部业务系统 | 自有 App、网页接入、消息回调推送 |

---

## 3. 核心功能需求

### 3.1 Bot 管理模块

| 能力 | 接口 | 说明 |
| --- | --- | --- |
| 创建 / 更新 / 删除 / 复制 Bot | `/api/bot/{create,update,delete,replicate}` | 支持 Bot 模板复制，便于快速搭建多个角色 |
| Bot 详情 / 列表 / 发现 | `/api/bot/{detail,list,explore/list}` | 个人列表与公开 Explore 双视图 |
| Prompt 配置与优化 | `/api/bot/prompt/{update,optimize}` | 提供 AI 优化 Prompt 能力，预定义 100+ 角色模板 |
| 模型绑定与参数 | `/api/bot/model/{detail,update}` | 每个 Bot 可独立绑定模型、Temperature、MaxToken 等参数 |
| 知识库绑定 | `/api/bot/knowledge/update` | 一个 Bot 可关联多个知识库单元 |
| 客服绑定与发布 | `/api/bot/customer/{list,update}` | 将 Bot 发布到企微客服多渠道 |

**业务规则**
- Bot 的 Prompt 支持运行时通过 `#config_prompt:xxx` 命令动态调整。
- 复制 Bot 时，知识库关联与模型配置默认随复制。
- Prompt 优化使用当前激活的 LLM Provider 作为优化器。

### 3.2 会话与消息模块

#### 3.2.1 会话能力
- **多会话切换**：每个用户可维护多个独立上下文会话（`#session:start / list / exchange:x`）。
- **自适应上下文**：根据 token 限制与模型上限自动裁剪历史，避免手动 `#clear`。
- **会话导出**：支持 JSON / TXT 格式（`#session:export:json|txt`）。
- **会话清空**：`#clear`（当前）/ `#session:clear`（全部）。

#### 3.2.2 消息类型
| 类型 | 输入 | 输出 |
| --- | --- | --- |
| 文本 | 微信文本消息 / API POST | 流式文本，自动分段回复 |
| 语音 | 企微 AMR 语音 | 文本（已去除 Markdown 标记的语音友好版本）|
| 图片 | 企微图片消息 | 文本（答题/识图场景）|
| 绘画 | `#draw:prompt` | 图片（SD / DALL-E）|

#### 3.2.3 消息推送
- **HTTP 推送**：`/api/msg/push`（应用消息）、`/api/msg/customer/push`（客服消息）。
- **WebSocket 推送**：独立 websocket 服务（默认 9501 端口），支持鉴权、心跳、bucket 分组。
- **极速响应**：分段流式输出，首字快速反馈，长答自动续写。

### 3.3 模型接入模块

| Provider | 支持模型 | 特性 |
| --- | --- | --- |
| **OpenAI** | gpt-3.5 / gpt-4 / gpt-4o / 自定义（one-api 兼容） | Azure OpenAI、余额查询、HTTP/SOCKS5 代理 |
| **DeepSeek** | deepseek-chat / deepseek-reasoner | 自定义 Host、Debug 模式、动态最大 token |
| **Google Gemini** | gemini-pro、embedding | 60 req/min 免费额度，支持向量化 |
| **Dify** | Workflow / Chat App | 通过 Dify Flow API 接入，可承载自定义编排 |

**通用能力**
- Provider 通过 `ModelProvider` 配置项切换，Bot 级别可覆盖全局配置。
- 全部 Provider 支持流式返回 + 自动分段。
- 反向域名代理（自建中转域名）+ HTTP / SOCKS5 代理双通道。

### 3.4 知识库模块（私有数据）

三层数据模型：

```
Knowledge（知识库，业务域，例如"产品手册"）
  └── KnowledgeUnit（知识单元，文档/网页，可独立开关）
        └── KnowledgeUnitSegment（分段，向量化的最小检索单元）
```

| 能力 | 接口 |
| --- | --- |
| Knowledge CRUD | `/api/knowledge/{create,update,delete,list}` |
| Unit CRUD + 开关 | `/api/knowledge/unit/{create,update,delete,list,detail,switch}` |
| Segment CRUD | `/api/knowledge/segments/{create,update,delete,list}` |

**业务规则**
- 段落级向量存储于 Milvus（私有部署）或 pgvector（PostgreSQL 内置）。
- Bot 关联 Knowledge 后，对话时自动检索 Top-K 段落并注入 Prompt 上下文。
- Unit 支持快速开关，便于灰度上线某份资料。

### 3.5 绘画模块

| 引擎 | 触发 | 特性 |
| --- | --- | --- |
| Stable Diffusion 1.5 | `#draw:prompt` | 支持 C 站参数粘贴（Step / CFG / Sampler / Size 等）|
| OpenAI DALL-E | `#draw:prompt` | 通过 OpenAI 接口直出 |

- 中英文 Prompt 均可，可自动翻译。
- 引擎切换在配置文件中指定。

### 3.6 企微集成模块

- **应用消息**：直连企微应用消息接口，支持文本 / 语音 / 图片 / 文件。
- **客服消息**：实现最新企微客服协议，支持公众号 / 视频号 / 小程序 / Web / App / 企微多渠道客户。
- **多应用（MultipleApplication）**：一套服务实例承载多个 AgentID，独立 Secret 与 Session。
- **自定义 API 域名**：`WeCom.QYAPIHost` 支持企微 API 自建反向代理。
- **可信 IP**：依赖企微管理后台的应用可信 IP 配置（部署文档已说明）。

### 3.7 插件模块

| 插件 | 能力 |
| --- | --- |
| **Search** | DuckDuckGo / Wikipedia / Bilibili 搜索能力 |
| **Webhook** | HTTP 触发 Shell 脚本，扩展任意能力 |
| **Built-in 指令** | `#help / #system / #clear / #config_* / #prompt:* / #session:* / #draw:* / #plugin:list` |

**扩展规则**
- 插件遵循统一 HTTP 协议（请求 / 响应 schema），通过 `plugins/` 独立部署，可水平扩展。
- 后续规划：summary、天气、长期记忆、midjourney 等。

### 3.8 用户与认证

- 注册：`/api/user/register`
- 登录：`/api/user/login`（返回 JWT）
- 详情：`/api/user/detail`
- 鉴权：除注册/登录外，所有 API 通过 JWT 中间件（`Auth`）保护。
- 全局中间件：`AccessLog` 统一访问日志。

---

## 4. 用户关键流程

### 4.1 终端用户（微信）对话流程
```
微信用户 → 企业微信应用/客服 → web 服务（/wecom 回调）
       → 解析消息类型（文本/语音/图片/指令）
       → 加载会话上下文 + 知识库向量召回
       → 调用对应模型 Provider（流式）
       → 自动分段写回企微（应用消息 / 客服消息）
       → 终端用户在微信收到回复
```

### 4.2 Bot 创建者管理流程
```
登录 agent-web → 创建 Bot → 选择模型 & 参数
              → 编写/优化 Prompt → 绑定知识库
              → 关联客服渠道 → 灰度验证 → 正式发布
```

### 4.3 知识库构建流程
```
新建 Knowledge → 上传/录入 Unit（文档/网页/QA）
               → 系统切分为 Segment 并向量化入库
               → Bot 关联 Knowledge
               → 对话时自动检索注入上下文
```

### 4.4 客服消息流程
```
公众号/视频号/小程序/Web 客户 → 企微客服 → 项目服务
                              → Bot 自动回复 OR 客服坐席人工接管
                              → 流式回写到客户渠道
```

---

## 5. 非功能性需求

| 维度 | 要求 |
| --- | --- |
| **性能** | 流式分段首字 < 2s（依赖模型供应商）；WebSocket 推送支持 bucket 分组、心跳保活 |
| **可用性** | Docker Compose 单机部署可用；支持多应用横向扩展；服务可独立重启 |
| **安全性** | JWT 鉴权；企微回调签名校验；不在日志中打印密钥；支持 Redis 密码、PG 密码独立配置 |
| **可观测性** | AccessLog 中间件；`docker logs` 排查；关键失败有 `应用消息-发送失败` / `客服消息-发送失败` 关键字 |
| **可扩展性** | 模型 Provider 可插拔；插件标准 HTTP 协议；多应用配置；多客服渠道 |
| **合规性** | 不直接对接微信个人号，依托企微合规链路；私有化部署，数据不出域 |
| **国际化** | 多语言语音输入 / 文本输入；中英文 Prompt 自动翻译 |

---

## 6. 技术架构

### 6.1 系统组件
```
┌──────────────┐      ┌────────────────────┐
│  微信 / 企微 │ ──▶ │  web (8888/8887)    │ ── 应用消息 / 客服消息回调
└──────────────┘      │  go-zero REST API  │
                      └─────────┬──────────┘
                                │
   ┌────────────────────────────┼────────────────────────────┐
   │                            │                            │
┌──▼───┐   ┌─────────────┐   ┌──▼─────────┐   ┌──────────┐
│Redis │   │  PostgreSQL │   │  Milvus    │   │ Plugins  │
│ 缓存 │   │ + pgvector  │   │ 向量库     │   │ search / │
└──────┘   └─────────────┘   └────────────┘   │ webhook  │
                                              └──────────┘
                      ┌─────────┴──────────┐
                      │  websocket (9501)  │
                      │  流式推送          │
                      └────────────────────┘
                                │
                  ┌─────────────┴────────────┐
                  │   OpenAI / DeepSeek      │
                  │   Gemini / Dify          │
                  └──────────────────────────┘
```

### 6.2 关键技术栈
- **后端框架**：go-zero（REST API）
- **ORM**：GORM + gorm.io/gen
- **数据库**：PostgreSQL + pgvector（已从 MySQL 迁移）
- **缓存**：Redis 7.0
- **向量库**：Milvus（可选）+ pgvector
- **WebSocket**：gws
- **大模型 SDK**：`sashabaranov/go-openai` 兼容封装
- **企微 SDK**：`whyiyhw/go-workwx`
- **多媒体**：`disintegration/imaging`、MP3 ↔ AMR 转换

### 6.3 部署形态（Docker Compose）
| 容器 | 默认端口 | 作用 |
| --- | --- | --- |
| web | 8888 / 8887 | 主 API + 企微回调 |
| websocket | 8884 → 9501 | 流式消息推送 |
| pgvector | 43306 → 5432 | 业务 + 向量库 |
| redis | 46379 → 6379 | 缓存与会话 |
| milvus（可选） | — | 大规模私有向量库 |

### 6.4 主要配置块（`chat-api.yaml`）
- `Name / Host / Port`：服务基础
- `PGSql`：PostgreSQL 连接
- `RedisCache`：Redis 连接
- `WeCom`：企微 CorpID / Token / EncodingAESKey / MultipleApplication / QYAPIHost
- `ModelProvider`：当前激活的 LLM
- `OpenAi / DeepSeek / Gemini / Dify`：各 Provider 的 Key / Host / 参数
- `Response`：流式开关 / 分段策略

---

## 7. 主要 API 一览

> 全部位于 `chat/service/chat/api/chat.api`。除 register/login 外都需 JWT。

**用户**
- `POST /api/user/register`
- `POST /api/user/login`
- `GET  /api/user/detail`

**Bot**
- `POST /api/bot/{create,update,delete,detail,list,replicate}`
- `POST /api/bot/prompt/{update,optimize}`
- `POST /api/bot/model/{detail,update}`
- `POST /api/bot/knowledge/update`
- `POST /api/bot/customer/{list,update}`
- `POST /api/bot/explore/list`

**对话**
- `POST /api/bot/chat`
- `POST /api/bot/chat/history`
- `POST /api/bot/chat/history/clear`

**消息推送**
- `POST /api/msg/push`
- `POST /api/msg/customer/push`

**知识库**
- `POST /api/knowledge/{create,update,delete,list}`
- `POST /api/knowledge/unit/{create,update,delete,list,detail,switch}`
- `POST /api/knowledge/segments/{create,update,delete,list}`

---

## 8. 命令式指令一览（终端用户在聊天中输入）

```
基础：    #help  #system  #clear
会话设置：#config_prompt:xxx  #config_model:xxx  #config_clear
模板：    #prompt:list  #prompt:set:xx
会话控制：#session:start | list | clear | exchange:xxx
导出：    #session:export:json | txt
绘画：    #draw:xxx
插件：    #plugin:list
```

---

## 9. 版本规划与里程碑

### 9.1 已完成（节选）
- v0.x：基础对话、Prompt 自定义、企微直连、多渠道客服、语音/图片消息、流式分段
- v0.x：自适应上下文、会话切换与导出、向量检索
- v0.x：基础插件（search）、Stable Diffusion 1.5、OpenAI 绘画、Key 余额查询
- v1.0.0（进行中）：
  - 数据库迁移 PostgreSQL + 向量化查询
  - Gemini-pro 支持
  - DeepSeek 支持（含 Debug 模式）
  - GPT-4o 与 one-api 兼容
  - web 管理端（agent-web）
  - 企微客服最新协议
  - 企微 API 自定义域名

### 9.2 规划中
- 消息超长处理策略
- 自适应上下文"省流模式"
- Midjourney 绘画接入
- 多 Key 轮询应对限流
- 长期记忆插件
- 私有化知识库插件标准化
- 同声转译（so-vits-svc）
- 阿里云免费额度集成
- OpenAI Token 累计与不足切换

---

## 10. 风险与依赖

| 类别 | 风险 / 依赖 | 缓解策略 |
| --- | --- | --- |
| **平台合规** | 企微应用资质、客服资质要求 | 文档指引开通流程，依赖企业实际资质 |
| **境内访问** | OpenAI / Gemini 在大陆访问受限 | 反向代理、海外服务器、one-api 中转 |
| **模型限流** | OpenAI / Gemini 速率限制 | 多 Key 轮询（规划中）、Provider 切换 |
| **数据安全** | 向量库与对话内容含敏感信息 | 私有化部署、Redis / PG 独立密码 |
| **依赖稳定性** | 第三方 SDK / API 变更 | go.mod 锁版本、Provider 适配层隔离 |
| **运维复杂度** | Docker Compose 多容器联动 | 单命令启动；FAQ 覆盖常见故障（redis、token 失效、可信 IP）|

---

## 11. 验收与质量要求

- 关键路径具备单元测试：`chat/service/chat/api/internal/logic/*_test.go`（bot create/delete/list/replicate/update、user login/register、chat 等已覆盖）。
- 部署后自检：通过 `#system` 指令确认配置加载正确；通过 `docker logs -f chat_web_1` 排查回调失败。
- 文档完整性：`doc/install.md`、`doc/config.md`、`doc/custom_support_service.md`、`doc/plugin.md`、`doc/draw.md`、`doc/frp.md`、`doc/CHANGELOG.md` 同步更新。

---

## 12. 附录

- 架构图：`doc/system.png`、`doc/system02.png`
- 完整安装：`doc/install.md`
- 配置说明：`doc/config.md`
- 能力详解：`doc/ability.md`
- 客服接入：`doc/custom_support_service.md`
- 插件开发：`doc/plugin.md`
- 绘画配置：`doc/draw.md`
- 内网穿透：`doc/frp.md`
- 变更日志：`doc/CHANGELOG.md`
