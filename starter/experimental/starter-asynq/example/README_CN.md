# starter-asynq 示例

单个实例扮演双角色：生产者入队一个任务，worker（配置开启）执行它，测试
断言往返。成功后自退出（`SIGTERM`）；任何失败以非零码退出。

## 运行

```bash
docker compose up -d   # redis:7 on 127.0.0.1:6379
go run .
./check.sh             # 起环境、跑示例、收环境 —— 冒烟测试
```

手动模式让 worker 保持运行：

```bash
go run . -manual
```
