# websocket-service

## 一个简单的 websocket 消息推送方案

## bucket 或者叫 room
- 通过 ws://127.0.0.1:9501/ws 连接上 websocket 服务后
- 默认会将 用户分配至 未授权的 bucket 

- 在用户认证成功后，会将用户分配至 已授权 bucket

- 开放api
    - http://127.0.0.1:9501/ws/status 用户状态查询
    - 查询参数（无）
    - 响应参数
```json
{
  "code": 200,
  "data": {
    "auth_len": 1,
    "auth_msg": "client 558338675 ,created_at 2023-12-23 14:53:45 updated_at 2023-12-23 14:55:46 \r\n",
    "un_auth_len": 0,
    "un_auth_msg": ""
  },
  "msg": "success"
}
```
  - http://127.0.0.1:9501/ws/send   给(全体/指定)用户推送消息 
  - 查询参数 
    - to_user `可以为发送人id  多人发送 id,id 如 12,13  全局发送 all	`
    - data `object` 自定义数据
    - msg `string` 发送提示消息
```json
{
    "to_user": "12",
    "data": {},
    "msg": "有新工单来了~"
}
```
  - 响应参数
```json
{"code":200,"data":[],"msg":"成功给1个用户发送消息"}
```
  - 这两个http api 满足目前的业务需要


## 业务码
- 200: 业务消息推送
- 201: auth 成功
- 202: 客户端 heartbeat 接受成功
- 203: 服务器心跳（每分钟服务器会下发一次心跳包）

- 501: 认证失败
- 502: 心跳包确认失败
- 503: 超时未认证/认证后未进行心跳保活，已被服务器主动踢掉

## 认证机制，目前设计为 结合 token 进行

- 连接后 ws.send(msg), 即可完成验证
- 请求参数 `{"action":"auth","token":"yourSystemToken"}`
- 成功后 onmessage 回调收到 `{"code":201,"msg":"success","data":{}}` 的回调消息

- 未认证的连接，将会收到 `{"code":501,"msg":"auth fail xxx","data":{}}` 的回调消息

## 心跳机制&断线重连

- 因为 `websocket` 的机制，服务端并不能即时的感知客户端的离线，主流是客户端主动发起心跳包来维持状态
- 目前对于未认证的连接，1分钟未进行认证授权，服务端将清理维护状态，并下发离线通知
- 对于已认证的连接，每分钟内需要 发送 一次 `ws.send("{\"action\":\"heartbeat\"}")` 心跳包，服务端才会继续维护其活跃状态
- 当 web 端，断线后(各种原因)，重新做3次连接与认证~