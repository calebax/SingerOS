# 静态资源预签名服务设计

## 概述

升级 storage-go 到 v0.0.4，当 storage 驱动为 local 时注册静态资源预签名路由，提供基于签名的上传和下载 URL。

## 目标

1. 升级 `github.com/ygpkg/storage-go` 到 `v0.0.4`
2. local 驱动下注册 `/v1/static/` 路由，基于 storage-go 的 `PresignPutObject` / `PresignGetObject` 提供预签名 URL
3. 新增对应单元测试

## 路由

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/v1/static/presign-upload` | 获取预签名上传 URL |
| `GET` | `/v1/static/presign-download` | 获取预签名下载 URL |

### POST `/v1/static/presign-upload`

```
请求: { "key": "path/to/file.png", "bucket": "my-bucket" }
响应: { "url": "http://...?signature=...", "expires_at": "2026-06-13T12:00:00Z" }
```

- `key` 必填，`bucket` 可选用默认值
- TTL 由服务端内部控制（1 小时）

### GET `/v1/static/presign-download?key=path/to/file.png&bucket=my-bucket`

```
响应: { "url": "http://...?signature=...", "expires_at": "2026-06-13T12:00:00Z" }
```

- `key` 必填，`bucket` 可选用默认值
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

- bucket 为空时默认用 `DefaultBucket()`
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

```
POST /v1/static/presign-upload { "key": "uploads/foo.png", "bucket": "xxx" }
  → static_handler.go 解析 key + bucket
  → filestore.PresignUpload(ctx, bucket, key)
  → bucket 空则 DefaultBucket()
  → storage.PresignPutObject(ctx, bucket, key, ttl)
  → 返回预签名 URL + expires_at

GET /v1/static/presign-download?key=uploads/foo.png&bucket=xxx
  → static_handler.go 解析 query 参数
  → filestore.PresignDownload(ctx, bucket, key)
  → bucket 空则 DefaultBucket()
  → storage.PresignGetObject(ctx, bucket, key, ttl)
  → 返回预签名 URL + expires_at
```

## 约束

- 路由仅在 storage 驱动为 `local` 时注册
- 无需鉴权，预签名 URL 本身就是安全凭证
- 签名机制由 storage-go v0.0.4 库提供，Leros 侧不自行实现签名
- 遵循项目现有蛇形命名（snake_case）和 handler 注册模式
