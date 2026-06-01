# SingerOS Gateway 消息网关

## 概述

SingerOS Gateway 是一个**独立进程**，负责连接多个外部消息平台（QQ、飞书、WhatsApp、GitHub 等），将平台的原始消息标准化后发布到 NATS 事件总线，并将下游 Agent 的回复路由回对应的平台。

Gateway 不承载 Agent 会话执行——它只是一个"管道"，专心做好连接、鉴权、标准化和路由。

## 项目位置

```
backend/gateway/       # 独立 Go Module（不依赖主项目）
├── cmd/gateway/       # 进程入口（Cobra CLI）
├── adapters/          # 各平台适配器
│   ├── github/        # GitHub Webhook
│   ├── qqbot/         # QQ Bot（WebSocket + QR 扫码绑定）
│   ├── feishu/        # 飞书/Lark（WebSocket + Webhook 双模式）
│   └── whatsapp/      # WhatsApp（Node.js Bridge 子进程）
├── pkg/               # 共享公共库
│   ├── types/         # 核心类型定义
│   ├── core/          # 适配器接口 + 注册表
│   ├── dispatch/      # 消息网关 + 鉴权 + 路由
│   ├── config/        # 配置加载 + 凭据存储
│   ├── store/         # 适配器状态持久化
│   ├── chunker/       # 出站消息分块器
│   ├── webhook/       # Webhook 安全守卫
│   ├── onboard/       # 平台绑定流程引擎
│   └── infra/         # 基础设施（NATS Publisher）
└── config.example.yaml
```

---

## 架构设计

### 分层架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Gateway 进程                                     │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │  入站 Pipeline:                                                     │  │
│  │    平台事件 → normalize → MessageEnvelope                           │  │
│  │    → dedup → middleware(Continue/Drop/Handled) → auth              │  │
│  │    → NATS Publish (interaction.{channel}.{type})                   │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │  出站 Pipeline:                                                     │  │
│  │    DeliveryTarget 解析 (origin/local/{ch}:{chat}:{thread})         │  │
│  │    → TextChunker (段落→换行→空格→硬切分)                           │  │
│  │    → adapter.Send() → 平台 SDK/API                                 │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌─────────┐                      │
│  │ QQ Bot  │ │ 飞书    │ │ WhatsApp │ │ GitHub  │                      │
│  │ (WS)    │ │ (WS/WH) │ │ (Bridge) │ │ (WH)    │                      │
│  └─────────┘ └─────────┘ └──────────┘ └─────────┘                      │
└──────────────────────────────────────┬──────────────────────────────────┘
                                       │ NATS JetStream
                                       ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      SingerOS Backend（主进程）                          │
│                                                                          │
│  后续阶段: Interaction Orchestrator                                      │
│    - 消费 interaction.{channel}.* 话题                                   │
│    - 分发到 Agent Runtime                                                │
└─────────────────────────────────────────────────────────────────────────┘
```

### 适配器接口体系（Interface Segregation）

每个平台适配器不必实现所有接口，只实现自己需要的：

```
Connector              — 所有适配器必须实现：身份元数据、能力声明
                         ├── Info() AdapterInfo
                         
Lifecycle              — 需要长连接/WebSocket 的平台（QQ、飞书、WhatsApp）
                         ├── Connect(ctx) error
                         ├── Disconnect(ctx) error  
                         └── Health(ctx) error
                         
Receiver               — 入站消息消费
                         └── OnMessage(callback) error
                         
Sender                 — 出站消息发送
                         ├── Send(ctx, target, msg) error
                         └── SendTyping(ctx, target) error

WebhookReceiver        — HTTP Webhook 事件接收
                         └── RegisterWebhookRoutes(mux) error

ManagedProcess         — 管理外部子进程（仅 WhatsApp Bridge）
                         ├── Pid() int
                         ├── Start(ctx) / Stop(ctx) / Restart(ctx)
