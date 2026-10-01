# starter-s3 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

一个 Client 原型 starter（`starter/DESIGN.md` §2.2），经 minio-go 说 S3
协议。结构上镜像 starter-elasticsearch（另一个 HTTP 传输型客户端
starter）；差异点在下面单独说明。

## 1. 职责与边界

- **负责**：`spring.s3.instances.<name>` 组的 bean 生命周期（多实例、容器托管
  停机）、fail-fast `ListBuckets` 探针、每实例健康指示器、声明传输层
  （三信号由 resilience 层发射）、客户端 HTTP 传输上的韧性 round-tripper。
- **不负责**：桶/对象管理策略、凭证轮换（只支持静态密钥 —— 轮换走配置）、
  S3 门面之外的厂商能力（自定义 driver 的地盘）、桶发现（endpoint 就指向
  一个服务，没有 `service-name`/discovery 四件套）。

## 2. 关键抽象与 Seam

- **`Client` 包装 bean** — 内嵌裸 `*minio.Client`，整套 SDK 方法表面因此原样提升
  （`NewClient` 是唯一构造函数）。minio 在构造期的 `minio.Options` 里固定传输层，因此 DefaultDriver 安装
  `dynamicTransport`（RWMutex 保护的 RoundTripper 间接层），由包装体把
  声明+韧性 传输换进去 —— 与 starter-elasticsearch 同因同法。
- **`Driver`（driver.go）** — 构造 seam：可选容器 bean 形式的 `Driver`
  接口 + 内置 `DefaultDriver` 装配凭证（`NewStaticV4`）、region、bucket
  寻址风格与动态传输层。公司/伞包 starter 提供自己的 `Driver` bean；无该
  bean 时 starter 回退到 `DefaultDriver`。bucket-lookup 配置串映射到 minio
  的 `BucketLookupType`（minio v7.0.74 里 `virtual-host` 是
  `BucketLookupDNS` 的别名）。
- **declareTransport（command.go）** — 逐请求声明 seam。它把请求身份放到
  context 上（`observability.WithOperation`），由内侧的 resilience 层读取
  并发射 span、指标与访问日志。它刻意位于 resilience round-tripper 之外：
  发射器在 `Execute` 入口读 operation，声明若嵌在执行器之内就会按次运行、
  无人读取。词表（observe.go）为 `db.system=s3`、`db.operation=<HTTP 方法>`
  （有界，可作指标标签）与 `db.statement=<URL path>`（无界，仅进 span 与
  日志，绝不进标签），访问日志 tag 为 `_app_s3_access`。
- **韧性** — `resilience.NewRoundTripper` 把 `http.DefaultTransport` 包上由注入的
  `*resilience.Manager` / `*fault.Injector` bean 组装出的执行器
  （`fault.WrapClientExecutor(mgr.ClientExecutorFor("s3", service), service, inj)`），
  以 `resilience.ServiceLabel("s3", endpoint)` 圈定作用域。

## 3. 约束

- 凭证必填（两个 key 都挂 `expr`）：实践中所有 S3 部署都要认证，匿名
  访问交给自定义 driver 比可空配置面更干净。
- fail-fast 探针兼作健康检查 —— `ListBuckets` 是无副作用前提下同时验证
  连通性与凭证的最便宜的调用。
- minio client 没有 `Close`；Destroy 只关闭韧性执行器。
- minio-go 钉在 **v7.0.74**（共享模块缓存里已有的版本）；后续 7.x 加了
  `BucketLookupVirtualHost` 常量，但别名映射让配置面保持稳定。

## 4. 权衡 / 已否决的备选

- **以 S3 协议为抽象 vs. 分厂商 starter**：S3 门面是各大云事实上的标准
  出口；分厂商家族会让维护成本翻倍而语义差异几乎为零。门面之外的厂商
  特性留给自定义 driver。
- **minio-go vs aws-sdk-go-v2**：minio-go 是单一依赖、API 面小而稳、原生
  理解 S3 兼容性（bucket 寻址模式）；aws-sdk-go-v2 模块图庞大且带 AWS
  特有的装配流程。
- **构造期静态包传输 vs. dynamicTransport**：真正的传输层（声明 +
  韧性）由包装体的构造函数装配，晚于 minio 固定传输层；间接层让客户端在换入
  之前也可用（DefaultTransport 直通），而不是失败或盲装。
