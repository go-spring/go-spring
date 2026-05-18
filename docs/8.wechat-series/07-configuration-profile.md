# Go-Spring 实战第 7 课 —— Profile 多环境配置：基础配置与环境差异如何避免复制

上一篇我们讲了配置优先级和合并语义。命令行参数、环境变量、Profile 配置、基础配置和默认值进入同一个 `path` 空间以后，Go-Spring 会按照确定的规则得到最终配置。

但真实项目里还有一个更具体的问题：这些配置文件应该怎样组织。开发、测试、生产环境通常只有一部分配置不同，比如数据库地址、超时时间、日志级别和功能开关。如果每个环境都复制一整份配置，后续维护时就需要在多份文件里反复同步相同内容。

Profile 解决的就是这类多环境组织问题。Go-Spring 不是让每个环境各自维护一套完整配置，而是先加载基础配置，再按当前激活的 Profile 叠加环境差异。这样，公共配置留在基础文件里，环境差异留在 Profile 文件里，最终仍然回到同一套 `Properties` 模型和优先级规则中。

## spring.profiles.active

Go-Spring 使用 `spring.profiles.active` 激活 Profile。命令行参数离本次启动最近，适合明确指定当前启动使用哪个环境。

```bash
./app -Dspring.profiles.active=prod
```

环境变量也可以激活 Profile。按照环境变量转换规则，`GS_SPRING_PROFILES_ACTIVE` 会进入配置系统，并转换成 `spring.profiles.active`。

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
```

多个 Profile 使用逗号分隔。

```bash
./app -Dspring.profiles.active=prod,metrics
```

这里的顺序不是无关紧要的列表顺序。Go-Spring 会按照声明顺序加载这些 Profile 对应的配置，后加载的 Profile 可以覆盖先加载 Profile 中的同名 key。因此，`prod,metrics` 表示先叠加生产环境差异，再叠加指标能力差异。

## 配置文件命名

Go-Spring 默认从 `./conf` 目录加载应用配置。配置文件支持基础配置和 Profile 配置两类命名。下面以 YAML 为例说明，其他已支持格式也遵循同样的 `app.*` 和 `app-{profile}.*` 约定。

- `app.yaml` 是基础配置，所有环境都会生效。
- `app-{profile}.yaml` 是 Profile 配置，只在对应 Profile 激活时生效。

典型目录结构如下。

```text
conf/
  app.yaml
  app-dev.yaml
  app-test.yaml
  app-prod.yaml
```

激活 `prod` 时，Go-Spring 会先加载 `app.yaml`，再加载 `app-prod.yaml`。如果两个文件里出现同名 key，Profile 配置按照第 6 篇讲过的优先级规则覆盖基础配置；如果只是对象下的不同子 key，则继续按照对象合并语义进入同一棵配置树。

这种组织方式的价值在于，Profile 文件只需要写差异。比如基础配置里放通用端口和默认超时。

```yaml
server:
  port: 8080
  timeout: 5s
logging:
  level: info
```

生产环境只需要覆盖真正不同的部分。

```yaml
server:
  timeout: 3s
logging:
  level: warn
```

激活 `prod` 后，最终配置会继续保留基础配置里的 `server.port`，同时使用生产 Profile 里的 `server.timeout` 和 `logging.level`。也就是说，Profile 文件不是另一套完整配置，而是对基础配置的差异补充。

## 配置目录

默认配置目录是 `./conf`。如果项目需要把配置文件放到其他目录，可以通过 `spring.app.config.dir` 修改。

```bash
export GS_SPRING_APP_CONFIG_DIR=./config
```

也可以通过命令行参数指定。

```bash
./myapp -Dspring.app.config.dir=./config
```

`spring.app.config.dir` 的特殊之处在于，这个配置项会影响后续配置文件从哪里加载。因此，配置目录必须在文件发现之前确定，通常放在命令行参数、环境变量或启动前的代码配置里。等基础配置和 Profile 配置开始加载以后，再改变目录就已经来不及影响这一轮配置发现。

## 多个 Profile

当同时激活多个 Profile 时，Go-Spring 会按照声明顺序加载，后面的 Profile 优先级更高。

```properties
spring.profiles.active=dev,metrics
```

如果 `dev` 和 `metrics` 都定义了同一个 key，最终会使用 `metrics` 中的值。这个规则适合表达“先选择环境，再叠加能力”的配置组织方式。

下面这个例子把环境维度和功能维度拆开。基础配置只放所有环境都需要的端口。

```yaml
server:
  port: 8080
```

`app-prod.yaml` 只表达生产环境差异。

```yaml
server:
  timeout: 3s
```

`app-metrics.yaml` 只表达指标能力差异。

```yaml
metrics:
  enabled: true
```

激活 `prod,metrics` 后，最终配置同时包含 `server.port`、`server.timeout` 和 `metrics.enabled`。这里没有任何一个 Profile 文件需要复制完整配置，这些文件只是按顺序把自己的差异合并到配置树里。

## Profile 建模

Profile 设计的关键不是多建几个环境文件，而是让每个 Profile 表达一个相对独立的维度。

常见拆法可以这样看。

- `dev`、`test`、`prod` 表达运行环境。
- `metrics`、`trace` 表达功能能力。
- `local` 表达本地开发覆盖。
- 同一批 key 尽量留在一个维度中维护。

只有维度相对正交，多个 Profile 才适合组合。`prod,metrics` 的含义应该是“生产环境，并且打开指标能力”，而不是一个需要读者猜测覆盖顺序和隐含依赖的组合。

如果把每一种组合都建成独立 Profile，比如 `prod-metrics`、`prod-trace`、`test-metrics`，Profile 很快就会退回复制配置的老问题。只有某个组合确实代表独立部署形态，并且有稳定的运维含义时，单独建 Profile 才更清楚。

## Profile 多环境配置

Profile 多环境配置把“公共配置”和“环境差异”分开放置。基础配置负责表达所有环境共享的部分，Profile 配置负责表达当前环境或能力维度的差异，多个 Profile 再按照声明顺序叠加。

这样组织以后，多环境配置不会变成几份完整文件之间的复制和同步。Profile 仍然沿用 Go-Spring 的同一套 `Properties`、`path`、来源优先级和合并语义，只是在文件组织层面给环境差异提供了更清楚的位置。
