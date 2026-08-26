# tlsconf 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`tlsconf` 是被 20+ starter(redis、各 gorm 方言、kafka、nats、mqtt、
grpc、gin、gateway、neo4j、cassandra、registry/lock 后端……)共享的唯一
TLS 配置面。在它出现之前,每个 starter 自行声明这些旋钮的一个子集、各带
各的默认值;现在只有一个嵌套配置块、一套默认值、一份加载逻辑。

## 1. 职责与边界

- **做:** 定义 `TLSConfig`(各 starter 曾经散落声明的字段之并集),
  以 `tls` 键绑定,并通过 `Build`(客户端)或 `BuildServer`(服务端)
  产出 `*tls.Config`,按需从磁盘加载密钥对与 CA bundle。
- **不做:**
  - 不做 provider 安装、不包 listener。starter 把返回的
    `*tls.Config` 交给自己的库或 `tls.NewListener`;本包到产出配置为
    止。
  - 不依赖 stdlib + `stdlib/errutil` 之外的东西。仓库任何模块都能引
    入而不拖进依赖图,正是这个助手存在的全部理由。
  - 不做证书热加载/轮换。`Build` 在构造时调用一次;轮换是 starter 的
    生命周期关注点。

## 2. 关键决策

- **配置面共享,语义必须共享。** 20+ starter embed 同一个
  `TLSConfig`,运维从 redis 换到 kafka、grpc,看到的是同样的
  `tls.enabled` / `cert-file` / `ca-file` 旋钮、同样的行为。这种一致
  性比 per-starter 灵活性更值钱 —— 所以 struct 是历史字段的并集而不是
  接口。
- **`MinVersion` 跟随 `crypto/tls` 默认 —— 有意为之。** 本包不钉死
  最低 TLS 版本。Go stdlib 会随旧协议废弃而抬升默认值(go-spring 跟随
  较新的 Go),跟随 stdlib 让所有 starter 的下限随平台指引自动前进,
  无需跨 20 个 starter 协同改配置。需要更严下限的 starter 仍可在返回
  的 config 上自设 `MinVersion`。在这里钉死版本,只会把下限冻结在助手
  编写之时的水平。
- **关闭时返回 `(nil, nil)`。** nil `*tls.Config` 对所有 client 库和
  `tls.NewListener` 类服务端路径都意味着"无 TLS",starter 可以无分支
  直传。替代方案 —— 返回空 config —— 会以默认参数启用 TLS,恰好与默认
  关闭的契约相反。
- **`BuildServer` 把 `CAFile` 读作客户端 CA bundle。** 服务端出现 CA
  文件只可能意味着"客户端出示的证书须由此签发",因此设置
  `ClientCAs` + `RequireAndVerifyClientCert`(即配了 CA 文件就启用
  mTLS)是最不意外的解读;留空保持单向 TLS。`ServerName` 与
  `InsecureSkipVerify` 描述的是校验"我们拨的对端",服务端不做此事,
  因此忽略而非误用。
- **默认关闭。** `enabled` 默认 false:不配置就不协商 TLS,与仓库全局
  的"未配置 = 不装配"口径一致。

## 3. 权衡与放弃的方案

- **单一 struct,不按角色拆两个。** 仅客户端字段与仅服务端字段分成
  两个类型会更精确,但会把共享的配置面再次一分为二;一个 struct 加
  `BuildServer` 的角色化解读,保住了运维词汇的单一。
- **通用 `tls:` 错误前缀。** `Build` 不知道自己服务于哪个组件;先加
  `tls:` 前缀,由 starter 再包(`errutil.Explain(err, "redis: ...")`)。
  为了装饰性收益把组件名传进来不值。
- **客户端不做 mTLS 自动推断。** 有些库凭客户端证书的存在推断双向
  TLS;这里客户端的 `CAFile` 永远只是校验用的根证书集。mTLS 的决定由
  服务端 `BuildServer` 的显式语义承载。
