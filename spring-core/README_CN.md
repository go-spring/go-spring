# Go-Spring

<div>
   <img src="https://img.shields.io/github/license/go-spring/spring-core" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/spring-core" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/spring-core?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/spring-core" >
      <img src="https://codecov.io/gh/go-spring/spring-core/branch/main/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://goreportcard.com/report/github.com/go-spring/spring-core">
      <img src="https://goreportcard.com/badge/github.com/go-spring/spring-core" alt="Go Report Card"/>
   </a>
   <a href="https://deepwiki.com/go-spring/spring-core">
      <img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki">
   </a>
</div>

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

**Go-Spring 是一个面向现代 Go 应用开发的高性能框架，灵感源自 Java 生态的 Spring / Spring Boot。**
其设计理念深度融合 Go 语言原生特性，既传承了 Spring 生态成熟的开发范式——依赖注入（DI）、自动配置与生命周期管理，
又规避了传统框架可能带来的复杂度与性能开销。
Go-Spring 让开发者在保持 Go 原生风格与执行效率的同时，尽享高层抽象与自动化开发带来的便利。

**无论你开发单体应用，还是构建微服务分布式系统，Go-Spring 都能提供统一而灵活的开发体验。**
框架以"开箱即用"的方式简化项目初始化，减少样板代码编写，且不强制侵入式架构，让开发者专注于业务逻辑的实现。
Go-Spring 致力于提升开发效率、增强可维护性、保障系统一致性，是 Go 语言生态中颇具里程碑意义的开发框架。

## 1. 🚀 特性一览

Go-Spring 融合了成熟的依赖注入与自动配置设计思想，秉持 Go 语言"大道至简"的哲学，
提供了丰富实用的特性，助力开发者高效构建现代 Go 应用：

1. ⚡ **极致启动性能，运行零反射**
   - 基于 Go 原生 `init()` 机制预注册 Bean，**无运行时扫描**，启动耗时仅毫秒级别；
   - 仅在**初始化阶段**使用反射完成依赖注入，初始化后**全程零反射运行**，性能媲美手写代码。

2. 🧩 **无侵入式 IoC 容器**
   - 不强制接口依赖或继承结构，业务逻辑保持原生 Go 风格，真正做到无侵入；
   - 支持单独使用依赖注入，亦可全栈框架开发，灵活不绑定，完全兼容 Go 标准库；
   - 提供完备的 Bean 生命周期管理，原生支持 `Init` 和 `Destroy` 钩子。

3. 💉 **灵活多样的 Bean 依赖注入**
   - 支持多种注入方式：结构体字段注入、构造函数注入、构造函数参数注入；
   - 支持按类型、名称、标签多种匹配策略，覆盖各类场景需求。

4. 🏷️ **便捷的 Value 配置绑定**
   - 配置值直接绑定到结构体字段，无需手动解析；
   - 支持默认值语法 `${key:=default}`，优雅兜底；
   - 内置字段校验功能，自动检查配置正确性。

5. 🎯 **强大的条件注入系统**
   - 支持根据配置、环境、上下文等条件动态决定 Bean 是否注册；
   - 提供多种常用条件类型，支持逻辑组合（与/或/非）；
   - 为模块化自动装配奠定坚实基础。

6. ⚙️ **分层配置体系**
   - 支持多来源（命令行、环境变量、配置文件、内存）、多格式（YAML、TOML、Properties）配置加载；
   - 清晰的配置优先级分层，自动覆盖，原生支持多环境隔离；
   - 支持配置导入，可集成远程配置中心，满足云原生部署需求。

7. 🔄 **配置热更新，实时生效**
   - 独创 `gs.Dync[T]` 泛型原生支持热更新，配置变更无需重启应用；
   - **完全兼容 Value 绑定语法**，使用方式一致，简单易上手；
   - 配置自动同步至字段，灰度发布、在线调参一气呵成。

8. 🏗️ **模块化自动装配**
   - 基于条件注入实现模块化自动装配；
   - 模块化设计，按需装配，真正开箱即用；
   - 生态提供丰富的 Starter 模块，快速集成各类功能。

9. 🔌 **清晰的应用运行模型**
   - 抽象 `Runner`（一次性任务）和 `Server`（长期服务）两种运行模型；
   - 内置 HTTP Server 启动器，支持多服务并发启动；
   - 完备的生命周期钩子，支持优雅启停、信号处理。

10. 🧪 **与 `go test` 原生集成的测试能力**
    - `gs.RunTest()` 一键启动容器，自动完成依赖注入；
    - 测试结束自动优雅关闭，无需额外脚手架代码。

11. 🪵 **生态开箱即用的日志系统**
    - Go-Spring 生态提供原生集成的结构化日志模块；
    - 统一日志接口，支持多输出，可适配各类日志实现。

## 2. 📦 安装方式

Go-Spring 使用 Go Modules 管理依赖，安装非常简单：

```bash
go get github.com/go-spring/spring-core
```

## 3. 🚀 快速开始

Go-Spring 主打"开箱即用"，下面通过两个示例，快速感受框架的强大能力：
- **示例一**展示 Go-Spring 如何与**标准库完美集成**，无需改变你的编码习惯
- **示例二**展示框架的一些核心特性，诸如依赖注入、配置绑定、动态刷新等

