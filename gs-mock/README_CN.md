# gs-mock

<div>
   <img src="https://img.shields.io/github/license/go-spring/gs-mock" alt="license"/>
   <a href="https://codecov.io/gh/go-spring/gs-mock" > 
      <img src="https://codecov.io/gh/go-spring/gs-mock/branch/main/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://deepwiki.com/go-spring/gs-mock"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`gs-mock` 是一个现代、类型安全的 Go Mock 库，**全面支持泛型**。
它弥补了传统 Go Mock 工具在**类型安全**与**使用复杂性**方面的不足，
并通过 `context.Context` 链路天然支持并发测试。

`gs-mock` 支持对以下对象进行 Mock：

* 接口（通过代码生成）
* 普通函数
* 结构体方法

非常适合微服务架构下的**单元测试**与**组件测试**场景。

## 特性

### 类型系统与语言特性

* **类型安全 & 泛型支持**

    * 原生支持泛型接口与泛型函数
    * IDE 可提供完整的类型推导与自动补全

* **多参数与多返回值支持**

    * 最多支持 **7 个参数**
    * 最多支持 **4 个返回值**
    * 覆盖绝大多数真实业务函数签名

### Mock 模式

* **Handle 模式**

    * 在单个回调中处理完整的 Mock 逻辑
    * 适合包含复杂条件判断的场景

* **When / Return 模式**

    * 基于条件匹配返回结果
    * 更适合声明式、可读性更高的 Mock 场景

### Mock 对象类型

* **接口 Mock**

    * 通过代码生成自动生成 Mock 实现
    * 不依赖 `context.Context` 参数

* **普通函数 & 结构体方法 Mock**

    * 通过 `context.Context` 传播 Mock 配置
    * 无需为函数或方法额外抽象接口

### 并发与上下文

* **并发测试支持**

    * 通过 `context.Context` 传递 Mock Manager
    * 确保并发测试场景下的 Mock 隔离性与安全性

## 安装

### 单独安装

```
go install github.com/go-spring/gs-mock@latest
```

### 通过 gs 工具集安装

```
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/go-spring/gs/HEAD/install.sh)"
```

## 快速开始

### 一、接口 Mock

#### 1. 定义接口

```
type Service interface {
    Do(n int, s string) (int, error)
    Format(s string, args ...any) string
}
```

#### 2. 生成 Mock 代码

```
//go:generate gs mock -o src_mock.go
```

> `gs mock` 表示使用 `gs` 工具集中的 `mock` 子命令，也可以直接使用 `gs-mock` 命令。

在接口文件顶部添加上述指令后，即可为当前包内的**所有接口**生成 Mock 代码。
如果只需要为特定接口生成 Mock，可以使用 `-i` 参数：

```
//go:generate gs-mock -o src_mock.go -i '!RepositoryV2,Repository'
```

**`-i` 参数示例说明：**

* `-i 'Repository'`
  仅生成 `Repository` 接口的 Mock
* `-i '!Repository'`
  生成除 `Repository` 外的所有接口
* `-i 'Repository,Service'`
  仅生成 `Repository` 和 `Service` 的 Mock
* `-i '!Repository,Service'`
  生成除 `Repository` 外的接口，但包含 `Service`

#### 3. 使用 Mock（Handle 模式）

```
r := gsmock.NewManager()
s := NewServiceMockImpl(r)

// Handle 模式：根据入参处理返回逻辑
s.MockDo().Handle(func(n int, s string) (int, error) {
    if n%2 == 0 {
        return n * 2, nil
    }
    return n + 1, errors.New("error")
})

fmt.Println(s.Do(1, "abc")) // 2 error
fmt.Println(s.Do(2, "abc")) // 4 <nil>
```

#### 4. 使用 Mock（When / Return 模式）

```
r := gsmock.NewManager()
s := NewServiceMockImpl(r)

// 针对 args[0] == "abc" 的情况，可变参数使用切片类型
s.MockFormat().When(func(s string, args []any) bool {
    return args[0] == "abc"
}).ReturnValue("abc")

// 针对 args[0] == "123" 的情况，可变参数使用切片类型
s.MockFormat().When(func(s string, args []any) bool {
    return args[0] == "123"
}).ReturnValue("123")

fmt.Println(s.Format("", "abc", "123")) // abc
fmt.Println(s.Format("", "123", "abc")) // 123
fmt.Println(s.Format("", "xyz", "abc")) // panic：没有找到匹配的 mock
```