```

各平台实现情况：

| 接口 | GitHub | QQ Bot | 飞书 | WhatsApp |
|------|--------|--------|------|----------|
| Connector | ✓ | ✓ | ✓ | ✓ |
| Lifecycle | ✓ | ✓ | ✓ | ✓ |
| Receiver | ✓ | ✓ | ✓ | ✓ |
| Sender | ✗ | ✓ | ✓ | ✓ |
| WebhookReceiver | ✗ | ✗ | ✓ | ✗ |
| ManagedProcess | ✗ | ✗ | ✗ | ✓ |

### 消息标准化

所有平台事件被统一转换为 `MessageEnvelope`，下游消费者只需处理这一种类型：

```go
type MessageEnvelope struct {
    MessageID       string            // 消息唯一 ID
    TraceID         string            // 分布式追踪 ID
    Channel         ChannelCode       // 来源平台
    MessageType     MessageType       // text / image / event / ...
    SessionKey      SessionKey        // 会话标识（平台+聊天类型+聊天ID+用户ID）
    Sender          SenderInfo        // 发送者信息
    Content         MessageContent    // 消息内容
    Attachments     []Attachment      // 媒体附件（标准化格式）
    Mentions        []Mention         // @提及列表
    CommandHint     *string           // 命令/按钮回调标记
    TransportMeta   *TransportMeta    // 传输层元数据（context token、cursor）
    CapabilitiesHint *CapabilitiesHint // 平台能力提示
}
```

### 中间件 Pipeline

入站消息经过 Middleware 链处理，每个中间件可返回 4 种动作：

| 返回值 | 语义 | 示例场景 |
|--------|------|----------|
| `Continue` | 继续下一个中间件 | 日志记录、统计 |
| `Drop` | 静默丢弃，不发 NATS | 噪音过滤、重复消息、非目标群聊 |
| `Handled` | 已被本地消费，不发 NATS | URL 验证、配对 ACK、健康检查 |
| `error` | 处理失败 | 异常情况 |

中间件按注册顺序执行，第一个非 `Continue` 的结果会短路后续 pipeline。

### TextChunker 分块策略

出站长消息按 4 级降级分块（`TextFormatMode` 区分 plain/markdown）：

```
段落边界 (\n\n)
    ↓ 仍超长
换行边界 (\n)
    ↓ 仍超长  
单词边界 (空格)
    ↓ 仍超长
硬切分（固定长度截断）
```

---

## 启动方式

### 前置条件

- Go 1.24+
- NATS Server（用于事件发布，运行本机即可）
- 本机安装 `node`（仅 WhatsApp 平台需要）

### 构建

```bash
cd backend/gateway
go build -o ../../bundles/gateway ./cmd/gateway/
```

### 启动

```bash
# 方式一：构建后运行
../../bundles/gateway run --config gateway.yaml

# 方式二：go run 直接运行（开发调试推荐）
go run ./cmd/gateway/ run --config gateway.yaml

# 方式三：设置别名
alias gw="go run ./cmd/gateway/"
gw setup --platform qqbot
gw run --config gateway.yaml
```

### 配置（非敏感）

从 `config.example.yaml` 复制一份：

```yaml
server:
  host: "0.0.0.0"
  port: 8080

nats:
  url: "nats://localhost:4222"

channels:
  github:
    enabled: false
  qqbot:
    enabled: true
  feishu:
    enabled: false
  whatsapp:
    enabled: false
```

### 平台凭据绑定

QQ Bot 支持扫码绑定（推荐）：

```bash
go run ./cmd/gateway/ setup --platform qqbot
```

飞书、WhatsApp 等其他平台目前需要手动输入凭据：

```bash
go run ./cmd/gateway/ setup --platform feishu
```

也可以直接编辑 `~/.singeros/credentials.env`：

```
QQBOT_APP_ID=12345678
QQBOT_CLIENT_SECRET=abcdef1234567890
FEISHU_APP_ID=cli_xxxxxxxx
FEISHU_APP_SECRET=xxxxxxxxxxxxxxxx
```

### 启动网关

```bash
go run ./cmd/gateway/ run --config gateway.yaml
```

启动日志示例：

```
Connected to NATS at nats://localhost:4222
Connecting github...
github connected
Connecting qqbot...
qqbot connected
Gateway started with 2 channel(s)
```

---

## 设计思路和灵感来源

SingerOS Gateway 的设计参考了三个优秀的开源项目，各取其长：

| 参考项目 | 借鉴内容 |
|----------|----------|
| **Hermes-Agent** | 多平台 `BasePlatformAdapter` 抽象 + `MessageEvent` 标准化 + WhatsApp Node Bridge 模式 + Feishu WebSocket/Webhook 双模 |
| **wechatbot** | Adapter 内部按 `protocol/auth/transport/messaging/media/storage` 分层 + Middleware 的 `handled` 短路语义 + TextChunker |
| **pi-agent** | RPC 子进程管理模式 + `Operations` 接口的可扩展性思路 |

### 与 Hermes 的设计差异

1. **进程模型**：Hermes 将 Gateway 和 Agent 放在同一进程中；SingerOS 将 Gateway 独立为单独进程，通过 NATS 与主项目解耦。
2. **中间件**：Hermes 没有中间件链，而是用 `pre_gateway_dispatch` 广播钩子 + `_admit()` 前置过滤。SingerOS 引入了更结构化的 Middleware Pipeline。
3. **凭据存储**：Hermes 用 `~/.hermes/.env` 环境变量文件；SingerOS 用 `~/.singeros/credentials.env` + StateStore 两层存储（凭据 vs 运行时状态）。
4. **编程语言**：Hermes 用 Python asyncio；SingerOS 用 Go + goroutine，天然适合高并发连接管理。

### 与 wechatbot 的设计差异

1. **wechatbot 是单平台 SDK**，不是多平台网关。SingerOS 借鉴了它的内部分层模式（特别是 `MiddlewareEngine` 的短路机制和 `MessagePoller` 的轮询循环），但将其扩展到多平台场景。
2. **Storage 接口**：wechatbot 用 `get/set/delete/has/clear` 5 个方法；SingerOS 用 `Get/Set/Delete` 3 个方法 + TTL，更简洁。

---

## 实现新的平台适配器

接入新平台的步骤分为 6 步。以"钉钉"为例演示完整流程。

### 第 1 步：创建适配器目录

```bash
mkdir -p backend/gateway/adapters/dingtalk
```

### 第 2 步：定义平台常量

```go
// adapters/dingtalk/constants.go
package dingtalk

