# Go-Spring 实战第 7 课：Profile 多环境配置：基础配置与环境差异如何组织

Go-Spring 的优先级和合并规则解决了“多份配置同时出现时怎么合成”。到了真实项目里，问题会继续往前走，即这些配置文件怎样组织，后续维护才不容易重复。

多环境配置最容易一不小心写成复制粘贴。开发、测试、生产环境往往只在少量配置上不同。如果为每个环境复制一整份配置，后面就要在几份文件里反复同步相同内容。更合理的做法，是让基础配置复用，让环境差异独立覆盖。

Go-Spring 通过 Profile 机制处理的就是这个问题。

## 如何激活一个或多个 Profile

可以通过命令行参数激活。

```bash
./app -Dspring.profiles.active=prod
```

也可以通过环境变量激活。

```bash
export GS_SPRING_PROFILES_ACTIVE=prod
```

多个 Profile 使用逗号分隔。

```bash
./app -Dspring.profiles.active=prod,metrics
```

这里的顺序会影响多个 Profile 之间的覆盖关系。也就是说，Profile 不只是一个集合，它还有先后顺序。

## 基础文件和 Profile 文件如何命名

Go-Spring 遵循和 Spring Boot 类似的命名约定。

- `app.yaml` 是基础配置，所有环境生效。
- `app-{profile}.yaml` 是 Profile 配置，优先级高于基础配置。

典型目录结构如下。

```text
conf/
  app.yaml
  app-dev.yaml
  app-test.yaml
  app-prod.yaml
```

激活 `prod` 时，框架先加载 `app.yaml`，再加载 `app-prod.yaml`。相同 key 由 Profile 配置覆盖，其他 key 继续使用基础配置。

这种方式鼓励我们只在 Profile 文件中写差异配置。因为通用部分已经留在基础文件里了，Profile 文件越少重复，就越容易看出当前环境到底改了什么。

## 配置目录也要在启动前确定

默认配置目录是 `./conf`。可以通过 `spring.app.config.dir` 修改。

```bash
export GS_SPRING_APP_CONFIG_DIR=./config
```

也可以这样写。

```bash
./myapp -Dspring.app.config.dir=./config
```

由于配置目录会影响后续文件加载，它通常会放在命令行参数、环境变量或启动前的代码配置里设置。

## 多个 Profile 按声明顺序覆盖

当同时激活多个 Profile 时，后面的优先级更高。

```properties
spring.profiles.active=dev,metrics
```

如果 `dev` 和 `metrics` 都定义了同一个 key，则 `metrics` 覆盖 `dev`。

这条规则适合表达叠加配置——先选择环境，再叠加功能开关。例如 `prod,metrics` 表示生产环境，同时启用指标相关配置。

比如基础配置里放通用端口。

```yaml
server:
  port: 8080
```

`app-prod.yaml` 只覆盖生产超时。

```yaml
server:
  timeout: 3s
```

`app-metrics.yaml` 只补充指标开关。

```yaml
metrics:
  enabled: true
```

激活 `prod,metrics` 后，最终配置同时包含 `server.port`、`server.timeout` 和 `metrics.enabled`。多个 Profile 叠加时，最清晰的状态就是每个文件只表达自己的维度。

## Profile 维度保持正交时更好组合

Profile 设计的关键不是多建几个环境文件，而是保持维度正交。

常见拆法可以这样看。

- `dev`、`test`、`prod` 表达环境维度。
- `metrics`、`trace` 表达功能维度。
- Profile 之间尽量保持独立。
- 同一批 key 尽量留在一个维度中维护。

正交 Profile 可以自由组合，例如 `dev,metrics`、`prod,metrics`。如果反过来每种组合都要单独建文件，Profile 就退化成了配置复制。

## 正交维度让 Profile 更容易组合

Profile 的关键不是多建几个文件，而是把环境、功能开关和基础设施差异拆成可以组合的维度。这样 `prod,metrics` 这类组合会有清晰含义，也能减少另一种形式的复制粘贴。反过来，如果每个组合都单独建文件，后面维护时还是会回到同步多份配置的问题。

Profile 解决的是多环境组织；配置系统还剩最后一块高级能力，即启动期如何导入和引用配置，运行期又如何读取可刷新的动态值。
