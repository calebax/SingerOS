# 私有化资源清单

本文档列出 Leros 私有化部署所需的软硬件资源、镜像、端口、存储与配置依赖，供售前评估、资源申请与交付核对使用。本文档针对 **k3s + Helm** 部署路径（`deployments/helm/leros`）。

## 1. 部署架构概览

私有化以单节点 k3s 集群为推荐形态，通过 Helm Chart 部署以下组件：

| 组件 | 角色 | 必选/可选 |
|------|------|:---:|
| `server` | Leros HTTP API 服务（控制平面） | 必选 |
| `worker` | 任务执行 Worker（工作平面） | 必选 |
| `web` | 前端 Web 软件包（仅用于测试） | 仅测试 |
| `postgresql` | Leros 业务数据库 | 必选 |
| `nats` | 消息队列（JetStream） | 必选 |
| `mysql` | account 统一登录服务数据库 | account 启用时必选 |
| `redis` | account 服务缓存 | account 启用时必选 |
| `account` | IAM/统一登录服务 | 可选（企业版认证） |
| `leros-traefik` | Ingress 控制器（可选，默认复用 k3s 自带） | 可选 |

## 2. 硬件资源

### 2.1 最小配置（单机 k3s，POC/体验）

承载 1 个 server、1 个 worker 及全部中间件，仅跑常用 Office 文档处理，不做大规模并发。

| 资源 | 建议 |
|------|------|
| CPU | 4 核 |
| 内存 | 8 GB |
| 磁盘 | 系统盘 80 GB + 数据盘 200 GB（SSD） |
| 网络 | 千兆内网，对外可访问 |

### 2.2 推荐配置（单机 k3s，生产）

承载 1 个 server、2~3 个 worker，worker 使用 `private` 版镜像（含 TeX Live、FFmpeg、Playwright 等重型组件）。

| 资源 | 建议 |
|------|------|
| CPU | 8 核 |
| 内存 | 32 GB（显式渲染/OCR 任务按需上调） |
| 磁盘 | 系统盘 80 GB + 数据盘 500 GB（SSD，建议 RAID1/10） |
| 网络 | 千兆内网，对外可访问 |

> `worker-base`（`private` 版）镜像约 4~5 GB；TeX Live、Playwright Chromium、FFmpeg 等重型组件主要在作业时占用内存与 CPU。需要大模型外部 API 出口，或通过 ModelRouter 代理时需具备外网/专线访问。

### 2.3 节点约束

- 私有化所有组件使用 `hostPath` 持久化，**所有组件须固定在同一节点**（通过 `nodeSelector` 指定 `kubernetes.io/hostname`），否则数据目录不共享。见 `values.yaml` 顶层的 `nodeSelector` 与 `dataHostPath`。

### 2.4 组件资源建议（Helm `resources`）

| 组件 | requests（CPU / 内存） | limits（CPU / 内存） |
|------|------|------|
| server | 100m / 128Mi | 1 / 1Gi |
| worker | 10m / 128Mi | 1 / 2Gi |
| web | 100m / 128Mi | 500m / 512Mi |
| postgresql | 50m / 128Mi | 500m / 512Mi |
| nats | 50m / 64Mi | 300m / 256Mi |
| mysql | 50m / 128Mi | 500m / 512Mi |
| redis | 50m / 64Mi | 300m / 256Mi |
| account | 10m / 64Mi | 300m / 512Mi |
| leros-traefik | 50m / 64Mi | 500m / 256Mi |

> 上表为本交付的**建议值**，是 `values.yaml` 配置参考；与 Chart 默认（`deployments/helm/leros/values.yaml.template`）可不同，部署时按需在 `resources` 覆盖即可。小内存环境（最小配置）可将 worker limits 内存进一步下调，重任务（PDF/OCR）再上调。

## 3. 镜像清单

默认私有镜像仓库 `registry.yygu.cn`（内网）。构建与版次细节见 `deployments/build/README.md`。

### 3.1 业务镜像

| 镜像 | 说明 | 版次 |
|------|------|------|
| `registry.yygu.cn/insmtx/leros:latest` | Leros 服务器 | server |
| `registry.yygu.cn/insmtx/leros-worker:latest` | Worker（FROM `leros-worker-base`） | worker |
| `registry.yygu.cn/insmtx/leros-worker-base:saas` | Worker 基础镜像（简化版） | saas |
| `registry.yygu.cn/insmtx/leros-worker-base:private` | Worker 基础镜像（全量版） | private |
| `registry.yygu.cn/ygapp/account-api:v0.1.0` | IAM/统一登录 | account |

