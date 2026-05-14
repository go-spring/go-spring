# Go-Spring 实战第 29 课：测试体系：纯单测、容器测试、断言和 Mock 如何选择

Go-Spring 的配置、IoC、运行时、日志、HTTP 和 Starter 都讲完以后，我们还要回到一个更朴素、也更实际的问题：这些能力怎样被验证。

Go-Spring 的能力再完整，最后都要落到测试上。Go-Spring 兼容 Go 原生 `go test`，不需要额外测试运行器。它还提供了 IoC 容器测试、断言库和 Mock 支持。

关键其实是选对测试层次：能纯单测就不要启动容器；如果需要验证装配、配置和多个 Bean 协作，再使用 IoC 测试。

这篇也不是要把断言库和 Mock 框架的所有 API 都列完，而是先帮我们判断什么时候该用哪一层测试。层次选对以后，工具细节才有意义。

判断顺序可以很直接：业务逻辑用纯单测，装配和配置用 `RunTest`，外部系统和不稳定依赖用 Mock 隔离。

## 纯单测优先验证业务逻辑

纯单测手动构造对象和依赖：

```go
type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUserName(id int) (string, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return "", err
	}
	return user.Name, nil
}
```

对应的单测只需要手动传入替身依赖，然后断言业务方法的返回结果：

```go
func TestUserService_GetUserName(t *testing.T) {
	mockRepo := &MockUserRepository{}
	service := NewUserService(mockRepo)

	name, err := service.GetUserName(1)

	assert.That(t, err).Nil()
	assert.That(t, name).Equal("Alice")
}
```

这类测试启动快、定位准，适合验证业务逻辑。也就是说呢，我们不应该为了使用 Go-Spring 而启动容器，测试层次越轻，反馈越快。

## 容器测试验证装配和配置

当测试需要覆盖依赖注入、配置绑定、条件装配或多个 Bean 协作时，可以使用 `gs.RunTest`：

```go
func TestOrderFlow(t *testing.T) {
	gs.RunTest(t, func(s *struct {
		OrderSvc *OrderService `autowire:""`
		StockSvc *StockService `autowire:""`
		Env      string        `value:"${app.env:=test}"`
	}) {
		err := s.OrderSvc.Create(1001)
		assert.That(t, err).Nil()
		assert.That(t, s.StockSvc.Remaining(1001)).Equal(99)
	})
}
```

`RunTest` 会创建测试对象作为 root Bean，启动 Go-Spring 测试容器，完成依赖注入后执行回调，最后关闭容器。这样测试用例只需要关心注入后的对象状态。

## 测试配置只作用于当前容器

测试前可以通过 `gs.Configure()` 添加配置。下面这段代码把数据库地址和环境名限制在当前测试容器里：

```go
func TestApp(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Property("database.url", "sqlite://:memory:")
		g.Property("app.env", "test")
	}).RunTest(t, func(s *struct {
		DB  *DB    `autowire:""`
		Env string `value:"${app.env}"`
	}) {
		assert.String(t, s.Env).Equal("test")
	})
}
```

这适合把测试环境的差异直接放进当前测试容器，而不是修改全局配置文件。这样测试之间也更容易隔离。

## 测试 Bean 用来替换依赖

如果要替换外部依赖，可以只给当前测试容器注册替代 Bean，不污染全局注册表：

```go
func TestUserService(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Provide(func() UserRepository {
			return &MockUserRepository{}
		})
	}).RunTest(t, func(s *struct {
		Service *UserService `autowire:""`
	}) {
		user, err := s.Service.GetUser(1)
		assert.That(t, err).Nil()
		assert.That(t, user.ID).Equal(1)
	})
}
```

每个 `RunTest` 会复制全局注册信息，`gs.Configure()` 中的 Bean 只作用于当前测试。

但由于全局 `init` 注册信息共享，基于 IoC 容器的测试目前不支持 `t.Parallel()`。

## assert 和 require 分工不同

Go-Spring 在 `github.com/go-spring/stdlib/testing` 下提供了 `assert` 和 `require`。

`assert` 失败后继续执行，适合收集多个断言结果。

`require` 失败后立即终止，适合前置条件。简单说就是，前置条件用 `require`，后续结果检查用 `assert`。

```go
require.That(t, err).Nil()

assert.That(t, user).NotNil()
assert.That(t, user.ID).Equal(1)
assert.That(t, user.Name).Equal("Alice")
```

常见入口可以按数据类型选择，下面这些调用分别覆盖普通值、错误、数字、字符串、切片和 map：

```go
assert.That(t, value).Equal(expected)
assert.Error(t, err).NotNil()
assert.Number(t, 42).GreaterThan(40)
assert.String(t, "hello").Contains("ell")
assert.Slice(t, []int{1, 2, 3}).Length(3)
assert.Map(t, map[string]int{"a": 1}).ContainsKey("a")
```

断言失败时也可以补一段业务语境，方便从测试输出里直接定位原因：

```go
assert.That(t, result).Equal(expected, "result should match expected")
```

## Mock 用来隔离外部依赖

Go-Spring 提供了 `gs-mock`，支持接口 Mock、函数 Mock 和方法 Mock。

接口 Mock 通常通过代码生成创建实现类，避免手写大量样板方法：

```go
//go:generate gs mock -o mock.go
```

如果当前包里接口很多，也可以只为指定接口生成 Mock：

```go
//go:generate gs mock -o mock.go -i "Service,Repository"
```

如果返回值需要根据入参动态计算，可以使用 Handle 模式：

```go
s.MockDo().Handle(func(n int, s string) (int, error) {
	if n%2 == 0 {
		return n * 2, nil
	}
	return 0, errors.New("odd number")
})
```

如果只是匹配固定条件并返回固定结果，When/Return 模式会更简洁：

```go
s.MockFormat().When(func(format string, args []any) bool {
	return args[0] == "abc"
}).ReturnValue("abc")
```

函数和方法 Mock 通过 `context.Context` 传递 Mock Manager。下面的例子把 Mock 规则绑到当前调用链，避免影响其他测试：

```go
r := gsmock.NewManager()
ctx := gsmock.WithManager(context.TODO(), r)

gsmock.Func22(GetUser, r).Handle(func(ctx context.Context, id int) (*User, error) {
	return &User{ID: id, Name: "Alice"}, nil
})

user, err := GetUser(ctx, 1)
```

使用函数或方法 Mock 时，还要避免编译器内联导致拦截失效，测试命令可以加上禁用内联参数：

```bash
go test -gcflags="all=-N -l" ./...
```

Mock 规则要在测试逻辑开始前注册完成，并按从具体到宽泛的顺序排列。否则宽泛规则可能先匹配，导致具体规则没有机会生效。

## 按测试层次控制反馈成本

纯业务逻辑优先纯单测。IoC 测试用来验证装配和配置行为。Mock 用来隔离外部依赖，而不是掩盖设计问题。

测试体系收束以后，整个系列就可以回到整体：配置、IoC、运行时、日志、HTTP、Starter 和测试共同组成 Go-Spring 的能力地图。
