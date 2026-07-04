# gs init 子流程

初始化一个新的 Go-Spring 项目骨架,行为对齐 `gs/gs/cmd/init.go`。

**独立运行**:不依赖已安装的 `gs`;拉取用 `git`,代码生成直接调 `gs-http-gen`。

## 功能

- 从 `https://github.com/go-spring/go-spring.git` 稀疏拉取最新 `layout/vX.Y.Z`;
- 按用户选的 layout 变体(`mvc`/`domain`)保留对应子目录,清理其它;
- 按用户选的文档语言(`zh`/`en`)保留对应 `<stem>.<lang><ext>` 文件,清理其它;
- 替换占位符 `GS_PROJECT_MODULE`、`GS_PROJECT_NAME`、`GS_PROJECT_LAYOUT`、`GS_PROJECT_LANG`、`GS_LAYOUT_VERSION`(仅文件内容);
- 在项目目录内调用 `gs-http-gen` 生成 HTTP 派生代码。

## 工作流程

### 0. 前置代理探测(必做)

github.com 直连常不通。开始前先提醒用户:如需代理,请在**当前 shell** 预先 export,例如:

```bash
export https_proxy=http://127.0.0.1:7890
export http_proxy=http://127.0.0.1:7890
```

本 skill **不会**自动配置代理,也不会改写用户 git 配置。

然后跑一次连通性探测(≤10s 超时):

```bash
git ls-remote --heads https://github.com/go-spring/go-spring.git HEAD
```

hang 住、`Could not resolve host`、`Failed to connect`、`HTTP2 framing layer` 等任一网络错误 → 立即终止,提示「无法访问 github.com,请先配置 https_proxy/http_proxy 后重试」,**不要**继续 clone。

### 1. 收集项目信息

- **module 路径**:从用户最初的自然语言请求里解析(如「初始化 github.com/you/hello」)。缺失时才回退到 `AskUserQuestion` 补问。校验等价 `golang.org/x/mod/module.CheckPath`,不允许带主版本后缀(如 `/v2`)。项目名 = 最后一段;转 PascalCase 后必须是合法 Go 标识符且非关键字。
- **layout 变体**:用 `AskUserQuestion` 询问,选项 `mvc`(默认)/ `domain`,其它值拒绝。
- **文档语言**:用 `AskUserQuestion` 询问,选项 `zh`(默认)/ `en`,其它值拒绝。
- **目录冲突**:当前目录下已存在同名目录时直接终止(不覆盖不删除,不作为问题回问)。

### 2. 前置检查

- `git` 在 PATH 中;
- 当前目录可写;
- `gs-http-gen` 在 PATH 中,未安装时提示 `go install go-spring.org/gs-http-gen@latest`;
- **不需要** `gs`。

### 3. 解析最新 layout tag

```bash
git ls-remote --tags --refs https://github.com/go-spring/go-spring.git 'layout/v*'
```

取 `refs/tags/layout/vX.Y.Z` 中 semver 最高、非预发布的作为 `<tag>`。一个都没有时报「no layout/v* release tags found on remote」。

### 4. 稀疏 clone layout 目录

在临时目录里(stdout/stderr 直接流式接 `os.Stdout/Stderr`):

```bash
git -c advice.detachedHead=false clone \
    --filter=blob:none --sparse --depth 1 \
    --branch <tag> --single-branch \
    https://github.com/go-spring/go-spring.git
cd go-spring && git sparse-checkout set layout
```

把 `go-spring/layout` 移出到临时目录顶层,删除剩下的 `go-spring/`。

### 5. 挑选 layout 变体

递归遍历 layout:目录名形如 `<base>-mvc` / `<base>-domain` 的,命中所选变体的 rename 成 `<base>` 并记录 `<base>`,未命中的 `rm -rf`。变体目录内部不再递归(约定不嵌套变体)。其它目录继续下钻。

### 5b. 挑选文档语言变体

