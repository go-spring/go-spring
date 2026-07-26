# starter-validation Example

演示 starter-validation 的数据校验与 i18n 国际化。

## 功能验证

- **Config-binding 校验**：配置绑定后校验 struct 字段（email 格式、port 最小值），失败时输出本地化错误信息
- **Web 路径校验**：HTTP handler 解码请求体并校验，失败时返回结构化 400
- **i18n**：同一套 `ValidationErrors` 渲染为英文或中文，消息从 `messages_*.yaml` 加载

## 手动验证

```bash
cd starter-validation/example
go run .
```

预期输出（英文）：
```
== config-binding path ==
 - admin must be a valid email address
 - port must be at least 1024

== web path (en) ==
 status=400 body={"errors":["email must be a valid email address","age must be at least 18"]}

== web path (zh) ==
 status=400 body={"errors":["email 必须是有效的电子邮件地址","age 必须大于等于 18"]}

== web path (valid) ==
 status=200 body=ok: a@b.com
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。