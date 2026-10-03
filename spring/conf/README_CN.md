# conf

[English](README.md) | [中文](README_CN.md)

`conf` 为 Go 应用提供灵活的配置绑定系统:从多种文件格式与配置源声明式地绑定到
Go 结构体,自带类型转换、占位符解析和基于表达式的校验。

## 核心概念

`conf` 的工作流程:

1. 通过 provider 从配置源加载数据
2. 将嵌套数据展平为键值存储
3. 按 tag 注解把属性绑定到结构体字段
4. 递归解析变量占位符
5. 用表达式校验取值

## Tag 语法

结构体字段用 `value` tag 指定配置键:

```go
value:"${key:=default}"
```

其中:

- `${key}` - 引用配置键
- `:=default` - 键不存在时的默认值(可选)

特性:

- 嵌套键:`${db.host}`、`${service.endpoint}`
- 默认值:`${DB_HOST:=localhost}`
- 链式默认值:`${A:=${B:=default}}` - A 缺失则尝试 B,再退到默认值
- 嵌套引用:`${prefix${suffix}}` - 变量嵌套展开
- 根绑定:`${ROOT}` - 从根层级绑定(用于顶层结构体)

示例:

```go
type ServerConfig struct {
    Host string `value:"${host:=localhost}"`
    Port int    `value:"${port:=8080}"`
}
```

## 支持的类型

开箱即支持:

1. **基础类型**:string、所有整数/浮点类型、bool(自动字符串转换)
2. **时间类型**:`time.Time`、`time.Duration`(内置转换器);`conf.ByteSize`
   表示人类可读的字节数(`"1.5GB"`),见下文
3. **结构体**:嵌套字段递归绑定
4. **切片**:两种输入格式:
   - 索引属性:`endpoints[0]=a`、`endpoints[1]=b`
   - 逗号分隔:`endpoints=a,b,c`
5. **Map**:点号子键绑定:`users.alice=Alice`、`users.bob=Bob`
6. **自定义类型**:用 `RegisterConverter` 注册自定义转换器

### ByteSize

`conf.ByteSize` 是从 `"1KB"`、`"1.5MB"`、`"1024"` 这类可读字符串解析出的字节
数,即 `time.Duration` 在字节域的对应物,可直接从属性绑定:

```go
var cfg struct {
    MaxMemory conf.ByteSize `value:"${max-memory:=1MB}"`
}
// cfg.MaxMemory.Bytes() == 1 << 20
```

后缀是二进制(1024 进位),与 JVM(`-Xmx`)、nginx、Kafka 的惯例一致。可接受
的后缀(大小写不敏感,数字与后缀间允许空白):`B`;`K`/`KB`/`KiB`;
`M`/`MB`/`MiB`;`G`/`GB`/`GiB`;`T`/`TB`/`TiB`。空后缀表示字节。允许
`"1.5GB"` 这类小数值,四舍五入到最近的字节。刻意不支持十进制(1000 进位)
语义,以避免配置文件里 KB 与 KiB 的歧义。

## 值解析

所有属性值在绑定前都经过变量解析:字符串里的 `${key}` 会被替换为配置存储中
解析后的值。解析是递归的,支持嵌套占位符。

```properties
host=localhost
port=8080
url=http://${host}:${port}/api
```

解析后 `url` 为 `http://localhost:8080/api`。

## 属性级解密

携带解密标记的属性值会在绑定前被解密,应用代码只看到明文:

```properties
db.password=ENC(aes:<base64 密文>)
# 或 Spring Cloud Config 风格:
db.password={cipher}aes:<base64 密文>
```

标记内必须指名驱动,因此多种解密方案可在同一份配置中共存。内置 `aes` 驱动
(AES-GCM)的密钥在带外提供,绝不进配置文件:

| 变量                            | 说明                                        |
|---------------------------------|---------------------------------------------|
| `GS_CONFIG_DECRYPT_AES_KEY`     | base64 编码的 AES 密钥(16/24/32 字节)     |
| `GS_CONFIG_DECRYPT_AES_KEY_FILE`| 保存 base64 密钥的文件路径                   |

要接入非对称方案或云 KMS,在 `init` 中注册驱动并在标记内指名。带标记却无法
解密的值会让启动失败,而非降级为损坏的默认值。详见 `decrypt` 与
`decrypt/aes` 包。

## 校验

用 `expr` tag 给任意字段加校验。表达式可访问:

- `$` - 当前字段的值
- 所有已注册的自定义校验函数

