# 测试

Go-Spring 兼容 Go 原生的 `go test` 机制，不需要额外的测试运行器，也不会改变项目的构建流程。
我们仍然可以使用 `go test` 运行测试，并与 Go 生态中的覆盖率、竞态检测和 CI 工具配合使用。

此外，Go-Spring 还提供了一组测试增强能力：
支持基于 IoC 容器运行集成测试，并提供类型安全的断言工具与 Mock 支持，从而减少样板代码，
让测试更加简洁、可维护。

## 纯单元测试

纯单元测试通常只验证单个函数、单个方法或单个服务的行为，不需要启动 IoC 容器。
此时建议使用 Go 原生测试方式：手动构造待测对象，并显式传入它依赖的对象或 Mock 实现。

例如，一个用户服务依赖用户仓库：

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

测试的时候我们可以直接创建一个 Mock `UserRepository` 实现，再传入待测服务：

```go
func TestUserService_GetUserName(t *testing.T) {
    mockRepo := &MockUserRepository{...}
    service := NewUserService(mockRepo)

    name, err := service.GetUserName(1)

    // 进行断言...
}
```

这种方式不依赖框架特性，启动成本低，运行速度快，也更容易定位问题。
对于只关心业务逻辑的测试，应当优先使用纯单元测试。

## 基于 IoC 容器的测试

当测试需要验证多个 Bean 的协作，或者需要覆盖配置注入、依赖注入、条件装配等容器行为时，
我们可以使用 `gs.RunTest` 启动一个基于 IoC 容器的测试。

`gs.RunTest` 会根据回调函数的参数类型创建一个测试对象。
这个测试对象会作为 root Bean 注册到容器，之后容器从它出发完成依赖图的构建和注入。
整体流程如下：

1. 读取回调函数的唯一参数类型，并创建对应的测试对象实例。
2. 将该测试对象注册为 root Bean，作为依赖注入的入口。
3. 启动测试容器，刷新配置并解析已注册的 Bean 定义。
4. 从 root Bean 开始递归注入依赖，同时处理 `value`、`autowire` 等标签。
5. 调用测试回调，传入已经完成注入的测试对象。
6. 测试回调结束后关闭容器，并执行相关销毁逻辑。

因此，测试代码不需要我们手动启动或关闭容器，只需要在回调函数中编写断言逻辑即可。
回调参数可以使用 `value`、`autowire` 等 Go-Spring 标签，和普通 Bean 的使用方式一致。

```go
func TestOrderFlow(t *testing.T) {
	gs.RunTest(t, func(s *struct {
		OrderSvc *OrderService `autowire:""`              // 注入订单服务
		StockSvc *StockService `autowire:""`              // 注入库存服务
		Env      string        `value:"${app.env:=test}"` // 注入配置
	}) {
		err := s.OrderSvc.Create(1001)
		assert.That(t, err).Nil()
		assert.That(t, s.StockSvc.Remaining(1001)).Equal(99)
	})
}
```

### 自定义配置

