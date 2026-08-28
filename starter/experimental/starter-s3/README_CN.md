# starter-s3

[English](README.md) | [中文](README_CN.md)

`starter-s3` 为 Go-Spring 提供 S3 协议对象存储支持：多实例
`*minio.Client` bean、fail-fast 启动探针（列举桶）、逐请求可观测
（span + 指标 + 访问日志）、韧性（HTTP 传输层限流/熔断/故障注入），以及
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
spring.s3.a.endpoint=127.0.0.1:9000
spring.s3.a.access-key-id=minioadmin
spring.s3.a.secret-access-key=minioadmin
spring.s3.a.region=us-east-1
spring.s3.a.use-ssl=false
spring.s3.a.bucket-lookup=auto
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

包装类型内嵌 `*minio.Client`，SDK 的所有方法原样提升可用。

## 核心特性

- **多实例客户端** — 每个 `spring.s3.<name>` 条目都是独立 bean，拥有
  各自的配置。
- **fail-fast 启动探针** — 启动期做一次 `ListBuckets` 往返，第一个对象
  操作之前就暴露配错的端点与被拒的凭证。
- **每实例健康指示器** — 同一探针注册为 `s3:<name>`，导入
  `starter-actuator` 后自动并入 `/readiness`。
- **可观测** — 每个请求经 observe kit 产出 OTel span、耗时指标与访问
  日志（`spring.s3.<name>.observability.level=off` 关闭；顶层
  `observability.*` 作为回退面，被实例 key 覆盖）。minio-go 自身不带
  OTel 埋点，因此由 starter 的传输层承载全部三信号。
- **韧性** — 限流、熔断、故障注入在客户端 HTTP 传输层经治理 seam 强制
  执行；未导入 `starter-governance` 时传输层仅做观测。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.s3.assets.endpoint=127.0.0.1:9000
spring.s3.assets.access-key-id=...
spring.s3.assets.secret-access-key=...
```

**自定义 driver** — 注册自己的 `Driver` 并用 `driver=<name>` 选中，替换
客户端装配（例如接入 IAM 角色凭证或自定义 `http.Transport`）：

```go
func init() {
    StarterS3.RegisterDriver("iam", iamDriver{})
}
```
