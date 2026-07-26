# starter-config-file 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-file` 属于 config-provider 形态（`starter/DESIGN.md` §2.5）
的集成层 starter：把本地文件或挂载目录变成 Go-Spring 的热更新配置源。它的
首要目标是让 Kubernetes ConfigMap / Secret 挂载在零代码前提下即时反映到应用
属性上。

## 1. 职责与边界

- 只在 `init()` 里通过 `conf.RegisterProvider` 注册一个 `file-watch` provider
  名称，再无别的顶层动作——无可注入 bean、无 server。
- 解析 provider source `file-watch:<path>[?format=..]`，读取文件或目录下每个
  可识别的文件，并合并 flatten 结果。
- 通过 fsnotify 监听目录，任一事件都会触发应用级属性刷新。
- **不做**与任何远程配置中心通信。那些是独立 starter
  （`starter-config-{nacos,etcd,consul,vault,k8s}`）。

## 2. 关键抽象与缝隙

- **Provider 缝隙。** `conf.RegisterProvider("file-watch", loadWatchedConfig)`。
  provider 运行在 `AppConfig.Refresh` 阶段，早于任何 bean 存在。
- **Refresh 钩子。** 容器域桥接 bean `configRefreshBridge`（命名
  `configFileRefreshBridge`，导出 `gs.Rooter`）注入 `*gs.PropertiesRefresher`，
  把 `RefreshProperties` 存入 `atomic.Pointer[func() error]`。
- **Watch 缝隙。** 每个目录一条 fsnotify watcher，用 `watched` 集合去重，避免
  重复 `Load` 造出重复 watch。

## 3. 约束

- **只监听目录，永不监听单文件。** kubelet 更新 ConfigMap / Secret 卷时会写
  一个新的时间戳目录，再原子 rename `..data` 软链。若监听单文件，首次更新后
  会指向失效 inode。source 是文件时，watcher 注册在 `filepath.Dir(path)`。
- **读目录时跳过点开头的条目。** 投射卷机制用到的 `..data` 与时间戳临时目录
  都以 `.` 开头，不能当作配置文件；真正的配置 key 是软链，目标落在 `..data`
  里，通过目录列表可以透明读到。
- **目录模式下未知扩展名静默跳过。** ConfigMap 经常带上应用不打算绑定的 key
  （比如 `README.md`）；未知格式的文件跳过而非致命，避免误伤。若强制
  `format=`，则对每个读到的条目一律按此格式解析——用于 ConfigMap key 没有
  扩展名的场景。
- **`optional:` 只容忍路径不存在。** 路径一旦存在，解析或读取错误始终致命，
  让格式配错立刻暴露。
- **桥接 bean 必须命名。** `gs.Rooter` 是 `any`；稳定命名
  `configFileRefreshBridge` 避免与应用自身的默认 Rooter 在 `__default__`
  上撞车。

## 4. 权衡 / 已否决方案

- **轮询——否决。** fsnotify 能立刻观察到 ConfigMap 软链切换；轮询循环带来的
  CPU 开销并不必要。
- **用 `net/url` 解析 source——否决。** 某些文件系统上路径可以合法包含 `?`；
  provider 只把结尾的 `?...` 当查询串处理（`strings.Cut` 一次即可），让解析
  最小且零依赖。
- **递归目录 watch——刻意不做。** ConfigMap 挂载天然是扁平的；递归遍历只会
  让排除投射卷内务目录更复杂。
