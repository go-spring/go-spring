# Go-Spring 实战第 7 课 —— Profile 多环境配置：基础配置与环境差异如何避免复制

上一篇我们讲了配置优先级和合并语义。然后我们知道，命令行参数、环境变量、Profile 配置、基础配置和默认值都可以进入同一个 `path` 空间，然后 Go-Spring 会按照确定的规则得到最终配置。

然而真实项目里还有一个更具体的问题：配置文件本身应该怎样组织。通常来说，开发、测试、生产等不同环境往往只存在少量的差异，比如数据库地址、外部服务超时时间、日志级别和能力开关等，大部分内容都是一样的。因此如果我们将每个环境都复制一整份配置，那么一旦公共字段需要调整，就要在多份文件里同步完成修改，这样就会带来维护成本。

Go-Spring 提供了 Profile 机制来解决这个问题。Go-Spring 会首先加载基础配置，然后按照当前激活的 Profile 加载和叠加差异配置。这样的话，我们就可以将公共配置写在基础文件，然后把环境差异写在 Profile 文件。

## 基础配置和 Profile 配置

Go-Spring 默认从 `./conf` 目录加载应用配置。配置文件分成两类：`app.*` 是基础配置，所有环境都会参与；`app-{profile}.*` 是 Profile 配置，只有对应 Profile 被激活时才会参与。

以 YAML 为例，项目目录通常可以这样组织。

```text
conf/
  app.yaml
  app-dev.yaml
  app-test.yaml
  app-prod.yaml
```

这组文件表达的是同一个配置模型的不同层次，而不是几套互不相干的配置。`app.yaml` 负责放公共字段，`app-prod.yaml` 只放生产环境真正不同的部分。

比如基础配置给出服务端口、默认超时时间和日志级别。

```yaml
server:
  port: 8080
  timeout: 5s
logging:
  level: info
```

生产环境只需要覆盖真正不同的值。

```yaml
server:
  timeout: 3s
logging:
  level: warn
```

激活 `prod` 后，Go-Spring 会先加载 `app.yaml`，再加载 `app-prod.yaml`。最终结果会保留基础配置里的 `server.port`，同时使用生产 Profile 中的 `server.timeout` 和 `logging.level`。

这里沿用的就是第 6 篇讲过的规则：叶子值按来源优先级覆盖，对象和 Map 按子 key 合并，Slice 仍然按整体替换处理。Profile 文件本身不需要写成完整配置，它只需要表达当前环境相对于基础配置的差异。

如果同一个 key 又出现在环境变量或命令行参数中，最终仍然由更高优先级的环境变量或命令行参数覆盖。Profile 配置高于基础配置，但不是最高优先级来源。

## spring.profiles.active

Go-Spring 使用 `spring.profiles.active` 决定本次启动要叠加哪些 Profile。这个选择通常来自命令行参数或环境变量，因为它们最接近一次具体启动。

命令行参数适合临时指定当前启动使用哪个环境。

```bash
./app -Dspring.profiles.active=prod
```

环境变量适合由部署系统注入。按照第 5 篇讲过的环境变量转换规则，`GS_SPRING_PROFILES_ACTIVE` 会进入配置系统，并转换成 `spring.profiles.active`。

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
```

也可以同时激活多个 Profile，多个名字之间用逗号分隔。

```bash
./app -Dspring.profiles.active=prod,metrics
```

这不是一个无序集合。Go-Spring 会按照声明顺序加载 Profile 文件，后加载的 Profile 可以覆盖先加载 Profile 中的同名 key。因此，`prod,metrics` 的含义是：先叠加生产环境差异，再叠加指标能力差异。

Profile 名称最好来自部署语义，而不是来自某段业务逻辑。`prod`、`test`、`metrics` 这样的名字能让人直接看出配置层的含义；如果名字变成某个临时需求或某个分支条件，后续很难判断它到底应该影响哪些配置。

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

## Profile 顺序

当同时激活多个 Profile 时，顺序本身就是优先级的一部分。后面的 Profile 更靠近最终意图，因此同名 key 会覆盖前面的 Profile。

下面这个例子把环境维度和功能维度拆开。基础配置只放所有环境都需要的端口。

```yaml
server:
  port: 8080
```

`app-prod.yaml` 表达生产环境差异。

```yaml
server:
  timeout: 3s
logging:
  level: warn
```

`app-metrics.yaml` 表达指标能力差异。

```yaml
metrics:
  enabled: true
logging:
  level: info
```

如果这样激活 Profile：

```properties
spring.profiles.active=prod,metrics
```

最终配置会同时包含 `server.port`、`server.timeout` 和 `metrics.enabled`。但 `logging.level` 会使用 `metrics` 中的 `info`，因为 `metrics` 在 `prod` 后面加载。

这条规则让多个 Profile 可以组合，但也要求组合关系清楚。`prod,metrics` 应该表达“生产环境，并且打开指标能力”；如果读者必须依赖隐含顺序才能理解某个覆盖结果，Profile 就已经承担了过多职责。

## Profile 维度

Profile 设计的关键不是多建几个环境文件，而是让每个 Profile 表达一个相对独立的维度。

常见的拆法可以这样看。

| Profile | 维度 | 含义 |
|---------|------|------|
| `dev`、`test`、`prod` | 运行环境 | 表达部署环境差异 |
| `metrics`、`trace` | 功能能力 | 表达某类基础设施能力是否启用 |
| `local` | 本地覆盖 | 表达开发机上的临时差异 |

只有维度相对正交，多个 Profile 才适合组合。环境维度负责数据库地址、外部依赖地址、日志级别这类部署差异；能力维度负责指标、追踪、调试开关这类功能差异。同一批 key 尽量留在同一个维度里维护，否则后加载的 Profile 虽然能覆盖前面的值，但配置意图会变得不清楚。

反过来看，如果把每一种组合都建成独立 Profile，比如 `prod-metrics`、`prod-trace`、`test-metrics`，Profile 很快就会退回到复制配置的老问题。只有某个组合确实代表独立部署形态，并且有稳定的运维含义时，单独建 Profile 才更清楚。

## Profile 多环境配置

Profile 多环境配置把“公共配置”和“环境差异”分开放置。基础配置负责表达所有环境共享的部分，Profile 配置负责表达当前环境或能力维度的差异，多个 Profile 再按照声明顺序叠加。

这样组织以后，多环境配置不会变成几份完整文件之间的复制和同步。Profile 仍然沿用 Go-Spring 的同一套 `Properties`、`path`、来源优先级和合并语义，只是在文件组织层面给环境差异提供了更清楚的位置。