### 示例一：最小 API 服务（与标准库无缝集成）

这个示例展示了 Go-Spring 与标准库 `net/http` 的**完美兼容性**，你可以直接使用标准库写法，框架只负责生命周期管理：

```go
package main

import (
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func main() {
	// 完全使用 Go 标准库 http.Handler 定义路由
	// Go-Spring 不会强制你替换成框架自定义的路由写法
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world!"))
	})

	// 仅用一行代码启动应用，框架自动接管：
	// - 信号处理（Ctrl+C 优雅退出）
	// - 生命周期管理
	// - 自动等待所有服务退出
	gs.Run()
}
```

访问方式：

```bash
curl http://127.0.0.1:9090/echo
# 输出: hello world!
```

这个最小示例已经体现了 Go-Spring 的设计哲学：

- ✅ **无侵入兼容**：直接使用 Go 标准库 `http`，无需改写任何代码  
- ✅ **零配置启动**：没有繁杂的配置文件，一行 `gs.Run()` 即可运行  
- ✅ **生命周期增强**：框架自动处理信号捕获、优雅退出，省去手写模板代码  
- ✅ **渐进式融入**：你可以只使用生命周期管理，也可以逐步引入 DI 和配置能力  

### 示例二：核心特性展示（依赖注入 + 动态配置）

这个示例展示了 Go-Spring 多项核心特性协同工作，代码中展示了**依赖注入**、**配置绑定**、**动态配置热更新**、**启动时自定义配置**等能力：

```go
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-spring/spring-core/gs"
)

const timeLayout = "2006-01-02 15:04:05.999 -0700 MST"

// 在 init() 阶段注册 Bean，基于 Go 原生机制，无需运行时扫描
func init() {
	gs.Provide(&Service{})

	// 参数 s *Service 会被框架自动注入
	gs.Provide(func(s *Service) *gs.HttpServeMux {
		http.HandleFunc("/echo", s.Echo)
		http.HandleFunc("/refresh", s.Refresh)
		return &gs.HttpServeMux{Handler: http.DefaultServeMux}
	})
}

type Service struct {
	// 按类型自动注入配置刷新器
	AppConfig   *gs.PropertiesRefresher `autowire:""`
	// 通过 value 标签将配置值绑定到字段
	StartTime   time.Time               `value:"${start-time}"`
	// 使用 gs.Dync[T] 泛型支持热更新，配置变更后自动同步
	RefreshTime gs.Dync[time.Time]      `value:"${refresh-time}"`
}

func (s *Service) Echo(w http.ResponseWriter, r *http.Request) {
	str := fmt.Sprintf("start-time: %s refresh-time: %s",
		s.StartTime.Format(timeLayout),
		s.RefreshTime.Value().Format(timeLayout))
	w.Write([]byte(str))
}

func (s *Service) Refresh(w http.ResponseWriter, r *http.Request) {
	// 模拟环境变量变更，触发配置刷新。根据配置优先级规则，环境变量优先级高于内存配置
	os.Setenv("GS_REFRESH-TIME", time.Now().Format(timeLayout))
	// 调用刷新接口，所有使用 Dync 包装的字段都会自动更新
	s.AppConfig.RefreshProperties()
	w.Write([]byte("OK!"))
}

func main() {
	// 在应用启动阶段通过代码设置配置，这属于内存配置层级
	gs.Configure(func(app gs.App) {
		app.Property("start-time", time.Now().Format(timeLayout))
		app.Property("refresh-time", time.Now().Format(timeLayout))
	}).Run()
}
```

访问方式：

```bash
curl http://127.0.0.1:9090/echo     # 查看当前时间（启动时间和刷新时间）
curl http://127.0.0.1:9090/refresh  # 触发热刷新，刷新时间会更新
```

这个示例涵盖了 Go-Spring 众多核心特性：

- ✅ **Bean 注册与依赖注入**：通过 `gs.Provide()` 注册 Bean，框架自动完成依赖注入  
- ✅ **启动自定义配置**：支持在应用启动阶段通过代码动态设置配置，灵活适应不同场景  
- ✅ **配置自动绑定**：`value` 标签直接将配置绑定到结构体字段，无需手动解析  
- ✅ **分层配置体系**：遵循优先级规则，环境变量 > 内存配置 > 默认值，天然支持多环境  
- ✅ **动态配置热更新**：通过 `gs.Dync[T]` 泛型原生支持，配置变更实时生效，无需重启应用  
- ✅ **配置刷新机制**：提供 `PropertiesRefresher` 支持手动触发配置重新加载，可配合配置中心使用  

## 4. 🧩 Bean 管理

在 Go-Spring 中，**Bean 是应用的核心构建单元**，概念类似于其他依赖注入框架中的"组件"。
整个系统围绕 Bean 的注册、初始化、依赖注入与生命周期管理进行组织。

Go-Spring 的设计哲学是 **"编译期准备就绪，运行极简"**：
- 不依赖运行时扫描，所有 Bean 都在 `init()` 阶段完成注册元信息收集
- 仅在**初始化阶段**使用反射完成依赖注入，初始化后**全程零反射运行**
- 类型安全由 Go 编译器保证，运行性能媲美手写代码

