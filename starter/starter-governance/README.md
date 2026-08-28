# starter-governance — 治理中心接线 + 动态源适配器

这个 starter 有两重身份：

1. **默认接线**（常驻 wiring bean，[wiring.go](wiring.go)）：把 `${govern}` 的 gs.Dync 绑定为治理中心的默认 Source、注册 executor/fault seam、触发 OnReady——`cloud/governance` 本体容器无关（不 import spring/gs），**blank import 本 starter即让 `${govern}` 全链生效**（所有 conf provider 的 watch 照常）。
2. **动态源适配器**：治理规则经 `governance.Source` 契约流入的自建刷新链路，配置后替换默认 dync 源。

## 定位

```
${govern} app.properties → gs.Dync 适配（默认路径，本 starter 不影响它）
独立规则文件 → FileSource ────────┐
治理控制台/规则 API → HTTPSource ──┼──→ governance.Source → Center → label diff → executor/fault 热更
Nacos dataId / etcd key（见对应 config starter）┘
```

规则文档统一走 `governance.ParseRules`：同一份文档（`govern.*` 键，properties/yaml/json/toml）在 file/http/nacos/etcd 各后端间**逐字节可移植**。

- **不配置则惰性**：导入本 starter 但不配 `govern.source.*` 时什么都不注册，默认 `${govern}` 路径原样生效。
- **配置即接管**：Source bean 注入治理中心（优先级：显式 `governance.SetSource` > 本 bean > Dync 默认），规则变更**只刷新治理**，不触发全应用配置 re-bind。
- 单一活跃源：进程只有一个生效 Source（治理中心契约如此），所以这里全部是条件单例 bean，不是 Group。

## file 源：独立规则文件

```properties
# app.properties —— 一行接线
govern.source.file.path=/etc/app/govern.yaml
```

规则文件（`govern.yaml`）的键与写在 app.properties 里的 `${govern}` 键**完全相同**，格式按扩展名识别（json/properties/yaml/toml）：

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
  rules:
    - resources: redis:cache
      attempt-timeout: 50ms
```

行为要点：

- **fsnotify 监听父目录**（非文件本身）：兼容编辑器原子重命名保存与 K8s ConfigMap 的 `..data` symlink 原子替换。
- **启动即校验**：路径缺失/解析失败直接启动报错，不静默装一个 disabled 中心。
- **坏编辑保底**：运行期解析失败或文件被截断清空（无任何 `govern.*` 键），保留上一份好配置并打日志——关治理的正确姿势是 `govern.enabled=false`（键存在），不是空文件。
- **无变更不推送**：DeepEqual 去重，touch 不触发 executor 重建。

## 排错

| 症状 | 原因 |
|---|---|
| 配了 `govern.source.file.path` 但规则仍走 `${govern}` | bean 必须 `Export(gs.As[governance.Source]())` 才能被中心注入——本 starter 已正确导出；若你自己写 Source bean 忘了 Export 就会静默回落默认源 |
| 热改没生效 | 看日志有没有 `reload ... failed (keeping last good config)`；确认改的是被监听路径的那个文件 |

## 后续

其它后端（apollo/consul/vault 直连等）按 FileSource 的模式加入：实现 `Snapshot/Subscribe(/Close)`，`OnProperty("govern.source.<name>")` 条件注册 + Export 为 `governance.Source`。已有直连适配器：**nacos** 在 starter-config-nacos（`govern.source.nacos.*`，ListenConfig 推送）、**etcd** 在 starter-config-etcd（`govern.source.etcd.*`，Watch 推送）——与对应配置中心的客户端设施同模块复用。

## http 源：治理控制台轮询

```properties
govern.source.http.url=https://console.example.com/rules/app.yaml
govern.source.http.interval=10s        # 默认 5s
govern.source.http.format=yaml         # 默认按 URL 扩展名推断
govern.source.http.headers.authorization=Bearer xxx
```

控制台短暂不可用（fetch 失败/非 200/坏文档）保留上一份好配置；文档变更经 DeepEqual 去重后推送。
### Log tag

Runtime logs from this module carry the tag `_app_governance` (governance center). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.governance.type=Logger
logger.governance.level=WARN
logger.governance.tag=_app_governance
```