我们可以在 `gs.RunTest()` 之前通过 `gs.Configure()` 调用 `g.Property()`，
为当前测试添加配置项：

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

		// 测试逻辑
	})
}
```

### 替换依赖

我们可以在 `gs.RunTest()` 之前通过 `gs.Configure()` 调用 `g.Provide()`，
为当前测试添加 Bean。

> 在运行基于 IoC 容器的测试时，受 Bean 发现机制影响，Go-Spring 不保证自动创建所有依赖项，
> 因此测试中的依赖注入都被视为可选的（nullable）。
> 如果某些关键 Bean 未被自动加载，可以通过这种方式显式注册需要的 Bean，以确保测试环境完整可控。

```go
func TestUserService(t *testing.T) {
	gs.Configure(func(g gs.App) {
		g.Provide(func() UserRepository {
			return &MockUserRepository{ /* 预置测试数据 */ }
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

### 隔离性说明

每个 `gs.RunTest()` 测试都会复制 `init` 阶段注册的 Bean 定义。
然后 `gs.Configure()` 中注册的 Bean 只作用于当前测试，不会影响其他测试用例。

由于全局 `init` 注册信息是共享的，因此基于 IoC 容器的测试目前不支持 `t.Parallel()`。

## 断言库

Go-Spring 在 `github.com/go-spring/stdlib/testing` 下提供了流式断言库，
包括 `assert` 和 `require` 两个子包。

### assert 与 require

`assert` 包在断言失败时会记录失败信息，但不会立即终止当前测试函数。
它适合检查多个互不依赖的条件，让一次测试运行尽可能暴露更多问题。

`require` 包在断言失败时会立即终止当前测试函数。
它适合检查后续逻辑的前置条件，例如对象必须非 nil、初始化必须成功等场景。

```go
import (
    "github.com/go-spring/stdlib/testing/assert"
    "github.com/go-spring/stdlib/testing/require"
)

func TestExample(t *testing.T) {
	user, err := service.GetUser(1)

	// err 不为 nil 时立即终止，避免后续代码继续访问无效对象
	require.That(t, err).Nil()

	// 后续断言即使失败，也会继续执行其他 assert
	assert.That(t, user).NotNil()
	assert.That(t, user.ID).Equal(1)
	assert.That(t, user.Name).Equal("Alice")
}
```

### 基础用法

断言库提供按类型区分的入口。不同的入口暴露不同的断言方法，可以帮助我们在编译期尽早发现类型误用。

任意类型可以使用 `That`：

```go
assert.That(t, 42).Equal(42)     // 相等断言
assert.That(t, user).NotNil()    // 非 nil 断言
assert.That(t, ok).True()        // 布尔值为 true
assert.That(t, available).False() // 布尔值为 false
```

错误类型可以使用 `Error`：

```go
import "os"

err := someFunc()
assert.Error(t, err).NotNil()           // 期望有错误
assert.Error(t, os.ErrNotExist).Is(err) // 使用 errors.Is 判断错误类型
```

数值类型可以使用 `Number`：

```go
import "math"

assert.Number(t, 42).GreaterThan(40)          // 大于
assert.Number(t, 100).Between(0, 200)         // 在区间内
assert.Number(t, 0).Zero()                    // 等于零
assert.Number(t, 3.14).InDelta(math.Pi, 0.01) // 浮点比较容差
```

字符串类型可以使用 `String`：

```go
assert.String(t, "user@example.com").IsEmail()                  // 验证邮箱格式
assert.String(t, "hello world").Contains("world")               // 包含子串
assert.String(t, "hello").HasPrefix("he")                       // 前缀检查
assert.String(t, `{"name": "bob"}`).JSONEqual(`{"name":"bob"}`) // JSON 结构相等
```

切片类型可以使用 `Slice`：

```go
assert.Slice(t, []int{1, 2, 3}).Contains(2) // 包含元素
assert.Slice(t, []int{1, 2, 3}).Length(3)   // 长度检查
assert.Slice(t, []int{1, 2, 3}).NotEmpty()  // 非空检查
assert.Slice(t, []int{1, 2, 3}).AllUnique() // 所有元素唯一
```

map 类型可以使用 `Map`：

```go
m := map[string]int{"a": 1, "b": 2}
assert.Map(t, m).ContainsKey("a")          // 包含 key
assert.Map(t, m).ContainsKeyValue("a", 1)  // 包含 key-value 对
assert.Map(t, m).Length(2)                 // 长度检查
```

检查 panic 可以使用顶层函数 `Panic`：

```go
assert.Panic(t, func() {
    panic("something wrong happened")
}, "wrong") // 期望发生 panic，并且信息包含 "wrong"
```

### 自定义错误消息

断言方法支持在末尾追加自定义错误信息，使断言失败时的输出包含更明确的业务语义，便于快速定位问题。

```go
assert.That(t, result).Equal(expected, "result should match expected")
assert.Number(t, age).GreaterThan(18, "user should be an adult")
```

## Mock 框架

Go-Spring 提供了 `gs-mock` 作为 Mock 框架。
它支持接口 Mock、普通函数 Mock 和结构体方法 Mock，并通过泛型生成类型安全的调用 API。

### 接口 Mock

接口 Mock 可以通过代码生成创建实现类，而不需要手写 Mock 结构体。假设有如下接口：

```go
type Service interface {
    Do(n int, s string) (int, error)
    Format(format string, args ...any) string
}
```

然后，我们需要在包级别添加 `go:generate` 指令：

```go
//go:generate gs mock -o mock.go
```

该命令会为当前包内的接口生成 Mock 代码。如果只需要为部分接口生成 Mock，可以使用 `-i` 参数：

```go
//go:generate gs mock -o mock.go -i "Service,Repository"
```

生成代码后，我们可以在测试中使用 Handle 模式或者 When/Return 模式进行 Mock 配置。

Handle 模式适合需要自定义逻辑的场景：

```go
func TestService_Do(t *testing.T) {
	r := gsmock.NewManager()
	s := NewServiceMockImpl(r)

	s.MockDo().Handle(func(n int, s string) (int, error) {
		if n%2 == 0 {
			return n * 2, nil
		}
		return 0, errors.New("odd number")
	})

	res, err := s.Do(2, "abc")
	assert.That(t, err).Nil()
	assert.That(t, res).Equal(4)
}
```

When/Return 模式适合根据入参匹配固定返回值的场景：

```go
func TestService_Format(t *testing.T) {
    r := gsmock.NewManager()
    s := NewServiceMockImpl(r)

    // 当 args 第一个元素是 "abc" 时返回 "abc"
    s.MockFormat().When(func(format string, args []any) bool {
        return args[0] == "abc"
    }).ReturnValue("abc")

    // 当 args 第一个元素是 "123" 时返回 "123"
    s.MockFormat().When(func(format string, args []any) bool {
        return args[0] == "123"
    }).ReturnValue("123")

    // 匹配规则按注册顺序执行，第一个匹配成功的规则会返回结果
    assert.That(t, s.Format("", "abc", "123")).Equal("abc")
    assert.That(t, s.Format("", "123", "abc")).Equal("123")
}
```

对于变参方法，可变参数会被整体打包为一个切片参数传入 `When` 回调，例如上述示例中的 `args []any`。

### 函数和方法 Mock

除接口 Mock 外，`gs-mock` 也可以 Mock 普通函数和结构体方法。
这种方式适合为已有代码补充测试，避免为了测试抽象接口。

普通函数 Mock 要求函数的第一个参数是 `context.Context`。
Mock Manager 会通过 Context 传递，从而隔离不同测试或不同调用链中的 Mock 配置。

```go
//go:noinline // 建议添加，防止函数被内联导致 Mock 失败
func GetUser(ctx context.Context, id int) (*User, error) {
    // 真实实现从数据库查询
}

func Test_GetUser(t *testing.T) {
	r := gsmock.NewManager()
	ctx := gsmock.WithManager(context.TODO(), r)

	// Func22 表示 2 个参数、2 个返回值
	gsmock.Func22(GetUser, r).Handle(func(ctx context.Context, id int) (*User, error) {
		if id == 1 {
			return &User{ID: 1, Name: "Alice"}, nil
		}
		return nil, errors.New("not found")
	})

	user, err := GetUser(ctx, 1)
	assert.That(t, err).Nil()
	assert.That(t, user.Name).Equal("Alice")
}
```

结构体方法的 Mock 方式与之类似。但需要注意的是，方法表达式会把接收者作为第一个参数传入：

```go
type Service struct {
    BaseURL string
}

func (s *Service) GetUser(ctx context.Context, id int) (*User, error) {
    // 真实实现
}

func TestService_GetUser(t *testing.T) {
    r := gsmock.NewManager()
    ctx := gsmock.WithManager(context.TODO(), r)

    // 使用方法表达式 (*Service).GetUser，而不是实例方法
    // 接收者 *Service 是第一个参数，ctx 是第二个参数
    gsmock.Func32((*Service).GetUser, r).Handle(func(s *Service, ctx context.Context, id int) (*User, error) {
        if s.BaseURL == "https://api.example.com" {
            return &User{ID: id, Name: "Mocked"}, nil
        }
        return nil, errors.New("wrong endpoint")
    })

    svc := &Service{BaseURL: "https://api.example.com"}
    user, err := svc.GetUser(ctx, 1)
    assert.That(t, err).Nil()
    assert.That(t, user.Name).Equal("Mocked")
}
```

### 使用提示

在使用函数或方法 Mock 时，需要注意以下事项：

- **禁用内联**：Go 编译器可能内联小函数，导致 Mock 无法拦截调用。
  我们可以在运行测试时添加参数 `-gcflags="all=-N -l"` 来禁用内联：

```bash
go test -gcflags="all=-N -l" ./...
```

- **通过 Context 隔离**：`gs-mock` 通过 `context.Context` 传递 Mock Manager。
  每个测试创建自己的 Manager，可以避免不同测试或 goroutine 之间互相影响。

- **提前注册规则**：所有 Mock 规则都应在测试逻辑开始前注册完成。
  不要在并发执行过程中动态注册规则，以免匹配顺序不可预期。

- **匹配顺序**：多个 When/Return 规则按注册顺序匹配，第一个匹配成功的规则会立即返回结果。
  建议按照从具体到宽泛的顺序注册规则。
