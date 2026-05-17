# Go-Spring 实战第 5 课 —— 配置来源：Reader、Provider、环境变量与命令行参数

前面几篇我们把配置的表达、绑定、复杂类型和校验给串起来了。至此，业务代码可以始终面对一个稳定的配置模型，即字段从某个 path 读取值，缺失时可以使用默认值，绑定后还可以进行校验。

但真实应用里的配置不会只来自一个 `app.properties`。本地开发可能使用 YAML，线上部署可能使用环境变量，临时排查问题可能使用命令行参数，公司内部也可能有统一的配置中心。如果每一种输入方式都直接影响绑定逻辑，那么配置系统很快就会变成一组彼此独立的入口。

那么 Go-Spring 是如何解决这个问题的呢？答案是将格式解析和配置来源读取分开处理。格式解析由 Reader 负责，配置来源读取由 Provider 负责。也就是说，Reader 关心的是“这段内容是什么语法”，Provider 关心的是“这段内容从哪里来”。无论输入来自文件、环境变量、命令行，还是后续接入的远程配置中心，都可以回到同一套 `Properties` 模型里。

## Reader

Reader 用于解析配置文件的内容。Go-Spring 开箱支持几种常见的配置格式。

| 格式 | 后缀 |
|------|------|
| Properties | `.properties` |
| YAML | `.yaml`、`.yml` |
| TOML | `.toml`、`.tml` |
| JSON | `.json` |

当 Go-Spring 加载配置文件时，会根据文件名的后缀选择对应的 Reader。

这一层抽象的价值在于，业务代码不用知道配置原本长什么样。比如下面的两段配置虽然文件格式不同，但进入 Go-Spring 以后都会落到 `server.port` 和 `server.timeout` 这两个 path 上。

```yaml
server:
  port: 8080
  timeout: 5s
```

```properties
server.port=8080
server.timeout=5s
```

也就是说，只要配置的格式能够被 Reader 转换成统一的结构，后面的绑定链路就不需要跟着变化。

### 扩展 Reader

如果配置的变化发生在文件语法上，比如项目里已经有一批 INI、HCL 或其他内部格式的配置文件，那么可以通过扩展 Reader 来支持这些格式。Reader 只需要返回树形 `map[string]any`，Go-Spring 会继续把它转换成统一的 `Properties`。

下面的例子演示了一个 INI Reader。

```go
func parseINI(b []byte) (map[string]any, error) {
	file, err := ini.Load(b)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	for _, section := range file.Sections() {
		values := make(map[string]any)
		for _, key := range section.Keys() {
			values[key.Name()] = key.Value()
		}

		if section.Name() == ini.DefaultSection {
			for k, v := range values {
				result[k] = v
			}
			continue
		}
		result[section.Name()] = values
	}
	return result, nil
}

func init() {
	conf.RegisterReader(parseINI, ".ini")
}
```

注册完成以后，`.ini` 文件就可以和内置格式一样进入配置加载流程。

Reader 的实现最好保持纯粹。它应该只处理语法解析和结构转换，不要在 Reader 里读取环境变量、访问网络或决定某个配置是否可选。否则格式解析和来源读取会重新耦合在一起，后续排查配置问题时也很难判断错误到底来自语法、来源还是运行环境。

## Provider

如果配置内容的格式没有变，但它不在默认的本地配置文件里，而是在环境变量、数据库、对象存储、Kubernetes ConfigMap、etcd、Nacos 或公司内部配置中心里，这时候就需要扩展 Provider，来把外部来源中的配置读进来。

Go-Spring 默认的配置加载主要围绕本地文件展开。其他来源如果要接入启动期配置流程，可以通过扩展 Provider 和 `spring.app.imports` 配置项的方式引入。

| 来源 | 适用场景 |
|------|----------|
| 本地文件系统 | 常规应用配置和本地覆盖 |
| Kubernetes ConfigMap | 容器平台上的配置分发 |
| etcd | 分布式 KV 配置 |
| Nacos | 配置中心 |
| ZooKeeper | 分布式协调系统中的配置数据 |
| 内部配置平台 | 公司自研配置服务或数据库配置 |

下面的例子演示了一个 JSON Provider，从环境变量里读取一段 JSON。

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

