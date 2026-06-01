# 渠道消息接入框架

## 目标

渠道网关负责接收不同外部平台的消息或 Webhook，并统一转换为 Leros 内部事件后发布到事件总线。业务编排、任务派发和 worker 执行都不直接依赖具体平台。

## 架构

```
外部平台 -> Connector -> Channel Gateway Registry -> EventBus -> 服务端编排 -> Worker Task -> Worker
```

- `backend/internal/api/connectors/`：具体平台连接器，只处理平台协议、签名校验、事件解析和路由注册。
- `backend/internal/api/gateway/`：渠道注册与装配层，负责根据配置启用连接器，避免在 API router 中硬编码平台分支。
- `backend/internal/infra/mq/`：事件总线边界，连接器将标准事件发布到 MQ。
- `backend/internal/worker/`：只消费任务消息和调用运行时，不读取数据库。

## 新增渠道流程

1. 在 `backend/internal/api/connectors/<channel>/` 实现 `connectors.Connector`。
2. 将平台配置放到 `backend/config/` 并挂到 `config.Config`。
3. 在 `backend/internal/api/gateway/channel_gateway.go` 的 `DefaultRegistry` 增加一个 `ChannelEntry`。
4. 在连接器内完成平台消息到内部事件的转换，并发布到对应 topic。
5. 为 registry 启用逻辑、事件转换和签名校验补测试。

## Worker 边界

Worker 不参与数据库使用。数据库只属于服务端控制面，例如 API handler、service、runnable 或 server-side gateway 需要持久化授权信息时使用。Worker 通过 NATS/HTTP 获取任务和上报结果，因此 `WorkerConfig` 不包含数据库配置。
