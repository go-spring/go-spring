# 测试

在 Go-Spring 生态中，测试始终是一等公民。得益于 Go 原生设计和 Go-Spring 框架的精心打磨，你可以写出既优雅又可靠的测试代码。无论你是想对单个函数进行单元测试，还是对完整的 IoC 容器服务进行集成测试，Go-Spring 都能提供恰到好处的支持。

## 测试基础

Go-Spring 完全兼容 Go 原生 `go test` 机制，不需要额外的测试运行器，也不会侵入你的构建流程。你依然使用 `go test` 命令运行测试，所有 Go 生态中的测试工具都可以无缝搭配使用。

实际上，Go-Spring 提供的是一套**组合拳**：你既可以在 IoC 容器的帮助下进行完整的集成测试，也可以脱离容器写出纯粹的单元测试，再配合功能丰富的断言库和类型安全的 Mock 框架，让你的测试代码既简洁又强大。

## 不使用 IoC 的单元测试

对于纯粹的单元测试，我们往往只需要测试单个函数或单个服务，并不需要启动整个 IoC 容器。这种情况下，你完全可以按照 Go 原生的方式编写测试，只需要通过依赖注入把待测对象的依赖准备好就行。依赖注入本身就是为了方便测试而生的，所以这种方式写出来的测试通常非常简洁。

举个例子，假设我们有一个用户服务，它依赖一个用户仓库：

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

在单元测试中，我们可以直接手动创建一个 Mock 仓库实例，然后传入待测服务：

```go
func TestUserService_GetUserName(t *testing.T) {
    mockRepo := &MockUserRepository{...}
    service := NewUserService(mockRepo)
    name, err := service.GetUserName(1)
    // 进行断言...
}
```

这种测试方式不依赖任何框架特性，运行速度快，调试方便，是单元测试的首选。Go-Spring 推荐你尽可能使用这种方式编写单元测试，保持测试的简洁性。

## 使用 IoC 容器的集成测试

当你需要测试多个 Bean 之间的协作，或者需要验证配置注入、依赖注入等 IoC 容器特性是否正常工作时，就可以使用 `gs.RunTest` 来快速启动一个测试容器。`RunTest` 会自动帮你完成容器启动、依赖注入，并在测试结束后优雅关闭容器。

最简单的用法就是直接传入测试函数，Go-Spring 会自动提取参数类型并创建根 Bean：

```go
func TestMyService(t *testing.T) {
    gs.RunTest(t, func(service *MyService) {
        // service 已经完成依赖注入，可以直接使用
        result := service.DoSomething()
        assert.Equal(t, "expected", result)
    })
}
```

你也可以手动创建 `gs.Context`，通过 `ctx.Provide` 注册自定义 Bean，然后调用 `ctx.Run`：

```go
func TestUserService(t *testing.T) {
	ctx := gs.NewContext()

	// 注册 Mock 实现覆盖默认的全局注册
	mockRepo := &MockUserRepository{
		// ...
	}
	ctx.Provide(func() UserRepository { return mockRepo })

	// 运行测试
	ctx.Run(func(service *UserService) {
		// 测试逻辑
		user, err := service.GetUser(1)
		assert.NoError(t, err)
		assert.Equal(t, 1, user.ID)
	})
}
```

这种方式不需要修改生产代码，就能轻松替换依赖。

在这个例子中，`RunTest` 会自动提取 `service *MyService` 的类型信息，将其注册为根 Bean，然后启动容器，最后回调你的测试函数。整个过程一气呵成，你不需要写任何容器启动或关闭的代码。

如果你需要在测试前自定义配置，比如覆盖某些属性，可以使用 `Configure` 链式调用：

```go
func TestMyService_WithCustomConfig(t *testing.T) {
    gs.Configure(func(g gs.App) {
        g.Property("database.url", "sqlite://:memory:")
        g.Provide(func() *MockDatabase { return &MockDatabase{} })
    }).RunTest(t, func(service *MyService) {
        // 现在 service 使用的是自定义配置
        result := service.DoSomething()
        assert.Equal(t, "expected", result)
    })
}
```

你也可以在测试结构体中直接使用 `value` 和 `autowire` 标签，就像在 regular Bean 中一样：

```go
func TestApp(t *testing.T) {
    gs.Configure(func(g gs.App) {
        g.Property("app.name", "test-app")
    }).RunTest(t, func(s *struct {
        Name    string        `value:"${app.name:=default}"`
        Service *MyService    `autowire:""`
    }) {
        assert.Equal(t, "test-app", s.Name)
        assert.NotNil(t, s.Service)
    })
}
```

关于数据隔离，这里需要特别说明一下。每个测试都会拷贝 `init` 阶段注册的 Bean，所以不同测试之间天然做到了数据隔离。如果你在 `Configure` 中注册了自定义 Bean，这些 Bean 也只会作用于当前测试，不会影响其他测试。这意味着你可以在同一个测试文件中写多个 `RunTest` 用例，而不用担心它们互相干扰。

