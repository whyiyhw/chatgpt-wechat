## v1.1.0-rc1（重构 / 兼容期，进行中）

> 详见 [PRD-refactor.md](./PRD-refactor.md)。本版本不破坏现网部署：未配 `AIBackend` 块时行为与 v1.0 完全一致。

### 新增
- **AIBackend 接口包**（`chat/common/aibackend`）：统一 AI 后端适配器契约（`OpenConversation / Chat / ChatStream / UploadFile / Health`），含 Factory/Registry、6 类标准错误（含 `IsRetryable`）。
- **Dify 适配器**（`chat/common/aibackend/dify`）：包装现有 `chat/common/dify`，`ChatStream` 走 Chatflow 事件流，支持 `workflow_started / node_started / node_finished / workflow_finished / message[_end]`，`UploadFile` 走 `/v1/files/upload` 上传图片得 `file_id`，`ChatReq.Files` 透传到 Dify Workflow（缺省 `Type=image`、按 URL/UploadFileID 推断 `TransferMethod`）。
- **`dify.APIError` 强类型错误**：上游 `client.go` 与 `api_workflow.go` 非 2xx 返回 `*APIError{Status, Code, Message}`，适配器 `mapError` 改用 `errors.As` 精确分类（HTTP status + Dify 业务码），不再依赖脆的字符串匹配。
- **AIBackend.Router 配置**：`AIBackend.Default / Backends / Routes / RolloutPercent`。Backends 强类型 yaml + Factory 内部 map 解码；`RolloutPercent` 按 `UserID` fnv-32a 稳定 hash 分流，0/100 边界跳过哈希。
- **可观测**：`Telemetry` / `Prometheus` yaml 模板就位，go-zero 自带 OTel + Prometheus，Endpoint 留空时仅进程内 `trace_id` 注入 logx。
- **Deprecation middleware**：v1.2 计划移除的路由响应附加 `Deprecation: true` + `Sunset: v1.2` + `Link rel="successor-version"` header。
- **启动期 deprecated 配置告警**：检测 `OpenAi/Gemini/DeepSeek/Dify.Key`、`Draw.Enable`、`ModelProvider.Company`，logx 打印迁移指引。
- **CLAUDE.md**：项目协作规范（goctl 严格生成、可观测走 go-zero 自带、AIBackend 接口硬性要求等）。

### 修改
- go-zero `v1.8.1 → v1.10.1`，goctl `v1.10.1`，完成一次全量 regen（业务零影响，仅生成产物头部加版本注释）。
- `chatlogic.go` 的 dify 分支按 `RolloutPercent` 灰度切到 `AIBackend.Router`；未配 Router 或灰度未命中时回落 legacy 路径。
- 36 个 handler 修复 `fmt.Errorf(<变量>)` 的 Go 1.24 vet 报错（改为 `errors.New(...)`）。

### Deprecated（v1.2 移除）
| 模块 | 路径 / 字段 | 替代方案 |
| --- | --- | --- |
| 自研知识库 | `/api/knowledge/**`、`/api/bot/knowledge/**` | Dify 数据集 |
| Prompt 优化 | `/api/bot/prompt/optimize` | 后端自带 |
| Bot 模板发现 | `/api/bot/explore/list` | 后端应用市场 |
| Stable Diffusion / DALL-E 直连 | `config.Draw.*` | 后端工作流 |
| Provider 直连配置 | `config.OpenAi/Gemini/DeepSeek/Dify.Key` | `AIBackend.Backends + Router` |
| `ModelProvider.Company` 分支 | `chatlogic.go` 中对应分支 | `AIBackend.Router` |

### 迁移指引
1. 升级到 v1.1：配置文件维持原样可直接运行。
2. 在 yaml 加 `AIBackend.Default / Backends`，`RolloutPercent` 从 `0` 起步。
3. 双轨观察 ≥ 2 周后逐步提升 `RolloutPercent`（建议 0 → 10 → 30 → 50 → 80 → 100）。
4. 现网全切到 Router 后，把知识库/绘画等已 Deprecated 的能力迁到 Dify。

---

## v1.0.0-bate
- 因为企业微信调整了api 客服消息跟着改动
  - https://developer.work.weixin.qq.com/document/51165
  - https://developer.work.weixin.qq.com/community/question/detail?content_id=16545017483768547791

### v0.5
#### 新增多渠道客服消息,支持自定义端口