这种设计从根源上避免了传统 IoC 框架因运行时反射带来的性能开销和调试复杂性，
特别适合构建 **高性能、可维护性强的大型系统**。

框架采用"**显式注册 + 标签声明 + 条件装配**"的组合方式：
- **显式注册**：所有 Bean 必须显式注册，没有隐式扫描，依赖关系一目了然
- **标签声明**：通过标签简洁声明注入规则，无需冗余配置
- **条件装配**：支持根据环境动态决定是否注册，天然适配模块化设计

由于不依赖运行时容器扫描，也没有"魔法配置"，这种做法在保证开发体验的同时，
进一步提升了调试和运维的可控性，真正实现了**零侵入、（运行时）零反射**的目标。

### 1️⃣ 注册方式

Go-Spring 提供了两种注册 Bean 的方式：

- **`gs.Provide(objOrCtor, args...)`** - 在 `init()` 函数中注册**全局 Bean**
- **`app.Provide(objOrCtor, args...)`** - 在**应用配置阶段**注册 Bean

示例：

```go
// 在 init() 中注册全局 Bean
func init() {
	gs.Provide(&Service{})        // 注册结构体实例
	gs.Provide(NewService)        // 使用构造函数注册
	gs.Provide(NewRepo, gs.ValueArg("db")) // 构造函数带参数
}

// 在应用配置中注册 Bean
gs.Configure(func(app gs.App) {
	app.Provide(&MyService{})
	app.Root(&Bootstrap{}) // 标记为根 Bean，触发依赖注入
})
```

> **💡 关于按需实例化与 `Root`**  
> Go-Spring 默认采用**按需实例化**策略——只有被依赖或者标记为 `Root` 的 Bean 才会被实例化。  
> 通过 `app.Root()` 标记的 Bean 会作为应用的入口点，框架会自动完成其依赖注入并实例化。  

### 2️⃣ 注入方式

Go-Spring 提供了多种灵活的依赖注入方式。

#### 1. 结构体字段注入

通过标签将配置项或 Bean 注入结构体字段，**适合绝大多数场景**，是最常用的注入方式。

```go
type App struct {
	Logger    *Logger      `autowire:""`           // 按类型自动注入 Bean
	Filters   []*Filter    `autowire:"access,*?"`  // 注入多个 Bean，允许不存在
	StartTime time.Time    `value:"${start-time}"` // 绑定配置值
}
```

语法说明：

- `value:"${key}"` 或 `value:"${key:=default}"`：绑定配置值到字段，支持默认值
- `autowire:""`：按**类型**自动注入，类型唯一时直接匹配
- `autowire:"?"`：按类型注入，允许不存在，不存在时字段为 nil
- `autowire:"name?"`：按类型和名称匹配，允许不存在，不存在时字段为 nil
- `autowire:"a,*?"`：先匹配名称为 a，再注入剩余同类型 Bean，注入顺序与注册顺序保持一致
- `autowire:"a,b,c"`：按指定名称精确匹配多个 Bean，顺序严格与声明顺序一致
- `autowire:"a,*?,b"`：精确匹配多个指定名称的 Bean，同时保留剩余其他 Bean，整体有序

#### 2. 构造函数注入

通过函数参数完成自动注入，Go-Spring 会自动推断并匹配依赖 Bean。

```go
func NewService(logger *Logger) *Service {
	return &Service{Logger: logger}
}

gs.Provide(NewService)
```

#### 3. 构造函数参数注入

可通过参数包装器**明确指定每个参数的注入行为**，更适用于复杂构造逻辑。

```go
gs.Provide(NewService,
	gs.TagArg("${log.level}"), // 从配置注入值
	gs.ValueArg(8080),         // 直接注入固定值
	gs.BindArg(connectDB),     // 通过函数处理后注入
)
```

可用的参数类型：

| 参数包装器 | 作用 | 使用场景 |
|-----------|------|---------|
| `gs.TagArg("${key}")` | 从配置中提取值并注入 | 需要把配置值直接作为构造参数 |
| `gs.ValueArg(val)` | 注入固定值 | 明确知道参数值，不需要从容器获取 |
| `gs.IndexArg(i, arg)` | 按参数位置指定注入 | 需要跳过某些参数，或对特定参数自定义注入 |
| `gs.BindArg(fn, args...)` | 通过函数处理后注入 | 需要对注入值做转换或自定义处理 |

这种方式虽然略显繁琐，但给予了你**完全控制注入过程**的能力，在复杂场景下非常有用。

### 3️⃣ 生命周期与配置选项

Go-Spring 提供了丰富的 API 用于配置 Bean 的元信息、生命周期钩子和依赖关系。通过链式调用，你可以完整定义一个 Bean 的所有行为。

```go
gs.Provide(NewService).
	Name("myService").                           // 指定 Bean 名称
	Init(func(s *Service) { ... }).              // 初始化函数
	Destroy(func(s *Service) { ... }).           // 销毁函数
	Condition(gs.OnProperty("feature.enabled")). // 条件注册
	DependsOn(gs.BeanIDFor[*Repo]()).            // 声明显式依赖
	Export(gs.As[ServiceInterface]()).           // 作为接口导出
	Export(gs.As[gs.Runner]())                   // 支持多接口导出
```