不过需要注意，由于全局 `init` 注册是共享的，IoC 测试目前不支持并行执行。如果你使用 `t.Parallel()`，可能会导致意外的结果。这一点在使用时需要留意。

我们来看一个更完整的例子，这是一个真实的测试用例：

```go
package gs_test

import (
    "fmt"
    "testing"

    "github.com/go-spring/spring-core/gs"
)

func init() {
    gs.Provide(func() *GlobalService {
        return &GlobalService{}
    })
}

type GlobalService struct {
    Name string `value:"${name:=global}"`
}

func TestBasic(t *testing.T) {
    gs.RunTest(t, func(s *App1Service) {
        fmt.Println(s.Name, s.Svr.Name)
        // 输出: app1 global
    })
}

func TestWithCustomProperty(t *testing.T) {
    gs.Configure(func(g gs.App) {
        g.Property("name", "myapp2")
    }).RunTest(t, func(s *struct {
        Name string         `value:"${name:=app2}"`
        Svr  *GlobalService `autowire:""`
        App1 *App1Service   `autowire:"?"`
    }) {
        fmt.Println(s.Name, s.Svr.Name, s.App1)
        // 输出: myapp2 myapp2 <nil>
        // 因为 App1Service 没有在这个测试前被注册，所以自动注入为 nil
        // `autowire:"?"` 表示允许 nullable
    })
}
```

从这个例子可以看出，`RunTest` 的使用非常灵活，既能兼容全局注册的 Bean，也能支持测试级别的自定义配置，满足各种集成测试场景的需求。

### 在集成测试中 Mock 依赖

在编写集成测试时，我们常常需要把某些真实的依赖替换成 Mock 对象，比如把真实的数据库访问层换成 Mock，避免测试依赖外部服务。这件事在 Go-Spring 中非常简单，你只需要在 `Configure` 中重新提供一个 Mock 实现即可，后注册的 Bean 会覆盖先前注册的 Bean。

来看一个例子，假设我们要测试 `UserService`，但不想使用真实的 `UserRepository`：

```go
func init() {
    // 生产环境注册真实实现
    gs.Provide(NewRealUserRepository)
    gs.Provide(NewUserService)
}

func TestUserService(t *testing.T) {
    gs.Configure(func(g gs.App) {
        // 在测试中覆盖，重新提供 Mock 实现
        g.Provide(func() UserRepository {
            return &MockUserRepository{
                // 预置测试数据
            }
        })
    }).RunTest(t, func(service *UserService) {
        // 这里 service 拿到的就是注入了 Mock 的 UserRepository
        result := service.GetUserName(1)
        assert.Equal(t, "Alice", result)
    })
}
```

这种机制非常灵活，你可以只替换你想 Mock 的那个 Bean，其他 Bean 依然使用真实实现，这样既保持了测试的集成性，又避免了对外部资源的依赖。如果你需要使用 `gs-mock` 生成的 Mock 对象，用法也一样：

```go
func TestUserService(t *testing.T) {
    r := gsmock.NewManager()
    mockRepo := NewUserRepositoryMockImpl(r)
    mockRepo.MockFindByID().ReturnValue(&User{ID: 1, Name: "Alice"}, nil)

    gs.Configure(func(g gs.App) {
        g.Provide(func() UserRepository {
            return mockRepo
        })
    }).RunTest(t, func(service *UserService) {
        name, err := service.GetUserName(1)
        assert.Nil(t, err)
        assert.Equal(t, "Alice", name)
    })
}
```

就是这么简单，只需要重新 Provide 一下就好了，Go-Spring 会自动使用你提供的新 Bean 覆盖原来的。

## 流畅的断言库

写完测试逻辑之后，下一步就是做断言了。Go-Spring 内置了一个功能丰富、类型安全的流式断言库，位于 `github.com/go-spring/stdlib/testing` 包下，分为 `assert` 和 `require` 两个子包，你可以直接拿来使用，不需要引入第三方断言库。

### assert vs require

Go-Spring 提供两种断言模式，满足不同场景的需求：

`assert` 包的断言在失败时**不会终止测试执行**，后续断言会继续执行。这样你可以在一次测试运行中看到所有失败的断言，方便一次性修复多个问题。比如你想同时检查多个字段是否正确，即便是第一个字段就错了，后面的检查还会继续进行，这样你可以一次性看到所有错误。

`require` 包的断言在失败时**会立即终止测试执行**。当关键条件不满足时，后续的测试已经没有意义，继续执行只会产生更多无意义的错误，甚至可能引发 panic。比如你先需要验证一个对象非 nil，才能调用它的方法，这时候用 `require` 就比较合适，如果对象为 nil 测试直接终止，避免后面代码 panic。