const (
    APIBaseURL = "https://api.dingtalk.com"
)

const (
    MsgTypeText = "text"
    MsgTypeImage = "image"
    ChatTypeDM = "dm"
    ChatTypeGroup = "group"
)
```

### 第 3 步：实现核心接口

**最小必须实现**：`Connector` + `Receiver`（如果只想收消息）

**完整实现**：`Connector` + `Lifecycle` + `Receiver` + `Sender`

以 WebSocket 平台为例：

```go
// adapters/dingtalk/adapter.go
package dingtalk

import (
    "context"
    "github.com/insmtx/Leros/backend/gateway/pkg/core"
    "github.com/insmtx/Leros/backend/gateway/pkg/types"
)

type Adapter struct {
    cfg      types.ChannelConfig
    callback core.MessageCallback
    // 平台特有的连接、token 等字段
}

func NewAdapter(cfg types.ChannelConfig) *Adapter {
    return &Adapter{cfg: cfg}
}

// --- Connector ---
func (a *Adapter) Info() types.AdapterInfo {
    return types.AdapterInfo{
        Code:        "dingtalk",
        Label:       "钉钉",
        Description: "DingTalk bot via WebSocket stream mode",
        Version:     "1.0.0",
        Capabilities: types.ChannelCapabilities{
            SupportsIM:    true,
            SupportsStream: false,
            NeedsLongConn:  true,
            MaxMessageLen:  2048,
        },
    }
}

// --- Lifecycle ---
func (a *Adapter) Connect(ctx context.Context) error {
    // 建立 WebSocket 连接
    // 启动消息接收循环
    return nil
}

func (a *Adapter) Disconnect(ctx context.Context) error {
    // 关闭 WebSocket 连接
    return nil
}

func (a *Adapter) Health(ctx context.Context) error {
    // 返回 nil 表示正常运行
    return nil
}

// --- Receiver ---
func (a *Adapter) OnMessage(callback core.MessageCallback) error {
    if a.callback != nil {
        return fmt.Errorf("OnMessage already registered")
    }
    a.callback = callback
    return nil
}

// --- Sender (出站消息) ---
func (a *Adapter) Send(ctx context.Context, target string, msg types.OutboundMessage) error {
    // 调用钉钉 API 发送消息
    return nil
}

func (a *Adapter) SendTyping(ctx context.Context, target string) error {
    return nil
}
```

### 第 4 步：消息标准化

将平台原始消息转为 `MessageEnvelope`：

```go
// adapters/dingtalk/normalizer.go
func (a *Adapter) normalize(raw platformMsg) *types.MessageEnvelope {
    return &types.MessageEnvelope{
        MessageID:   raw.MsgID,
        TraceID:     genTraceID(),
        Channel:     "dingtalk",
        MessageType: types.MessageTypeText,
        SessionKey: types.SessionKey{
            Channel:  "dingtalk",
            ChatType: types.ChatTypeDM,
            ChatID:   raw.ChatID,
            UserID:   raw.SenderID,
        },
        Sender: types.SenderInfo{
            UserID:   raw.SenderID,
            Username: raw.SenderName,
        },
        Content: types.MessageContent{
            Text: raw.Text,
        },
        Attachments: attachments,
        Mentions:    mentions,
    }
}

// 收到消息后调用回调
func (a *Adapter) onMessage(raw platformMsg) {
    env := a.normalize(raw)
    a.callback(ctx, env)
}
```

### 第 5 步：注册到 Gateway

在 `cmd/gateway/adapters.go` 中添加工厂函数：

```go
import "github.com/insmtx/Leros/backend/gateway/adapters/dingtalk"