完整配置选项说明：

| 选项 | 作用 | 说明 |
|------|------|------|
| `Name(string)` | 指定 Bean 名称 | 同类型存在多个 Bean 时用于区分，配合 `autowire:"name"` 使用 |
| `Init(fn)` | 初始化函数 | Bean 依赖注入完成后调用，同时支持 `InitMethod("Init")` 按方法名指定 |
| `Destroy(fn)` | 销毁函数 | 应用关闭时调用，同时支持 `DestroyMethod("Close")` 按方法名指定 |
| `DependsOn(...)` | 声明依赖 | 指定 Bean 依赖的其他 Bean，保证正确的初始化顺序 |
| `Condition(...)` | 条件注册 | 只有满足条件才注册当前 Bean，不满足则跳过注册 |
| `Export(as)` | 接口导出 | 将 Bean 作为特定接口注册到容器，方便按接口注入，支持多次调用导出多个接口 |

### 4️⃣ 配置类与子 Bean

Go-Spring 支持类似 Spring Boot `@Configuration` 的能力——你可以将一个 Bean 标记为**配置类**，
框架会自动扫描配置类的方法，将方法返回值自动注册为子 Bean。这种方式非常适合模块化组织配置。

#### 使用方式

```go
// 定义配置类
type DataSourceConfig struct {}

// 方法返回值会自动注册为 Bean
func (c *DataSourceConfig) PrimaryDB() *sql.DB {
	// 在这里编写数据库连接创建逻辑
	return &sql.DB{ /* ... */ }
}

// 多个方法可以定义多个相关 Bean
func (c *DataSourceConfig) ReplicaDB() *sql.DB {
	return &sql.DB{ /* ... */ }
}

func init() {
	// 通过 .Configuration() 标记为配置类，框架会自动扫描并注册所有子 Bean
	gs.Provide(&DataSourceConfig{}).Configuration()
}
```

#### 包含/排除规则

你可以通过正则表达式精确控制哪些方法需要被扫描注册：

```go
func init() {
	// 只包含匹配 New.* 模式的方法
	gs.Provide(&Config{}).Configuration(gs.Configuration{
		Includes: []string{"New.*"}, // 包含模式
		Excludes: []string{"Test.*"}, // 排除模式
	})
}
```

- 如果不指定 `Includes`，默认扫描所有公共方法
- 正则语法遵循 Go 标准 `regexp` 包规范，请避免使用 `*` 这类不完整的正则表达式
- 被扫描的方法必须**返回 Bean 实例**，支持两种签名：`(T)` 或 `(T, error)`

## 5. ⚙️ 配置管理

Go-Spring 提供了**分层设计、灵活强大**的配置管理体系，支持从多种来源加载配置，原生满足多环境隔离、动态更新等企业级需求。
无论是本地开发、容器化部署，还是云原生架构，Go-Spring 都能提供一致、简洁的配置体验。

框架会在启动时自动合并不同来源的配置项，并按照**优先级规则**自动覆盖，让你无需手动处理配置合并逻辑。

Go-Spring 开箱支持三种主流配置格式：**YAML**（`.yaml`/`.yml`，推荐）、**Properties**（`.properties`）和
**TOML**（`.toml`），框架会自动根据文件扩展名识别格式。

### 1️⃣ 🔖 配置绑定

Go-Spring 最便捷的配置方式是通过 `value` 标签将配置直接绑定到结构体字段，无需手动解析：

```go
type ServerConfig struct {
	Port    int    `value:"${server.port:=8080}"`      // 带默认值
	Host    string `value:"${server.host:=localhost}"` // 带默认值
	Enabled bool   `value:"${server.enabled:=true}"`   // 布尔类型
}
```

**语法说明：**
- `${key}`：绑定配置键 `key` 的值到字段
- `${key:=default}`：如果配置键不存在，使用 `default` 作为默认值
- 支持几乎所有 Go 基础类型：`int`/`int64`/`uint`/`float64`/`bool`/`string` 等，也支持 `time.Duration` 等自定义类型

### 2️⃣ 📌 配置优先级

Go-Spring 采用清晰的**优先级分层**设计，高优先级配置会自动覆盖低优先级同名配置。优先级从高到低排列如下：

| 优先级 | 配置来源           | 说明 | 使用场景 |
|:------:|----------------|------|---------|
| 1 ⬆️ | **命令行参数**      | `-Dkey=value` | 临时覆盖配置，调试快速验证 |
| 2 | **环境变量**       | 系统环境变量 | 容器化部署，十二要素应用 |
| 3 | **profile 配置** | `app-{profile}.ext` | 多环境隔离（开发/测试/生产） |
| 4 | **app 基础配置**   | `app.ext` | 默认基础配置 |
| 5 | **内存配置**       | `app.Property()` 程序化设置 | 单元测试，动态覆盖 |
| 6 ⬇️ | **标签默认值**      | `${key:=default}` | 最后兜底，缺省值 |

