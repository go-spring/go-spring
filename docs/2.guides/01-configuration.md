# 配置管理

Go-Spring 提供了统一且强大的配置管理系统，让你从开发到生产都能从容应对。
如果你用过 Java Spring Boot，会发现这套设计思路非常相似，很多概念都能直接对上，学习成本很低。

## Properties

Go-Spring 配置系统设计得非常简洁：**不管你用什么格式写配置，最终都会转换成扁平化的 key-value 结构**，
我们称之为 `Properties`。这个设计和 Spring Boot 的 `Environment` 抽象是一样的。

这样做最大的好处就是**统一了配置访问接口**：上层的绑定、校验、优先级合并这些逻辑，
完全不需要关心原始配置是什么格式。因为不管你用什么格式写，最终都可以用同一种方式进行访问。

需要注意的是，**key 匹配是大小写敏感的**，框架不提供自动大小写转换，也不支持松散匹配（比如驼峰转下划线、
省略分隔符自动匹配等），key 是什么字符串就是什么字符串，必须完全一致才能匹配。

### Path 语法

Go-Spring 使用业界标准的 path 语法来定位配置项，语法非常直观：

- 使用点号 `.` 分隔嵌套层级：比如 `a.b.c` 表示 `a` → `b` → `c`
- 使用方括号 `[index]` 表示数组索引：比如 `a.b[0].c` 表示 `a.b` 数组第一个元素的 `c` 属性

举个例子，这是一个典型的 YAML 配置：

```yaml
app:
  port: 8080
  database:
    - host: localhost
      port: 5432
    - host: repli.ca
      port: 5433
```

将它展开成扁平化 properties 之后就是这样：

```
app.port = 8080
app.database[0].host = localhost
app.database[0].port = 5432
app.database[1].host = repli.ca
app.database[1].port = 5433
```

是不是一目了然？每一个配置项都有唯一的 path，非常清晰，不会有歧义。

## 配置绑定

配置加载完了，下一步就是把配置值**绑定**到 Go 代码的变量里，这样你才能在代码里使用它。

Go-Spring 提供了两种绑定方式，方便应用于不同的场景：

- **结构体标签绑定**（推荐）：适用于绝大多数日常开发，使用声明式写法最简洁
- **手动 Bind 函数绑定**：通常在模块（module）中创建多个 bean 时使用

### 结构体标签绑定

这种方式最简单，你只需要定义一个结构体，然后在字段上加上 `value` 标签即可。
这就相当于 Spring Boot 里的 `@Value` 或者 `@ConfigurationProperties`。

```go
type ServerConfig struct {
	Port        int           `value:"${port:=8080}"`
	Timeout     time.Duration `value:"${timeout:=30s}"`
	EnableSSL   bool          `value:"${enable-ssl:=true}"`
	Endpoints   []string      `value:"${endpoints}"`
}

// 对于 App.Config 字段：
// - 使用 `${server}` 作为前缀进行配置绑定
// - 即 ServerConfig 中的每个字段，都会从以 `server.` 开头的配置项中读取
//   例如：server.port、server.timeout 等
type App struct {
	Config ServerConfig `value:"${server}"`
}
```

我们将标签的语法 `value:"${key:=defaultValue}"` 拆开来看：

- `key` 就是前面说的配置项 path，对应配置文件里的配置
- `:=defaultValue` 是**可选**的，如果配置里找不到这个 key，就用你给的默认值
- 如果省略了默认值，配置里又确实不存在这个 key，那就是**必填字段**，绑定会直接报错

> 💡 **提示:** 如果你写 `${:=default}` 这种空 key，那就直接用这个默认值，不会去配置里找。
> 这在你想硬编码一个值但又保留配置可能性的时候很有用。

### 支持的数据类型

Go-Spring 开箱即用地支持非常丰富的数据类型，从基础类型到嵌套结构体，再到自定义类型，都能处理。

#### 基础类型

Go 基础类型不用任何额外配置，直接就能绑定：

- **布尔类型** (`bool`)：支持 `true`/`false`、`1`/`0`、`t`/`f` 等多种写法
- **整数类型** (`int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`)：支持十进制和十六进制写法
- **浮点类型** (`float32`, `float64`)：支持科学计数法
- **字符串** (`string`)：原封不动保存

对于没有转换器的结构体，Go-Spring 会递归绑定每个字段，并且支持任意深度的嵌套。
这样你就可以把相关配置组织成清晰的结构体层级。

