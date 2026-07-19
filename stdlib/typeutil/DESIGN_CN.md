# typeutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`typeutil` 集中收敛了 Go-Spring 容器在扫描注入
候选或注册 provider 时用到的"这是什么类型？"判定。

## 1. 职责与边界

- 定义"基本值类型"、"构造器"、"bean 类型"、"注入 / 绑定目标"这些术语。
  这些名字直接出现在容器错误信息里，因此定义都放在同一处便于 grep。
- 不是通用反射工具库。任何需要触碰运行时值（而非 `reflect.Type`）的逻辑
  都不放在这里，通常留在使用它的代码旁边。

## 2. 关键决策

- **`IsBeanType` 形态**：`chan`、`func`、`interface`、`*struct`。值类型的
  struct 被有意排除 —— 容器面向引用工作，才能注入代理 / 切面。需要值语义的
  调用点应显式取指针。
- **`IsConstructor` 形态**：要么 `func() T`（T 不是 error），要么
  `func() (T, error)`。其它形态（多返回值、单 `func() error`）会在容器上层
  被拒。
- **`IsPropBindingTarget` 与 `IsBeanInjectionTarget` 分开**：容器把"给我
  一个配置值"和"给我一个依赖"当作两条注入路径，它们的合法目标形态不同。
- `IsErrorType` 与 `IsBeanInjectionTarget` 处理了 nil `reflect.Type`
  （返回 `false`），其它函数没有做同样的守卫，可能为 nil 的调用点需自己
  先判。

## 3. 约束

- 零依赖。除 `reflect` 和泛型约束外别无他物。这个包被容器 / 切面几乎所有
  内部代码引用，添加新依赖的门槛非常高。