> **💡 优先级规则核心**  
> **后加载的配置优先级更高**，越靠近运行时的配置越优先，符合直觉。  

> **💡 配置导入规则**  
> 无论是基础配置还是 profile 配置，都支持通过 `spring.app.imports` 导入外部配置，  
> **后导入的配置优先级高于文件自身原有配置**，按导入顺序依次覆盖。

### 3️⃣ 📝 各配置来源详细说明

#### 1. 命令行参数
使用 `-Dkey=value` 格式注入，优先级最高，适合快速覆盖运行时配置：
```bash
go run main.go -Dserver.port=9090 -Dapp.env=production
```

#### 2. 环境变量
直接读取操作系统环境变量，容器化部署的最佳实践：
```bash
export SERVER_PORT=9090
export APP_ENV=production
export SPRING_PROFILES_ACTIVE=dev
```

> 💡 Go-Spring 会自动将环境变量中的下划线转为点号，例如 `SERVER_PORT` 映射到 `server.port`。

#### 3. profile 配置（多环境隔离）
通过激活不同的 profile 实现环境隔离，文件命名格式为 `app-{profile}.{ext}`：
```bash
# 激活 dev 环境
export SPRING_PROFILES_ACTIVE=dev
```
框架会自动加载 `app-dev.yaml`（或其他格式），优先级高于基础配置。profile 配置中导入的配置也遵循后导入优先规则。

#### 4. 基础配置文件
默认加载 `conf/app` 加扩展名（如 `conf/app.yaml`），适合存放通用基础配置：
```
./conf/app.yaml
./conf/app.properties
```
基础配置中支持通过 `spring.app.imports` 导入外部配置。

#### 配置导入（import）
无论是基础配置还是 profile 配置，都支持通过 `spring.app.imports` 导入外部配置，方便拆分和复用：

```yaml
# app.yaml
spring:
  app:
    imports:
      - "database.yaml"       # 拆分出的数据库配置
      - "redis.yaml"          # 拆分出的 Redis 配置
      - "nacos://server.json" # 从远程配置中心导入（需扩展）
```

导入顺序按声明顺序执行，**后导入的配置会覆盖先导入和原有配置的同名键**。此机制非常适合配置拆分和集成远程配置中心（Nacos、etcd 等）。

#### 5. 应用内存配置
在应用启动阶段通过代码程序化设置配置，常用于测试或动态场景：
```go
gs.Configure(func(app gs.App) {
    app.Property("app.name", "test-app")
    app.Property("feature.enabled", true)
})
```

#### 6. 结构体标签默认值
通过标签内嵌默认值，作为配置体系的最后兜底：
```go
type Config struct {
	Port int    `value:"${server.port:=8080}"`
	Env  string `value:"${app.env:=development}"`
}
```

## 6. 🔍 条件注入

Go-Spring 借鉴 Spring 的 `@Conditional` 思想，实现了灵活强大的条件注入系统。通过配置、环境、上下文等条件动态决定 Bean
是否注册，实现"按需装配"。这在多环境部署、插件化架构、功能开关、灰度发布等场景中尤为关键。

### 1️⃣ 🎯 常用条件类型

- **`gs.OnProperty("key")`**：当指定配置 key 存在时激活
- **`gs.OnBean[Type]("name")`**：当指定类型/名称的 Bean 存在时激活
- **`gs.OnMissingBean[Type]("name")`**：当指定类型/名称的 Bean 不存在时激活
- **`gs.OnSingleBean[Type]("name")`**：当指定类型/名称的 Bean 是唯一实例时激活
- **`gs.OnFunc(func(ctx gs.ConditionContext) (bool, error))`**：使用自定义条件逻辑判断是否激活

示例：

```go
gs.Provide(NewService).
	Condition(gs.OnProperty("service.enabled"))
```

只有当配置文件中存在 `service.enabled=true` 时，`NewService` 才会注册。

### 2️⃣ 🔁 支持组合条件

Go-Spring 支持组合多个条件，构建更复杂的判断逻辑：

- **`gs.Not(...)`** - 对条件取反
- **`gs.And(...)`** - 所有条件都满足时成立
- **`gs.Or(...)`** - 任一条件满足即成立
- **`gs.None(...)`** - 所有条件都不满足时成立

示例：

```go
gs.Provide(NewService).
  Condition(
      gs.And(
          gs.OnProperty("feature.enabled"),
          gs.Not(gs.OnBean[*DeprecatedService]()),
      ),
  )
```

该 Bean 会在 `feature.enabled` 开启且未注册 `*DeprecatedService` 时启用。

## 7. 📦 Module 与 Starter 机制

Go-Spring 借鉴了 Spring Boot 的 Starter 理念，提供了 **Module** 机制来实现自动配置和模块化装配。
通过 Module，你可以将相关的 Bean 组织在一起，实现"开箱即用"的功能模块。

### 1️⃣ 🎯 什么是 Module？

Module 是 Go-Spring 的**条件化配置模块**机制，它可以根据配置属性动态决定是否注册一组相关的 Bean。
这非常适合用于：

