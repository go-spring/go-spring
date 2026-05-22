# Go-Spring 实战第 7 课 —— Profile 多环境配置：基础配置与环境差异如何避免复制

上一篇我们讲了配置优先级和合并语义。然后我们知道，命令行参数、环境变量、Profile 配置、基础配置和默认值都可以进入同一个 `path` 空间，然后 Go-Spring 会按照确定的规则得到最终配置。

然而真实项目里还有一个更具体的问题：配置文件本身应该怎样组织。通常来说，开发、测试、生产等不同环境往往只存在少量的差异，比如数据库地址、外部服务超时时间、日志级别和能力开关等，大部分内容都是一样的。如果我们将每个环境都复制一整份配置，那么一旦公共字段需要调整，就要在多份文件里同步完成修改，这样就会带来维护成本。

Go-Spring 提供了 Profile 机制来解决这个问题。Go-Spring 会首先加载基础配置，然后按照当前激活的 Profile 加载和叠加差异配置。这样的话，我们就可以将公共配置写在基础文件，然后把环境差异写在 Profile 文件。

## 基础配置和 Profile 配置

Go-Spring 将配置文件分成两类：一种是基础配置，文件名是`app.*`，然后所有的环境都会加载它；一种是 Profile 配置，文件名是`app-{profile}.*`，然后只有对应 Profile 被激活时才会被加载。

> `*` 表示任意文件类型，比如 YAML、JSON 等。
> `{profile}` 表示当前激活的 Profile 名称，比如 `prod`、`dev` 等。

如果我们规定在项目中只能使用 YAML 配置文件，那么项目的配置目录通常可以这样进行组织:

```text
conf/
  app.yaml        # 基础配置
  app-dev.yaml    # 开发环境配置
  app-test.yaml   # 测试环境配置
  app-prod.yaml   # 生产环境配置
```

当我们了解了 Go-Spring 的 Profile 机制以后，再看到上面的配置目录时，就会知道 `app.yaml` 是基础配置，其他的 `app-{profile}.yaml` 都是 Profile 配置。

假设我们有个项目，`app.yaml` 给出服务端口、默认超时时间和日志级别等公共配置：

```yaml
# 基础配置
server:
  port: 8080
  timeout: 5s
logging:
  level: info
```

`app-prod.yaml` 给出生产环境的超时时间和日志级别等差异配置：

```yaml
# 生产环境配置
server:
  timeout: 3s
logging:
  level: warn
```

当我们激活 `prod` Profile 后，根据 Go-Spring 的合并语义——叶子值按来源优先级覆盖，对象和 Map 
按子 key 合并，Slice 按整体替换处理，最终会得到这样的配置：

```yaml
# 生产环境的最终配置
server:
  port: 8080
  timeout: 3s
logging:
  level: warn
```

当然，上面的结果只展示了基础配置和 Profile 配置合并后的状态。如果同一个 key 同时出现在环境变量或命令行参数中，最终仍会被优先级更高的环境变量或命令行参数覆盖。也就是说，Profile 配置高于基础配置，但不是最高优先级。

## spring.profiles.active

在 Go-Spring 中， Profile 需要被显式激活。Go-Spring 使用 `spring.profiles.active` 配置 key 决定本次启动要激活哪些 Profile。

`spring.profiles.active` 可以是一个逗号分隔的字符串，每一项都是一个 Profile 名称，例如 `prod`、`test`、`metrics` 等。根据 Go-Spring 的配置加载顺序，我们推荐通过命令行参数或环境变量来设置它，因为这两类来源都会在基础配置之前加载。

比如，我们可以通过命令行参数激活 `prod` Profile：

```bash
./app -Dspring.profiles.active=prod
```

也可以通过环境变量激活 `prod` Profile：

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
./app
```

还可以同时激活多个 Profile：

```bash
./app -Dspring.profiles.active=prod,metrics
```

对于多个 Profile，Go-Spring 会按照声明顺序进行加载，后加载的 Profile 可以覆盖先加载 Profile 中的同名 key。因此，`prod,metrics` 表示先叠加 `prod` Profile 的差异，再叠加 `metrics` Profile 的差异。

## Profile 维度

**Profile 的设计很重要**。

通常，Profile 的名称最好来自部署语义或基础设施能力，而不是某段具体业务逻辑；而且不同的 Profile 之间应尽量保持维度正交。只有维度相对正交，多个 Profile 的组合才最容易理解和维护。

常见的拆法可以如下：

| Profile | 维度 | 含义 |
|---------|------|------|
| `dev`、`test`、`prod` | 运行环境 | 表达开发、测试、生产等运行环境差异 |
| `metrics`、`trace` | 基础设施能力 | 表达监控指标、链路追踪等能力是否启用 |

同一批 key 应尽量留在同一个维度中维护。例如，数据库地址、日志级别这类与运行环境强相关的配置，适合放在 `dev`、`test`、`prod` 这样的环境 Profile 中；监控和追踪这类能力开关，则更适合放在 `metrics`、`trace` 这样的能力 Profile 中。

如果同一批 key 分散在多个维度里，后加载的 Profile 依然可以覆盖前面的值，但配置意图会变得不清楚。久而久之，读配置的人很难判断某个值到底是环境差异、能力差异，还是一次临时覆盖。

## spring.app.config.dir

Go-Spring 默认的配置目录是 `./conf`。如果项目需要将配置文件放到其他目录，可以通过 `spring.app.config.dir` 配置 key 来修改。这里同样推荐使用环境变量或命令行参数来指定。

比如，我们可以通过环境变量来指定配置目录：

```bash
export GS_SPRING_APP_CONFIG_DIR=./config
./app
```

也可以通过命令行参数来指定配置目录：

```bash
./myapp -Dspring.app.config.dir=./config
```

`spring.app.config.dir` 和 `spring.profiles.active` 有些不同。通常来说，它的值通常在所有环境中应当保持一致，只是不同项目可能采用不同的配置目录。因此，我们也可以在代码中为项目设置它的值。

我们可以通过下面的代码来指定配置目录：

```go
gs.Configure(func(app gs.App) {
  app.Property("spring.app.config.dir", "./config")
})
```

## Profile 多环境配置

Profile 多环境配置的意义是把“公共配置”和“环境差异”分开放置。基础配置负责表达所有环境共享的部分，Profile 配置负责表达当前环境或能力维度的差异，然后多个 Profile 再按照声明顺序依次叠加。

这样组织之后，配置结构更清晰，修改公共字段时也更不容易遗漏。