- 相关配置
```yaml
WeCom:                                              # 企业微信配置
  Port:                                             # 企业微信回调监听端口（可选）默认为8887
  CustomerServiceSecret: "xxxx-xxxx-xxxx"           # 企业微信客服消息 Secret
  Token: "xxxxxxxxxx"                               # 企业微信应用/客服消息 Token
  EncodingAESKey: "xxxxxxxxxxxxxxxx"                # 企业微信应用/客服消息 EncodingAESKey
```

### v0.4
#### 新增8887端口，支持服务器ip直接对接企业微信

#### 新增OCR支持，可用于答题相关场景
- [点击查看示例](./image25.jpg)
- 相关配置
```yaml
OCR:                                                # OCR配置 ,开启图片识别（可选）
  Company: "ali"                                    # 识别公司，目前支持阿里云（可选）
  AliYun:                                           # 阿里云配置
    AccessKeyId: ""                                 # 阿里云 key
    AccessKeySecret: ""                             # 阿里云 secret
```
#### 新增预定义模板指令 `#prompt:list`
![image24.png](./image28.png)
#### 新增预定义模板选择 `#prompt:set:{id}`
![image27.png](./image27.png)

#### changelog 升级说明
- 从低版本，已部署系统升级上来的需要清理数据库信息才能重新 构建
- 如 在 chat 目录下执行 去清理数据
```shell
sudo docker-compose down
sudo rm -rf ./build/mysql/data/*
sudo rm -rf ./build/redis/data/*
sudo docker-compose build 
sudo docker-compose up -d
```
#### fix
- 数据库对于中文字符乱码问题
- 微信端发送文字长度限制问题
- 优化了上下文过长时的错误提示文案

### v0.3.1
- 新增 WeCom.Welcome 自定义15天欢迎语
- 新增 反向代理支持（可选）,将 OpenAi.Host 换成 对应的反向代理域名即可

```shell
WeCom:
  Welcome: "您好！我是 ChatGPT，一个由 OpenAI 训练的大型语言模型，我可以回答您的问题和进行交流。请告诉我您需要了解些什么，我会尽力为您提供答案。发送#help 查看更多功能"

OpenAi:
  Key: "xxxxxxxxxxxxxxxxxxxxx"
  Host: "https://api.openai.com"
```
### v0.3
- 新增最新版本的 gpt3.5 api 配置 与 prompt 自定义配置
```
WeCom:
  BasePrompt: "你是 ChatGPT, 一个由 OpenAI 训练的大型语言模型, 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。"
  Model: "text-davinci-003"
  MultipleApplication:
  - AgentID: 1000002
    AgentSecret: "55sO-xxxxxxxxxxxxxxxxxxxxxxx"
    Model: "gpt-3.5-turbo"
    BasePrompt: "你是 ChatGPT, 一个由 OpenAI 训练的大型语言模型, 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。"
```
- 新增 指令对话模式

![image22.png](./image22.png)

- 新增 通过 指令对话模式 实时更换 角色与 model
  ![image23.png](./image23.png)

#### changelog 升级说明
- 从低版本，已部署系统升级上来的需要清理数据库信息才能重新 构建
- 如 在 chat 目录下执行 去清理数据
```shell
sudo docker-compose down
sudo rm -rf ./build/mysql/data/*
sudo rm -rf ./build/redis/data/*
sudo docker-compose build 
sudo docker-compose up -d
```

### v0.2.2
- 新增多应用支持(可选), 如果你有多个应用只开了一个服务，现在你可以开启多应用配置，相关参数就在企业微信应用详情页
```
  MultipleApplication:
  - AgentID: 1000002
    AgentSecret: "55sO-xxxxxxxxxxxxxxxxxxxxxxx"
  - AgentID: 1000003
    AgentSecret: "lwAFxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```
- 因为 服务器在国内，新增代理设置来保证服务可用, 暂时只支持socket5（可选）
```
Proxy:
  Enable: false
  Socket5: "127.0.0.1:1080"
```

### v0.2.1
- 简化后端运维操作，增加 docker-compose 来编排服务

### v0.2

- 后端代码已发布
  - 需要 docker 以及简单的运维操作，实现已经卸载
  - 如需使用，请先配置相关数据库与 redis , 各类 密钥 通过 `chat\service\chat\api\etc\chat-api.yaml` 进行配置
  - over😀
- 增加 阿里云新增 req_token 环境变量来进行验证 请求合法性

### v0.1

- add 支持记忆多轮对话与记忆清理
- fix 对非文本格式数据进行回复拒绝

为了支持多轮对话，新增菜单配置  企业微信>应用管理>自定义菜单
![image14.png](./image14.png)
效果如下
![image15.png](./image15.png)