- 🧩 **开发各种功能的 Starter**（如 Redis、MySQL、gRPC 等）
- 🏗️ **按功能模块组织代码**，实现松耦合架构
- ⚡ **根据配置自动启用/禁用功能**，真正按需装配

Module 的核心接口非常简洁：

```go
gs.Module(condition gs.PropertyCondition, fn func(r gs.BeanProvider, p flatten.Storage) error)
```

- `condition`：属性条件，只有满足条件时才会执行模块内的 Bean 注册（通常使用 `gs.OnProperty("key")` 创建）
- `fn`：模块初始化函数，在函数内批量注册该模块的所有 Bean
- `r`：Bean 注册器，用法和全局的 `gs.Provide()` 完全一致
- `p`：配置存储，可从中读取配置进行动态绑定

### 2️⃣ 💡 典型场景：自定义 Starter

假设你要开发一个 Redis Starter，可以这样组织代码：

```go
package redis

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/flatten"
)

// Cache 缓存接口
type Cache interface {
	Get(key string) (string, error)
	Set(key string, value string) error
}

func init() {
	// 当检测到 redis.host 配置存在时，自动启用 Redis 模块
	// 如果条件不满足，模块内的 Bean 不会被注册，不影响应用启动
	gs.Module(gs.OnProperty("redis.host"),
		func(r gs.BeanProvider, p flatten.Storage) error {
			// 1. 注册 Redis Client，指定名称、初始化方法和销毁方法
			r.Provide(NewRedisClient).
				Name("redisClient").
				InitMethod("Connect").   // 依赖注入完成后调用 Connect 建立连接
				DestroyMethod("Close")   // 应用关闭时调用 Close 释放连接资源

			// 2. 注册基于 Redis 的 Cache 实现，并导出为 Cache 接口
			// 这样其他组件可以按接口注入，与具体实现解耦
			r.Provide(NewRedisCache).
				Export(gs.As[Cache]())

			// 还可以继续注册其他相关的 Bean...
			return nil
		})
}
```

用户使用时非常简单，只需在配置文件中添加：

```yaml
redis:
  host: localhost
  port: 6379
  password: xxx
  db: 0
```

Go-Spring 会自动检测配置，满足条件时自动执行模块注册所有相关 Bean。用户**无需编写任何代码手动注册**，真正做到开箱即用！

### 3️⃣ ✨ 特殊用法：Group 批量注册

Go-Spring 还提供了 `gs.Group` 便捷语法，用于处理一类常见场景：**从配置中的 map 批量创建多个 Bean**。
每个 map 条目会自动转换为一个命名 Bean，map key 作为 Bean 名称。使用示例：

```go
// 从配置批量创建多个 HTTP 客户端
gs.Group(
	"${http.clients}",           // 配置中 map 类型配置的路径
	func(cfg HTTPClientConfig) (*HTTPClient, error) {
		return NewHTTPClient(cfg) // 为每个配置条目创建一个客户端实例
	},
	func(c *HTTPClient) error {
		return c.Close()          // 可选：销毁函数用于资源清理
	},
)
```

对应的 YAML 配置：

```yaml
http:
  clients:
    serviceA:  # map key "serviceA" 会成为这个 Bean 的名称
      baseURL: "http://a.example.com"
      timeout: 30s
    serviceB:  # map key "serviceB" 会成为这个 Bean 的名称
      baseURL: "http://b.example.com"
      timeout: 60s
```

这种方式非常适合**多数据源**、**多租户**、**动态插件**等需要根据配置批量创建 Bean 的场景。

## 8. 🔁 动态配置

Go-Spring 原生支持**轻量级配置热更新**机制。通过泛型类型 `gs.Dync[T]` 和 `RefreshProperties()`，
应用可以在运行时实时感知配置变更，无需重启应用。 
这一特性在微服务架构的**灰度发布**、**动态调参**、**配置中心集成**等场景中非常实用。

### 1️⃣ 🌡 使用方式

分为两步：**声明动态字段**和**触发刷新**。

#### 1. 使用 `gs.Dync[T]` 声明动态字段

通过泛型类型 `gs.Dync[T]` 包装字段，框架会自动监听配置变化并实时更新：

```go
type Config struct {
	Version gs.Dync[string] `value:"${app.version}"` // 声明为动态配置
}
```

使用时，通过 `.Value()` 方法获取当前最新值：

```go
version := config.Version.Value() // 总是获取最新值
```

框架会在配置变更时**自动更新**内部值，无需你手动处理。

#### 2. 调用 `RefreshProperties()` 触发刷新

当外部配置发生变化后，需要注入 `*gs.PropertiesRefresher` 并调用其方法触发刷新：

```go
func RefreshHandler(w http.ResponseWriter, r *http.Request, refresher *gs.PropertiesRefresher) {
	// 模拟配置变更（实际场景中通常由配置中心推送变更）
	os.Setenv("APP_VERSION", "v2.0.1")
	// 触发刷新，所有 gs.Dync[T] 字段会自动更新
	_ = refresher.RefreshProperties()
	fmt.Fprintln(w, "Version updated!")
}
```

## 9. ⏳ 应用生命周期与服务模型

