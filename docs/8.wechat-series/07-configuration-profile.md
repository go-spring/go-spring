# Profile 与多环境配置

## 本篇要解决的问题

开发、测试、生产环境往往只在少量配置上不同。合理的多环境配置应该让基础配置复用，让环境差异独立覆盖，而不是为每个环境复制一整份配置。

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

这种方式鼓励只在 Profile 文件中写差异配置。

## 自定义配置目录

默认配置目录是 `./conf`。可以通过 `spring.app.config.dir` 修改：

```bash
export GS_SPRING_APP_CONFIG_DIR=./config
```

或：

```bash
./myapp -Dspring.app.config.dir=./config
```

由于配置目录会影响后续文件加载，它通常应该通过命令行参数、环境变量或启动前的代码配置设置。

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

正交 Profile 可以自由组合，例如 `dev,metrics`、`prod,metrics`。如果每种组合都要单独建文件，Profile 就退化成了配置复制。

## 边界

本篇只讨论配置文件层面的 Profile。Profile 还会影响 IoC 条件装配，后续 IoC 板块会单独讨论 `.OnProfiles()` 与 Bean 注册边界。

