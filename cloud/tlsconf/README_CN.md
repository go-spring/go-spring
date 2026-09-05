# tlsconf

[English](README.md) | [中文](README_CN.md)

`tlsconf` 是所有终结或发起 TLS 的 Go-Spring starter(redis、各 gorm
方言、kafka、nats、mqtt、grpc、gin、gateway、neo4j、cassandra、
registry/lock 后端……)共用的 TLS 配置助手。它把默认关闭的 `tls.*`
配置块绑定为 `*tls.Config`,并在提供时从磁盘加载密钥对与 CA bundle。
只依赖 Go 标准库和 `go-spring.org/stdlib/errutil`,仓库里任何模块都
能采用而不引入额外依赖图。

## 安装

```
go get go-spring.org/cloud
```

## 嵌入方式

在 starter 的 per-instance Config 里以 `tls` 键 embed
`TLSConfig`,绑定出的属性即 `spring.<client>.<name>.tls.*`:

```go
type Config struct {
    ...
    TLS tlsconf.TLSConfig `value:"${tls:=}"`
}
```

| 属性 | 默认 | 含义 |
|---|---|---|
| `tls.enabled` | `false` | 打开 TLS;不配置绝不协商。 |
| `tls.cert-file` / `tls.key-file` | 空 | 本端出示的 PEM 密钥对。 |
| `tls.ca-file` | 空 | 校验对端的 CA bundle(PEM)。空 = 宿主根证书集。 |
| `tls.server-name` | 空 | 覆盖对端证书校验名(按 IP 拨号、discovery 标签场景)。 |
| `tls.insecure-skip-verify` | `false` | 关闭校验。仅限本地测试。 |

配置面被 20+ starter 共享,语义也随之共享:运维从 redis 换到 kafka、
grpc,看到的是同样的 `tls.enabled` / `cert-file` / `ca-file` 旋钮、
同样的行为。

## BuildClient(客户端)

```go
tlsCfg, err := c.TLS.BuildClient()
if err != nil { return err }
client := somelib.NewClient(somelib.WithTLS(tlsCfg))
```

`enabled=false` 时 `BuildClient` 返回 `(nil, nil)` —— 即"无 TLS",所有 client
库都接受 nil `*tls.Config`,starter 可以无分支直传。`CAFile` 设置
`RootCAs`;在客户端它永远只是校验用的根证书集,不触发 mTLS。
`MinVersion` 跟随 `crypto/tls` 默认;需要更严下限的 starter 可在返回
的 config 上自设 `MinVersion`。

错误带通用 `tls:` 前缀(`BuildClient` 不知道自己服务于哪个组件);需要组件
前缀时用 `errutil.Explain(err, "redis: ...")` 再包一层。

## BuildServer(服务端)

```go
tlsCfg, err := c.TLS.BuildServer()
```

与服务端语义有两处不同:

- `CAFile` 是**客户端**证书签发 CA 的 bundle:设置 `ClientCAs` 并打开
  `RequireAndVerifyClientCert` —— 配了 CA 文件即启用 mTLS。留空则是单
  向 TLS。
- `ServerName` 与 `InsecureSkipVerify` 是客户端旋钮;在服务端它们描述
  的是校验"我们拨的对端",故两者均忽略。

与 `BuildClient` 相同,`enabled=false` 时返回 `(nil, nil)`。

## 边界

- 本包到产出 `*tls.Config` 为止:不装 provider、不包 listener。
  starter 把结果交给自己的库或 `tls.NewListener`。
- 不做证书热加载/轮换。`BuildClient` 在构造时调用一次;轮换是 starter 的
  生命周期关注点。
