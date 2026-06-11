# Go-Spring 实战第 29 课 —— 测试体系：纯单测、容器测试和断言边界

Starter 把组件接入规则封装起来以后，工程复杂性并没有消失，只是从“怎么重复写初始化代码”转成了“怎么持续确认装配仍然正确”。配置、条件、Bean、生命周期和外部依赖一旦组合起来，测试就不能只看单个函数的返回值。

但这并不意味着所有测试都要启动 Go-Spring 容器。容器测试能验证装配，也会带来更高成本；断言库能让失败更清楚，但不能替代测试分层。至于外部依赖和不稳定调用的隔离，应该放到更专门的 Mock 边界里处理。

Go-Spring 测试体系的核心不是提供一个新的测试运行器。它仍然运行在 Go 原生 `go test` 之上，只是在需要验证 IoC 装配、配置绑定和断言表达时，补充 `RunTest`、测试配置、替代 Bean、`assert` 和 `require`。

## 纯业务单测

先把最便宜的测试留住。如果被测对象只依赖普通接口，并不依赖 Go-Spring 的装配语义，就应该直接手动构造。

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

对应测试只需要传入替身依赖，直接断言业务行为。

```go
func TestUserService_GetUserName(t *testing.T) {
	mockRepo := &MockUserRepository{}
	service := NewUserService(mockRepo)

	name, err := service.GetUserName(1)

	assert.That(t, err).Nil()
	assert.That(t, name).Equal("Alice")
}
```

这类测试不需要 `gs.RunTest`。它验证的是业务逻辑，而不是配置绑定、条件注册或 Bean 解析。把它留在纯业务单测层，可以让反馈更快，失败定位也更直接。

## RunTest 容器测试

当测试目标变成“这些 Bean 是否能按配置正确装配”时，才需要启动 Go-Spring 测试容器。`gs.RunTest` 的语义是：测试对象可以作为 root Bean 进入容器，完成注入后再执行断言。

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

`RunTest` 会创建一个测试 App，把回调参数中的测试对象放进容器，完成 `autowire` 和 `value` 注入，然后执行回调。回调结束后，测试容器会关闭，相关生命周期也会收束。

所以，`RunTest` 适合验证装配行为：某个 Bean 是否存在，配置是否能绑定，条件是否生效，多个 Bean 是否能协作。它不适合替代所有单元测试，否则简单业务逻辑也会被放进更重的测试路径。

## 测试配置

容器测试常常需要临时配置，例如把数据库地址换成内存实现，把环境名设置为 `test`。`gs.Configure()` 可以把配置限制在当前测试 App 中。

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

这里的配置不会要求修改全局配置文件。它只服务于当前测试创建的 App，并参与同一套配置绑定规则。也就是说，测试配置仍然是 Go-Spring 配置系统的一部分，只是来源从外部文件变成了测试代码里的显式输入。

这种写法适合表达测试场景差异。比如某个条件 Bean 需要 `feature.enabled=true` 才会出现，测试就应该在当前容器里补上这个配置，而不是依赖开发机上的环境变量。

## 替代 Bean

配置可以改变装配条件，但有些依赖需要直接替换，例如外部服务客户端、真实数据库仓储或消息发送器。测试可以在当前 App 中注册替代 Bean。

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

每个 `RunTest` 会基于 `init` 阶段的全局注册信息创建测试容器，`gs.Configure()` 中注册的 Bean 只进入当前 App。这样替代依赖不会污染其他测试。

这里也有一个边界：全局 `init` 注册信息仍然是共享来源，基于 Go-Spring IoC 容器的测试目前不支持 `t.Parallel()`。如果测试需要并行执行，优先把它保持在纯业务单测层，避免共享注册信息带来的干扰。

## assert 与 require

测试失败时，断言节奏也会影响定位效率。Go-Spring 在 `github.com/go-spring/stdlib/testing` 下提供 `assert` 和 `require`，两者的区别在于失败后是否继续执行。

前置条件用 `require`，后续结果检查用 `assert`。

```go
require.That(t, err).Nil()

assert.That(t, user).NotNil()
assert.That(t, user.ID).Equal(1)
assert.That(t, user.Name).Equal("Alice")
```

`require` 失败后会立即终止当前测试，适合“后面断言依赖这个条件成立”的场景。`assert` 失败后继续执行，适合一次收集多个字段差异。

断言入口可以按数据类型选择。

```go
assert.That(t, value).Equal(expected)
assert.Error(t, err).NotNil()
assert.Number(t, 42).GreaterThan(40)
assert.String(t, "hello").Contains("ell")
assert.Slice(t, []int{1, 2, 3}).Length(3)
assert.Map(t, map[string]int{"a": 1}).ContainsKey("a")
```

断言失败时也可以补充业务语境。

```go
assert.That(t, result).Equal(expected, "result should match expected")
```

这些断言不会改变测试模型，但能让失败输出更接近业务语义。尤其是容器测试失败时，清晰断言能帮助区分是装配失败、配置错误，还是业务结果不符合预期。

## 测试分层

Go-Spring 测试体系的边界可以按问题来判断。验证纯业务计算，用纯业务单测；验证配置绑定、条件注册和依赖注入，用 `RunTest`；表达失败语义，用 `assert` 和 `require`。如果测试目标需要隔离外部系统、不稳定调用或难复现错误，再引入 Mock。

这样分层以后，Go-Spring 的框架能力不会把所有测试都推向容器层。测试体系在整体模型里的位置，是给配置、IoC、Starter、HTTP 和生命周期提供持续验证，而不是替代 Go 原生测试习惯。
