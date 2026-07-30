# bufutil

[English](README.md) | [中文](README_CN.md)

`bufutil` 提供用于旁路复制的有界缓冲区，要求复制过程绝不反向压迫或报错给主流程。

`LimitedBuffer` 是带字节上限的 `bytes.Buffer`：超过上限的写入被静默丢弃，但 `Write` 永远报告完整写入量并返回 nil 错误。这使它成为 `io.TeeReader` 的天然接收端--把请求/响应体镜像到抓取缓冲区用于日志时，体照常流向 handler 不受影响，而抓取有上限，防止失控的体耗尽内存。

它刻意不同于普通的 `bytes.Buffer`（会无限增长），也不同于 `io.LimitReader`（只限制读取、溢出会报错）：`LimitedBuffer` 是写入侧、不报错的上限。

## 用法

通过 `io.TeeReader` 抓取 body，且无内存耗尽风险：

```go
import (
    "io"
    "go-spring.org/stdlib/bufutil"
)

// 把请求体镜像到有界缓冲区供访问日志使用。handler 仍能读到全部字节；缓冲区最多保留 512 KiB。
capture := bufutil.New(512 * 1024)
body = io.TeeReader(r.Body, capture)
// ... handler 读取 body ...
log.Printf("req.body=%s", capture.String())
```

### API

| 方法 | 说明 |
|---|---|
| `New(max int) *LimitedBuffer` | 创建最多保留 `max` 字节的缓冲区（`max < 0` 时 panic）。 |
| `Write(p []byte) (int, error)` | 追加至上限，丢弃溢出；永远返回 `len(p), nil`。 |
| `WriteString(s string) (int, error)` | `Write` 的字符串便捷封装。 |
| `Bytes() []byte` | 已缓冲字节（别名内部存储）。 |
| `String() string` | 已缓冲字节的字符串形式。 |
| `Len() int` / `Cap() int` | 当前大小 / 上限。 |
| `Reset()` | 清空内容（保留上限）以便复用。 |

设计理由见 [DESIGN_CN.md](DESIGN_CN.md)。
