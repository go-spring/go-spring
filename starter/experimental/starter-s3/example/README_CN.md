# starter-s3 示例

一次面向 MinIO 的对象存储往返：应用检查/创建 `go-spring-example` 桶，
上传一个对象、读回、stat、再删除。往返成功后自退出（`SIGTERM`）；任何
失败都会以非零码退出。

## 运行

```bash
docker compose up -d   # MinIO :9000（控制台 :9001）+ 建桶任务
go run .
./check.sh             # 起环境、跑示例、收环境 —— 冒烟测试
```

手动验证模式会让服务保持运行：

```bash
go run . -manual
```