示例:

```go
type Config struct {
    Port  int    `value:"${port}" expr:"$ > 0 && $ < 65536"`
    Email string `value:"${email}" expr:"contains($, '@')"`
    Data  string `value:"${data}" expr:"len($) > 3"`
}
```

注册自定义校验函数:

```go
// 注册一个校验时间在未来 的函数
conf.RegisterValidateFunc("future", func(t time.Time) (bool, error) {
    return t.After(time.Now()), nil
})

// 在校验中使用:
type Event struct {
    StartTime time.Time `value:"${start-time}" expr:"future($)"`
}
```

更多表达式语法见 [expr-lang/expr](https://github.com/expr-lang/expr)。

### 跨字段校验

单字段的 expr tag 无法表达字段间的关系。跨字段约束请在配置结构体上实现
`Validator` 接口。`Validate` 会在所有字段绑定完成后自动调用,切片与 map 里
的嵌套结构体同样适用。

```go
type ServerConfig struct {
    Host string `value:"${host}"`
    Port int    `value:"${port:=0}"`
}

func (c *ServerConfig) Validate() error {
    if c.Port > 0 && c.Host == "" {
        return fmt.Errorf("host required when port is set")
    }
    if c.Port < 0 || c.Port > 65535 {
        return fmt.Errorf("port %d out of range [0, 65535]", c.Port)
    }
    return nil
}
```

校验是层级化的:每个嵌套结构体可携带自己的跨字段规则,自底向上独立校验。
与 expr tag 自然组合 - 同一结构体可同时使用单字段与跨字段校验。

与构造函数校验的对比:

|                        | conf.Validator | 构造函数体       |
|------------------------|----------------|------------------|
| conf.Bind 路径         | ✓(自动)       | ✗(不会被调用)  |
| gs.TagArg 路径         | ✓(自动)       | ✓(手动)         |
| 外部依赖               | ✗(无注入)     | ✓(可注入 bean)  |
| 各绑定入口行为一致     | ✓              | 取决于调用方     |

约束是类型内在属性时用 `Validator`;校验需要访问其他 bean 或外部状态时用构
造函数校验。

## 加载配置

`conf.Load` 接受如下格式的 source URI:

```
[optional:]<provider>:<location>
```

示例:

```go
// 从 YAML 文件加载(按扩展名自动识别格式)
props, err := conf.Load("config.yaml")

// 显式 file provider
props, err = conf.Load("file:config.yaml")

// 可选文件 - 文件不存在不报错
props, err = conf.Load("optional:file:config.yaml")
```

用 `RegisterProvider` 注册自定义 provider(etcd、Consul、环境变量等)以支持
更多配置源。provider 是带生命周期的对象:除 `Load` 外还实现 `Close`,运行时在应用退出时
调用一次,用来停止 `Load` 期间装上的 watcher 和 listener。

### 支持的文件格式

内置 reader 覆盖以下扩展名:

- JSON(`.json`)
- Java Properties(`.properties`、`.props`)
- YAML(`.yaml`、`.yml`)
- TOML(`.toml`、`.tml`)

用 `RegisterReader` 注册自定义 reader 以支持更多格式。

## 快速开始

```go
package main

import (
    "fmt"
    "log"

    "go-spring.org/spring/conf"
    "go-spring.org/stdlib/flatten"
)

type Config struct {
    Host string `value:"${host:=localhost}"`
    Port int    `value:"${port:=8080}"`
}

func main() {
    // 从文件加载配置
    props, err := conf.Load("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // 绑定到结构体(默认 ${ROOT} - 从根层级绑定所有键)
    var cfg Config
    if err := conf.Bind(flatten.NewPropertiesStorage(props), &cfg); err != nil {
        log.Fatal(err)
    }

    // 使用 cfg.Host 和 cfg.Port
    fmt.Printf("Server starting on %s:%d\n", cfg.Host, cfg.Port)
}
```

`Bind` 也可以只绑定某个键前缀之下:

```go
var cfg AppConfig
conf.Bind(flatten.NewPropertiesStorage(props), &cfg, "${app}") // 所有字段在 app.* 之下查找键
```

## 扩展点

所有扩展必须在 `init()` 期注册:

- `RegisterProvider` - 新增配置源 provider
- `RegisterReader` - 新增文件格式支持
- `RegisterConverter` - 新增自定义类型的转换
- `RegisterValidateFunc` - 新增自定义校验函数
- `RegisterDecryptor` - 新增属性级解密方案
