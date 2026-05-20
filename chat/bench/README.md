# 性能压测

v1.1 RC 起的性能基线工具。配合 [doc/PRD-refactor.md §4.3](../../doc/PRD-refactor.md#43-v11-性能-slo) 使用。

## SLO 表（v1.1）

| 指标 | 目标 | 本目录覆盖度 |
| --- | --- | --- |
| 企微回调 → 后端首字（不含 Dify 推理） | P95 < 300ms | ⚠️ 本地 stand-in，真实需打 wecom XML 端口 |
| 流式分段首段写回企微 | P95 < 1.5s | ⏳ 待补 |
| WebSocket 推送时延 | P95 < 200ms | ⏳ 待补 |
| 单实例并发会话 | ≥ 200 | ✅ k6 stages.target |
| 错误率（5xx / 后端不可用） | < 0.5% | ✅ thresholds.http_req_failed |

## 前置依赖

1. 安装 [k6](https://k6.io/)：
   ```bash
   brew install k6                  # macOS
   # 或 docker run -i --rm grafana/k6 run - <chat-api.js
   ```
2. chat-api 本地起来：`cd chat/service/chat/api && go run chat.go -f etc/chat-api.yaml`
3. yaml 里至少配好 `WeCom.MultipleApplication` 与 `Dify`（或 `AIBackend.Backends`），让 dify 渠道能跑通
4. 拿一个 JWT：
   ```bash
   curl -s -X POST http://localhost:8888/api/user/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"<u>","password":"<p>"}' | jq -r .data.token
   ```

## 跑

```bash
# 默认压 200 并发，1 分钟 sustain
k6 run \
  -e BASE_URL=http://localhost:8888 \
  -e JWT=<token> \
  -e AGENT_ID=1000002 \
  k6/chat-api.js

# 自定义峰值并发（例如开 RolloutPercent 灰度逐档压）
k6 run -e STAGE_MAX_VU=50 ... k6/chat-api.js
```

## 与灰度配合的推荐压测流程

| 阶段 | RolloutPercent | k6 STAGE_MAX_VU | 关注指标 |
| --- | --- | --- | --- |
| Baseline | 0 (全 legacy) | 200 | 老路径基线 P95 / 错误率 |
| 灰度 10% | 10 | 200 | 新老两路 P95 是否同档 |
| 灰度 50% | 50 | 200 | 新路径 P95 是否仍 < 300ms |
| 全量 | 100 | 200 | 达到 SLO 即可锁定 v1.1 GA |

每档跑完后，从 Prometheus 拉对应指标对比：

```promql
histogram_quantile(0.95,
  rate(chatgpt_wechat_ai_backend_duration_seconds_bucket{backend="dify-main"}[5m]))

histogram_quantile(0.95,
  rate(chatgpt_wechat_ai_backend_first_token_ms_bucket{backend="dify-main"}[5m]))

sum by (status) (
  rate(chatgpt_wechat_ai_backend_request_total{backend="dify-main"}[5m]))
```

## 已知限制

- **本地 stand-in**：`/api/msg/push` 不是企微真实回调入口，PRD §4.3 第一行 SLO 严格意义上要在 wecom XML 端口（默认 8887）上测量；待补。
- **不含后端推理**：脚本的 P95 限定的是 chat-api 自身处理时延，Dify 推理耗时不计入。
- **不覆盖 WebSocket**：流式分段写回企微的 P95 需要单独的 WebSocket 压测脚本，待补。
- **JWT 复用**：脚本所有 VU 共用一个 JWT；如需多用户压测请改成动态 login。
