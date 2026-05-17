# Go-Spring 实战第 4 课 —— 配置校验：使用 expr 标签拦截非法配置

前面几篇我们解决了“配置能不能绑定到合适的 Go 类型”这个问题。但是绑定成功只能说明格式转过去了，并不代表这个值一定可用。

比如端口号可以绑定成整数，但 `99999` 不是合法端口。比如日志级别可以绑定成字符串，但拼错后可能导致行为不符合预期。也就是说，配置绑定解决的是“能不能转过去”的问题，配置校验则要回答“这个值能不能用”的问题。而且，更麻烦的是，这类错误如果拖到运行期才暴露出来，那么排查成本会比启动失败要高得多。

所以，Go-Spring 提供了配置校验的功能，可以把这类问题前移到启动阶段。我们希望应用在错误配置下直接启动失败，而不是带着隐患进入运行期。

## expr 标签

Go-Spring 基于 `expr-lang/expr` 提供了表达式校验。用法是在字段上添加 `expr` 标签，表达式中的 `$` 表示当前字段值。

下面这组字段覆盖了几个常见规则。

```go
type ServerConfig struct {
	// 端口号必须在 0-65535 范围内
	Port int `value:"${server.port:=8080}" expr:"$ > 0 && $ < 65536"`
	// 日志级别必须是 debug/info/warn/error 中的一个
	LogLevel string `value:"${log.level:=info}" expr:"$ in ['debug', 'info', 'warn', 'error']"`
	// 用户名必须是 3-31 个字母、数字或下划线
	Username string `value:"${auth.username}" expr:"$ matches '^[a-z][a-z0-9_]{3,31}$'"`
	// 超时时间必须大于等于 1s
	Timeout time.Duration `value:"${timeout:=5s}" expr:"$ >= duration(\"1s\")"`
	// 重试次数必须在 0-10 范围内
	RetryCount int `value:"${retry:=3}" expr:"$ >= 0 && $ <= 10"`
}
```

表达式校验用于描述字段本身的业务约束。这样依赖，配置和校验就都放在同一个字段声明上，后续维护时也不会漏掉规则。

## 必填字段

Go-Spring 不需要我们额外写 required 校验。因为在配置绑定的时候，如果字段没有默认值，同时配置中也不存在对应 key，那么绑定阶段就会失败。

也就是说，配置的存在性由绑定机制处理，字段的业务合法性由校验表达式处理。将两件事分开，配置结构也更清楚。

## 自定义校验函数

expr 内置的表达式不一定能覆盖所有业务的规则，所以 Go-Spring 支持使用自定义校验函数来补充。

看下面的例子。自定义校验函数要求返回 `bool`，Go-Spring 根据返回值理解成校验是否通过。

```go
func init() {
	// validPort 校验端口号是否在 0-65535 范围内
	conf.RegisterValidateFunc[int]("validPort", func(port int) bool {
		return port > 0 && port < 65536
	})
	// minLength 校验字符串长度是否大于等于 3
	conf.RegisterValidateFunc[string]("minLength", func(s string) bool {
		return len(s) >= 3
	})
}
```

注册之后，字段上的 `expr` 标签就可以像调用内置函数一样调用这些校验函数。

```go
type ServerConfig struct {
	Port     int    `value:"${port}" expr:"validPort($)"`
	Username string `value:"${auth.username}" expr:"minLength($)"`
	APIKey   string `value:"${security.api-key}" expr:"minLength($) && $ contains 'prod-'"`
}
```

> 自定义校验函数不能返回 error 是因为用在布尔表达式中的时候，如果同时返回 bool 和 error 两个值会导致表达式无法合理书写。也许后续会解决这个问题。

## 配置校验不替代运行期业务判断

配置校验不是替代业务判断，而是把系统运行的前置条件明确写在配置结构旁边，并在启动阶段统一执行。

这对线上服务尤其重要——配置错误如果能在发布阶段暴露，就不必等请求进入后再以业务错误、连接失败或异常日志的形式出现。失败越早，问题越容易定位在配置本身。

## 配置校验守住启动前置条件

配置校验更像应用启动前的守门动作，即把端口范围、枚举值、账号格式、超时下限这些前置条件写在配置结构旁边，并在启动时一次性检查。

当配置模型、绑定和校验都确定以后，视角就可以继续前移，即配置从哪里来，格式如何扩展，外部配置中心又怎样进入同一套模型。
这样复杂规则仍然留在启动期校验链路里，而不是分散到业务初始化代码里。