#### 内置特殊转换器

除了基础类型，Go-Spring 还内置了几个常用类型的转换器，拿来即用：

| 类型 | 说明 | 示例 |
|------|------|------|
| `time.Duration` | 时间时长，自动解析字符串格式 | `30s`, `5m`, `1h30m` 都支持 |
| `time.Time` | 时间点，支持常见日期格式 | 支持 `2006-01-02` `2006-01-02 15:04:05` |

比如你想配置一个超时时间，可以直接这样写：

```go
type Config struct {
	Timeout time.Duration `value:"${timeout:=30s}"`
}
```

配置文件里写 `timeout=5m`，绑定完直接就是 `5 * time.Minute`，非常方便，完全不用你自己手动解析。

#### 自定义类型转换器

如果你有自己的自定义类型，也可以注册类型转换器告诉框架怎么把字符串转换成你的类型。用法非常简单。

**什么时候你会需要自定义转换器？**
- 你定义了枚举类型，需要从友好的字符串名称解析
- 需要特殊的格式转换逻辑，比如从字符串解析加密数据
- 第三方库的类型需要自定义解析规则，等等

**完整示例**：

```go
import (
	"strconv"
	"github.com/go-spring/spring-core/conf"
)

// 自定义状态枚举类型
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// String 实现 Stringer 接口，方便打印日志
func (s Status) String() string {
	switch s {
	case StatusDisabled:
		return "disabled"
	case StatusEnabled:
		return "enabled"
	default:
		return strconv.Itoa(int(s))
	}
}

// 👉 在 init() 函数中注册转换器（必须在程序启动前完成注册）
func init() {
	conf.RegisterConverter(func(s string) (Status, error) {
		switch s {
		case "disabled", "off":
			return StatusDisabled, nil
		case "enabled", "on":
			return StatusEnabled, nil
		default:
			// 同时支持数字形式直接输入
			v, err := strconv.Atoi(s)
			if err != nil {
				return 0, err
			}
			return Status(v), nil
		}
	})
}
```

注册完就能直接在结构体字段里用了：

```go
type AppConfig struct {
	Status Status `value:"${app.status:=enabled}"`
}
```

然后配置文件里可以这么写：

```yaml
app:
  status: paused
```

> 💡 **记住**：转换器是全局注册的，必须在 `init()` 里注册，注册一次，整个应用都能用。

#### 切片（数组）绑定

切片类型支持两种输入方式，方便灵活适配不同场景：

**方式一：多行展开格式（推荐用于复杂元素）**

这在 YAML/TOML 里很常见，每个元素单独一行：

```yaml
apps:
  - a
  - b
  - c
```

展开后就是：

```properties
apps[0]=a
apps[1]=b
apps[2]=c
```

这种方式适合元素比较复杂的情况，比如每个元素都是一个对象。

**方式二：逗号分隔字符串（适合简单列表）**

如果就是简单的字符串列表，一行写完更简洁：

```properties
apps=a,b,c
```

两种写法最终绑定出来都是 `[]string{"a", "b", "c"}`，效果一样。默认用英文逗号 `,` 分隔。

#### Map 绑定

Map 类型绑定也很方便，所有以该 path 为前缀的子节点都会自动绑定进去：

```properties
database.connections.master.host=localhost
database.connections.master.port=5432
database.connections.slave.host=replica
database.connections.slave.port=5433
```

如果你绑定到 `map[string]DatabaseConfig`，那么 `connections["master"]` 和
`connections["slave"]` 就会分别包含对应的配置，完全不用你自己遍历。框架都帮你处理好了。

### 手动 Bind 函数绑定

通常来说我们只会在模块（module）中创建多个 bean 时使用手动绑定，示例如下。

```go
package main

import (
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/flatten"
)

func init() {
	// 注册一个模块
	gs.Module(nil, func(r gs.BeanProvider, p flatten.Storage) error {
		var config ServerConfig
		// 将 `${server}` 前缀下的配置绑定到 ServerConfig 结构体
		if err := conf.Bind(p, &config, "${server}"); err != nil {
			return err
		}
		// 使用 config 注册相关 Bean
		return nil
	})
}
```

`Bind` 的函数签名：

```go
func Bind(storage flatten.Storage, target any, tag ...string) error
```

