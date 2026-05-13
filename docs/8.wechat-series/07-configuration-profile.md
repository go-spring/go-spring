# Profile 与多环境配置

优先级和合并规则解决了“多份配置同时出现时怎么合成”。但真实项目里，我们还需要进一步解决“这些配置文件应该怎样组织”。

多环境配置最容易写成复制粘贴。开发、测试、生产环境往往只在少量配置上不同。如果为每个环境复制一整份配置，后面就要在几份文件里反复同步相同内容。更合理的做法，是让基础配置复用，让环境差异独立覆盖。

Go-Spring 通过 Profile 机制处理这个问题。

## 激活 Profile

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

这里的顺序是有意义的，后面会影响多个 Profile 之间的覆盖关系。也就是说，Profile 不只是一个集合，它还有先后顺序。

## 文件命名约定

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

## 自定义配置目录

默认配置目录是 `./conf`。可以通过 `spring.app.config.dir` 修改：

```bash
export GS_SPRING_APP_CONFIG_DIR=./config
```

或：

```bash
./myapp -Dspring.app.config.dir=./config
```

由于配置目录会影响后续文件加载，所以它通常应该通过命令行参数、环境变量或启动前的代码配置设置。

## 多个 Profile 的优先级

当同时激活多个 Profile 时，后面的优先级更高：

```properties
spring.profiles.active=dev,metrics
```

如果 `dev` 和 `metrics` 都定义了同一个 key，则 `metrics` 覆盖 `dev`。

这个规则适合表达叠加配置：先选择环境，再叠加功能开关。例如 `prod,metrics` 表示生产环境，同时启用指标相关配置。

## 保持 Profile 正交

Profile 设计的关键不是多建几个环境文件，而是保持维度正交。

建议：

- `dev`、`test`、`prod` 表达环境维度。
- `metrics`、`trace` 表达功能维度。
- 不要让多个 Profile 互相依赖。
- 不要在多个 Profile 中反复覆盖同一批 key。

正交 Profile 可以自由组合，例如 `dev,metrics`、`prod,metrics`。如果反过来每种组合都要单独建文件，Profile 就退化成了配置复制。

## Profile 的重点是正交

Profile 的关键不是多建几个文件，而是把环境、功能开关和基础设施差异拆成可以组合的维度。这样 `prod,metrics` 这类组合才有清晰含义，不会变成另一种形式的复制粘贴。

Profile 还会影响 IoC 装配。后面进入容器部分时，我们会再看 `.OnProfiles()` 如何决定 Bean 是否参与注册。
