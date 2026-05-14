# Go-Spring 实战第 4 课：配置校验：启动阶段发现非法端口、枚举和格式错误

上一篇我们解决了“配置能不能绑定到合适的 Go 类型”。但绑定成功只能说明格式转过去了而已，不代表这个值一定可用。

端口可以绑定成整数，但 `99999` 不是合法端口；日志级别可以绑定成字符串，但拼错后可能导致行为不符合预期。也就是说，绑定解决的是“能不能转过去”，而校验要继续回答“这个值能不能用”。更麻烦的是，这类错误如果拖到运行期才暴露，排查成本会比启动失败高得多。

所以，Go-Spring 的配置校验要做的，就是把这些问题前移到启动阶段。我们希望应用在错误配置下直接启动失败，而不是带着隐患进入运行期。

## 用表达式描述字段合法性

Go-Spring 基于 `expr-lang/expr` 提供了表达式校验。用法是在字段上添加 `expr` 标签，表达式中的 `$` 表示当前字段值。

```go
type ServerConfig struct {
	Port int `value:"${server.port:=8080}" expr:"$ > 0 && $ < 65536"`

	LogLevel string `value:"${log.level:=info}" expr:"$ in ['debug', 'info', 'warn', 'error']"`

	Username string `value:"${auth.username}" expr:"$ matches '^[a-z][a-z0-9_]{3,31}$'"`

	Timeout time.Duration `value:"${timeout:=5s}" expr:"$ >= duration(\"1s\")"`

	RetryCount int `value:"${retry:=3}" expr:"$ >= 0 && $ <= 10"`
}
```

常见表达式包括下面几类。

| 表达式 | 含义 |
|--------|------|
| `$ > 0` | 当前值必须大于 0 |
| `$ < 65536` | 当前值必须小于 65536 |
| `$ in ['debug', 'info', 'warn', 'error']` | 枚举值校验 |
| `$ matches '^[a-z][a-z0-9_]{3,31}$'` | 正则校验 |
| `$ > 0 && $ < 65536` | 多条件同时满足 |

表达式校验适合描述字段本身的业务约束。这样配置和校验放在同一个字段声明上，后续维护时也更不容易漏掉规则。

## 必填由绑定负责，业务规则交给校验

这里有一个容易多写的点要注意，即我们不需要额外写 required 校验。只要字段没有默认值，且配置中不存在对应 key，绑定阶段已经会失败。

通常只有两类情况需要 `expr`。

- 字段有默认值，但默认值和外部配置都必须满足业务规则。
- 字段存在，但还要满足范围、枚举、格式等规则。

存在性由绑定机制处理，业务合法性由校验表达式处理。两件事分开以后，配置结构会更清楚，也能避免把“有没有”和“对不对”混在一条规则里。

## 复杂规则用自定义校验函数扩展

如果表达式内置操作不够用，就可以注册自定义校验函数。

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

注册后，字段上的 `expr` 标签就可以像调用内置函数一样调用这些校验函数。

```go
type ServerConfig struct {
	Port     int    `value:"${port}" expr:"validPort($)"`
	Username string `value:"${auth.username}" expr:"minLength($)"`
	APIKey   string `value:"${security.api-key}" expr:"minLength($) && $ contains 'prod-'"`
}
```

自定义函数返回 `bool` 表示校验是否通过。之后，Go-Spring 负责处理校验失败时的错误传播。

## 为什么错误要尽早暴露

配置校验不是替代业务判断，而是把系统运行的前置条件明确写在配置结构旁边，并在启动阶段统一执行。

这对线上服务尤其重要——配置错误如果能在发布阶段暴露，就不必等请求进入后再以业务错误、连接失败或异常日志的形式出现。失败越早，问题越容易定位在配置本身。

## 配置校验是启动前置条件

配置校验更像应用启动前的守门动作，即把端口范围、枚举值、账号格式、超时下限这些前置条件写在配置结构旁边，并在启动时一次性检查。

当配置模型、绑定和校验都确定以后，视角就可以继续前移，即配置从哪里来，格式如何扩展，外部配置中心又怎样进入同一套模型。
