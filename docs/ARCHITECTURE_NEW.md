好，这一块如果设计不对，后面你多 Agent / 多引擎 / HA 基本都会推倒重来。我直接给你一个**可以落地、支持多实例 + 多引擎 + 可扩展**的完整设计。

---

# 一、先把核心问题说透（你到底在设计什么）

你要做的不是一个简单的 Runtime，而是：

> **一个“Agent 执行调度内核（Execution Kernel）”**

它要解决 5 件事：

1. 多 Agent Runtime（OpenClaw / 自研 / ClaudeCode）
2. 多实例（横向扩展）
3. 任务调度（队列 / 优先级 / 隔离）
4. 流式执行（Streaming / Event）
5. 高可用（失败重试 / 超时 / 心跳）

---

# 二、整体架构（建议直接按这个落地）

```text
                 ┌────────────────────────┐
                 │     Agent Layer        │
                 │ (决策 / Workflow 编排) │
                 └──────────┬─────────────┘
                            ↓
                ┌─────────────────────────┐
                │   Agent Runtime Facade  │  ← 统一入口
                └──────────┬──────────────┘
                           ↓
        ┌──────────────────────────────────────┐
        │        Runtime Scheduler              │
        │  - 路由（选哪个 Runtime）              │
        │  - 负载均衡                          │
        │  - 限流 / 优先级                     │
        └───────┬───────────────┬─────────────┘
                ↓               ↓
     ┌────────────────┐   ┌────────────────┐
     │ OpenClaw Pool  │   │ Custom Pool    │
     │ (多实例)       │   │ (多实例)       │
     └────────────────┘   └────────────────┘
                ↓               ↓
         执行结果 / 事件流（统一回传 EventBus）
```

---

# 三、核心抽象设计（最关键）

## 1️⃣ Runtime 抽象接口（统一执行模型）

```go
type AgentRuntime interface {
    Name() string

    Run(ctx context.Context, req *AgentRequest) (*AgentResponse, error)

    Stream(ctx context.Context, req *AgentRequest) (EventStream, error)

    HealthCheck(ctx context.Context) error
}
```

---

## 2️⃣ 请求 / 响应模型（必须统一）

```go
type AgentRequest struct {
    TaskID      string
    AgentID     string
    RuntimeType string // openclaw / custom / claude_code

    Input       any
    Context     map[string]any

    Tools       []ToolSpec
    Timeout     time.Duration
    Priority    int
}
```

```go
type AgentResponse struct {
    TaskID  string
    Output  any
    Status  string // success / failed / timeout
    Error   error
}
```

---

## 3️⃣ Event Stream（支撑流式 + 多 Agent）

```go
type Event struct {
    TaskID   string
    Type     string // token / tool_call / log / result
    Payload  any
    Time     time.Time
}

type EventStream <-chan Event
```

---

# 四、Runtime 注册与发现机制

## Runtime Registry

```go
type RuntimeRegistry struct {
    runtimes map[string][]AgentRuntime // 一个类型多个实例
}
```

```go
func (r *RuntimeRegistry) Register(rt AgentRuntime) {
    r.runtimes[rt.Name()] = append(r.runtimes[rt.Name()], rt)
}
```

---

# 五、调度器设计（核心中的核心）

## 1️⃣ Scheduler 结构

```go
type Scheduler struct {
    registry *RuntimeRegistry

    lb       LoadBalancer
    queue    TaskQueue
}
```

---

## 2️⃣ 调度流程（重点）

```text
1. 接收任务
2. 根据 RuntimeType 选择 runtime group
3. 负载均衡选择一个实例
4. 投递执行
5. 监听执行流
6. 回传 EventBus
```

---

## 3️⃣ 调度代码骨架

```go
func (s *Scheduler) Dispatch(ctx context.Context, req *AgentRequest) error {
    runtimes := s.registry.Get(req.RuntimeType)

    rt := s.lb.Select(runtimes)

    stream, err := rt.Stream(ctx, req)
    if err != nil {
        return err
    }

    go s.handleStream(req.TaskID, stream)

    return nil
}
```

---

# 六、多实例负载均衡（必须支持）

## LoadBalancer 接口

```go
type LoadBalancer interface {
    Select([]AgentRuntime) AgentRuntime
}
```

---

## 常见策略（建议支持 3 个）

### 1️⃣ Round Robin（基础）

```go
type RoundRobin struct {
    idx atomic.Int64
}
```

---

### 2️⃣ Least Load（推荐）

```go
type RuntimeStats struct {
    ActiveTasks int
}
```

👉 选择任务最少的实例

---

### 3️⃣ 权重调度（高级）

```go
type WeightedRuntime struct {
    Runtime AgentRuntime
    Weight  int
}
```

---

# 七、任务队列（支撑削峰 & HA）

## TaskQueue 抽象

```go
type TaskQueue interface {
    Push(req *AgentRequest) error
    Pop(ctx context.Context) (*AgentRequest, error)
}
```

---

## 实现建议

你有 3 个级别选择：

### ✅ 简单版（本地）

* channel + goroutine

### ✅ 生产版（推荐）

* Redis + stream / list

### 🚀 企业级

* Apache Kafka

---

# 八、Runtime Pool（多实例管理）

```go
type RuntimePool struct {
    runtimes []AgentRuntime
    stats    map[string]*RuntimeStats
}
```

---

## 必须做的三件事：

### 1️⃣ 心跳检测

```go
func (p *RuntimePool) HealthCheck() {
    // 定期检查实例状态
}
```

---

### 2️⃣ 自动摘除

```go
if err != nil {
    // 标记实例 unhealthy
}
```

---

### 3️⃣ 自动恢复

```go
// 健康恢复重新加入
```

---

# 九、超时 & 取消机制（必须有）

```go
ctx, cancel := context.WithTimeout(ctx, req.Timeout)
defer cancel()
```

---

## 支持：

* 用户取消任务
* 系统超时
* Agent 崩溃回收

---

# 十、EventBus 集成（你架构的关键）

```text
Runtime → Event → EventBus → UI / Workflow / 其他 Agent
```

👉 强制规则：

> Runtime 不直接操作外部系统，只发事件

---

# 十一、三种 Runtime 接入标准（非常关键）

## 1️⃣ OpenClaw Runtime

* Tool-based
* 强执行能力

---

## 2️⃣ 自研 Runtime

* 轻量 LLM + Skill 调度

---

## 3️⃣ Claude Code Runtime

* 强代码能力
* 文件系统操作

---

👉 统一抽象之后：

```text
Agent 不关心底层是谁
```

---

# 十二、最终架构形态（你应该达到的）

```text
                ┌──────────────┐
                │   Agent      │
                └──────┬───────┘
                       ↓
               ┌──────────────┐
               │  Scheduler   │
               └──────┬───────┘
          ┌───────────┼────────────┐
          ↓           ↓            ↓
   OpenClaw      Custom RT    ClaudeCode
   Cluster       Cluster      Cluster
```

---

# 十三、我帮你点一个关键升级点（很多人做不到）

你可以再往上走一步：

## 👉 Runtime Mesh（下一阶段）

```text
Runtime 不只是被调度
而是可以互相调用
```

例如：

* OpenClaw 调 ClaudeCode 写代码
* ClaudeCode 调 OpenClaw 做 UI 操作

👉 这一步会让你变成：

> **真正的 Agent OS，而不是 Agent Framework**

---

# 总结一句话

> 你设计的不是“调用哪个 Agent”，而是一个“可调度、可扩展、可替换的 Agent 执行内核”