```go
import (
    "github.com/go-spring/stdlib/testing/assert"
    "github.com/go-spring/stdlib/testing/require"
)

func TestExample(t *testing.T) {
    user, err := service.GetUser(1)
    
    // 如果 err 不为 nil，测试直接终止，不执行后面代码
    require.Nil(t, err)
    
    // 即便这里有断言失败，测试还会继续执行后面的断言
    assert.NotNil(t, user)
    assert.Equal(t, 1, user.ID)
    assert.Equal(t, "Alice", user.Name)
}
```

### 基础用法

Go-Spring 的断言采用流畅的流式 API，根据不同类型进入不同的断言方法链，编译就能检查类型，不会出现类型错误。

对于任意类型，都可以使用 `That` 开头：

```go
// 普通值断言
assert.That(t, 42).Equal(42)          // 相等断言
assert.That(t, user).NotNil()          // 非 nil 断言
assert.That(t, ok).True()              // 布尔值为 true
assert.That(t, available).False()      // 布尔值为 false
```

然后是错误类型专用的断言入口 `Error`：

```go
err := someFunc()
assert.Error(t, err).NotNil()           // 期望有错误
assert.Error(t, err).Nil()              // 期望没有错误
assert.Error(t, err).Is(os.IsNotExist)   // 使用 errors.Is 判断错误类型
```

对于数值类型，使用 `Number` 入口：

```go
import "math"

assert.Number(t, 42).GreaterThan(40)    // 大于
assert.Number(t, 100).Between(0, 200)    // 在区间内
assert.Number(t, 0).Zero()               // 等于零
assert.Number(t, 3.14).InDelta(math.Pi, 0.01) // 浮Point比较容差
```

对于字符串类型，使用 `String` 入口：

```go
assert.String(t, "user@example.com").IsEmail()      // 验证邮箱格式
assert.String(t, "hello world").Contains("world")   // 包含子串
assert.String(t, "hello").HasPrefix("he")            // 前缀检查
assert.String(t, `{"name": "bob"}`).JSONEqual(`{"name":"bob"}`) // JSON 结构相等
```

对于切片类型，使用 `Slice` 入口：

```go
assert.Slice(t, []int{1, 2, 3}).Contains(2)         // 包含元素
assert.Slice(t, []int{1, 2, 3}).Length(3)           // 长度检查
assert.Slice(t, []int{1, 2, 3}).NotEmpty()           // 非空检查
assert.Slice(t, []int{1, 2, 3}).AllUnique()         // 所有元素唯一
```

对于 map 类型，使用 `Map` 入口：

```go
m := map[string]int{"a": 1, "b": 2}
assert.Map(t, m).ContainsKey("a")                    // 包含 key
assert.Map(t, m).ContainsKeyValue("a", 1)           // 包含 key-value 对
assert.Map(t, m).Length(2)                           // 长度检查
```

最后还有专门检查 panic 的顶层函数：

```go
assert.Panic(t, func() {
    panic("something wrong happened")
}, "wrong")  // 期望发生 panic，并且信息包含 "wrong"
```

### 自定义错误消息

所有断言方法都支持在最后添加自定义错误消息，方便定位问题：

```go
assert.That(t, result).Equal(expected, "result should match expected")
assert.Number(t, age).GreaterThan(18, "user should be an adult")
```

当断言失败时，自定义消息会一并显示在错误输出中，帮你更快定位问题。

可以看到，这套断言 API 覆盖了日常测试中的绝大多数断言需求，而且通过类型特定的入口保证了类型安全，编译器就能帮你查出错误，不用等到运行时才发现问题。

## 强大的 Mock 框架

单元测试离不开 Mock。Go-Spring 提供了 `gs-mock` 这个现代的 Mock 框架，它天然支持泛型，类型安全，并发安全，使用体验非常好。

`gs-mock` 支持三种 Mock 场景：接口 Mock、普通函数 Mock、结构体方法 Mock。我们一个一个来看。

### 接口 Mock

对于接口，`gs-mock` 通过代码生成自动生成 Mock 实现，不需要你手动写 Mock，而且生成的代码完全支持泛型，IDE 可以正常补全，类型绝对安全。

第一步，先定义你的接口：

```go
type Service interface {
    Do(n int, s string) (int, error)
    Format(format string, args ...any) string
}
```

第二步，在包级别添加 `go:generate` 指令：

```go
//go:generate gs mock -o mock.go
```

这会为当前包内所有接口生成 Mock 代码。如果你只想为特定接口生成，可以使用 `-i` 参数：

```go
//go:generate gs mock -o mock.go -i "Service,Repository"
```

生成完成后，就可以在测试中使用了。`gs-mock` 支持两种 Mock 模式：Handle 模式和 When/Return 模式。

