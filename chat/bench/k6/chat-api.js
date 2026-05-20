// k6 压测脚本：chat-api /api/msg/push（dify 渠道）
//
// 用途：作为 v1.1 RC 的性能基线工具，配合 doc/PRD-refactor.md §4.3 的 SLO 表使用。
// 注意：这是本地 stand-in。真实"企微回调 → 后端首字"压测需要把入口换成 wecom
// XML 回调端口（默认 8887）+ 真实 Dify 实例，本脚本只覆盖 chat-api 层。
//
// 用法：
//   k6 run \
//     -e BASE_URL=http://localhost:8888 \
//     -e JWT="<先调 /api/user/login 拿到的 token>" \
//     -e AGENT_ID=1000002 \
//     bench/k6/chat-api.js
//
// 关键环境变量：
//   BASE_URL    chat-api 地址，默认 http://localhost:8888
//   JWT         必填，鉴权 token；本地可调 /api/user/login 取
//   AGENT_ID    必填，企微应用 ID（对应 yaml MultipleApplication[i].AgentID）
//   STAGE_MAX_VU  峰值并发数，默认 200（PRD §4.3 单实例并发会话目标）

import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8888';
const JWT = __ENV.JWT || '';
const AGENT_ID = parseInt(__ENV.AGENT_ID || '1000002', 10);
const STAGE_MAX_VU = parseInt(__ENV.STAGE_MAX_VU || '200', 10);

export const options = {
  stages: [
    { duration: '30s', target: Math.max(1, Math.floor(STAGE_MAX_VU / 4)) }, // warm up
    { duration: '1m', target: STAGE_MAX_VU },                                // sustain
    { duration: '30s', target: 0 },                                          // ramp down
  ],
  thresholds: {
    // PRD §4.3 v1.1 SLO（chat-api 层，不含 Dify 推理耗时）
    'http_req_duration{path:msg-push}': ['p(95)<300'],
    'http_req_failed{path:msg-push}': ['rate<0.005'],
    // 全局兜底：任一 check 失败率 > 0.5% 立刻整轮失败
    'checks': ['rate>0.995'],
  },
  // 控制台打印更详细的失败 sample，方便定位
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

export function setup() {
  if (!JWT) {
    throw new Error('JWT env var is required. Run /api/user/login first to get a token.');
  }
  return { jwt: JWT, agentId: AGENT_ID };
}

const headers = (jwt) => ({
  'Content-Type': 'application/json',
  Authorization: `Bearer ${jwt}`,
});

export default function (data) {
  const payload = JSON.stringify({
    agent_id: data.agentId,
    user_id: `vu-${__VU}-iter-${__ITER}`,
    msg: '你好，请帮我介绍下你自己',
    channel: 'dify',
  });

  const res = http.post(`${BASE_URL}/api/msg/push`, payload, {
    headers: headers(data.jwt),
    tags: { path: 'msg-push' },
  });

  check(res, {
    'status 200': (r) => r.status === 200,
    'message ok': (r) => {
      try {
        const body = r.json();
        // chat-api 的响应包是 { code, message, data: { message: "ok" } } 或类似
        return (
          (body && body.data && body.data.message === 'ok') ||
          (body && body.message === 'ok')
        );
      } catch (e) {
        return false;
      }
    },
  });

  // 每个 VU 之间留 1s 模拟真人对话间隔，避免直接拍满 CPU 失真
  sleep(1);
}