Go-Spring 将应用运行阶段的组件抽象为两个核心角色：`Runner` 和 `Server`，职责划分清晰：

| 角色 | 执行方式 | 典型场景 |
|:----:|:--------:|---------|
| **Runner** | 一次性执行 | 数据库初始化、缓存预热、数据迁移等启动任务 |
| **Server** | 长期运行 | HTTP 服务、gRPC 服务、WebSocket 服务等 |

所有角色都通过 `.Export(gs.As[Interface]())` 方式注册。

> **设计说明**：早期版本曾包含 `Job` 类型用于后台定时任务，但为了简化模型、降低认知负担，最新版本已将其移除。
> 对于需要持续运行的后台任务，建议直接使用 `Server` 接口实现，在 `Run` 方法中用循环处理即可。

### 1️⃣ 示例：Runner

```go
package main

import (
	"context"
	"fmt"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	// 注册 Bootstrap 并导出为 Runner 接口
	gs.Provide(&Bootstrap{}).Export(gs.As[gs.Runner]())
}

type Bootstrap struct{}

func (b *Bootstrap) Run(ctx context.Context) error {
	fmt.Println("Bootstrap: 完成初始化...")
	return nil // 如果返回错误，应用启动会被终止
}

func main() {
	gs.Run()
}
```

### 2️⃣ 📌 自定义 Server

Go-Spring 提供了通用的 `Server` 接口，让你可以方便地集成各种服务组件。
所有注册的 `Server` 都会自动接入应用生命周期，框架会处理**并发启动**、**优雅关闭**、**信号处理**等通用逻辑。

**Server 接口定义：**

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

- `Run(ctx context.Context, sig ReadySignal)`：启动服务，等待启动信号后正式对外提供服务
- `Stop() error`：优雅关闭服务，释放资源

**ReadySignal 接口：**

```go
type ReadySignal interface {
	TriggerAndWait() <-chan struct{}
}
```

`ReadySignal` 的作用是**等待所有 Server 完成监听绑定后，再统一对外提供服务**，避免启动未完成就接受请求导致报错。

### 3️⃣ 示例：HTTP Server 接入

```go
package main

import (
	"context"
	"net"
	"net/http"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Provide(NewServer).Export(gs.As[gs.Server]())
}

type MyServer struct {
	svr *http.Server
}

// NewServer 创建 HTTP 服务实例
func NewServer() *MyServer {
	return &MyServer{
		svr: &http.Server{Addr: ":8080"},
	}
}

func (s *MyServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	// 先完成端口监听绑定
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return err // 绑定失败直接返回，终止启动
	}
	// 等待所有 Server 完成启动，然后开始接受连接
	<-sig.TriggerAndWait()
	// 正式开始服务
	return s.svr.Serve(ln)
}

func (s *MyServer) Stop() error {
	// 优雅关闭 HTTP 服务
	return s.svr.Shutdown(context.Background())
}
```

### 4️⃣ 示例：gRPC Server 接入

```go
package main

import (
	"context"
	"net"
	"github.com/go-spring/spring-core/gs"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	svr *grpc.Server
}

func (s *GRPCServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	lis, err := net.Listen("tcp", ":9595")
	if err != nil {
		return err
	}
	<-sig.TriggerAndWait() // 等待所有服务启动完成
	return s.svr.Serve(lis)
}

func (s *GRPCServer) Stop() error {
	s.svr.GracefulStop() // 优雅停止
	return nil
}
```

### 5️⃣ 💡 多 Server 并发运行

所有通过 `.Export(gs.As[gs.Server]())` 注册的服务，框架会在 `gs.Run()` 时**并发启动**，并统一处理退出信号：

```go
func init() {
	// HTTP 和 gRPC 服务并发运行
	gs.Provide(&HTTPServer{}).Export(gs.As[gs.Server]())
	gs.Provide(&GRPCServer{}).Export(gs.As[gs.Server]())
}
```

收到退出信号（如 Ctrl+C）后，框架会统一调用所有 Server 的 `Stop()` 方法实现优雅关闭。

## 10. 🧪 单元测试

得益于 Go-Spring 的**非侵入式设计**，你完全可以按照 Go 原生方式编写单元测试，并不强制要求使用框架特殊的测试能力。

对于简单的单元测试，直接手动实例化被测试对象，手动传入依赖即可：

```go
func TestMyService(t *testing.T) {
	// 手动创建依赖（可以用 Mock）
	mockRepo := NewMockRepo()
	// 手动实例化被测试服务
	service := NewMyService(mockRepo)

	// 直接测试，不需要启动容器
	result := service.DoSomething()
	assert.Equal(t, "ok", result)
}
```

### 1️⃣ 什么时候用 `gs.RunTest()`

当你需要编写**集成测试**，需要完整启动容器、自动完成依赖注入时，Go-Spring 提供了 `gs.RunTest()` 与 `go test` 原生集成，非常方便：

```go
package main

import (
	"testing"
	"github.com/go-spring/spring-core/gs"
	"github.com/stretchr/testify/assert"
)

func TestExample(t *testing.T) {
	// gs.RunTest 自动创建容器、完成依赖注入，测试结束自动关闭
	gs.RunTest(t, func(ts *struct {
		DB    *MyDB    `autowire:""`
		Cache *Cache  `autowire:""`
	}) {
		// 所有依赖已经自动注入，可以直接使用
		result := ts.DB.Query("SELECT ...")
		assert.NotNil(t, result)
	})
}
```

