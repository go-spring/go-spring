# BookMan

[English](README.md) | [中文](README_CN.md)

BookMan 是一个小型图书管理示例，用来展示 Go-Spring 在一个接近真实分层应用里的常见用法：
配置加载、Bean 注入、HTTP 路由、业务服务、DAO、SDK 封装、动态配置刷新和后台任务优雅退出。

## 一、目录结构

```text
conf/                     配置文件目录
logs/                     日志文件目录
public/                   静态文件目录
internal/
  app/                    应用层模块
    common/httpsvr/       HTTP Server 与中间件
    controller/           HTTP Controller
  biz/                    业务层模块
    job/                  后台任务
    service/book_service/ 图书业务服务
  dao/book_dao/           内存数据访问层
  idl/http/proto/         HTTP 接口定义与路由注册
  sdk/book_sdk/           外部服务 SDK 封装示例
main.go                   启动入口与自测 Runner
init.go                   Banner 与工作目录初始化
```

## 二、示例重点

- `main.go` 注册 `gs.Runner`，应用启动后自动执行一组 HTTP 请求，演示完整 CRUD 链路，并在结束后发送 `SIGTERM` 触发优雅退出。
- `internal/app/common/httpsvr` 自定义 `http.ServeMux`，注册生成式路由，并通过中间件记录访问日志。
- `internal/app/controller` 按业务能力组织 Controller，Controller 只处理 HTTP 编解码，把业务交给 Service。
- `internal/biz/service/book_service` 组合 DAO 与 SDK，返回带价格和动态配置字段的图书数据。
- `internal/dao/book_dao` 使用内存 Map 模拟数据存储，便于测试和阅读。
- `internal/biz/job` 演示后台任务如何监听应用上下文并优雅退出。

## 三、HTTP 接口

```text
GET    /books          查询图书列表
GET    /books/{isbn}   查询单本图书
POST   /books          新增或更新图书
DELETE /books/{isbn}   删除图书
GET    /               静态首页
```

`POST /books` 请求体示例：

```json
{
  "title": "Clean Architecture",
  "author": "Robert C. Martin",
  "isbn": "978-0134494166",
  "publisher": "Prentice Hall"
}
```

## 四、运行

```bash
go run .
```

启动后，Runner 会自动请求上述接口，打印每一步的 HTTP 状态和响应内容；
随后刷新 `dync.refresh.time` 动态配置，再次查询图书列表，最后发送退出信号验证后台任务的优雅停止。