这里大家可能会问，为什么上面的 Provider 没有使用 Reader 来解析 JSON？因为 `envjson` 已经表明环境变量的值就是 JSON。Provider 直接使用 `json.Unmarshal` 解析即可，不需要再绕回文件 Reader。

### spring.app.imports

Provider 注册以后，需要通过 `spring.app.imports` 才能使用。`spring.app.imports` 允许在一个配置文件中引用其他配置。

`spring.app.imports` 支持逗号分隔的多个配置来源。每个配置来源由 Provider 名称、来源地址和可选的 `optional:` 标记组成，中间用冒号分隔。

比如下面这个例子就是上面 envjson Provider 的使用示例。它表示从环境变量 `APP_CONFIG` 中读取 JSON 配置。

```properties
spring.app.imports=envjson:APP_CONFIG
```

下面这个例子展示了 `optional:` 的用法。它表示从环境变量 `LOCAL_OVERRIDES` 中读取 JSON 配置，但如果这个环境变量不存在，也不会报错。

```properties
spring.app.imports=optional:envjson:LOCAL_OVERRIDES
```

此时，`APP_CONFIG` 和 `LOCAL_OVERRIDES` 的值应该是一段完整的 JSON 字符串，例如：

```bash
export APP_CONFIG='{"server":{"port":9000},"database":{"host":"localhost"}}'
```

当这段 JSON 进入 Go-Spring 的配置体系后，会被展开成类似下面的配置。

```properties
server.port=9000
database.host=localhost
```

## 环境变量

Go-Spring 支持读取带 `GS_` 前缀的环境变量，并且按照如下规则将其转换成配置 key。

1. 去掉 `GS_` 前缀。
2. 将下划线 `_` 替换为点号 `.`。
3. 转为小写。

下面的两个环境变量展示了这种转换规则。

```bash
export GS_SERVER_PORT=8080
export GS_DATABASE_DEFAULT_HOST=localhost
```

它们在经过转换后会变成下面两个配置 key。

```properties
server.port=8080
database.default.host=localhost
```

这个转换规则让环境变量和配置 path 之间有了稳定的映射。如果配置文件里面写了 `database.default.host`，那么容器环境里面就可以写 `GS_DATABASE_DEFAULT_HOST`，然后最终的业务代码都只绑定到同一个 path。

### 原始绑定

我们也可以直接使用不带 `GS_` 前缀的环境变量。Go-Spring 不会把系统里所有环境变量都当成应用配置加载进来，这样可以避免 `PATH`、`HOME`、`USER` 这类系统变量污染配置空间。

比如运行平台提供了 `PORT` 环境变量，然后我们可以在字段上直接绑定这个名称。

```go
type ServerConfig struct {
	Port int `value:"${PORT}"`
}
```

这种写法适合少量平台约定变量。对于应用自己的配置，更推荐使用 `GS_` 前缀，让配置名称落在 Go-Spring 的 path 命名空间里。

## 命令行参数

命令行参数离本次启动最近，非常适合临时覆盖端口、Profile、开关以及其他排查参数。Go-Spring 默认识别 `-D` 前缀的参数。

```bash
./myapp -Dserver.port=9000 -Denv=prod -Ddebug
```

上面的命令行参数在进入配置系统以后，会变成下面的三个配置项。

```properties
server.port=9000
env=prod
debug=true
```

通常我们使用 `-Dkey=value` 这种形式的参数，表示给 key 设置明确的值。如果只有 key 而没有值，会被解析成 `true`。

如果 `-D` 和现有命令行风格冲突，也可以通过环境变量 `GS_ARGS_PREFIX` 来修改命令行的配置前缀。

下面这个例子将命令行参数的前缀设置成了 `--config.`。

```bash
export GS_ARGS_PREFIX="--config."
./myapp --config.server.port=9000
```

经过转换后，上面的命令行参数会变成下面的配置项。

```properties
server.port=9000
```

命令行参数不适合承载很长、很复杂的配置。它更像一次启动的明确指令。如果配置需要长期维护、多人协作或者表达环境差异，那么放回配置文件、Profile 或配置中心会更清楚。

## 配置来源

Reader 和 Provider 的拆分，让本地文件、环境变量、命令行参数和远程配置中心都能进入同一套 `Properties` 模型。Reader 负责把不同格式收敛成统一结构，Provider 负责把不同来源接入配置体系。这样一来，后续的绑定、校验和使用方式就可以继续围绕同一个 path 空间展开。
