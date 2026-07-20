# i18n 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`i18n` 是 stdlib 层零依赖的本地化消息抽象。它同时支撑业务文案与
`validation.ValidationErrors` 的渲染,而无需在基础层引入任何第三方 i18n 库。

## 1. 职责与边界

- 解析 `(ctx, key, args...) -> string`。Locale 通过 `LocaleFrom(ctx)` 获取,
  下游调用点几乎不显式设置——应用中间件从 `Accept-Language` 一次性写入。
- 位置占位符 `{0}`、`{1}`、... 的插值。索引对不上的占位符原样保留,让模板
  漂移可见。
- 模板存储。内置 `MapSource`;接口保持开放,调用者可自行实现
  `MessageSource` 接入数据库/远程配置等后端。
- 明确不读文件。stdlib 不能反向依赖 `spring/conf` parser,故 `MapSource` 只
  收已解析好的 map,由外层选择 reader。

## 2. 关键抽象与缝隙

- `MessageSource` 是唯一接口——`Message(ctx, key, args...)`。
- `MapSource.AddParsed` 接收 yaml/json reader 的嵌套 map 形态。**同时**处理
  `map[string]any` 与 `map[any]any`——yaml.v2 的嵌套返回后者;只处理其中一种
  会让整棵子树被 `fmt.Sprint` 塌成一个叶子。
- `Localizer(src, ctx)` 把 `MessageSource` 柯里化为
  `func(key, args...) string`——恰好是 `validation.ValidationErrors.Localize`
  需要的形状。这样 validation 不用 import i18n,任何其他查询函数也能挂进来。
- `ErrMessageNotFound` 是 "not found" 错误的哨兵。想优雅降级的调用方直接忽
  略 error、用返回的字符串(即 key)即可;`Localizer` 吞掉错误返回 `""`,让
  `Localize` 回退到 `FieldError.Default()` 兜底。

## 3. 约束(禁止破坏)

- **零依赖**。stdlib 是基础层;引入 `x/text` 或 ICU 会把这依赖泄漏给所有下游。
- **不在本层读文件/URL**。看似轻易解析 `messages.yaml` 会反转依赖方向
  (stdlib → spring/conf)。parser 在装配层,`MapSource` 只收解析后的 map。
- **查找顺序**固定:请求 locale → 默认 locale → key。默认 locale 是部分翻译
  bundle 的兜底,是调用方可依赖的不变量。
- **locale 键类型未导出**(`localeKey struct{}`),避免与其它包 context key
  碰撞。
- **插值器保留未匹配占位符**。不要静默丢弃;可见性是诊断路径。

## 4. 权衡 / 未做的方案

- **不是 ICU MessageFormat**。复数、序数、性别一致等模板不在范围内。位置插
  值对校验消息和典型业务文案足够。
- **无复数 DSL**。若日后需要,可通过二级接口扩展 `MessageSource`;当前接口保
  持精简。
- **init 时不自动扫消息文件**。那会需要 parser 与文件系统约定,均反转依赖方
  向。装配放在外层。
