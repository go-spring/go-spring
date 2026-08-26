# tlsconf
[English](README.md) | [中文](README_CN.md)

`tlsconf` 是所有终结或发起 TLS 的 Go-Spring starter 共用的 TLS 配置助手。
它把默认关闭的 `tls.*` 配置块绑定为 `*tls.Config`,并在提供时从磁盘加
载密钥对与 CA bundle。只依赖 Go 标准库和
`go-spring.org/stdlib/errutil`,仓库里任何模块都能采用而不引入额外依赖
图。

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

## Build(客户端)

```go
tlsCfg, err := c.TLS.Build()
if err != nil { return err }
client := somelib.NewClient(somelib.WithTLS(tlsCfg))
```

`enabled=false` 时 `Build` 返回 `(nil, nil)` —— 即"无 TLS",所有 client
库都接受 nil `*tls.Config`。`CAFile` 设置 `RootCAs`;错误带通用 `tls:`
前缀,需要组件前缀时用 `errutil.Explain(err, "redis: ...")` 再包一层。

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

与 `Build` 相同,`enabled=false` 时返回 `(nil, nil)`。

## 安装

```
go get go-spring.org/cloud
```
