# starter-webhook 示例

一次自包含的通知往返：应用自起一个本地 HTTP receiver 充当聊天平台
webhook，经 starter 发送一条通知，并断言到达的 JSON 载荷。成功后自退出
（`SIGTERM`）；任何失败都以非零码退出。无需 docker。

## 运行

```bash
go run .
./check.sh   # 冒烟测试
```

手动模式让 receiver 保持运行，便于手工驱动真实的 Notifier：

```bash
go run . -manual   # receiver 监听 http://127.0.0.1:18080/hook
```
