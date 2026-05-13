# 配置校验

配置能绑定成功，只能说明格式转过去了，不代表它一定可用。

端口可以绑定成整数，但 `99999` 不是合法端口；日志级别可以绑定成字符串，但拼错后可能导致行为不符合预期。更麻烦的是，这类错误如果拖到运行期才暴露，排查成本会比启动失败高得多。

Go-Spring 的配置校验用于把这些问题前移到启动阶段，让应用在错误配置下直接启动失败，而不是带着隐患进入运行期。

## 表达式校验

Go-Spring 基于 `expr-lang/expr` 提供表达式校验。使用方式是在字段上添加 `expr` 标签，表达式中的 `$` 表示当前字段值。

```go
type ServerConfig struct {
	Port int `value:"${server.port:=8080}" expr:"$ > 0 && $ < 65536"`

	LogLevel string `value:"${log.level:=info}" expr:"$ in ['debug', 'info', 'warn', 'error']"`

	Username string `value:"${auth.username}" expr:"$ matches '^[a-z][a-z0-9_]{3,31}$'"`

	Timeout time.Duration `value:"${timeout:=5s}" expr:"$ >= duration(\"1s\")"`

	RetryCount int `value:"${retry:=3}" expr:"$ >= 0 && $ <= 10"`
}
```

常见表达式包括：

| 表达式 | 含义 |
|--------|------|
| `$ > 0` | 当前值必须大于 0 |
| `$ < 65536` | 当前值必须小于 65536 |
| `$ in ['debug', 'info', 'warn', 'error']` | 枚举值校验 |
| `$ matches '^[a-z][a-z0-9_]{3,31}$'` | 正则校验 |
| `$ > 0 && $ < 65536` | 多条件同时满足 |

表达式校验适合描述字段本身的业务约束。配置和校验放在同一个字段声明上，也能减少后续维护成本。

## 必填校验的误区

Go-Spring 不需要额外写 required 校验。只要字段没有默认值，且配置中不存在对应 key，绑定阶段已经会失败。

通常只有两类情况需要 `expr`：

- 字段有默认值，但默认值和外部配置都必须满足业务规则。
- 字段存在，但还要满足范围、枚举、格式等规则。

也就是说，存在性由绑定机制处理，业务合法性由校验表达式处理。

## 自定义校验函数

当表达式内置操作不够用时，可以注册自定义校验函数：

```go
func init() {
	conf.RegisterValidateFunc[int]("validPort", func(port int) bool {
		return port > 0 && port < 65536
	})

	conf.RegisterValidateFunc[string]("minLength", func(s string) bool {
		return len(s) >= 3
	})
}
```

注册后可以在标签中使用：

```go
type ServerConfig struct {
	Port     int    `value:"${port}" expr:"validPort($)"`
	Username string `value:"${auth.username}" expr:"minLength($)"`
	APIKey   string `value:"${security.api-key}" expr:"minLength($) && $ contains 'prod-'"`
}
```

自定义函数返回 `bool` 表示校验是否通过。框架负责处理校验失败时的错误传播。

## 启动阶段的意义

配置校验的核心价值不是替代业务判断，而是把系统运行的前置条件明确写在配置结构旁边，并在启动阶段统一执行。

这对线上服务尤其重要：配置错误应该在发布阶段暴露，而不是等请求进入后才以业务错误、连接失败或异常日志的形式出现。

## 把错误挡在启动阶段

配置校验不替代业务判断，它更像应用启动前的守门动作：把端口范围、枚举值、账号格式、超时下限这些前置条件写在配置结构旁边，并在启动时一次性检查。

配置模型、绑定和校验连起来之后，下一个问题就变成了输入来源：配置可以从哪里来，格式如何扩展，远程配置中心又该怎样接入。
