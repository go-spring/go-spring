# md5util 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。之所以与 `hashutil` 分开，是要让"使用 MD5"这件事
在导入语句里显式可见。

## 1. 职责与边界

- 一步返回 `MD5(string) string`，直接给出绝大多数场景需要的 hex 形式
  （缓存 key、ETag、指纹）。
- 不提供 HMAC 或流式 API。分块哈希、密钥派生请直接使用 `crypto/md5`（或者
  更好的选择是现代哈希算法）。

## 2. 设计说明

- 通过 `encoding/hex.EncodeToString` 输出小写 hex，与常见数据库 / 缓存约定
  一致。
- 保持"一个函数一个包"就是设计意图：将来引入 SHA-1、SHA-256、HMAC 等能力
  都要开一个新包，让调用方通过 import 显式声明。
