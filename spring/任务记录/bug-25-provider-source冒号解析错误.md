# Bug 25：Provider Source 冒号解析错误

- 状态：已提交
- 位置：`conf/provider/provider.go`
- 预期来源：`Load` 注释示例明确支持 `etcd:localhost:2379/config` 和 `optional:etcd:localhost:2379/config` 这类自定义 provider source。
- 问题：当前解析逻辑对所有包含两个冒号的 source 使用 `strings.SplitN(source, ":", 3)`，非 optional 的 `etcd:localhost:2379/config` 会被解析为 provider=`localhost`、source=`2379/config`。
- 影响：自定义配置 provider 的 source 中只要包含冒号，例如 host:port，就会被错误路由到不存在的 provider。
- 复现：新增 `TestLoadCustomProviderSourceWithColon`，注册测试 provider 并加载 `colonProviderForTest:localhost:2379/config`。
- 修复：只在 `optional:` 前缀存在时先剥离 optional 标记，随后始终按第一个冒号拆分 provider 和 source。
- 验证：
  - `go test ./conf/provider -run TestLoadCustomProviderSourceWithColon`：修复前 provider 被误解析为 `localhost`；修复后通过。
  - `go test ./conf/provider ./conf ./gs/internal/gs_conf`：通过。
  - `go test ./...`：通过。
  - `go test -race ./conf/provider`：通过。
- 提交：本提交 `parse provider source with colon`。
- 备注：保留无 provider 前缀时默认使用 `file` provider 的行为。
