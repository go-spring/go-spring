# starter-mesh Example

演示 starter-mesh 的服务网格模式。

## 功能验证

- **Mesh 模式切换**：对比 mesh on/off 两种模式的负载均衡行为
- **直通模式**：mesh on 时降级为直通，绕过 discovery 和负载均衡
- **负载均衡**：mesh off 时通过 discovery + LB 均匀分发到多端点

## 手动验证

```bash
cd starter-mesh/example
go run .
```

预期输出：
```
OK: mesh mode degrades discovery + load balancing to a pass-through
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。