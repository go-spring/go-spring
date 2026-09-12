# starter-tdengine 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

一个 Client 原型 starter（`starter/DESIGN.md` §2.2），经官方 driver-go v3
的 taosWS 连接器接入 TDengine。两个承重决策是线路协议与韧性/可观测的挂
载缝。

## 1. 职责与边界

- **负责**：`spring.tdengine.instances.<name>` 组的 bean 生命周期、DSN 解析为
  taosWS 连接器、守卫过的 `*sql.DB` 池、启动探活与每实例指示器、逐语句
  守卫。
- **不负责**：时序惯用法的 SQL 构建（超级表、无模式写入仍是经
  `database/sql` 的原生 SQL）、STMT 参数绑定（`database/sql` 不建模；需要
  时直接用 SDK）、连接级负载均衡（DSN 指向单个 taosAdapter）。

## 2. 关键抽象与 Seam

- **websocket（taosWS）线路** — driver-go v3 提供三种连接器：需要安装
  原生 libtaos 的 CGO `taosSql`、吞吐/功能较弱的 REST `taosRestful`、
  纯 Go 的 websocket `taosWS`。starter 选 taosWS：零安装、功能完整；
  REST 连接器保留为自定义 driver 的空间。
- **`Client` 包装 = `*sql.DB` + 武装槽位** — database/sql 没有可在构造后
  替换的传输层或回调链，因此 DefaultDriver 用 `guardedConnector`/
  `guardedConn` 包住 taosWS 连接器：池中每条连接在每次
  `ExecContext`/`QueryContext` 时查询 `clientSlot`。Init 构建观察器 +
  执行器（经中立的 `resilience.ExecutorFor` / `fault.InjectorFor` seam）
  并武装槽位；此前语句原样直通。这是 starter-gorm 回调链与 HTTP starter
  RoundTripper 适配器在 database/sql 里的对应物。
- **health** — `PingContext`，与 gorm 家族探针同形。

## 3. 约束

- driver-go v3.8.2 要求 websocket 线路上的 TDengine 服务端 ≥ 3.3.6.0；
  更老的服务端会在启动探活时报驱动的版本错误。
- TDengine 没有事务；包装器的 `Begin` 委托给驱动，由它报错。
- DSN 用驱动的统一格式（`user:pass@ws(host:port)/db`）—— 故意不拆成
  host/port/user 字段，驱动的参数面（`?readTimeout=...` 等）原样透传。

## 4. 权衡 / 已否决的备选

- **taosWS vs taosSql（CGO）**：CGO 驱动会逼每个构建/部署主机安装原生
  客户端并复杂化交叉编译；websocket 驱动以少量协议开销换零依赖构建 ——
  对框架 starter 是正确的默认。
- **`*sql.DB` vs 原生 `taosws.Conn` API**：原生 API 暴露 `database/sql`
  无法建模的无模式 STMT 写入，但 `*sql.DB` bean 白拿连接池、context 与
  整个 `database/sql` 工具链；原生路径经自定义 driver 仍可达。
- **driver.Conn 层守卫 vs 助手方法**：守卫 `ExecContext`/`QueryContext`
  覆盖所有调用方（包括架在池上的 ORM），无需记得用 `Guarded*` 助手 ——
  这是选择更深一层包装的最强理由。