> **注意**
>
> * 不要在同一个方法上混合使用 `Handle` 与 `When/Return` 模式
> * 当存在多个 `When/Return` 配置时，按注册顺序进行匹配，第一个匹配成功的配置会被执行

### 二、函数 Mock

#### 1. 定义普通函数

```
//go:noinline // 防止函数被内联
func Do(ctx context.Context, n int) int { return n }
```

#### 2. Mock 普通函数

```
r := gsmock.NewManager()
ctx := gsmock.WithManager(context.TODO(), r)

// 通过 ReturnValue 返回固定值
gsmock.Func21(Do, r).ReturnValue(2)

fmt.Println(Do(ctx, 1)) // 2
```

**说明：**

* 普通函数 Mock 要求：

    * 第一个参数必须是 `context.Context`
    * Mock 配置通过 Context 链路传播

* `Func21` 表示：

    * 2 个参数
    * 1 个返回值

* 对变参函数可使用 `VarFuncNN` 系列，如 `VarFunc21`

### 三、结构体方法 Mock

#### 1. 定义结构体方法

```
type Service struct{ m int }

func (s *Service) Do(ctx context.Context, n int) int {
    return n
}
```

#### 2. Mock 结构体方法

```
r := gsmock.NewManager()
ctx := gsmock.WithManager(context.TODO(), r)

// 此时第一个参数变成了接收者类型，后面才是方法原来的参数
gsmock.Func31((*Service).Do, r).Handle(func(s *Service, ctx context.Context, n int) int {
    return n + s.m
})

fmt.Println((&Service{m: 1}).Do(ctx, 1)) // 2
fmt.Println((&Service{m: 2}).Do(ctx, 1)) // 3
```

**注意：**

* 使用**方法表达式**（如 `(*Service).Do`），而不是实例方法值（如 `s.Do`）
* 接收者会作为 Mock 回调函数的**第一个参数**，此时 `ctx` 成为**第二个参数**
* 测试时需添加 `-gcflags="all=-N -l"` 以防止方法被内联

> 更多示例和用法参见 [example](example) 目录。

## 常见问题

### 1. 内联优化导致 Mock 失效

* **问题描述**：
  在某些情况下，函数或方法在编译阶段被 Go 编译器内联优化，导致 Mock 逻辑未被触发，从而表现为 Mock 失效。

* **解决方案**：
  在运行测试时显式禁用编译器优化：

  ```
  go test -gcflags="all=-N -l" ./...
  ```

* **说明**：
  该参数会关闭内联与部分优化行为，确保 Mock 框架能够正确拦截函数或方法调用。

### 2. Context 参数要求

* **问题描述**：
  在对普通函数或结构体方法进行 Mock 时，函数签名要求第一个参数必须是 `context.Context`。

* **解决方案**：
  在设计可测试函数时，建议将 `context.Context` 作为第一个参数纳入函数签名，以便通过 Context 链路传播 Mock Manager。

* **注意事项**：

    * 该限制 **仅适用于普通函数与结构体方法的 Mock**
    * **接口 Mock 不要求** 接口方法包含 `context.Context` 参数

### 3. When / Return 注册顺序问题

* **问题描述**：
  当同一个方法上配置了多个 `When/Return` Mock 规则时，匹配顺序会直接影响最终执行结果。

* **解决方案**：
  **第一个匹配成功的规则会被立即执行**，后续规则将被忽略，因此可以按照 **从条件更具体到条件更宽泛** 的顺序注册
  `When/Return` 规则。

### 4. Manager 的作用域与并发安全

* **问题描述**：
  `Mock Manager` 本身并非 goroutine 安全对象，如果在并发逻辑执行过程中动态注册 Mock，可能引发不可预期的行为。

* **解决方案**：
  所有 Mock 的注册操作必须在 **任何并发逻辑启动之前** 完成，并通过 `context.Context` 将 Manager 传递至各个 goroutine 中使用。

### 5. 变参函数的 Mock 方式

* **问题描述**：
  对于变参函数（例如 `Printf(format string, args ...any)`），其参数结构在 Mock 时与普通函数存在差异，需要特殊处理。

* **解决方案**：
  使用 `VarFuncNN` 系列类型对变参函数进行 Mock。

* **说明**：

    * 变参部分会被整体包装为一个切片参数传入 Mock 回调函数
    * 变参函数对应的 Mock 类型统一以 `Var` 作为前缀

## 许可证

本项目采用 Apache License Version 2.0 许可证。
