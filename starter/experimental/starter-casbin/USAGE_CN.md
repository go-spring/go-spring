# starter-casbin 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均经 starter 源码核对
(`starter.go`、`config.go`)并锚定可自断言的 [example/](example/)
(`example/check.sh`,零外部依赖)。**Casbin 自身语义(model 语言、policy 格式、Enforce/API、
adapter、watcher)见[官方文档](https://casbin.org/docs/overview)**——以下全部是 go-spring 增量。

**激活条件**:`init()` 里的 `gs.Module` 监听 `spring.casbin`——每个 `spring.casbin.instances.<name>`
配置项生成一个名为 `<name>` 的 `*StarterCasbin.Enforcer` **容器 bean**。*enforcer 是
bean,它们用的 adapter/watcher 也是**——config 的 `adapter` / `watcher` 键是** bean 名**,
每个实例的 Module 把对应 bean 注入 enforcer 构造器(键为空则传 nil)。没有包级注册表。
没有 `enabled` key;零个 `spring.casbin.instances.*` 即零个 enforcer,starter 完全惰性。

---

## 1. 完整工程示例

一个用 RBAC enforcer 回答鉴权问题的 HTTP 服务,通过注册的 watcher 做策略热更新。
文件树(与 `example/` 同构):

```
demo/
├── go.mod
├── main.go
└── conf/
    ├── app.properties
    ├── model.conf
    └── policy.csv
```

**go.mod**(关键依赖):

```
require (
    github.com/casbin/casbin/v2        v2.135.0
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-casbin       latest
)
```

**conf/model.conf**——最小 RBAC 模型(语法见[官方文档](https://casbin.org/docs/syntax-for-models)):

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

**conf/policy.csv**:

```csv
p, admin,  /data, read
p, admin,  /data, write
p, viewer, /data, read
g, alice, admin
g, bob,   viewer
```

**main.go**:

```go
package main

import (
    "net/http"
    "os"

    "github.com/casbin/casbin/v2/persist"
    fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
    "go-spring.org/spring/gs"

    StarterCasbin "go-spring.org/starter-casbin"
    _ "go-spring.org/starter-casbin"
)

// Service 纯注入消费 enforcer。bean 名来自配置组 key
// (${spring.casbin.instances.rbac.*} -> "rbac"),因此用 `autowire:"rbac"`。
// *StarterCasbin.Enforcer 内嵌 *casbin.Enforcer,Enforce/AddPolicy/...
// 用法与上游完全一致。
type Service struct {
    Enforcer *StarterCasbin.Enforcer `autowire:"rbac"`
}

func (s *Service) Allowed(sub, obj, act string) bool {
    ok, err := s.Enforcer.Enforce(sub, obj, act)
    return err == nil && ok
}

func main() {
    // 把 config 按名引用的 adapter/watcher 作为普通 bean 贡献(gs.Run 之前)。
    // starter 的 Module 会按名把它们注入 enforcer。
    gs.Provide(func() persist.Adapter { return fileadapter.NewAdapter("./conf/policy.csv") }).
        Name("file").Export(gs.As[persist.Adapter]())

    // 注册为 root 对象,容器才会实例化(没有别的注入方;
    // 仅被内层引用的 bean 在 prod 不构建)。
    svr := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

    http.HandleFunc("/enforce", func(w http.ResponseWriter, r *http.Request) {
        q := r.URL.Query()
        s := svr.Interface().(*Service)
        if s.Allowed(q.Get("sub"), q.Get("obj"), q.Get("act")) {
            _, _ = w.Write([]byte("allow"))
            return
        }
        _, _ = w.Write([]byte("deny"))
    })

    if err := http.ListenAndServe(":9090", nil); err != nil {
        os.Exit(1)
    }
}
```

**conf/app.properties**——完整注释配置面:

```properties
# 一个组 key 一个 enforcer。"rbac" 即 autowire 的 bean 名。
# model 文件(必填,无默认)。
spring.casbin.instances.rbac.model=./conf/model.conf
# 本 enforcer 用的 persist.Adapter 的 bean 名(starter.go newEnforcer 按名注入)。
# 与 `policy` 互斥——两者都设是启动错误。
spring.casbin.instances.rbac.adapter=file
# 启用热更新的 persist.Watcher 的 bean 名。设置后,对端的变更信号触发自动
# LoadPolicy(热更新/多实例同步)。
spring.casbin.instances.rbac.watcher=local
# adapter= 的免依赖替代:纯文件策略。
# spring.casbin.instances.rbac.policy=./conf/policy.csv
# AddPolicy/RemovePolicy 是否回写存储(默认 true)。
spring.casbin.instances.rbac.autoSave=true
```

**验证**(example 的 `runTest` 自断言以下全部):

```bash
cd example && ./check.sh                      # 退出码 0,打印 "hot reload applied"
go run . -manual &                            # 保持 server 存活
curl 'http://127.0.0.1:9090/enforce?sub=alice&obj=/data&act=write'   # allow
curl 'http://127.0.0.1:9090/enforce?sub=bob&obj=/data&act=write'     # deny
curl 'http://127.0.0.1:9090/enforce?sub=carol&obj=/data&act=read'    # deny
```

---

## 2. 装配与时序

### 2.1 生命周期时间线

```
func init()(用户代码,gs.Run 之前)
  └─ gs.Provide(func() persist.Adapter { return a }).Name("file").Export(gs.As[persist.Adapter]())
  └─ gs.Provide(func() persist.Watcher { return w }).Name("local").Export(gs.As[persist.Watcher]())

import starter-casbin
  └─ init():gs.Module(OnProperty("spring.casbin"), registerEnforcer)  [starter.go]

gs.Run()
  ├─ 配置绑定:每个 spring.casbin.instances.<name>.* → Config(value tag)
  ├─ 每个 key:registerEnforcer → Provide(Enforcer ctor) 命名为 <name>
  │    ctor 按 c.Adapter/c.Watcher 的 bean 名绑定 adapter、watcher
  │    (adapterArg/watcherArg:键为空时用 ValueArg 传 nil)
  ├─ bean 构造:newEnforcer(ctx, c, adapter, watcher)
  │    ├─ 有 adapter → casbin.NewEnforcer(model, adapter)
  │    ├─ 否则      → casbin.NewEnforcer(model, policy 文件)
  │    ├─ e.EnableAutoSave(c.AutoSave)
  │    └─ 有 watcher → e.SetWatcher(w) →
  │       w.SetUpdateCallback(func(string){ _ = e.LoadPolicy() })
  ├─ Run/服务:你的代码对注入的 bean 调 Enforce
  └─ 停机:destroyEnforcer → 只 close watcher
```

设计理由(引源码注释):

- **adapter/watcher 用命名 bean,而非 side-registry**:starter 刻意不带任何数据库/存储
  驱动——内置 GORM/Redis/etcd adapter 会把依赖拖进只需要文件策略的项目。应用把自己用的
  adapter 作为普通 bean 贡献;每个实例的 `gs.Module`(group 工厂无法注入额外 bean)把它
  按名解析进 enforcer。
- **`*Enforcer` 包装而非裸 `*casbin.Enforcer`**(starter.go):starter 要持有 Casbin
  自己不关闭的资源——watcher 的后台工作,在 `destroyEnforcer` 释放。内嵌让上游 API
  全量提升,调用方无感。
- **watcher 回调**(starter.go):"经典回调重新加载策略,让本实例拿到对端变更"——
  这就是热更新机制的全部;没有轮询,也不涉及 gs.Dync。

### 2.2 一次鉴权决策,逐步走读

`GET /enforce?sub=alice&obj=/data&act=write`:

1. Handler 取到注入的 `*Enforcer`(bean "rbac")。
2. `Enforce("alice", "/data", "write")` → 内嵌的 `*casbin.Enforcer` 用加载的策略矩阵
   求值 model 的 matcher(上游语义,见官方文档)。
3. 角色继承 `g, alice, admin` 命中 admin 的 `p, admin, /data, write` → **allow**。
4. 若对端实例此前 `AddPolicy` 并触发了 watcher,回调已经跑过 `LoadPolicy`,本决策
   用的是新矩阵——不重启、不改代码。

---

## 3. 逐 key 行为参考

所有 key 都在 `spring.casbin.instances.<name>.*` 下(一组一个 enforcer bean)。

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `model` | string | — | **必填**。Casbin model 文件路径,作为 `NewEnforcer` 第一个参数。 | 缺失/非法 → 构造期容器失败(`failed to create casbin enforcer`)。 |
| `policy` | string | `""` | 文件策略,仅在 `adapter` 为空时使用(`newEnforcer`)。**与 `adapter` 互斥**——两者都设启动即失败。 | 无 adapter 且为空 → enforcer 以**空策略**启动——全部 deny,无启动报错(value tag 表达不了"条件必填")。 |
| `adapter` | string | `""` | 注入进 enforcer 的 `persist.Adapter` 的 **bean 名**(经 `adapterArg` → `gs.TagArg`)。 | 未提供同名 bean → 启动即注入失败(fail-fast)。 |
| `watcher` | string | `""` | 经 `watcherArg` 按 **bean 名** 注入的 `persist.Watcher`;接 `SetWatcher` + 重载回调;停机时由 `destroyEnforcer` 关闭。 | 未提供同名 bean → 启动即注入失败。 |
| `autoSave` | bool | `true` | 传给 `e.EnableAutoSave`——`AddPolicy`/`RemovePolicy` 是否回写 adapter/策略文件。 | `false` 却指望持久化 → 重载后变更丢失。 |

⚠ 耦合:`adapter` 与 `policy` 是互斥的存储选择——两者都设即 fail-fast 启动错误
(`casbin: `policy` and `adapter` are mutually exclusive`)。
⚠ `adapter`/`watcher` 的值是 **bean 名**:按名提供同名 bean(可
`Export(gs.As[persist.Adapter]())` / `gs.As[persist.Watcher]()`),且装配期必须存在。

---

## 4. 验证与故障演练

### 4.1 鉴权决策演练(与 example `runTest` 一致)

```bash
curl '...?sub=alice&obj=/data&act=read'    # allow(admin 角色)
curl '...?sub=alice&obj=/data&act=write'   # allow
curl '...?sub=bob&obj=/data&act=read'      # allow(viewer)
curl '...?sub=bob&obj=/data&act=write'     # deny
curl '...?sub=carol&obj=/data&act=read'    # deny(未知主体)
```

### 4.2 热更新演练(watcher 路径,免重启)

1. 以 `-manual` 启动 example(其 `localWatcher` 代演分布式 watcher)。
2. 向底层存储追加授权:`echo 'g, carol, admin' >> <policyPath>`。
3. 触发 watcher——example 里是 `watcher.Update()`;Redis/etcd watcher 场景由对端
   调 `Update()` 完成。
4. 再问一次:`curl '...?sub=carol&obj=/data&act=read'` → **allow**。回调在你发问前
   跑了 `LoadPolicy`(starter.go:85)。

### 4.3 配错演练

- `adapter` 指向没有对应 bean 的名字 → 装配期注入失败(fail-fast),不会带着缺失的
  存储源启动。
- `policy` 与 `adapter` 都不设 → 正常启动;所有 Enforce 返回 deny。这就是 §6 标记的
  静默失败模式。
- 停机演练:配了 watcher 时,SIGTERM 触发 `destroyEnforcer` 关闭 watcher——用一个
  `Close()` 会打日志的 watcher 验证;无 watcher 时是空操作。

### 4.4 启动诊断

构造期一条 Debug 日志(`creating casbin enforcer model=... adapter=... watcher=...`,
tag `app-def`,starter.go:57),失败时一条 Error。无运行期日志 tag、无健康指示器、
无指标——见 §6。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败:adapter/watcher 同名 bean 找不到 | `adapter`/`watcher` 指向你没提供(或 `.Name()` 不同)的 bean | `gs.Provide(...).Name("<key值>")`,并 Export 成 `persist.Adapter`/`persist.Watcher` 接口。 |
| 启动失败:`failed to create casbin enforcer` | `model` 路径错误或 model/policy 语法非法 | 检查路径(相对进程 cwd)与 model 文件,对照官方语法文档。 |
| 全部 deny 且无报错 | `policy`、`adapter` 均空 → 加载了空策略 | 设 `policy`(或 adapter)——此场景不会 fail-fast。 |
| 改了 `policy` 文件但决策不变 | 策略只在构造期加载一次;没配 watcher | 对 bean 调 `LoadPolicy()`,或配 watcher 做热更新。 |
| 启动失败:`policy` and `adapter` are mutually exclusive | `policy` 与 `adapter` 同时设置 | 只保留一个存储源,删掉另一个 key。 |
| 多实例:一台的策略变更不传播 | watcher 回调只在变更方显式 `Update()` 时重载 | 确保变更实例在 `SavePolicy`/`AddPolicy` 后调 `watcher.Update()`(上游 watcher 语义)。 |
| `AddPolicy` 返回 ok 但重启后丢失 | `autoSave=false` | 设 `spring.casbin.instances.<name>.autoSave=true` 或显式 `SavePolicy()`。 |
| prod 里 bean 没构建,单测正常 | Service 非 root 可达(无 Export/注入方) | `gs.Provide(&S{}).Export(gs.As[gs.Rooter]())`——见 root-reachable bean 约定。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 每实例 5 个 |
| 必填 | 1(`model`;`policy` 条件必填,见 ⚠) |
| quickstart 前置外部依赖 | 0 |
| "注意/坑"条数 | 3 |

设计嫌疑(已复核):

1. `policy` 条件必填(仅无 `adapter` 时),value tag 表达不了——空策略 enforcer
   静默启动而非 fail-fast。
2. 有状态、可热更新的组件,却没有任何 observe 接线(health/metric)。

本次撰写解决:

3. ~~adapter/watcher 用与 IoC 容器平行的包级 side-registry——两套名字空间,且 group
   工厂无法把 bean 注入 adapter。~~ 已修复:starter 现在用每实例 `gs.Module` 注册
   enforcer,把 adapter/watcher 按 bean 名(`gs.TagArg`)注入;config 键为空时传 nil。
   adapter/watcher 是普通应用 bean,不再有第二套名字空间。
4. ~~`adapter` 与 `policy` 同时设置时静默覆盖 `policy`~~——已修复:两者都设现在是
   fail-fast 启动错误(`newEnforcer`)。
