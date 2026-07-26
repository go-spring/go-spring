# starter-config-k8s 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-k8s` 属于 config-provider 形态（`starter/DESIGN.md` §2.5）
的集成层 starter：通过 client-go informer 直接从 API server 读取 ConfigMap
或 Secret，让它成为 Go-Spring 的热更新配置源。它与 `starter-config-file`
互补。

## 1. 职责与边界

- 只在 `init()` 里通过 `conf.RegisterProvider` 注册一个 `k8s` provider 名称，
  再无别的顶层动作——无可注入 bean、无 server。
- 解析 provider source
  `k8s:<kind>/<name>?namespace=&key=&format=&kubeconfig=`，拉取对象，按扩展
  名或强制 `format` 解析选中的 data 条目，合并 flatten 结果。
- 对该对象安装一个 client-go informer，任意 add/update/delete 事件都会触发
  应用级属性刷新。

## 2. 关键抽象与缝隙

- **Provider 缝隙。** `conf.RegisterProvider("k8s", loadK8sConfig)`。provider
  运行在 `AppConfig.Refresh` 阶段，早于任何 bean 存在。
- **`k8sClient` 接口。** provider 收窄的接口（只需 `CoreV1` 读能力）而不是
  直接接 `*kubernetes.Clientset`，测试可以注入 client-go 的 fake clientset，
  无需真集群。`buildClient` 从 in-cluster 配置或 kubeconfig 文件构建真 client。
- **Informer 缝隙。** `ensureWatch` 下每个 `(kind, namespace, name)` 三元组
  一个 shared informer；任何事件都会调用 refresh 钩子。
- **Refresh 钩子。** 容器域桥接 bean，导出为 `gs.Rooter`，命名以避开
  `__default__` 冲突。

## 3. 约束

- **返回前必须先注册 informer。** `ensureWatch` 在 provider 返回之前调用，
  避免“初次读取与 informer sync 之间”的变更被漏掉。
- **ConfigMap 的 `Data` 与 `BinaryData` 需合并。** ConfigMap 有效负载可能
  两处都放；provider 把它们合并为 `name -> bytes` 单一 map 统一处理。
- **whole-object 模式下未知扩展名跳过。** ConfigMap 常携带非配置条目
  （`README.md`、模板文件等）；provider 只解析扩展名映射到已知 reader 的
  条目。指定 `?key=<one>` 时必须解析成功——那种情况的未知格式是硬错误
  （报错时提示用户设置 `format=`）。
- **Secret payload 已经是 `[]byte`。** 无需 base64 解码；client-go 对象
  暴露的已经是解码值。
- **RBAC 在调用侧。** provider 只对目标 kind 执行 `Get`/`Watch`；对应
  ServiceAccount 必须具备该 kind 在指定 namespace 的 `get,list,watch` 权限。

## 4. 权衡 / 已否决方案

- **文件与 API 合成一个 mega-provider——否决。** 挂载文件与 API 访问在
  RBAC、失败模式、时延（kubelet 约 1 分钟 Secret 轮换 vs API 秒级）上差
  异明显。两个 starter 让心智模型与失败模式各自清晰，应用按部署形态空
  导入其中之一。
- **自造 watcher 替换 `SharedInformerFactory`——否决。** informer 已提供
  resync、连接重试、事件合并；手写这些是无谓重复。
