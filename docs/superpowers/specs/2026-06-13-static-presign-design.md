# 静态资源预签名服务设计

## 概述

升级 storage-go 到 v0.0.4，当 storage 驱动为 local 时注册静态资源预签名路由，基于 S3 协议规范提供预签名上传和下载 URL。

## 目标

1. 升级 `github.com/ygpkg/storage-go` 到 `v0.0.4`
2. local 驱动下注册 `/v1/static/` 路由，基于 storage-go 的 `PresignPutObject` / `PresignGetObject` 提供预签名 URL
3. 新增对应单元测试

## 路由

遵循 S3 协议风格，使用 `/{bucket}/{key}` 路径结构，`?presign` 查询参数区分"获取签名 URL"和"直接访问"。

| 方法 | 路径 | 说明 |
|------|------|------|
| `PUT` | `/v1/static/:bucket/*key` | 获取预签名上传 URL |
| `GET` | `/v1/static/:bucket/*key` | 获取预签名下载 URL |

### PUT `/v1/static/:bucket/*key`

```
请求: PUT /v1/static/dev-bucket/path/to/file.png?presign
响应:
  HTTP 200
  Content-Type: text/plain
  X-Presign-Expires-At: 2026-06-13T12:00:00Z

  https://storage.example.com/dev-bucket/path/to/file.png?X-Amz-Algorithm=...
```

- `bucket` 和 `key` 由 URL 路径提取
- `?presign` 查询参数标识此次请求用于获取预签名 URL
- 预签名 URL 以纯文本形式在响应 body 中返回
- 过期时间通过 `X-Presign-Expires-At` 响应头传递
- TTL 由服务端内部控制（1 小时）
- 客户端获取预签名 URL 后，自行通过 HTTP PUT 直接上传文件，PutOptions（Content-Type、Metadata 等）由客户端在 PUT 请求时通过 HTTP Header 自行携带

### GET `/v1/static/:bucket/*key`

```
请求: GET /v1/static/dev-bucket/path/to/file.png?presign
响应:
  HTTP 200
  Content-Type: text/plain
  X-Presign-Expires-At: 2026-06-13T12:00:00Z

  https://storage.example.com/dev-bucket/path/to/file.png?X-Amz-Algorithm=...
```

- `bucket` 和 `key` 由 URL 路径提取
- `?presign` 查询参数标识此次请求用于获取预签名 URL
- 预签名 URL 以纯文本形式在响应 body 中返回
- 过期时间通过 `X-Presign-Expires-At` 响应头传递
- TTL 由服务端内部控制（1 小时）

## 组件变更

### 1. `go.mod` — 升级 storage-go

`github.com/ygpkg/storage-go` `v0.0.3` → `v0.0.4`

v0.0.4 新增 `Ext` 接口中的 `PresignGetObject` / `PresignPutObject` 方法。

### 2. `backend/internal/infra/filestore/init.go` — 新增 `IsLocal()`

```go
var driverType storage.DriverType

func IsLocal() bool {
    return driverType == "local"
}
```

`Init()` 中保存 `driverType`。

### 3. `backend/internal/infra/filestore/presign.go` — 新增预设签名封装

```go
const defaultPresignTTL = 1 * time.Hour

func PresignUpload(ctx context.Context, bucket, key string) (string, time.Time, error)
func PresignDownload(ctx context.Context, bucket, key string) (string, time.Time, error)
```

- 内部调用 `storage.PresignPutObject` / `storage.PresignGetObject`
- 返回预签名 URL 和过期时间

### 4. `backend/internal/api/handler/static_handler.go` — 新增 Handler

`StaticHandler` 结构体，`RegisterRoutes(r gin.IRouter)` 注册两个路由。

### 5. `backend/internal/api/router.go` — 条件注册

```go
if filestore.IsLocal() {
    handler.RegisterStaticRoutes(v1)
    logs.Info("Static routes registered successfully")
}
```

### 6. 测试文件

- `backend/internal/infra/filestore/presign_test.go` — 测试 Presign 封装函数
- `backend/internal/api/handler/static_handler_test.go` — 测试 handler 路由请求/响应

## 数据流

### 获取预签名上传 URL

```
PUT /v1/static/dev-bucket/path/to/file.png?presign
  → static_handler.go 解析 bucket + key，检测 presign 参数
  → filestore.PresignUpload(ctx, bucket, key)
  → storage.PresignPutObject(ctx, bucket, key, 1h)
  → 返回预签名 URL + expires_at

# 客户端后续使用预签名 URL 直接上传
PUT {presigned_url}
  Content-Type: image/png
  x-amz-meta-foo: bar
  <file binary data>
```

### 获取预签名下载 URL

```
GET /v1/static/dev-bucket/path/to/file.png?presign
  → static_handler.go 解析 bucket + key，检测 presign 参数
  → filestore.PresignDownload(ctx, bucket, key)
  → storage.PresignGetObject(ctx, bucket, key, 1h)
  → 返回预签名 URL + expires_at

# 客户端后续使用预签名 URL 直接下载
GET {presigned_url}
  → 返回文件内容
```

## 错误码

| 场景 | HTTP 状态码 | 说明 |
|------|------------|------|
| 缺少 bucket 或 key | 400 | 路径参数不完整 |
| 缺少 `?presign` 参数 | 400 | 未指定 presign 操作 |
| bucket 不存在 / 配置错误 | 500 | storage 操作失败 |
| 非 local 驱动 | 404 | 路由未注册 |

## 约束

- 路由仅在 storage 驱动为 `local` 时注册
- 无需鉴权，预签名 URL 本身就是安全凭证
- 签名机制由 storage-go v0.0.4 库提供，Leros 侧不自行实现签名
- PutOptions（Content-Type、Metadata 等）由客户端在后续 PUT 请求时自行携带，服务端不处理
- 遵循 S3 风格路径 `/{bucket}/{key}` 和项目现有 handler 注册模式
- 响应头命名使用 kebab-case（`X-Presign-Expires-At`），遵循 HTTP 头惯例
