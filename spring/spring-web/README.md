# spring-web

为社区优秀的 Web 服务器提供一个抽象层，通过定义 `WebContainer`、`WebContext`、`Filter` 三大基本组件，使得底层实现可以灵活切换。

- [WebContainer](#webcontainer)
    - [Route](#route)
    - [HandleRequest](#handlerequest)
    - [RequestMapping](#requestmapping)
    - [RequestBinding](#requestbinding)
    - [HandleGet](#handleget)
    - [GetMapping](#getmapping)
    - [GetBinding](#getbinding)
    - [HandlePost](#handlepost)
    - [PostMapping](#postmapping)
    - [PostBinding](#postbinding)
    - [HandlePut](#handleput)
    - [PutMapping](#putmapping)
    - [PutBinding](#putbinding)
    - [HandleDelete](#handledelete)
    - [DeleteMapping](#deletemapping)
    - [DeleteBinding](#deletebinding)
    - [AddFilter](#addfilter)
    - [SetLoggerFilter](#setloggerfilter)
    - [AddRouter](#addrouter)
    - [Swagger](#swagger)
    - [Start](#start)
    - [Stop](#stop)
- [WebContext](#webcontext)
    - [NativeContext](#nativecontext)
    - [Get](#get)
    - [Set](#set)
    - [Request](#request)
    - [SetRequest](#setrequest)
    - [Context](#context)
    - [IsTLS](#istls)
    - [IsWebSocket](#iswebsocket)
    - [Scheme](#scheme)
    - [ClientIP](#clientip)
    - [Path](#path)
    - [Handler](#handler)
    - [ContentType](#contenttype)
    - [GetHeader](#getheader)
    - [GetRawData](#getrawdata)
    - [PathParam](#pathparam)
    - [PathParamNames](#pathparamnames)
    - [PathParamValues](#pathparamvalues)
    - [QueryParam](#queryparam)
    - [QueryParams](#queryparams)
    - [QueryString](#querystring)
    - [FormValue](#formvalue)
    - [FormParams](#formparams)
    - [FormFile](#formfile)
    - [SaveUploadedFile](#saveuploadedfile)
    - [MultipartForm](#multipartform)
    - [Cookie](#cookie)
    - [Cookies](#cookies)
    - [Bind](#bind)
    - [ResponseWriter](#responsewriter)
    - [Status](#status)
    - [Header](#header)
    - [SetCookie](#setcookie)
    - [NoContent](#nocontent)
    - [String](#string)
    - [HTML](#html)
    - [HTMLBlob](#htmlblob)
    - [JSON](#json)
    - [JSONPretty](#jsonpretty)
    - [JSONBlob](#jsonblob)
    - [JSONP](#jsonp)
    - [JSONPBlob](#jsonpblob)
    - [XML](#xml)
    - [XMLPretty](#xmlpretty)
    - [XMLBlob](#xmlblob)
    - [Blob](#blob)
    - [File](#file)
    - [Attachment](#attachment)
    - [Inline](#inline)
    - [Redirect](#redirect)
    - [SSEvent](#ssevent)
- [Handler](#handler-1)
    - [FUNC](#func)
    - [HTTP](#http)
    - [WrapF](#wrapf)
    - [WrapH](#wraph)
    - [BIND](#bind-1)
- [路由风格](#路由风格)
- [全局变量](#全局变量)
    - [ErrorHandler](#errorhandler)
    - [LoggerFilter](#loggerfilter)
    - [Validator](#validator)

### WebContainer

定义一个 Web 服务器，具有注册路由、设置中间件、注册 Swagger 响应器等功能。

#### Route

返回和 Mapping 绑定的路由分组。

    Route(basePath string, filters ...Filter) *Router

#### HandleRequest

注册任意 HTTP 方法处理函数。

    HandleRequest(method uint32, path string, fn Handler, filters ...Filter) *Mapper

#### RequestMapping

注册任意 HTTP 方法处理函数。

    RequestMapping(method uint32, path string, fn HandlerFunc, filters ...Filter) *Mapper

#### RequestBinding

注册任意 HTTP 方法处理函数。

    RequestBinding(method uint32, path string, fn interface{}, filters ...Filter) *Mapper

#### HandleGet

注册 GET 方法处理函数。

    HandleGet(path string, fn Handler, filters ...Filter) *Mapper

#### GetMapping

注册 GET 方法处理函数。

    GetMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

#### GetBinding

注册 GET 方法处理函数。

    GetBinding(path string, fn interface{}, filters ...Filter) *Mapper

#### HandlePost

注册 POST 方法处理函数。

    HandlePost(path string, fn Handler, filters ...Filter) *Mapper

#### PostMapping

注册 POST 方法处理函数。

    PostMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

#### PostBinding

注册 POST 方法处理函数。

    PostBinding(path string, fn interface{}, filters ...Filter) *Mapper

#### HandlePut

注册 PUT 方法处理函数。

    HandlePut(path string, fn Handler, filters ...Filter) *Mapper

#### PutMapping

注册 PUT 方法处理函数。

    PutMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

#### PutBinding

注册 PUT 方法处理函数。

    PutBinding(path string, fn interface{}, filters ...Filter) *Mapper

#### HandleDelete

注册 DELETE 方法处理函数。

    HandleDelete(path string, fn Handler, filters ...Filter) *Mapper

#### DeleteMapping

注册 DELETE 方法处理函数。

    DeleteMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

#### DeleteBinding

注册 DELETE 方法处理函数。

    DeleteBinding(path string, fn interface{}, filters ...Filter) *Mapper

#### AddFilter

添加过滤器。

    AddFilter(filter ...Filter)

#### SetLoggerFilter

设置 Logger Filter。

    SetLoggerFilter(filter Filter)

#### AddRouter

添加新的路由信息。

    AddRouter(router RootRouter)

#### Swagger

返回和容器绑定的 Swagger 对象。

    Swagger() *Swagger

#### Start

启动 Web 容器。

    Start() error

#### Stop

停止 Web 容器。

    Stop(ctx context.Context) error

### WebContext

封装 *http.Request 和 http.ResponseWriter 对象，简化操作接口。

#### NativeContext

返回封装的底层上下文对象。

    NativeContext() interface{}

#### Get

retrieves data from the context.

    Get(key string) interface{}

#### Set

saves data in the context.

    Set(key string, val interface{})

#### Request

returns `*http.Request`.

    Request() *http.Request

#### SetRequest

sets `*http.Request`.

    SetRequest(r *http.Request)

#### Context

返回 Request 绑定的 context.Context 对象。

	Context() context.Context

#### IsTLS

returns true if HTTP connection is TLS otherwise false.

    IsTLS() bool

#### IsWebSocket

returns true if HTTP connection is WebSocket otherwise false.

    IsWebSocket() bool

#### Scheme

returns the HTTP protocol scheme, `http` or `https`.

    Scheme() string

#### ClientIP

implements a best effort algorithm to return the real client IP, it parses X-Real-IP and X-Forwarded-For in order to
work properly with reverse-proxies such us: nginx or haproxy. Use X-Forwarded-For before X-Real-Ip as nginx uses
X-Real-Ip with the proxy's IP.

    ClientIP() string

#### Path

returns the registered path for the handler.

    Path() string

#### Handler

returns the matched handler by router.

    Handler() Handler

#### ContentType

returns the Content-Type header of the request.

    ContentType() string

#### GetHeader

returns value from request headers.

    GetHeader(key string) string

#### GetRawData

return stream data.

    GetRawData() ([]byte, error)

#### PathParam

returns path parameter by name.

    PathParam(name string) string

#### PathParamNames

returns path parameter names.

    PathParamNames() []string

#### PathParamValues

returns path parameter values.

    PathParamValues() []string

#### QueryParam

returns the query param for the provided name.

    QueryParam(name string) string

#### QueryParams

returns the query parameters as `url.Values`.

    QueryParams() url.Values

#### QueryString

returns the URL query string.

    QueryString() string

#### FormValue

returns the form field value for the provided name.

    FormValue(name string) string

#### FormParams

returns the form parameters as `url.Values`.

    FormParams() (url.Values, error)

#### FormFile

returns the multipart form file for the provided name.

    FormFile(name string) (*multipart.FileHeader, error)

#### SaveUploadedFile

uploads the form file to specific dst.

    SaveUploadedFile(file *multipart.FileHeader, dst string) error

#### MultipartForm

returns the multipart form.

    MultipartForm() (*multipart.Form, error)

#### Cookie

returns the named cookie provided in the request.

    Cookie(name string) (*http.Cookie, error)

#### Cookies

returns the HTTP cookies sent with the request.

    Cookies() []*http.Cookie

#### Bind

binds the request body into provided type `i`. The default binder does it based on Content-Type header.

    Bind(i interface{}) error

#### ResponseWriter

returns `http.ResponseWriter`.

    ResponseWriter() ResponseWriter

#### Status

sets the HTTP response code.

    Status(code int)

#### Header

is a intelligent shortcut for c.Writer.Header().Set(key, value). It writes a header in the response. If value == "",
this method removes the header `c.Writer.Header().Del(key)`

    Header(key, value string)

#### SetCookie

adds a `Set-Cookie` header in HTTP response.

    SetCookie(cookie *http.Cookie)

#### NoContent

sends a response with no body and a status code. Maybe panic.

    NoContent()

#### String

writes the given string into the response body. Maybe panic.

    String(format string, values ...interface{})

#### HTML

sends an HTTP response with status code. Maybe panic.

    HTML(html string)

#### HTMLBlob

sends an HTTP blob response with status code. Maybe panic.

    HTMLBlob(b []byte)

#### JSON

sends a JSON response with status code. Maybe panic.

    JSON(i interface{})

#### JSONPretty

sends a pretty-print JSON with status code. Maybe panic.

    JSONPretty(i interface{}, indent string)

#### JSONBlob

sends a JSON blob response with status code. Maybe panic.

    JSONBlob(b []byte)

#### JSONP

sends a JSONP response with status code. It uses `callback`
to construct the JSONP payload. Maybe panic.

    JSONP(callback string, i interface{})

#### JSONPBlob

sends a JSONP blob response with status code. It uses
`callback` to construct the JSONP payload. Maybe panic.

    JSONPBlob(callback string, b []byte)

#### XML

sends an XML response with status code. Maybe panic.

    XML(i interface{})

#### XMLPretty

sends a pretty-print XML with status code. Maybe panic.

    XMLPretty(i interface{}, indent string)

#### XMLBlob

sends an XML blob response with status code. Maybe panic.

    XMLBlob(b []byte)

#### Blob

sends a blob response with status code and content type. Maybe panic.

    Blob(contentType string, b []byte)

#### File

sends a response with the content of the file. Maybe panic.

    File(file string)

#### Attachment

sends a response as attachment, prompting client to save the file. Maybe panic.

    Attachment(file string, name string)

#### Inline

sends a response as inline, opening the file in the browser. Maybe panic.

    Inline(file string, name string)

#### Redirect

redirects the request to a provided URL with status code. Maybe panic.

    Redirect(url string)

#### SSEvent

writes a Server-Sent Event into the body stream. Maybe panic.

    SSEvent(name string, message interface{})

### Handler

以函数的方式实现 Handler。

#### FUNC

标准 Web 处理函数的辅助函数。

    func FUNC(fn HandlerFunc) Handler

#### HTTP

标准 Http 处理函数的辅助函数。

    func HTTP(fn http.HandlerFunc) Handler

#### WrapF

标准 Http 处理函数的辅助函数。

    func WrapF(fn http.HandlerFunc) Handler

#### WrapH

标准 Http 处理函数的辅助函数。

    func WrapH(h http.Handler) Handler

#### BIND

转换成 BIND 形式的 Web 处理接口。

    func BIND(fn interface{}) Handler

### 路由风格

提供 echo、gin 和 {} 三种可无缝切换的路由风格：`/a/:b/c/:d/*` 是 echo 风格；`/a/:b/c/:d/*e` 是 gin 风格；`/a/{b}/c/{e:*}` 、`/a/{b}/c/{*:e}`
、`/a/{b}/c/{*}` 是 {} 风格。

### 全局变量

#### Validator

全局参数校验器。

#### ErrorHandler

自定义错误处理函数。

#### LoggerFilter

全局的日志过滤器，Container 如果没有设置日志过滤器则会使用全局的日志过滤器。