func newDingTalkAdapter(cfg types.ChannelConfig) *dingtalk.Adapter {
    return dingtalk.NewAdapter(cfg)
}
```

在 `cmd/gateway/main.go` 的 `registerBuiltins` 中添加注册条目：

```go
registry.MustRegister(core.ChannelEntry{
    Code:        "dingtalk",
    Label:       "钉钉",
    Description: "DingTalk bot via WebSocket stream mode",
    Version:     "1.0.0",
    Order:       220,
    Capabilities: types.ChannelCapabilities{
        SupportsIM:    true,
        NeedsLongConn:  true,
        MaxMessageLen:  2048,
    },
    Enabled: func() bool {
        chCfg, ok := cfg.Channels["dingtalk"]
        return ok && chCfg.Enabled
    },
    Factory: func(chCfg types.ChannelConfig) (any, error) {
        return newDingTalkAdapter(chCfg), nil
    },
})
```

### 第 6 步：配置示例

在 `config.example.yaml` 中添加：

```yaml
channels:
  dingtalk:
    enabled: false
    extra:
      app_key: "dingxxxxxxxxxxxx"
      app_secret: "xxxxxxxxxxxxxxxxxxxx"
```

### Webhook 平台额外步骤

如果平台是纯 Webhook 模式（如 GitHub），不需要实现 `Lifecycle.Connect()` 中的长连接逻辑，而是实现 `WebhookReceiver`：

```go
func (a *Adapter) RegisterWebhookRoutes(mux *http.ServeMux) error {
    mux.HandleFunc("/webhooks/dingtalk", a.handleWebhook)
    return nil
}
```

Gateway 会在启动时为所有 `WebhookReceiver` 注册 HTTP 路由。

### 凭据绑定（可选）

如果平台支持 QR 码扫码绑定，可以实现 `onboard.PlatformOnboarder` 接口：

```go
type DingTalkOnboarder struct { ... }

func (o *DingTalkOnboarder) Init(ctx) (state, error)    { /* 初始化绑定 */ }
func (o *DingTalkOnboarder) Begin(ctx, state) (...)      { /* 生成 QR 码 */ }
func (o *DingTalkOnboarder) Poll(ctx, state, token) (...) { /* 轮询结果 */ }
func (o *DingTalkOnboarder) Decrypt(ctx, state, raw) (...) { /* 解密凭据 */ }
func (o *DingTalkOnboarder) Probe(ctx, result) error     { /* 验证凭据 */ }
```

然后在 `cmd/gateway/setup.go` 中添加对应的 `setupDingTalk()` 函数。

---

## 数据流示例

以 QQ Bot 为例，完整的一次"用户发消息 → Agent 回复"的链路：

```
1. 用户在 QQ 群 @机器人 "帮我查天气"

2. QQ Server → WebSocket → wsConn.readLoop()
   → OpCode 0 Dispatch → t="GROUP_AT_MESSAGE_CREATE"

3. Adapter.handleGroupMessage() 
   → normalizeGroup() → MessageEnvelope{
        Channel: "qqbot",
        MessageType: "text",
        SessionKey: {ChatType: "group", ChatID: "grp_xxx", UserID: "user_xxx"},
        Content: {Text: "帮我查天气"},
        Mentions: [{UserID: "bot_openid"}],
     }

4. Adapter.dispatch() → callback → MessageGateway.Handle()

5. Pipeline:
   去重检查 → ✓ 
   Middleware 链 → Continue
   鉴权检查 → ✓
   Publish("interaction.qqbot.text", envelope) → NATS

6. [后续阶段] Interaction Orchestrator 消费 → Agent Runtime 处理
   → 产出回复 "今天北京晴，25°C"

7. [后续阶段] 回复路由回 Gateway → DeliveryRouter
   → Adapter.Send(ctx, "grp_xxx", OutboundMessage{Text: "今天北京晴，25°C"})
   → QQ REST API POST /v2/groups/grp_xxx/messages → 用户收到回复
```

---

## 关键设计决策 FAQ

**Q: 为什么 Gateway 是独立进程而不是嵌入主项目的 Gin 路由？**

A: 长连接平台（QQ Bot WebSocket、飞书 WebSocket、WhatsApp Bridge）需要独立的连接生命周期管理，不适合嵌入请求-响应模型的 HTTP Server。独立进程可以独立扩缩容、独立重启、独立监控。

**Q: 为什么凭据和配置分开存（credentials.env + config.yaml）？**

A: `credentials.env` 存敏感信息（app_secret、token），`config.yaml` 存非敏感策略配置（enabled、group_policy）。两者在网关启动时自动合并。运维上可以给 `credentials.env` 更严格的权限控制。

**Q: 入站 Pipeline 中 drop 和 handled 的区别？**

A: `drop` 是 policy/filter 结果（噪音、非目标群聊），不做任何处理。`handled` 是 Gateway 本地已消费（URL 验证、配对 ACK），不需要再发 NATS。两者分开统计，便于监控和问题排查。

**Q: 旧的 `backend/internal/api/gateway/` 什么时候删除？**

A: 等 GitHub webhook 入口完全迁移到独立 Gateway 进程后删除。目前仅标记 `Deprecated`，不作为新平台接入入口。
