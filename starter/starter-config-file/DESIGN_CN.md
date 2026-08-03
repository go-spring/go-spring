# starter-config-file 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-file` 属于 config-provider 形态（`starter/DESIGN.md` §2.5）
的集成层 starter：把本地文件系统配置变成 Go-Spring 的热更新配置源。它注册两个
provider，共用同一套监听 + 刷新桥接：

- **`file-watch`** —— 每次 import 一份配置文档（承载 `application.yaml` 的
  ConfigMap key）。分层覆盖靠 `spring.app.imports` 行序叠加；provider 绝不合并目录。
- **`configtree`** —— 标量 key 文件目录（Secret / env 风格 ConfigMap 挂载）。每个
  叶子文件成为一个属性：key 为带点的相对路径，value 为未解析的内容。

## 1. 职责与边界

- 在 `init()` 里通过 `conf.RegisterProvider` 注册 `file-watch` 与 `configtree` 两个
  provider 名称，再无别的顶层动作——无面向用户的导出 bean、无 server（内部 controller bean
  纯属桥接管路，由 starter.go 的共享 init 注册）。
- `file-watch`：接收 `file-watch:<file>`，拒绝目录，读取单个文件（按扩展名走共享 conf reader
  注册表），返回 flatten 结果。
- `configtree`：解析 `configtree:<dir>`，拒绝文件，遍历目录树，每个非点号叶子文件
  产出一条（路径→key、内容→value）。
- 两者都通过共享 controller 安装 fsnotify 监听；任一事件都会触发应用级属性刷新。
- **不做**与任何远程配置中心通信。那些是独立 starter
  （`starter-config-{nacos,etcd,consul,vault,k8s}`）。

## 2. 关键抽象与缝隙

- **Provider 缝隙。** `init()` 里调用两次 `conf.RegisterProvider`——一次 `file-watch`
  （→ `Load`）、一次 `configtree`（→ `LoadConfigTree`），都绑定到同一个 controller 单例
  的方法。provider 运行在 `AppConfig.Refresh` 阶段，早于任何 bean 存在。
- **Refresh 钩子。** 容器域桥接 bean `configFileController`（命名 `configFileController`，
  导出 `gs.Rooter`）注入 `*gs.PropertiesRefresher`，直接存到包级 `fileWatchController`
  单例上。接线前 `TriggerRefresh` 是安全空操作；接线后调用 `RefreshProperties`。
- **Watch 缝隙。** 每个目录一条 fsnotify watcher，用 `watched` 集合去重，避免
  重复 `Load` 造出重复 watch。

## 3. 约束

- **file-watch 只接受单个文件，传目录即报错。** 优先级归属于 `spring.app.imports` 的行序
  （框架的分层存储），而非目录内容——把任意文件合并进同一个属性集没有良定义的
  先后，因此 file-watch 拒绝这么做。分层覆盖应按优先级每个文件写一行 import。
- **只监听父目录/树中目录，永不监听文件本身。** kubelet 更新 ConfigMap / Secret 卷时会写
  一个新的时间戳目录，再原子 rename `..data` 软链；常见编辑器也以原子 rename
  方式保存。两种情况下文件 inode 都会变，若监听单文件，首次更新后会指向失效
  inode。因此 watcher 注册在 `filepath.Dir(path)`（file-watch）或树中的每个目录
  （configtree），它们在软链交换过程中都保持稳定。
- **configtree：路径即 key、内容即 value，不合并、无优先级。** 每个非点号叶子产出恰好
  一条属性，key 为带点的相对路径。路径唯一，两个叶子永远不会冲突，所以无需定义源内优先级
  ——当年让目录合并模式翻车的优先级问题，在这里被结构上排除。value 是 trim 后的裸字符串
  （不做格式解析）；不接受 `?format=`。
- **`optional:` 只容忍文件不存在。** 文件一旦存在，解析或读取错误始终致命，
  让配错的文件立刻暴露。
- **桥接 bean 必须命名。** `gs.Rooter` 是 `any`；稳定命名
  `configFileController` 避免与应用自身的默认 Rooter 在 `__default__`
  上撞车。

## 4. 权衡 / 已否决方案

- **轮询——否决。** fsnotify 能立刻观察到 ConfigMap 软链切换；轮询循环带来的
  CPU 开销并不必要。
- **per-provider format map / `?format=` 覆盖——否决。** 共享的 `conf/reader` 注册表已经把
  扩展名映射到 reader；file-watch 直接委托 `reader.ReadFile`，不再重复维护这层映射。按名强制
  reader 的 `?format=` query 也一并砍掉：file-watch 指向一个具体文件，要求它有可识别扩展名是
  合理的，同时让 provider 无 query、依赖更轻（不再为格式解析 import 各 reader）。
- **目录合并——否决。** 由标量 key 文件组成的目录（路径→key、内容→value、不解析）
  是另一种模型：天然无重叠、无优先级问题，应作为独立的 configtree 风格 provider
  存在，而非塞进 file-watch。二者混用会把一个定义不清的"目录内优先级"强加到原本
  优先级干净的分层模型上。
