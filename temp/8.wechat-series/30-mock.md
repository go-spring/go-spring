# Go-Spring 实战第 30 课 —— Mock 边界：接口、函数和方法替身

上一课把测试分层说清楚以后，还有一块需要单独处理：当被测逻辑依赖外部系统、不稳定调用或难复现错误时，测试怎样隔离这些边界。

Mock 的价值不在于让所有对象都变成替身。纯业务对象可以手动构造，容器装配可以用 `RunTest` 验证，只有当某个调用边界会让测试变慢、变脆弱或者难以控制时，Mock 才应该进入测试。

Go-Spring 提供 `gs-mock`，覆盖接口 Mock、函数 Mock 和方法 Mock。它要解决的是“被测逻辑需要隔离某个调用边界”，而不是替代 Go 原生测试方式。

## 接口 Mock

接口 Mock 最常见。业务代码已经依赖接口时，测试只需要提供一个实现类，就可以控制下游返回值。接口较小时，手写实现并不复杂；接口较大或者需要频繁调整规则时，可以把样板实现交给工具生成。

```go
//go:generate gs mock -o mock.go
```

如果只想为指定接口生成 Mock，可以限制接口列表。

```go
//go:generate gs mock -o mock.go -i "Service,Repository"
```

生成命令只是让替身实现可用。真正影响测试语义的，是每个测试里写下的调用规则。也就是说，生成代码不应该替你决定业务场景，测试仍然要明确表达这次调用期望什么输入、返回什么结果。

## 调用规则

生成后的 Mock 可以按调用规则返回结果。`Handle` 模式适合返回值依赖入参的场景。

```go
s.MockDo().Handle(func(n int, s string) (int, error) {
	if n%2 == 0 {
		return n * 2, nil
	}
	return 0, errors.New("odd number")
})
```

这段规则没有把返回值写死，而是根据入参决定结果。它适合表达计算型替身、分支错误和边界值。

`When`/`Return` 模式适合固定匹配条件。

```go
s.MockFormat().When(func(format string, args []any) bool {
	return args[0] == "abc"
}).ReturnValue("abc")
```

这类规则读起来更像“当满足某个条件时，返回某个固定值”。如果一个 Mock 里同时有多条规则，通常应该把具体规则放在前面，宽泛兜底规则放在后面。否则宽泛规则可能先匹配，导致具体场景永远走不到。

## 函数和方法 Mock

有些遗留代码并没有把依赖抽成接口，而是直接调用包级函数或具体方法。为了让这类代码也能被隔离测试，`gs-mock` 支持函数和方法 Mock。

函数和方法 Mock 使用 `context.Context` 传递 Mock Manager。Mock 规则可以绑定到当前调用链，而不是全局影响所有调用。

```go
r := gsmock.NewManager()
ctx := gsmock.WithManager(context.TODO(), r)

gsmock.Func22(GetUser, r).Handle(func(ctx context.Context, id int) (*User, error) {
	return &User{ID: id, Name: "Alice"}, nil
})

user, err := GetUser(ctx, 1)
```

这种写法适合处理外部 SDK、历史工具函数，或者暂时不适合重构成接口的调用点。新的业务模块仍然应该优先通过接口或小对象表达依赖，因为那样的测试通常更简单，也更接近普通 Go 代码。

## 编译边界

函数或方法 Mock 有一个编译边界：如果目标调用被编译器内联，拦截可能失效。测试命令可以加上禁用内联参数。

```bash
go test -gcflags="all=-N -l" ./...
```

这个要求只影响需要函数或方法 Mock 的测试。接口 Mock 不依赖这种拦截方式，通常也不需要调整编译参数。

如果某个测试因为内联、泛型实例化或编译优化变得难以判断，优先考虑把调用边界抽成接口或工厂函数。Mock 工具可以兜住遗留代码，但不应该成为新代码隐藏依赖的理由。

## Mock 边界

Mock 规则越多，测试越容易变成另一套手写流程。因此，Mock 应该服务于隔离边界，而不是复刻被测对象内部逻辑。

比较稳的判断方式是：如果依赖只是一个普通 Go 对象，而且创建成本很低，直接构造它；如果要验证配置、条件和注入，启动测试容器；如果依赖来自网络、数据库、消息队列、系统时间、随机数、外部 SDK 或难以复现的错误路径，再考虑 Mock。

这样使用 Mock 时，它承担的是测试隔离职责。业务逻辑仍然通过普通单测验证，装配关系仍然通过 `RunTest` 验证，Mock 只负责把不稳定边界固定下来。