### 2️⃣ ✨ 核心特点

- ✅ **完全原生兼容**：与标准 `go test` 无缝集成，无需特殊测试运行器
- ✅ **自动依赖注入**：在测试参数结构体中声明需要的 Bean，框架自动注入
- ✅ **自动资源清理**：测试结束自动调用销毁方法，优雅关闭

## 11. 📚 与其他框架的对比

下表是 Go-Spring 和其他主流 Go 依赖注入框架的功能对比：

| 功能点 | Go-Spring | Wire | fx | dig |
|:-------|:---------:|:----:|:--:|:---:|
| 运行时 IoC 容器 | ✓ | ✗ | ✓ | ✓ |
| 无运行时扫描（基于 init() 预注册） | ✓ | ✓ | ✗ | ✗ |
| 零反射运行（初始化后无反射） | ✓ | ✓ | ✗ | ✗ |
| 编译期类型校验 | 部分 | ✓ | ✗ | ✗ |
| 条件 Bean 支持 | ✓ | ✗ | ✗ | ✗ |
| 模块化自动装配（Starter 机制） | ✓ | ✗ | ✗ | ✗ |
| 动态配置热更新 | ✓ | ✗ | ✗ | ✗ |
| 生命周期管理 | ✓ | ✗ | ✓ | ✗ |
| 配置属性自动绑定 | ✓ | ✗ | ✗ | ✗ |
| 零侵入设计（无需修改原结构体） | ✓ | ✓ | ✗ | ✓ |

## 12. 🤝 与其他 Go 生态的关系

Go-Spring **并不打算替代任何现有的 Go 框架**，而是扮演"粘合剂"的角色，帮你整合整个 Go 生态。

### 1️⃣ 设计理念

Go-Spring 深度尊重 Go 原生生态，框架本身完全兼容标准库和各类第三方框架：

- ✅ **可以配合 Gin/Echo/Chi 等任何 Web 框架使用**，框架不强制替换你的路由写法
- ✅ **可以配合 gRPC/protobuf 生态使用**，自动装配服务
- ✅ **可以配合 sql/database 或 ORM 框架使用**，配置驱动多数据源
- ✅ **完全兼容 Go 标准库 `net/http`、`context` 等**，无任何侵入改造

### 2️⃣ 定位与分工

| 组件 | Go-Spring 做什么 | 你可以选择 |
|:---:|----------|---------|
| **依赖注入** | ✅ 全权负责 | - |
| **配置管理** | ✅ 全权负责（多来源、热更新） | - |
| **Web 路由** | 可选择集成 | Gin、Echo、Chi、标准库 `net/http` |
| **ORM/数据库** | 可选择集成 | GORM、XORM、sqlx、标准库 `database/sql` |
| **日志** | 提供统一接口 | Zap、Logrus、slog 等 |
| **服务发现/注册** | 可通过 Starter 集成 | etcd、Consul、Nacos |

一句话总结：**Go-Spring 帮你管好依赖和配置，其他的交给你熟悉的工具**。

## 13. 📖 进一步学习

想要快速上手？可以看看这些资源：

- 📖 **完整文档**：[go-spring/go-spring](https://github.com/go-spring/go-spring)
- 💡 **示例项目**：[go-spring/examples](https://github.com/go-spring/go-spring/tree/master/docs/4.examples)
- 📦 **生态 Starter**：[Go-Spring 组织](https://github.com/go-spring) 维护了许多开箱即用的模块

## 14. 🏢 谁在使用 Go-Spring？

多家公司的生产环境正在使用 Go-Spring 构建微服务应用：

- ...

> 如果你的公司或项目也在使用 Go-Spring，欢迎提交 PR 将你的项目展示在这里！

## 15. 💬 问题反馈与交流

- 🐛 **Bug 反馈**：[GitHub Issues](https://github.com/go-spring/spring-core/issues)
- 💡 **功能建议**：欢迎提 Issue 参与讨论
- ⭐ **Star 支持**：如果你喜欢这个项目，欢迎给个 star 鼓励我们！

## 16. 🤝 参与贡献

Go-Spring 是一个开源社区驱动的项目，我们欢迎**所有形式的贡献**：

- 修正文档错误
- 修复 Bug
- 提交功能建议
- 贡献新功能
- 分享你的使用经验

请查阅 [CONTRIBUTING.md](CONTRIBUTING.md) 获取参与方式。

### 💬 QQ 交流群

欢迎加入 QQ 群交流讨论：

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="140" alt="qq-group"/>

### 📱 微信公众号

关注微信公众号获取最新动态：

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="140" alt="wechat-public"/>

### 🎉 特别鸣谢

感谢 JetBrains 公司提供的 [IntelliJ IDEA](https://www.jetbrains.com/idea/) 开源许可，为项目开发提供了极大便利。

### 🛡️ License

Apache License 2.0，详见 [LICENSE](LICENSE)。
