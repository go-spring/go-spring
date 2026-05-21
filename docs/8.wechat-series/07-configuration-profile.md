# Go-Spring 实战第 7 课 —— Profile 多环境配置：基础配置与环境差异如何避免复制

上一篇我们讲了配置优先级和合并语义。然后我们知道，命令行参数、环境变量、Profile 配置、基础配置和默认值都可以进入同一个 `path` 空间，然后 Go-Spring 会按照确定的规则得到最终配置。

然而真实项目里还有一个更具体的问题：配置文件本身应该怎样组织。通常来说，开发、测试、生产等不同环境往往只存在少量的差异，比如数据库地址、外部服务超时时间、日志级别和能力开关等，大部分内容都是一样的。因此如果我们将每个环境都复制一整份配置，那么一旦公共字段需要调整，就要在多份文件里同步完成修改，这样就会带来维护成本。

Go-Spring 提供了 Profile 机制来解决这个问题。Go-Spring 会首先加载基础配置，然后按照当前激活的 Profile 加载和叠加差异配置。这样的话，我们就可以将公共配置写在基础文件，然后把环境差异写在 Profile 文件。

## 基础配置和 Profile 配置

Go-Spring 将配置文件分成两类：一种是基础配置，文件名是`app.*`，然后所有的环境都会加载它；一种是 Profile 配置，文件名是`app-{profile}.*`，然后只有对应 Profile 被激活时才会被加载。

> * 表示任意文件类型，比如 YAML、JSON 等。
> {profile} 表示当前激活的 Profile 名称，比如 `prod`、`dev` 等。

如果我们规定在项目中只能使用 YAML 配置文件，那么项目的配置目录通常可以这样进行组织。

```text
conf/
  app.yaml        # 基础配置
  app-dev.yaml    # 开发环境配置
  app-test.yaml   # 测试环境配置
  app-prod.yaml   # 生产环境配置  
```

当我们了解了 Go-Spring 的 Profile 机制以后，再看到上面的配置目录时，就会知道 `app.yaml` 是基础配置，其他的 `app-{profile}.yaml` 都是 Profile 配置。

比如我们有个项目，`app.yaml` 给出服务端口、默认超时时间和日志级别等公共配置。

```yaml
# 基础配置
server:
  port: 8080
  timeout: 5s
logging:
  level: info
```

`app-prod.yaml` 给出生产环境的超时时间和日志级别等差异配置。

```yaml
# 生产环境配置
server:
  timeout: 3s
logging:
  level: warn
```

当我们激活 `prod` Profile 后，根据 Go-Spring 的合并语义（叶子值按来源优先级覆盖，对象和 Map 按子 key 合并，Slice 按整体替换处理），最终会得到这样的配置。

```yaml
# 生产环境的最终配置
server:
  port: 8080
  timeout: 3s
logging:
  level: warn
```

当然，上面的最终配置只是说明了基础配置和 Profile 配置合并之后的结果，
如果同一个 key 又出现在环境变量或命令行参数中，那么最终仍然由更高优先级的环境变量或命令行参数覆盖。Profile 配置高于基础配置，但不是最高优先级。

## spring.profiles.active

前面我们提到说 Profile 机制需要激活。那么一个 Profile 如何才能被激活呢？

Go-Spring 使用 `spring.profiles.active` 配置 key 来决定本次启动要激活的 Profile。它可以是一个逗号分隔的字符串，每个字符串都是一个 Profile 名称，比如 `prod`、`test`、`metrics` 等。

根据 Go-Spring 加载配置的顺序，我们知道设置 `spring.profiles.active` 的最佳方式是命令行参数或者环境变量。因为它们都是在基础配置之前加载的。

比如，我们可以通过命令行参数来指定 `prod` Profile。

```bash
./app -Dspring.profiles.active=prod
```

也可以通过环境变量来指定 `prod` Profile。

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
./app
```

我们还可以同时激活多个 Profile。

```bash
./app -Dspring.profiles.active=prod,metrics
```

对于多个 Profile，Go-Spring 会按照声明顺序加载 Profile 文件，后加载的 Profile 可以覆盖先加载 Profile 中的同名 key。因此，`prod,metrics` 的含义是：先叠加 `prod` Profile 的差异，再叠加 `metrics` Profile 的差异。

## Profile 维度

特别需要说明的是，Profile 的名称最好来自部署语义，而不是来自某段业务逻辑，并且保持正交。只有维度相对正交，多个 Profile 才适合进行组合。

常见的拆法可以这样。

| Profile | 维度 | 含义 |
|---------|------|------|
| `dev`、`test`、`prod` | 运行环境 | 表达运行环境差异 |
| `metrics`、`trace` | 功能能力 | 表达某类基础设施能力是否启用 todo (这个维度不太好) |

同一批 key 尽量留在同一个维度里维护，否则后加载的 Profile 虽然能覆盖前面的值，但配置意图会变得不清楚。todo (这里需要解释下)

## spring.app.config.dir

默认配置目录是 `./conf`。如果项目需要把配置文件放到其他目录，可以通过 `spring.app.config.dir` 修改。

```bash
export GS_SPRING_APP_CONFIG_DIR=./config
```

也可以通过命令行参数指定。

```bash
./myapp -Dspring.app.config.dir=./config
```

`spring.app.config.dir` 和普通业务配置有一个重要区别：它会影响 Go-Spring 到哪里发现 `app.*` 和 `app-{profile}.*` 文件。因此，这个配置项必须在文件发现之前就能解析出来，通常放在命令行参数、环境变量或应用内置默认配置里。

等基础配置和 Profile 配置已经开始加载以后，再修改 `spring.app.config.dir`，就来不及影响这一轮文件发现了。所以它更像启动入口的定位参数，而不是某个环境文件内部的普通配置项。

## Profile 多环境配置

Profile 多环境配置把“公共配置”和“环境差异”分开放置。基础配置负责表达所有环境共享的部分，Profile 配置负责表达当前环境或能力维度的差异，多个 Profile 再按照声明顺序叠加。

这样组织以后，多环境配置不会变成几份完整文件之间的复制和同步。Profile 仍然沿用 Go-Spring 的同一套 `Properties`、`path`、来源优先级和合并语义，只是在文件组织层面给环境差异提供了更清楚的位置。
