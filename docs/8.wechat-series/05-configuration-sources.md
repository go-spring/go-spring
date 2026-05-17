# Go-Spring 实战第 5 课 —— 配置来源扩展：Reader 与 Provider

前面几章我们看过了 Go-Spring 怎样表达、绑定、校验配置。现在咱们来看看配置是如何加载进系统的。

在真实应用里配置不会只来自一个本地文件。它还可能来自 YAML、TOML、JSON、环境变量、命令行参数、远程配置中心。Go-Spring 为了统一处理这些来源，把格式解析和来源读取进行了分离处理。

Go-Spring 把格式解析交给了 Reader，把来源读取交给了 Provider。如果输入内容的格式或者语法变了，就扩展 Reader。如果数据来自新的地方，就扩展 Provider。

## Reader 只负责把文件格式解析成同一棵配置树

Go-Spring 开箱支持常见配置格式。

| 格式 | 文件后缀 | 适用场景 |
|------|----------|----------|
| Properties | `.properties` | 简单键值对 |
| YAML | `.yaml`、`.yml` | 可读性好，适合人工维护 |
| TOML | `.toml`、`.tml` | 语义明确，适合复杂配置 |
| JSON | `.json` | 机器友好，适合程序生成 |

Go-Spring 会根据文件后缀自动选择解析器。无论原始格式是什么，最终都会转成统一的 `Properties`。这样后续绑定和校验就不用关心文件格式了。格式可以不同，但进入系统后的表达必须保持一致。

## 自定义 Reader 只扩展入口解析层

如果项目需要支持特殊格式，可以实现 `reader.Reader` 函数类型，并通过 `conf.RegisterReader` 注册。

下面的例子把 INI 内容转换成树形 `map`。关键点在返回值，Reader 只负责完成格式解析，后续扁平化和绑定仍然交给 Go-Spring 配置链路。

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

注册后，应用就可以像加载内置格式一样加载 `.ini` 配置文件。因为 Reader 的边界只在入口解析，所以新增格式只影响这一层，后面的配置模型保持不变。

## 来源和格式拆开，扩展才不互相牵连

配置来源表示配置数据从哪里读取。当前最常用的是本地文件系统，也可以通过 Provider 接入远程配置中心、数据库或公司内部配置服务。

可以通过 Provider 接入的典型来源包括下面几类。

| 来源 | 说明 |
|------|------|
| 本地文件系统 | 从磁盘加载配置文件 |
| Kubernetes ConfigMap | 适合容器平台配置管理 |
| etcd | 分布式 KV 配置 |
| Nacos | 配置中心 |
| ZooKeeper | 分布式协调系统 |

这张表不是内置能力清单，而是在说明 Provider 这一层覆盖的方向。项目如果有自己的基础设施，只要实现 Provider 并最终交回统一的配置数据，后面的绑定、校验和合并流程就不用变化。

## Provider 只负责把外部来源交回配置数据

Provider 负责从特定来源加载配置。下面示例从环境变量读取 JSON 配置。

这段代码演示的是来源扩展，而不是新格式扩展。JSON 解析只是为了把环境变量里的文本还原成树形数据，Provider 的职责是从 `source` 指向的位置取回配置并交给后续流程。

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

注册后可以通过 `spring.app.imports` 使用。

```properties
spring.app.imports=envjson:APP_CONFIG
spring.app.imports=optional:envjson:LOCAL_OVERRIDES
```

`optional:` 表示配置不存在时不报错。这样本地覆盖文件、开发者私有配置或非必需的外部配置就可以按需提供，而不会影响正常启动。

## GS_ 环境变量会先转换成配置 key

Go-Spring 会自动读取带 `GS_` 前缀的环境变量，并按规则转换成配置 key。

1. 去掉 `GS_` 前缀。
2. 将下划线 `_` 替换为点号 `.`。
3. 转为小写。

下面两个环境变量展示了转换规则，即 `GS_` 之后的部分会先转成小写，再把下划线变成点号。

```bash
export GS_SERVER_PORT=8080
export GS_DATABASE_DEFAULT_HOST=localhost
```

进入 Go-Spring 配置系统后，它们会变成下面两个配置 key。

```properties
server.port=8080
database.default.host=localhost
```

如果部署平台已经约定了现成变量名，也可以不走 `GS_` 转换，直接按原始环境变量名绑定。

```go
type ServerConfig struct {
	Port int `value:"${PORT}"`
}
```

这时候 Go-Spring 会读取系统环境变量 `PORT`。如果部署平台已经约定了环境变量名称，这种写法会更直接。

## 命令行参数适合表达本次启动的临时覆盖

命令行参数通常离本次启动最近，所以它很适合做临时覆盖。下面这个启动命令同时覆盖端口、环境和布尔开关，适合发布或排查时临时调整。

```bash
./myapp -Dserver.port=9000 -Denv=prod -Ddebug
```

进入配置系统后，它会被解析成三个明确的 key。

```properties
server.port=9000
env=prod
debug=true
```

如果需要修改参数前缀，可以设置 `GS_ARGS_PREFIX`。

```bash
export GS_ARGS_PREFIX="--config."
./myapp --config.server.port=9000
```

## 多入口统一后还必须确定覆盖顺序

Reader 负责格式，Provider 负责来源。这个拆分让本地文件、环境变量、命令行参数和远程配置中心都能进入同一套 `Properties` 模型。

多个入口统一以后，还会遇到下一个问题，即同一个 key 来自不同来源时谁覆盖谁，以及 Map 和 Slice 这类结构怎样合并。
