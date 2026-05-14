# Go-Spring 实战第 7 课：Profile 多环境配置：基础配置与环境差异如何组织

Go-Spring 的优先级和合并规则解决了“多份配置同时出现时怎么合成”。但真实项目里，我们还需要进一步解决“这些配置文件应该怎样组织”。

多环境配置最容易一不小心写成复制粘贴。开发、测试、生产环境往往只在少量配置上不同。如果为每个环境复制一整份配置，后面就要在几份文件里反复同步相同内容。更合理的做法，是让基础配置复用，让环境差异独立覆盖。

Go-Spring 通过 Profile 机制处理的就是这个问题。

## 如何激活一个或多个 Profile

可以通过命令行参数激活：

```bash
./app -Dspring.profiles.active=prod
```

也可以通过环境变量激活：

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
```

多个 Profile 使用逗号分隔：

```bash
./app -Dspring.profiles.active=prod,metrics
```

这里的顺序是有意义的，后面会影响多个 Profile 之间的覆盖关系。也就是说呢，Profile 不只是一个集合，它还有先后顺序。

## 基础文件和 Profile 文件如何命名

Go-Spring 遵循和 Spring Boot 类似的命名约定：

- `app.yaml`：基础配置，所有环境生效。
- `app-{profile}.yaml`：Profile 配置，优先级高于基础配置。

典型目录结构：

```text
conf/
  app.yaml
  app-dev.yaml
  app-test.yaml
  app-prod.yaml
```

激活 `prod` 时，框架先加载 `app.yaml`，再加载 `app-prod.yaml`。相同 key 由 Profile 配置覆盖，其他 key 继续使用基础配置。

这种方式鼓励我们只在 Profile 文件中写差异配置。这样基础配置越稳定，Profile 文件就越容易阅读。

## 配置目录也应在启动前确定

默认配置目录是 `./conf`。可以通过 `spring.app.config.dir` 修改：

```bash
export GS_SPRING_APP_CONFIG_DIR=./config
```

或：

```bash
./myapp -Dspring.app.config.dir=./config
```

由于配置目录会影响后续文件加载，所以它通常应该通过命令行参数、环境变量或启动前的代码配置设置。

## 多个 Profile 按声明顺序覆盖

当同时激活多个 Profile 时，后面的优先级更高：

```properties
spring.profiles.active=dev,metrics
```

如果 `dev` 和 `metrics` 都定义了同一个 key，则 `metrics` 覆盖 `dev`。

这个规则适合表达叠加配置：先选择环境，再叠加功能开关。例如 `prod,metrics` 表示生产环境，同时启用指标相关配置。

比如基础配置里放通用端口：

```yaml
server:
  port: 8080
```

`app-prod.yaml` 只覆盖生产超时：

```yaml
server:
  timeout: 3s
```

`app-metrics.yaml` 只补充指标开关：

```yaml
metrics:
  enabled: true
```

激活 `prod,metrics` 后，最终配置同时包含 `server.port`、`server.timeout` 和 `metrics.enabled`。这就是多个 Profile 叠加时最理想的状态：每个文件只负责自己的维度。

## Profile 维度要保持正交

Profile 设计的关键不是多建几个环境文件，而是保持维度正交。

建议：

- `dev`、`test`、`prod` 表达环境维度。
- `metrics`、`trace` 表达功能维度。
- 不要让多个 Profile 互相依赖。
- 不要在多个 Profile 中反复覆盖同一批 key。

正交 Profile 可以自由组合，例如 `dev,metrics`、`prod,metrics`。如果反过来每种组合都要单独建文件，Profile 就退化成了配置复制。

## 正交才是 Profile 的核心价值

Profile 的关键不是多建几个文件，而是把环境、功能开关和基础设施差异拆成可以组合的维度。这样 `prod,metrics` 这类组合才有清晰含义，不会变成另一种形式的复制粘贴。

## 下一篇预告

下一篇会收束配置系统最后一块能力：配置导入、变量引用和动态刷新，看看启动期组合与运行期读取如何共用同一套模型。
