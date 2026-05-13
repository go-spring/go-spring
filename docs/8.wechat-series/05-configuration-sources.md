# 配置来源与格式扩展

配置绑定和校验解决的是“怎么用”，但配置还得先进入系统。

真实应用里的配置不会只来自一个本地文件。它可能来自 YAML、TOML、JSON、环境变量、命令行参数，也可能来自远程配置中心。Go-Spring 把这件事拆成两个扩展点：

- Reader：负责解析配置格式。
- Provider：负责从某个来源读取配置。

格式和来源分离后，本地文件、环境变量、远程配置中心都可以接入同一套配置模型，后续绑定、校验和合并也就有了统一入口。

## 支持的配置格式

Go-Spring 开箱支持常见配置格式：

| 格式 | 文件后缀 | 适用场景 |
|------|----------|----------|
| Properties | `.properties` | 简单键值对 |
| YAML | `.yaml`、`.yml` | 可读性好，适合人工维护 |
| TOML | `.toml`、`.tml` | 语义明确，适合复杂配置 |
| JSON | `.json` | 机器友好，适合程序生成 |

框架会根据文件后缀自动选择解析器。无论原始格式是什么，最终都会转成统一的 `Properties`。

## 自定义格式解析器

如果需要支持特殊格式，可以实现 `reader.Reader` 函数类型，并通过 `conf.RegisterReader` 注册。

```go
func parseINI(b []byte) (map[string]any, error) {
	parsed, err := ini.Load(b)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	// 将 INI 内容转换成树形 map
	return result, nil
}

func init() {
	conf.RegisterReader(parseINI, ".ini")
}
```

注册后，应用就可以像加载内置格式一样加载 `.ini` 配置文件。

## 支持的配置来源

配置来源表示配置数据从哪里读取。当前最常用的是本地文件系统，也可以通过 Provider 接入远程配置中心、数据库或公司内部配置服务。

规划中的典型来源包括：

| 来源 | 说明 |
|------|------|
| 本地文件系统 | 从磁盘加载配置文件 |
| Kubernetes ConfigMap | 适合容器平台配置管理 |
| etcd | 分布式 KV 配置 |
| Nacos | 配置中心 |
| ZooKeeper | 分布式协调系统 |

不是所有来源都需要框架内置。项目可以按自己的基础设施实现 Provider。

## 自定义配置提供者

Provider 负责从特定来源加载配置。下面示例从环境变量读取 JSON 配置：

```go
func envJSONProvider(optional bool, source string) (map[string]string, error) {
	envVal := os.Getenv(source)
	if envVal == "" {
		if optional {
			return nil, nil
		}
		return nil, fmt.Errorf("environment variable %s not found", source)
	}

	var tree map[string]any
	if err := json.Unmarshal([]byte(envVal), &tree); err != nil {
		return nil, err
	}

	return flatten.Flatten(tree), nil
}

func init() {
	conf.RegisterProvider("envjson", envJSONProvider)
}
```

注册后可以通过 `spring.app.imports` 使用：

```properties
spring.app.imports=envjson:APP_CONFIG
spring.app.imports=optional:envjson:LOCAL_OVERRIDES
```

`optional:` 表示配置不存在时不报错，适合本地覆盖文件或可选配置中心。

## 环境变量

Go-Spring 会自动读取带 `GS_` 前缀的环境变量，并按规则转换成配置 key：

1. 去掉 `GS_` 前缀。
2. 将下划线 `_` 替换为点号 `.`。
3. 转为小写。

例如：

```bash
export GS_SERVER_PORT=8080
export GS_DATABASE_DEFAULT_HOST=localhost
```

对应配置：

```properties
server.port=8080
database.default.host=localhost
```

也可以直接绑定任意环境变量：

```go
type ServerConfig struct {
	Port int `value:"${PORT}"`
}
```

这时会读取系统环境变量 `PORT`。

## 命令行参数

命令行参数适合临时覆盖配置：

```bash
./myapp -Dserver.port=9000 -Denv=prod -Ddebug
```

解析结果是：

```properties
server.port=9000
env=prod
debug=true
```

如果需要修改参数前缀，可以设置 `GS_ARGS_PREFIX`：

```bash
export GS_ARGS_PREFIX="--config."
./myapp --config.server.port=9000
```

## 输入通道统一以后

Reader 负责格式，Provider 负责来源。这个拆分让本地文件、环境变量、命令行参数和远程配置中心都能进入同一套 `Properties` 模型。

一旦配置可以从多处进入系统，就必须回答另一个问题：谁覆盖谁，Map 和 Slice 又该怎样合并。这个问题留给配置优先级和合并语义。
