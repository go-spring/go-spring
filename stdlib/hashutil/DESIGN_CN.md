# hashutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。当前是围绕 `hash/fnv` 的单函数文件。

## 1. 职责与边界

- 提供一步到位的"字符串 -> uint64"，让分片 / 分桶场景不必重复 `New64a` +
  `Write` + `Sum64` 三件套。
- 不是密码学哈希包。MD5 独立在 `md5util`；如果未来加入 SHA 族或 HMAC，也
  不会放在这里。

## 2. 设计说明

- 通过 `hash/fnv` 转发而非手写 FNV-1a 循环。可读性和与其它 `hash.Hash` 用户
  的一致性优先于榨那几纳秒。
- 不提供流式 API。需要分批喂数据的场景请直接使用 `hash/fnv`。
