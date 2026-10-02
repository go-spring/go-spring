# starter-s3

[English](README.md) | [中文](README_CN.md)

`starter-s3` 为 Go-Spring 提供 S3 协议对象存储支持：多实例
`*minio.Client` bean、可选 fail-fast 启动探针（列举桶）、逐请求可观测
（starter 声明调用身份，resilience 层发射 span + 指标 + 访问日志）、
韧性（HTTP 传输层限流/熔断/故障注入），以及
每实例健康指示器。基于 [minio-go](https://github.com/minio/minio-go)，
一套配置面覆盖所有 S3 兼容端点。

## 兼容性

- **原生**：MinIO、AWS S3。
- **S3 兼容端点**：阿里云 OSS（`oss-<region>.aliyuncs.com`，需
  `bucket-lookup=path`）、腾讯云 COS（`cos.<region>.myqcloud.com`，需
  `bucket-lookup=path`）以及其他提供 S3 门面的云。端点不支持
  virtual-host 寻址时设 `bucket-lookup=path`；S3 门面之外的厂商特有
  能力（如 OSS 图片处理子资源）不在覆盖范围 —— 需要时用自定义 driver。

## 安装

```bash
go get go-spring.org/starter-s3
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-s3"
```

### 2. 配置

```properties
spring.s3.instances.a.endpoint=127.0.0.1:9000
spring.s3.instances.a.access-key-id=minioadmin
spring.s3.instances.a.secret-access-key=minioadmin
spring.s3.instances.a.region=us-east-1
spring.s3.instances.a.use-ssl=false
spring.s3.instances.a.bucket-lookup=auto
```

### 3. 注入

```go
type Service struct {
    Client *StarterS3.Client `autowire:"a"`
}
```

### 4. 使用

```go
_, err := s.Client.PutObject(ctx, "bucket", "key",
    bytes.NewReader(data), int64(len(data)),
    minio.PutObjectOptions{ContentType: "text/plain"})
```

包装类型是 `*Client`，而非裸 `*minio.Client`：`NewClient` 是唯一构造函数，包装体内嵌
裸 client，SDK 的完整方法集因此原样提升，每次调用仍经过声明 + 治理传输层。

## 核心特性

- **多实例客户端** — 每个 `spring.s3.instances.<name>` 条目都是独立 bean，拥有
  各自的配置。
- **fail-fast 启动探针** — 启动期做一次 `ListBuckets` 往返（opt-in：`ping=true`；
  默认关闭），第一个对象操作之前就暴露配错的端点与被拒的凭证。
- **每实例健康指示器** — 同一探针注册为 `s3:<name>`，导入
  `starter-actuator` 后自动并入 `/readiness`。
- **可观测** — starter 只“声明”每个请求的语义身份（`declareTransport` 把它放到
  context 上）；同一 HTTP 传输层上的 resilience 层在唯一能看到整次调用（含重试）
  的位置“发射”信号。一个 OTel client span（属性 `db.system`/`db.operation`/
  `db.statement`）、`db.client.operation.duration` 直方图与 `db.client.attempt.duration`
  直方图（尝试级——下游每次尝试自身耗时，不含退避）、以及访问日志（tag
  `_app_s3_access`）。`db.system`/`db.operation`（HTTP 方法）有界，可作指标标签；
  `db.statement`（URL path）是逐调用明细，只进 span 与日志，绝不进标签。
  minio-go 自身不带 OTel 埋点。
- **韧性** — 限流、熔断、故障注入在客户端 HTTP 传输层经注入的治理 bean
  在构造期构建的执行器强制执行；手工构建的客户端（无容器治理）降级为仅观测的
  unmanaged 执行器，并告警一次。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.s3.instances.assets.endpoint=127.0.0.1:9000
spring.s3.instances.assets.access-key-id=...
spring.s3.instances.assets.secret-access-key=...
```

**自定义 driver** — 把自己的 `Driver` 作为可选容器 bean 提供，替换
客户端装配（例如接入 IAM 角色凭证或自定义 `http.Transport`）。
`spring.s3` 下每个 client 都经它构建；无该 bean 时 starter 回退到内置
`DefaultDriver`：

```go
func init() {
    gs.Provide(func() StarterS3.Driver { return iamDriver{} })
}
```

## 设计说明

* **凭证是静态且必填的。** 内置 driver 以固定的 access key/secret 认证，两个 key
  都挂 `expr` 必填——公开（匿名）桶需要自定义 `Driver`。轮换凭证是改配置，不是
  运行期调用。
* **不接服务发现。** 一个 S3 实例是一个端点，而不是可发现的服务：与数据库／
  消息队列客户端不同，这里没有 `service-name`／discovery 四件套，`lb://` 也不适用。
* **客户端无物可关。** minio-go 的 client 没有 `Close`，因此实例停机只释放
  resilience 执行器——没有套接字或连接池需要排空。
* **`bucket-lookup` 的两种写法。** minio-go v7.0.74 里 `virtual-host` 是 `dns` 的
  别名；两种写法都可用，且在 7.x 上保持有效。