参数说明：
- `storage` - 配置存储对象，包含已加载的所有配置
- `target` - 绑定目标，**必须传指针**，不然改不了目标变量的值
- `tag` - 可选，要绑定的配置项 path，支持完整的标签语法，不填表示绑定整个配置

## 配置校验

配置绑定成功了，不代表配置一定正确。比如端口号你填了 `99999`，超过了 1-65535 的范围，
虽然能绑定成功，但运行的时候肯定出问题。

Go-Spring 支持对配置值做校验，这样**在应用启动阶段就能发现错误**，从而避免带着错误配置上线，
将问题消灭在萌芽状态。

### 表达式校验

Go-Spring 基于优秀的 [`expr-lang/expr`](https://github.com/expr-lang/expr) 库，提供了非常灵活的表达式校验。

用法很简单：你只需要在结构体标签上加个 `expr:"..."`，表达式里用 `$` 表示当前字段的值就行了。

比如一些最常用的表达式，开箱即用：

| 表达式 | 含义 |
|--------|------|
| `$ > 0` | 当前值必须大于 0 |
| `$ < 65536` | 当前值必须小于 65536 |
| `$ in ['debug', 'info', 'warn', 'error']` | 必须是这几个枚举值之一 |
| `$ matches '^[a-z][a-z0-9_]{3,31}$'` | 字符串必须匹配正则表达式 |
| `$ contains 'prefix-'` | 字符串必须包含这个子串 |
| `$ > 0 && $ < 65536` | 多个条件，"并且"关系 |
| `$ < 10 || $ > 100` | 多个条件，"或者"关系 |

来看几个日常开发中常用的例子：

```go
type ServerConfig struct {
	// 端口号必须在 1 - 65535 之间才合法
	Port int `value:"${server.port:=8080}" expr:"$ > 0 && $ < 65536"`

	// 日志级别必须是这四个值之一
	LogLevel string `value:"${log.level:=info}" expr:"$ in ['debug', 'info', 'warn', 'error']"`

	// 用户名必须符合命名规则
	Username string `value:"${auth.username}" expr:"$ matches '^[a-z][a-z0-9_]{3,31}$'"`

	// 超时至少要 1 秒以上
	Timeout time.Duration `value:"${timeout:=5s}" expr:"$ >= duration(\"1s\")"`

	// 重试次数不能太多也不能太少
	RetryCount int `value:"${retry:=3}" expr:"$ >= 0 && $ <= 10"`
}
```

看到没？一行表达式就搞定了，不用你写一堆 `if-else` 判断代码，干净利落。
而且把配置和校验放在一起，也方便维护。

expr 库支持的语法非常丰富，这里只列了最常用的几种。
如果你需要更复杂的校验，可以直接看 [expr-lang/expr](https://github.com/expr-lang/expr) 官方文档。

### 必填校验的误区

这里有一个常见问题：**我什么时候需要自己写必填校验？**

其实你不用操心：如果你的字段没有默认值，配置里又确实不存在这个 key 的话，
那么**绑定过程就已经失败了**，不需要你额外写表达式。

只有两种情况你需要自己写校验表达式：

1. **你给了默认值，但要求默认值也必须满足某个条件**（比如 `port` 默认 8080，但必须大于 0）
2. **字段存在，但要求它必须满足某种业务规则**（比如 `retry` 必须在 0-10 之间）

所以记住这个原则：**框架已经帮你做了存在性检查，你只需要额外校验业务规则**。

### 自定义校验函数

如果内置的表达式操作满足不了你的需求，你还可以注册全局自定义校验函数，然后在表达式里直接用。
函数接受任意类型的参数，返回 `bool` 表示是否通过校验。

**完整示例**：

```go
import "github.com/go-spring/spring-core/conf"

// 在 init() 中注册自定义函数
func init() {
	// 注册一个判断质数的函数，要求端口号必须是质数
	conf.RegisterValidateFunc[int]("isPrime", func(n int) bool {
		for i := 2; i*i <= n; i++ {
			if n%i == 0 {
				return false
			}
		}
		return n > 1
	})

	// 再注册一个检查端口范围的函数
	conf.RegisterValidateFunc[int]("validPort", func(port int) bool {
		return port > 0 && port < 65536
	})

	// 注册一个检查字符串最小长度的函数
	conf.RegisterValidateFunc[string]("minLength", func(s string) bool {
		return len(s) >= 3
	})
}
```

注册完之后，就可以直接在标签里用了：

```go
type ServerConfig struct {
	// 端口号必须是质数，同时还要满足端口范围
	Port      int    `value:"${port}" expr:"isPrime($) && validPort($)"`
	// 用户名长度至少 3 个字符
	Username  string `value:"${auth.username}" expr:"minLength($)"`
	// API Key 必须满足多个条件
	APIKey    string `value:"${security.api-key}" expr:"minLength($) && $ contains 'prod-'"`
}
```

自定义校验函数可以和表达式原生操作混合使用，轻松构建复杂的校验规则。
你的函数只需要负责返回 `true` 或 `false`，框架会自动处理错误提示。

## 配置加载：来源与格式

现在我们理解了配置模型、绑定、类型和校验，接下来看看配置从哪里来。
Go-Spring 支持多种配置来源和格式，覆盖绝大多数使用场景。

### 支持的配置格式

Go-Spring 开箱支持四种最常见的配置格式：

| 格式 | 文件后缀 | 适用场景 |
|:-----|:---------|:---------|
| Properties | `.properties` | 简单键值对，Java 生态传统格式 |
| YAML | `.yaml`/`.yml` | 可读性好，支持注释，目前最流行 |
| TOML | `.toml`/`.tml` | 语义化明确，适合复杂配置 |
| JSON | `.json` | 机器友好，程序生成配置常用 |

框架会根据文件后缀自动选择对应的解析器，你完全不用关心解析细节。
如果你有特殊格式需求，也可以注册自定义解析器。

### 自定义配置格式解析器

如果你需要支持一种特殊的配置文件格式，只需要实现 `reader.Reader` 函数类型，
然后调用 `conf.RegisterReader` 注册就行了。

**完整示例 - 自定义 INI 格式解析器**：

```go
import (
	"github.com/go-spring/spring-core/conf"
)

// 实现 INI 格式解析
func parseINI(b []byte) (map[string]any, error) {
	// 调用你喜欢的 INI 解析库
	parsed, err := ini.Load(b)
	if err != nil {
		return nil, err
	}

	// 转换为 map[string]any 树形结构返回
	result := make(map[string]any)
	...
	return result, nil
}

// 在 init 中注册，绑定 .ini 扩展名
func init() {
	conf.RegisterReader(parseINI, ".ini")
}
```

这样注册之后，你的应用就能直接加载 `.ini` 格式的配置文件了，和内置格式用起来完全一样。

### 支持的配置来源

除了本地文件，Go-Spring 还支持从各种远程配置中心加载配置：

| 来源 | 说明 |
|:-----|:-----|
| 本地文件系统 | 最常用，从本地磁盘加载配置文件 |
| Kubernetes ConfigMap（暂未支持） | 运行在 K8s 上时直接从 ConfigMap 加载 |
| etcd（暂未支持） | 从 etcd 集群加载配置 |
| Nacos（暂未支持） | 从阿里 Nacos 配置中心加载 |
| ZooKeeper（暂未支持） | 从 ZooKeeper 加载 |

这个列表还在不断增加，当然你也可以实现自己的配置提供者（Provider），接入自定义的配置中心。

### 自定义配置提供者

配置提供者负责从特定来源（本地文件、远程服务、数据库等）加载配置数据。
如果你需要从一个特殊的地方加载配置（比如公司内部的配置中心、etcd、数据库等），
只需要实现 `provider.Provider` 函数类型，然后调用 `conf.RegisterProvider` 注册就行了。

**完整示例 - 从环境变量读取 JSON 配置**：

```go
import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/stdlib/flatten"
)

// 定义一个从环境变量读取 JSON 的 Provider
func envJSONProvider(optional bool, source string) (map[string]string, error) {
	// source 参数就是环境变量名称
	envVal := os.Getenv(source)
	if envVal == "" {
		if optional {
			// 可选配置不存在，返回 nil 不报错
			return nil, nil
		}
		return nil, fmt.Errorf("environment variable %s not found", source)
	}

	// 解析 JSON 到树形结构
	var tree map[string]any
	if err := json.Unmarshal([]byte(envVal), &tree); err != nil {
		return nil, err
	}

	// 展开为扁平化 map 返回，key 是 path，value 是字符串
	return flatten.Flatten(tree), nil
}

// 在 init 中注册 Provider
func init() {
	conf.RegisterProvider("envjson", envJSONProvider)
}
```

注册之后，你就可以在配置导入中使用这个自定义 Provider 了：

```properties
# 在 spring.app.imports 中使用自定义 provider
# 格式: <provider>:<source>
spring.app.imports=envjson:APP_CONFIG

# 也可以标记为可选，如果不存在也不报错
# 格式: optional:<provider>:<source>
spring.app.imports=optional:envjson:LOCAL_OVERRIDES
```

这里 `APP_CONFIG` 就是环境变量的名称，使用前先设置好它：

```bash
# 先把 JSON 配置导出到环境变量
export APP_CONFIG='{"server":{"port":9000},"database":{"host":"localhost"}}'
```

启动应用后，框架就会从这个环境变量读取并解析 JSON 配置了。

### 环境变量

Go-Spring 会自动从环境变量中读取配置，这在**容器部署**时特别有用。转换规则是这样的：

1. 以 `GS_` 前缀开头的环境变量会被当作配置处理（这样避免把系统所有环境变量都加载进来，干净）
2. 然后是去掉 `GS_` 前缀 → 把下划线 `_` 替换成点 `.` → 最后转为小写

举个例子一看就懂：

```bash
# 你设置的环境变量
export GS_SERVER_PORT=8080
export GS_DATABASE_DEFAULT_HOST=localhost

# 框架自动转换成
server.port=8080
database.default.host=localhost
```

完美匹配我们前面说的 path 语法。

除此之外，你也可以**直接绑定任意环境变量**，不需要遵循 `GS_` 前缀规则：
只需要在配置中直接使用环境变量的名称即可（环境变量通常都是大写字母命名，
所以这里的 key 也需要和环境变量名称完全一致）。

示例：

```go
type ServerConfig struct {
	Port int `value:"${PORT}"`
}
```

这时候框架会直接读取系统环境变量 `PORT` 的值来绑定。

### 命令行参数

启动应用时，你也可以通过命令行参数**临时覆盖配置**，这在开发调试的时候非常方便。规则是这样的：

- 以 `-D` 为前缀开头的参数会被当作配置项处理
- 支持 `-Dkey=value` 和 `-D key=value` 两种写法
- 如果只写 `-Dkey` 没有值，默认值就是 `true`

来看一个完整例子：

```bash
./myapp -Dserver.port=9000 -Denv=prod -Ddebug
```

解析出来就是：

```
server.port=9000
env=prod
debug=true
```

如果你不喜欢 `-D` 前缀，还可以通过环境变量 `GS_ARGS_PREFIX` 修改：

```bash
export GS_ARGS_PREFIX="--config."
./myapp --config.server.port=9000
```

## 层次配置与优先级

Go-Spring 支持从**多个来源**加载配置（基础配置文件、Profile 配置文件、环境变量、命令行等等），
这些配置可能会重叠，需要确定哪个最终生效。

框架会按照优先级顺序合并配置，完整的优先级顺序从高到低是这样的：

| 优先级 | 来源               |
|:----:|------------------|
| 1 (最高) | **命令行参数**        |
| 2 | **环境变量**         |
| 3 | **Profile 特定配置** |
| 4 | **基础配置文件**       |
| 5 | **应用内置默认配置**   |
| 6 (最低) | **结构体标签默认值**   |

这个优先级顺序和 Spring Boot 保持一致，符合大家的使用习惯：
**命令行临时覆盖优先级最高，然后是环境变量，然后是配置文件，最后才是默认值**。

大家只需要记住一个核心原则：**越具体、越靠近运行环境的配置，优先级越高**。

### 合并语义：三种类型不同规则

当多个配置来源合并时，Go-Spring 对不同类型的配置采用不同的合并语义。
理解这一点是设计配置的核心，懂了你就不会对合并结果感到困惑：

| 配置类型 | 合并语义 | 说明 |
|:---------|:---------|:-----|
| **叶子值** | **覆盖语义** | 高优先级配置的同 key 叶子值直接覆盖低优先级的值，找到就停止搜索 |
| **Map 对象** | **合并语义** | 所有层的 key 都会合并在一起，不同层可以互补 |
| **Slice 数组** | **覆盖语义** | 高优先级一旦定义了这个数组，低优先级的整个数组都被忽略，完全替换 |

我们来看三个例子，看完你就明白了。

**例子一：叶子值覆盖**

这是最常见的场景。假设基础配置里 `app.port = 8080`，你在开发环境想改成 9000，只需要写：

```properties
# 低优先级（基础配置）
app.port = 8080

# 高优先级（环境覆盖）
app.port = 9000
```

合并结果就是 `app.port = 9000`。高优先级直接覆盖，完全符合直觉。

**例子二：Map 合并**

Map 对象采用**合并语义**，不同层的 key 会合并在一起：

```properties
# 低优先级配置（基础配置）
server.port=8080

# 高优先级配置（环境覆盖）
server.host=localhost
```

合并后的 Map 包含两个 key：`port` 和 `host`。`port` 的值来自低优先级，`host` 来自高优先级。
结果就是：

```properties
server.port=8080  (保留低优先级)
server.host=localhost (来自高优先级)
```

如果 key 重复了，值仍然遵循覆盖语义，高优先级覆盖低优先级。

这种合并方式非常方便：你只需要在高优先级配置里写需要修改的 key，不需要把整个 Map 重写一遍。

**例子三：Slice 数组覆盖**

Slice 数组和 Map 不同，采用**整体覆盖**语义：
一旦高优先级层定义了这个数组，低优先级的整个数组都被忽略。

```
# source1 (低优先级):
my.list[0]=a
my.list[1]=b

# source2 (高优先级):
my.list[0]=c
```

最终结果是 `[c]`，而不是 `[c, b]`。因为高优先级已经定义了 `my.list`，低优先级的整个数组被忽略了。

这是一个设计决策：对于数组，通常你要么**完全重新定义**，要么就不定义。
部分修改数组在实践中不太常见，而且容易引起混淆。
所以 Go-Spring 选择了简单清晰的语义：**整个替换**。

## Profile 多环境配置

开发、测试、生产环境的配置往往不一样：开发环境用本地数据库，生产环境用生产数据库，端口号可能也不一样……

Go-Spring 通过 **Profile 机制**来支持多环境配置，这和 Spring 的概念完全一样。
用了 Profile，你可以把不同环境的配置分开存放，按需加载。

### 激活 Profile

你可以通过两种方式激活 Profile：

```bash
# 命令行参数
./app -Dspring.profiles.active=prod

# 环境变量
export GS_SPRING_PROFILES_ACTIVE=prod
```

支持同时激活多个 Profile，用逗号分隔：

```bash
-Dspring.profiles.active=prod,metrics
```

### 配置文件的命名约定

Go-Spring 遵循和 Spring Boot 一样的命名约定：

- `app.yaml` - **基础配置**，所有环境都生效
- `app-{profile}.yaml` - **特定 Profile 配置**，优先级高于基础配置

所以你的项目结构通常是这样：

```
conf/
  app.yaml          # 通用基础配置，所有环境共享
  app-dev.yaml      # 开发环境特殊配置
  app-test.yaml     # 测试环境特殊配置
  app-prod.yaml     # 生产环境特殊配置
```

在激活 `prod` Profile 的时候，加载顺序是：

1. 先加载 `app.yaml` 基础配置
2. 再加载 `app-prod.yaml` Profile 配置
3. Profile 配置覆盖基础配置中相同的 key

这样就实现了 **"基础配置共用，环境配置只放差异"** 的最佳实践，避免了重复代码。

### 自定义配置目录

默认情况下，Go-Spring 会从 `./conf` 目录加载配置文件。
如果你想使用其他目录，可以通过 `spring.app.config.dir` 来修改：

```bash
# 通过环境变量指定配置目录
export GS_SPRING_APP_CONFIG_DIR=./config

# 或者通过命令行参数指定
./myapp -Dspring.app.config.dir=./config
```

然后 `spring.app.config.dir` 本身会按照正常的优先级规则解析，
因此你可以通过环境变量、命令行参数等各种方式覆盖它。

### 多个 Profile 的优先级

当同时激活多个 Profile 时，**后面出现的优先级比前面的高**。比如：

```
spring.profiles.active=dev,metrics
```

`metrics` 的优先级比 `dev` 高，如果有相同的配置项，`metrics` 的值会覆盖 `dev` 的值。
这符合直觉，越靠后越优先。

### 设计建议：保持 Profile 正交性

在设计 Profile 时，建议保持各个 Profile 之间的**正交性**：

- 每个 Profile 应该只负责**一个维度**的配置变化
- 避免多个 Profile 之间出现相互依赖或配置重叠
- 不同维度的配置应当可以**独立组合使用**

例如：
- `dev`/`test`/`prod` 是**环境维度**，表示不同运行环境
- `metrics`/`trace` 是**功能维度**，表示是否开启特定功能

这两个维度正交，你可以自由组合 `dev,metrics` 或 `prod,metrics`，而不需要为每种组合单独准备配置文件。

## 配置导入

有时候你想把一个大配置文件**拆成几个小文件**，方便维护；或者你想从远程配置中心加载一些配置。
这时候就可以用配置导入功能：

```properties
# 在主配置中导入其他配置文件，逗号分隔多个
spring.app.imports=./dev.properties,http://config-server/app.properties

# optional: 前缀表示这个配置文件是可选的，如果不存在也不会报错
spring.app.imports=optional:./local.overrides
```

无论是基础配置还是 Profile 配置，都可以使用导入功能。

- 如果在 `app.yaml`（基础配置）中导入了其他配置，那么被导入配置的优先级会比 `app.yaml` 本身高
- 如果在 `app-prod.yaml`（Profile 配置）中导入了其他配置，那么被导入配置的优先级也会比 `app-prod.yaml` 本身高

这里遵循的核心原则是：**后加载的优先级更高**。
配置系统按照发现顺序加载，后发现、后加载的配置，优先级就更高。

这样使用导入功能，你可以把公共配置抽出来放到单独文件，然后在不同环境重复使用，很方便。

## 变量引用

配置支持完整的变量引用语法，你可以在任意配置值中引用其他配置项的值。这在很多场景下非常有用，比如：
- 抽取公共前缀，多处复用
- 组合多个配置项拼接成新的值
- 引用环境变量
- 提供灵活的默认值，等等

### 常见用法举例

```properties
# 1. 直接引用其他配置项
server.port=${port}

# 2. 带默认值，如果配置中找不到 port 就用 8080
server.port=${port:=8080}

# 3. 组合多个配置项拼接成新的值，支持和普通文本混合
app.home=${user.home}/myapp
app.url=http://${app.host}:${app.port}/api

# 4. 直接引用环境变量
redis.password=${REDIS_PASSWORD:=}
```

### 嵌套引用

框架支持**嵌套引用**，即可以在一个引用中使用另一个引用，并且支持任意深度：

```properties
env=prod
config.file=config/${env}.properties
```

框架会自动递归解析所有依赖，并且保证正确展开。

总而言之，变量引用让配置更加灵活，你可以通过组合和复用写出非常简洁的配置。

## 动态配置

很多时候你可能需要**在不重启应用的情况下刷新配置**，Go-Spring 原生支持动态配置，
而且语法和静态配置完全一致，学习成本为零。

只需要把你的字段类型声明为 `gs.Dync[T]` 泛型就可以了：

```go
import "github.com/go-spring/spring-core/gs"

type AppConfig struct {
	// 静态配置，启动后就不变了
	Port int `value:"${server.port}"`

	// 动态配置，运行期可以自动刷新
	Timeout       gs.Dync[time.Duration] `value:"${server.timeout:=30s}"`
	MaxConns      gs.Dync[int]           `value:"${server.max-conns:=100}"`
	EnableFeature gs.Dync[bool]          `value:"${feature.xxx.enable:=false}"`
}
```

使用的时候调用 `Value()` 方法就能拿到当前最新的值：

```go
func (a *App) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 每次读取都是最新的值
	timeout := a.Config.Timeout.Value()
	// ...
}
```

`Dync[T]` 是并发安全的，多个 goroutine 同时读取完全没问题。

动态刷新会保证**原子提交**：要么所有配置都更新成功，要么一个都不更新，不会出现部分更新的中间状态。
为了保证这个特性，框架会在刷新之前**预校验所有配置**，如果校验失败，整个刷新会被取消，
不会影响当前正在使用的配置。

需要特别说明的是，对于连接池这类需要动态刷新的资源，我们不需要特别的机制就能满足它们的要求。
因为资源一般都是有有效期的，不会瞬间就切换到新的资源上，
而是需要你在业务层面控制平滑过渡（比如逐步回收旧连接）。
因此框架只需要提供**动态刷新值**的核心语义即可，当创建新连接的时候自然会应用最新的配置。

另外，我们推荐在业务层面使用**版本号机制**来避免不必要的刷新。
这样只有当版本发生变化时才会触发真正的资源重载。

具体如何实现动态刷新操作可以参考后文。