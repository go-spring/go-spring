# flatten 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`flatten` 位于零依赖的 `stdlib` 基础层，负责在层级化配置数据与 Go-Spring
绑定器期望看到的 `key -> string` 扁平模型之间搭桥。

## 1. 职责与边界

- 定义"扁平属性 key"的形态（点号分隔 + 方括号索引），并提供层级 <-> 扁平的
  相互转换。
- 提供绑定器所依赖的 `Storage` 接口，以及框架需要的几种具体实现：单个扁平
  源（`PropertiesStorage`）、带前缀视图（`PrefixedStorage`）、多层优先级链
  （`LayeredStorage`）。
- 不是配置加载器。它不会读文件、环境变量或命令行，来源由调用方自己拼装
  `Properties` 并放入某一层。

## 2. 关键抽象

- `Flatten` —— 面向展示的单向转换，把 JSON 形状的 `map[string]any` 打平为
  `map[string]string`。**不可逆**，主要用于日志、对比、以及作为 `Storage`
  的输入。
- `Path` + `Split/JoinPath` —— key 路径的可往返表示，供需要逐段遍历 key
  的绑定逻辑使用。
- `Storage` 接口 —— 绑定器实际需要的三种能力（`Value`、`MapKeys`、
  `SliceEntries`）加上 `Exists`（用于属性条件判断）。保持最小，方便未来
  接入远程配置等替代实现。

## 3. 约束与取舍

- `Flatten` 只支持 JSON 原生类型（map/slice/基本类型/nil），结构体、非字符串
  的 map key、自定义类型都被显式排除。
- `LayeredStorage` 内部混用两种覆盖规则：
  - **叶子值与切片**：高优先级层胜出，一旦命中就停止；这意味着下层的部分
    切片一旦上层定义了同名切片就会被整体遮蔽。
  - **Map**：所有层的 key 会合并，但每个叶子值本身仍按覆盖规则解析。
  非对称是有意的 —— 合并数组语义不清，合并 map key 才是调用方期望的形态。
- `PrefixedStorage.SliceEntries` 会把自己加的前缀从返回 key 上剥掉，
  保证调用方看到的是自己的命名空间。
- `LayeredStorage.Data()` 是给自省用的快照（例如 actuator 的 env 端点），
  不是绑定路径。
