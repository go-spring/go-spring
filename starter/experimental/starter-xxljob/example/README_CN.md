# starter-xxljob 示例

自包含：示例启动一个 mock xxl-job admin（只实现注册与触发所需的最小 REST
面），启动执行器、注册 handler、经 mock admin 触发任务、断言 handler 已
运行。无需 docker，也无需真实 xxl-job-admin。

## 运行

```bash
go run .
./check.sh   # 冒烟测试
```

手动模式让执行器保持运行：

```bash
go run . -manual
```