### 3.2 中间件镜像

| 镜像 | 用途 |
|------|------|
| `registry.yygu.cn/library/postgres:18.4` | PostgreSQL（Leros 业务库） |
| `registry.yygu.cn/library/nats:2.12.7` | NATS JetStream |
| `registry.yygu.cn/library/mysql:8.4` | account 的 MySQL |
| `registry.yygu.cn/library/redis:7.4` | account 的 Redis |
| `registry.cn-beijing.aliyuncs.com/yygu/corekg:busybox_1.36.1` | Worker workspace 初始化镜像 |
| `registry.yygu.cn/rancher/mirrored-library-traefik:3.3.6` | Traefik（可选） |

> 私有化通常将镜像预导入节点本地（`docker load` / `ctr -n k8s.io images import`），本地镜像无需镜像拉取凭证，相应关闭 `imagePullSecret`。若客户未预导入而走内网镜像仓库拉取，则需提前同步并配置内网仓库及凭证。

### 3.3 worker-base 版次差异（`private` 相对 `saas`）

| 组件 | saas | private |
|---|:---:|:---:|
| PDF/LaTeX（TeX Live 全量含中文字体） | ❌ | ✅ |
| Poppler、Ghostscript | ❌ | ✅ |
| ImageMagick、Inkscape、Matplotlib | ❌ | ✅ |
| FFmpeg | ❌ | ✅ |
| Tesseract OCR（eng + chi_sim + osd） | ❌ | ✅ |
| Playwright + Chromium | ❌ | ✅ |
| 其余（LibreOffice、Pandoc、CJK 字体、Python/Node 文档库、claude-code/codex/opencode） | ✅ | ✅ |

> 私有化客户常需自由生成 PDF（含中文排版）、识别扫描件，推荐使用 `private` 版 worker。

## 4. 端口清单

### 4.1 中间件端口（集群内部）

NATS 默认开启认证，端口不对集群外暴露。访问方式以 Helm Service（ClusterIP）为准。

### 4.2 对外访问端口

本方案部署独立 Traefik（`leros-traefik`）+ Ingress，无域名时用 NodePort 访问。

| 端口 | 用途 | 来源 |
|------|------|------|
| `38081`（hostPort/NodePort） | Leros HTTP API / Web 前端 / account | `traefik`（错开 k3s 已占用的 80） |
| `8443`（hostPort/NodePort） | HTTPS | `traefik`（错开 k3s 已占用的 443） |

## 5. 持久化存储

| 宿主机目录（`dataHostPath` 默认 `/data/leros`） | 用途 |
|------|------|
| `<dataHostPath>/postgresql` | PostgreSQL 数据 |
| `<dataHostPath>/nats` | NATS JetStream 数据 |
| `<dataHostPath>/mysql` | account MySQL 数据（启用时） |
| `<dataHostPath>/redis` | account Redis 数据（启用时） |
| `<dataHostPath>/workspace` | Worker 工作空间 |
| `<dataHostPath>/storage` | 产物/文件存储（`storage.localDir`） |

> 单点 `hostPath` 建议关联独立数据盘，并纳入客户备份策略。多副本/多节点 HA 需改用共享存储或外部中间件。

## 6. 外部依赖

| 依赖 | 用途 | 必选 |
|------|------|:---:|
| 大模型 API（OpenAI / Anthropic / DeepSeek 等）或 ModelRouter 代理 | LLM 推理 | ✅ |
| 邮件/短信通道（默认需配置） | 通知、验证码 | 视产品需要 |
| 企业微信/GitHub/GitLab 等连接器回调 | 渠道集成 | 视产品需要 |
| IAM 服务（企业版认证时才需要） | 统一登录 | account/IAM 启用时 |

## 7. 附：上线核对清单

- [ ] 镜像已预导入节点本地（或已同步至内网仓库）；预导入时 `imagePullSecret` 已关闭
- [ ] 所有组件固定同一节点（`nodeSelector`）
- [ ] `dataHostPath` 数据盘已挂载并纳入备份
- [ ] JWT Secret、NATS 口令、数据库口令、存储签名密钥均已替换为随机强口令（默认由 `gen-values.sh` 生成）
- [ ] LLM `apiKey` 已填写
- [ ] 域名与 TLS 证书已配置（`ingress` / Traefik）
- [ ] 外网/专线到模型服务链路验证通过
