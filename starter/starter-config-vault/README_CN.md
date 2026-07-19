# starter-config-vault

[English](README.md) | [中文](README_CN.md)

`starter-config-vault` 将 [HashiCorp Vault](https://www.vaultproject.io/) 集成为
Go-Spring 的**远程配置中心**与**密钥存储**,基于
github.com/hashicorp/vault/api 实现。空导入该包即注册一个 `vault` 配置提供者:
启动时读取 KV secret、将其字段暴露为应用属性,并在运行时热更新 —— 无需重启。

该 starter 只承担配置中心角色。它与 `spring/conf/decrypt` 的
[属性级解密](#属性级解密) 天然配合:配置值可以是 `ENC(...)` 密文,在绑定前被解密。
Vault Agent / CSI 挂载到*文件*的 secret 应改用 `starter-config-file` 读取;本
starter 直接对接 Vault API。

## 安装

```bash
go get go-spring.org/starter-config-vault
```

## 快速开始

### 1. 导入包

```go
import _ "go-spring.org/starter-config-vault"
```

### 2. 从 Vault 导入配置

在配置文件中按 `[optional:]vault:<host>:<port>/<mount>/<path>?<query>` 语法声明导入:

```properties
spring.app.imports=optional:vault:127.0.0.1:8200/secret/gs-config-demo?kv-version=2
```

查询参数:

| 键           | 默认值       | 说明                                                       |
|--------------|--------------|------------------------------------------------------------|
| `kv-version` | `2`          | KV 引擎版本:`1` 或 `2`                                      |
| `scheme`     | `http`       | `http` 或 `https`                                          |
| `namespace`  | (空)         | Vault 企业版 namespace                                      |
| `key`        | (空)         | 只读取某个字段作为文档,而非映射全部字段                     |
| `format`     | `properties` | 该字段的格式:`properties`/`yaml`/`toml`/`json`             |
| `prefix`     | (空)         | 为产生的每个属性 key 添加前缀                               |
| `poll-ms`    | `5000`       | 变更检测的轮询间隔(毫秒)                                  |
| `token`      | (空)         | Vault token —— **不推荐**,优先用 `VAULT_TOKEN`(见下)     |

**两种内容模式:**

- **整 secret(默认):** secret 的每个字段成为一个属性。
  secret `{ "demo.message": "hi", "db.password": "x" }` 产生
  `demo.message=hi` 与 `db.password=x`。
- **单字段(`?key=...`):** 指定字段保存一整份文档,按 `format` 解析。适合把
  `properties`/`yaml` 文本整块存进一个字段。

加 `optional:` 前缀可让 secret 尚不存在时应用照常启动;写入后再补齐取值。

### 3. 在带外提供 token

Vault token 是凭据,不得写进配置文件。提供者按以下顺序解析:

1. `token` 查询参数(仅本地演示可用,其余场景不推荐);
2. `VAULT_TOKEN` 环境变量;
3. 由 `token-file` 查询参数或 `VAULT_TOKEN_FILE` 环境变量指定的 token 文件
   (如 Kubernetes 注入的 token)。

三者皆无则启动即 fail-fast 报清晰错误。

### 4. 绑定动态字段

将导入的 key 绑定到 `gs.Dync[T]` 字段即可实时更新:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

secret 变更时,提供者的轮询 watcher 触发一次应用属性刷新,所有绑定的
`gs.Dync` 字段原子更新。完整的 写入 → 热更新 流程见
[example-config](example-config/example.go)。

## 属性级解密

独立于 Vault,Go-Spring 支持 Jasypt 风格的属性解密:任何解析后被 `ENC(...)`
包裹或以 `{cipher}` 开头的值,都会在绑定前解密,应用代码只看到明文。

```properties
db.password=ENC(<base64 密文>)
# 或 Spring Cloud Config 风格:
db.password={cipher}<base64 密文>
```

内置 `aes` 驱动使用 AES-GCM。密钥在带外提供 —— 绝不进配置文件:

| 变量                         | 说明                                       |
|------------------------------|--------------------------------------------|
| `GS_CONFIG_DECRYPT_KEY`      | base64 编码的 AES 密钥(16/24/32 字节)     |
| `GS_CONFIG_DECRYPT_KEY_FILE` | 保存 base64 密钥的文件路径                  |
| `GS_CONFIG_DECRYPT_DRIVER`   | 驱动名,默认 `aes`                          |

带标记却无法解密的值会让启动失败,而非降级为损坏的默认值。要接入非对称方案或
云 KMS,在 `init` 中注册驱动并用环境变量选择:

```go
conf.RegisterDecryptDriver("kms", func() (decrypt.Decryptor, error) { ... })
```

该能力适用于任意配置源(本地文件、Vault、Nacos……);Vault 示例端到端演示了一个
`ENC(...)` 值。

## 工作原理

- 启动时 `spring.app.imports` 调用 `vault` 提供者:据 source 字符串建客户端、解析
  token、读取 KV secret、启动轮询 watcher。
- secret 变更在下一次轮询被检测到,调用框架的 `PropertiesRefresher`,重新加载所有
  配置源(重跑本提供者)并通过两阶段原子提交重绑所有 `gs.Dync` 字段。
- 绑定过程中,任何被 `ENC(...)` / `{cipher}` 包裹的值由 `spring/conf/decrypt`
  解密。