递归遍历 layout:**文件名**(含符号链接)形如 `<stem>.<lang><ext>` 且 `<lang>` ∈ {`zh`,`en`} 的,命中所选语言的 rename 成 `<stem><ext>`,未命中的 `rm -f`。目录继续下钻,符号链接不解引用(名字改了,target 文本保持原样;layout 里跨语言 symlink 已指向剥后名字,不会 dangling)。

### 6. 内容替换

替换表:

- `GS_PROJECT_MODULE` → module path
- `GS_PROJECT_NAME` → 项目名 PascalCase(`my-hello` → `MyHello`,`user_svc` → `UserSvc`)
- `GS_PROJECT_LAYOUT` → `mvc` 或 `domain`
- `GS_PROJECT_LANG` → `zh` 或 `en`
- `GS_LAYOUT_VERSION` → 步骤 3 解析出的 `vX.Y.Z`
- 每个记录的 `<base>`,追加 `<base>-<layout>` → `<base>`(修正内容里的相对路径引用)

规则:

- **文件名不替换**。layout 里带占位符的文件名都落在 `idl/http/proto/` 下,`gs gen` 会 `rm -rf` 后重建,重命名多此一举。
- 占位符按 **key 长度倒序**逐个应用,避免以后新增的短占位符是长占位符的前缀而破坏后者(如 `GS_PROJECT_LAYOUT` vs 假想的 `GS_PROJECT_LAYOUT_VERSION`)。
- 写回保留原 mode。

### 7. 落地项目 + 生成 HTTP 代码

把改造好的 layout 目录 rename 成 `./<项目名>`,取绝对路径 `<projectDir>`。

代码生成对齐 `gs/gs/cmd/proto/http.go:GenHttp`,**直接调 `gs-http-gen`**:

仅当 `<projectDir>/idl/http/` 存在且有 `.idl` 文件时执行(`idl/grpc`、`idl/thrift` 目前无对应生成器,跳过不报错)。

```bash
rm -rf <projectDir>/idl/http/proto && mkdir -p <projectDir>/idl/http/proto
# cwd 必须是 idl/http,gs-http-gen 从 cwd 读 .idl
cd <projectDir>/idl/http && gs-http-gen --server --output <projectDir>/idl/http/proto
```

stdout/stderr 流式输出。失败用 `errutil.Explain(err, "run gs-http-gen")` 包装,**保留** `<projectDir>` 供用户排查,不要清理。

### 8. 清理

无论成功失败都删掉步骤 4 的临时目录。

## 完成后输出

- 项目绝对路径、layout tag 与变体、文档语言;
- HTTP 代码是否已生成(以及跳过的协议目录);
- 下一步建议:`cd <项目名>` → `go mod tidy`。

## 错误分支速查

| 情况 | 响应 |
| --- | --- |
| github.com 不可达 | 「无法访问 github.com,请先配置 https_proxy/http_proxy 后重试」,立即终止 |
| `gs-http-gen` 未安装 | 「gs-http-gen not found; install via `go install go-spring.org/gs-http-gen@latest`」 |
| `-m` 为空 | 「module name is required」 |
| module path 非法 | 「invalid module path %q」 |
| module 带主版本后缀 | 「module path %q has major version suffix %q; drop it when initializing a new project」 |
| layout 非 `mvc`/`domain` | 「unknown layout %q; supported: mvc, domain」 |
| lang 非 `zh`/`en` | 「unknown lang %q; supported: zh, en」 |
| 目录已存在 | 「directory %q already exists」 |
| 无法派生合法 Go 包名 | 「cannot derive a Go package name from %q」 |
| 拉取 layout tag 失败 | 「no layout/v* release tags found on remote」或原始 git 错误 |
| `gs-http-gen` 失败 | 「run gs-http-gen: ...」,保留项目目录 |

## 关键约束

- 所有子进程调用一律**流式输出**;仅解析输出(如 `git ls-remote --tags`)时才 buffer。
- **禁止**依赖 `gs` 命令;代码生成一律走 `gs-http-gen`。
- **禁止**自动设置代理或改写用户 git 配置。
- layout 目录划分(job / mqsvr 等)是项目约定,不要反向质疑。
- 生成完成后不要主动 `go build` / `go test` / 改动生成物,除非用户明确要求。