Handle 模式适合复杂逻辑，你可以在回调中完全控制返回结果：

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
    
    // 现在调用 s.Do 会执行我们的 Mock 逻辑
    res, err := s.Do(2, "abc")
    assert.Nil(t, err)
    assert.Equal(t, 4, res)
}
```

When/Return 模式更简洁，适合根据条件返回不同结果的场景：

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
    
    // 匹配规则按照注册顺序，第一个匹配成功就返回结果
    assert.Equal(t, "abc", s.Format("", "abc", "123"))
    assert.Equal(t, "123", s.Format("", "123", "abc"))
}
```

需要注意的是，对于变参方法，变参会整体被打包为一个切片参数传入 When 的回调函数，就像上面例子中的 `args []any` 那样。

### 函数和方法 Mock

除了接口，`gs-mock` 还可以直接 Mock 普通函数和结构体方法，不需要你额外抽象出接口。这对于改造已有代码的测试非常友好。

普通函数 Mock 要求函数第一个参数是 `context.Context`，Mock 配置会通过 Context 链路传播，这样天然保证了并发测试的隔离性。

来看一个普通函数 Mock 的例子：

```go
//go:noinline // 建议添加，防止函数被内联导致 Mock 失败
func GetUser(ctx context.Context, id int) (*User, error) {
    // 真实实现从数据库查询
}

func Test_GetUser(t *testing.T) {
    r := gsmock.NewManager()
    ctx := gsmock.WithManager(context.TODO(), r)
    
    // Func21 表示 2 个参数，1 个返回值
    gsmock.Func21(GetUser, r).Handle(func(ctx context.Context, id int) (*User, error) {
        if id == 1 {
            return &User{ID: 1, Name: "Alice"}, nil
        }
        return nil, errors.New("not found")
    })
    
    // 调用时会执行 Mock 逻辑
    user, err := GetUser(ctx, 1)
    assert.Nil(t, err)
    assert.Equal(t, "Alice", user.Name)
}
```

对于结构体方法，Mock 方式类似，只需要注意接收者会作为第一个参数传入：

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
    
    // 使用方法表达式 (*Service).GetUser，不是实例方法
    // 接收者 *Service 成为第一个参数，ctx 成为第二个参数
    gsmock.Func32((*Service).GetUser, r).Handle(func(s *Service, ctx context.Context, id int) (*User, error) {
        // 你甚至可以根据 s.BaseURL 决定返回结果
        if s.BaseURL == "https://api.example.com" {
            return &User{ID: id, Name: "Mocked"}, nil
        }
        return nil, errors.New("wrong endpoint")
    })
    
    // 调用时会执行 Mock 逻辑
    svc := &Service{BaseURL: "https://api.example.com"}
    user, err := svc.GetUser(ctx, 1)
    assert.Nil(t, err)
    assert.Equal(t, "Mocked", user.Name)
}
```

### 一些使用提示

使用函数或方法 Mock 时，有几点需要特别注意：

第一，Go 编译器会对小函数进行内联优化，如果函数被内联了，Mock 框架就无法拦截调用，导致 Mock 失效。解决办法很简单，运行测试时加上参数：

```bash
go test -gcflags="all=-N -l" ./...
```

这会禁用编译器优化和内联，保证 Mock 正常工作。

第二，`gs-mock` 通过 `context.Context` 传递 Mock Manager，每个测试创建自己的 Manager，所以并发测试是安全的，不同 goroutine 之间不会互相干扰。

第三，所有 Mock 注册都应该在测试逻辑开始执行之前完成，不要在并发运行过程中动态注册 Mock，虽然 Manager 本身是并发安全的，但动态注册可能导致不可预期的匹配顺序问题。

第四，当你在同一个方法上配置了多个 When/Return 规则，匹配是按照注册顺序进行的，第一个匹配成功的规则就会立即返回结果，后面的规则不会再试。所以建议你按照**从具体到宽泛**的顺序注册规则，把更具体的条件放在前面，更宽泛的放在后面。

## 总结

Go-Spring 的测试体系可以用一句话概括：**尊重原生，灵活扩展**。你可以：

- 写不依赖 IoC 的单元测试，配合 `assert/require` 断言和 `gs-mock` Mock，干净利落
- 写基于 IoC 容器的集成测试，使用 `RunTest` 一键启动，自动注入，隔离保证

整个体系没有侵入性，你依然用 `go test` 运行测试，现有的 Go 测试工具都能无缝协作。关键是，每一部分都精心设计，用起来顺手，类型安全，不容易出错，帮你把更多精力放在测试逻辑本身，而不是和框架较劲。

无论是简单的单元测试还是复杂的集成测试，Go-Spring 都能给你恰到好处的支持，让写测试变成一件轻松愉快的事情。
