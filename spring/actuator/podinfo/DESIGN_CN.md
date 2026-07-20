# podinfo Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`podinfo` 是 Task 05 云原生部署脚手架的一条腿——零依赖 stdlib helper,把 K8s Pod
元数据暴露给应用。另外两条腿是 `gs k8s` 代码生成器与 layout 的 k8s config profile。

## 1. 职责与边界

- 从配置属性绑定 Pod 元数据(name / namespace / IP / node / service account /
  labels path),让应用像用普通 autowire 字段一样读 Pod 事实。
- 已知挂载路径时,解析 Downward API labels 文件。
- 拒绝访问 Kubernetes API server。所有字段来自配置(Downward API env)或挂载
  文件,不 import client-go、不 informer、不 watch。

## 2. 关键抽象与缝隙

- **带 `value` 标签的 `PodInfo` struct**——`${pod.name:=}` 等。默认空值让同一
  段代码在 K8s 外也能 wire,得到零值而非启动失败。
- **`Labels()`**——按 Downward API 每行 `key="value"` 读并 `Unquote`。格式坏的
  行降级用原值;未设 `pod.labels.path` 时返回空 map + nil error。
- **`Metadata()`**——非空标量字段,可作为服务发现注册元数据。刻意排除
  `LabelsPath`:那是实现细节,不是要发布的元数据。

## 3. 约束

- `spring/podinfo` 是 `go-spring.org/stdlib` 的**子包**,非独立 module:无独立
  `go.mod`,不出现在 `go.work` 中。
- struct 带 `value` 标签但不 import IoC 容器,守住零依赖。调用方用
  `gs.Object(&podinfo.PodInfo{})` 注册。
- 环境变量约定:`GS_POD_NAME` / `GS_POD_NAMESPACE` / `GS_POD_IP` /
  `GS_NODE_NAME` / `GS_POD_SERVICE_ACCOUNT`。`pod.labels.path` 走 **profile
  文件**(通常 `app-k8s.properties`),因为 `GS_` env→property 映射无法产出连
  字符或任意路径——不要把它加为 env。

## 4. 取舍与被否决方案

- **不引入 K8s 客户端依赖。** 引 client-go 会破坏零依赖约定并拉入巨大依赖树;
  Downward API 已覆盖 workload 真正需要的一切。
- **不自动注册。** stdlib 不能 import `gs`,故 podinfo 不自注册。应用一行
  `gs.Object(&podinfo.PodInfo{})` 即可。
