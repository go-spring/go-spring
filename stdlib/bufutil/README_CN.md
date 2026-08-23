# bufutil

[English](README.md) | [中文](README_CN.md)

`bufutil` 提供一个有界缓冲区，用来在主流程之外旁路复制一份数据流。复制丢了尾部没关系，但绝不能反过来拖慢或报错给主流程。

`LimitedBuffer` 是带字节上限的 `bytes.Buffer`。写入超过上限的部分被静默丢弃，`Write` 却始终返回完整写入量和 nil 错误。典型用法是访问日志抓取请求/响应体（用 `io.TeeReader` 镜像 body）、流式/SSE 分块抓取（每块之间 `Reset()` 复用缓冲区），以及其它截断无妨的调试/审计拷贝。

如果需要完整数据、精确字节计数、流控反压或并发写入，那么它不合适。

## 使用方式

通过 `io.TeeReader` 抓取 body：

```go
import (
    "io"
    "go-spring.org/stdlib/bufutil"
)

// 把请求体镜像到有界缓冲区供访问日志使用。
// handler 仍能读到全部字节；缓冲区最多保留 512 KiB。
capture := bufutil.NewLimitedBuffer(512 * 1024)
body = io.TeeReader(r.Body, capture)
// ... handler 读取 body ...
log.Printf("req.body=%s", capture.String())
```

流式响应可复用同一个缓冲区逐块抓取：

```go
buf := bufutil.NewLimitedBuffer(l.limit)
for each event {
    buf.Reset() // 保留上限，丢弃上一块
    io.Copy(buf, eventReader)
    log.Printf("event=%s", buf.String())
}
```

### API 列表

| 方法 | 说明 |
|---|---|
| `NewLimitedBuffer(max int) *LimitedBuffer` | 创建最多保留 `max` 字节的缓冲区（`max < 0` 时 panic）。 |
| `Write(p []byte) (int, error)` | 追加至上限，丢弃溢出；永远返回 `len(p), nil`。 |
| `WriteString(s string) (int, error)` | `Write` 的字符串便捷封装。 |
| `Bytes() []byte` | 已缓冲字节（别名内部存储）。 |
| `String() string` | 已缓冲字节的字符串形式。 |
| `Len() int` / `Cap() int` | 当前大小 / 上限。 |
| `Reset()` | 清空内容（保留上限）以便复用。 |

## 关键设计

- **静默溢出，但报告全量写入**：`Write` 丢弃超限字节，返回的仍是 `len(p), nil`。缓冲区写满时 `io.TeeReader` 因此不会被阻塞，改成报错或少写反而会破坏主流程的读取。
- **零值即上限 0**：零值 `LimitedBuffer` 丢弃一切，是个安全的默认值，需要非零上限就用 `NewLimitedBuffer(max)` 显式设置。负上限直接 panic。
- **`Bytes()` 返回内部存储的别名**：继承自 `bytes.Buffer`，下一次 `Write`/`Reset` 之后切片即失效。要稳定副本，得赶在下次写入前拷贝一份。
- **有损**：超过上限的数据直接丢了，也不记录丢了多少。需要精确字节计数，就在外层自己统计。
- **按字节截断，不按字符截断**：多字节 UTF-8 字符（中文、emoji）可能在尾部被劈开，`Bytes()`/`String()` 留下非法 UTF-8。好在 UTF-8 自同步，截断点之前的内容不受影响，要干净字符串用 `strings.ToValidUTF8` 处理一次即可。
- **不可并发使用**，跟 `bytes.Buffer` 一样。上限构造时定一次就不再变，`Reset` 只清内容、不动上限。

## 许可证

`bufutil` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
