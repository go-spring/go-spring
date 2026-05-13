# 测试体系

框架能力再完整，最后都要落到测试上。

Go-Spring 兼容 Go 原生 `go test`，不需要额外测试运行器。它同时提供 IoC 容器测试、断言库和 Mock 支持。

关键是选对测试层次：能纯单测就不要启动容器；需要验证装配、配置和多个 Bean 协作时，再使用 IoC 测试。

## 纯单元测试

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

测试：

```go
func TestUserService_GetUserName(t *testing.T) {
	mockRepo := &MockUserRepository{}
	service := NewUserService(mockRepo)

	name, err := service.GetUserName(1)

	assert.That(t, err).Nil()
	assert.That(t, name).Equal("Alice")
}
```

这类测试启动快、定位准，适合验证业务逻辑。

## 基于 IoC 容器的测试

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

`RunTest` 会创建测试对象作为 root Bean，启动测试容器，完成依赖注入后执行回调，最后关闭容器。

## 自定义配置

测试前可以通过 `gs.Configure()` 添加配置：

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

## 替换依赖

测试可以为当前容器注册替代 Bean：

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

由于全局 `init` 注册信息共享，基于 IoC 容器的测试目前不支持 `t.Parallel()`。

## assert 与 require

Go-Spring 在 `github.com/go-spring/stdlib/testing` 下提供 `assert` 和 `require`。

`assert` 失败后继续执行，适合收集多个断言结果。

`require` 失败后立即终止，适合前置条件。

```go
require.That(t, err).Nil()

assert.That(t, user).NotNil()
assert.That(t, user.ID).Equal(1)
assert.That(t, user.Name).Equal("Alice")
```

常见入口包括：

```go
assert.That(t, value).Equal(expected)
assert.Error(t, err).NotNil()
assert.Number(t, 42).GreaterThan(40)
assert.String(t, "hello").Contains("ell")
assert.Slice(t, []int{1, 2, 3}).Length(3)
assert.Map(t, map[string]int{"a": 1}).ContainsKey("a")
```

也支持自定义错误消息：

```go
assert.That(t, result).Equal(expected, "result should match expected")
```

## Mock 框架

Go-Spring 提供 `gs-mock`，支持接口 Mock、函数 Mock 和方法 Mock。

接口 Mock 通过生成代码：

```go
//go:generate gs mock -o mock.go
```

只生成部分接口：

```go
//go:generate gs mock -o mock.go -i "Service,Repository"
```

Handle 模式适合自定义逻辑：

```go
s.MockDo().Handle(func(n int, s string) (int, error) {
	if n%2 == 0 {
		return n * 2, nil
	}
	return 0, errors.New("odd number")
})
```

When/Return 模式适合固定匹配：

```go
s.MockFormat().When(func(format string, args []any) bool {
	return args[0] == "abc"
}).ReturnValue("abc")
```

函数和方法 Mock 通过 `context.Context` 传递 Mock Manager，用于隔离测试：

```go
r := gsmock.NewManager()
ctx := gsmock.WithManager(context.TODO(), r)

gsmock.Func22(GetUser, r).Handle(func(ctx context.Context, id int) (*User, error) {
	return &User{ID: id, Name: "Alice"}, nil
})

user, err := GetUser(ctx, 1)
```

使用函数或方法 Mock 时，建议禁用内联：

```bash
go test -gcflags="all=-N -l" ./...
```

Mock 规则应在测试逻辑开始前注册完成，并按从具体到宽泛的顺序排列。

## 测试要按层次选择

纯业务逻辑优先纯单测。IoC 测试用于验证装配和配置行为。Mock 用于隔离外部依赖，而不是掩盖设计问题。

最后一篇回到整体，看看配置、IoC、运行时、日志、HTTP、Starter 和测试如何组成 Go-Spring 的能力地图。
