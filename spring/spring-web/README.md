# spring-web

### WebServer

一个 WebServer 包含多个 WebContainer。

#### 构造函数

```
func NewWebServer() *WebServer
```

#### 添加 WebContainer 实例

```
func (s *WebServer) AddContainer(container ...WebContainer) *WebServer
```

#### 返回 WebContainer 实例列表

```
func (s *WebServer) Containers() []WebContainer
```

#### 添加 WebFilter 实例

```
func (s *WebServer) AddFilter(filter ...Filter) *WebServer
```

#### 获取过滤器列表

```
func (s *WebServer) Filters() []Filter
```

#### 获取 Logger Filter

```
func (s *WebServer) GetLoggerFilter() Filter
```

#### 设置共用的日志过滤器

```
func (s *WebServer) SetLoggerFilter(filter Filter) *WebServer
```

#### 重新设置过滤器列表

```
func (s *WebServer) ResetFilters(filters []Filter)
```

#### 设置容器自身的错误回调

```
func (s *WebServer) SetErrorCallback(fn func(error)) *WebServer
```

#### 启动 Web 容器，非阻塞调用

```
func (s *WebServer) Start()
```

#### 停止 Web 容器，阻塞调用

```
func (s *WebServer) Stop(ctx context.Context)
```

### WebContainer

#### Route

返回和 Mapping 绑定的路由分组。

    Route(basePath string, filters ...Filter) *Router

#### Request

注册任意 HTTP 方法处理函数。

    Request(method uint32, path string, fn Handler, filters ...Filter) *Mapper

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

#### Mappers

返回映射器列表。

    Mappers() map[string]*Mapper

#### AddMapper

添加一个 Mapper。

    AddMapper(m *Mapper) *Mapper

#### Config

获取 Web 容器配置。

    Config() ContainerConfig

#### GetFilters

返回过滤器列表。

    GetFilters() []Filter

#### ResetFilters

重新设置过滤器列表。

    ResetFilters(filters []Filter)

#### AddFilter

添加过滤器。

    AddFilter(filter ...Filter)

#### GetLoggerFilter

获取 Logger Filter。

    GetLoggerFilter() Filter

#### SetLoggerFilter

设置 Logger Filter。

    SetLoggerFilter(filter Filter)

#### GetErrorCallback

返回容器自身的错误回调。

    GetErrorCallback() func(error)

#### SetErrorCallback

设置容器自身的错误回调。

    SetErrorCallback(fn func(error))

#### AddRouter

添加新的路由信息。

    AddRouter(router *Router)

#### EnableSwagger

是否启用 Swagger 功能。

    EnableSwagger() bool

#### SetEnableSwagger

设置是否启用 Swagger 功能。

    SetEnableSwagger(enable bool)

#### Swagger

返回和容器绑定的 Swagger 对象。

    Swagger() *Swagger

#### Start

启动 Web 容器，非阻塞。

    Start()

#### Stop

停止 Web 容器，阻塞。

    Stop(ctx context.Context)

### WebContext
    
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

implements a best effort algorithm to return the real client IP,
it parses X-Real-IP and X-Forwarded-For in order to work properly with
reverse-proxies such us: nginx or haproxy. Use X-Forwarded-For before
X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.

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

binds the request body into provided type `i`. The default binder
does it based on Content-Type header.

    Bind(i interface{}) error

#### ResponseWriter

returns `http.ResponseWriter`.

    ResponseWriter() ResponseWriter

#### Status

sets the HTTP response code.

    Status(code int)

#### Header

is a intelligent shortcut for c.Writer.Header().Set(key, value).
It writes a header in the response.
If value == "", this method removes the header `c.Writer.Header().Del(key)`

    Header(key, value string)

#### SetCookie

adds a `Set-Cookie` header in HTTP response.

    SetCookie(cookie *http.Cookie)

#### NoContent

sends a response with no body and a status code. Maybe panic.

    NoContent(code int)

#### String

writes the given string into the response body. Maybe panic.

    String(code int, format string, values ...interface{})

#### HTML

sends an HTTP response with status code. Maybe panic.

    HTML(code int, html string)

#### HTMLBlob

sends an HTTP blob response with status code. Maybe panic.

    HTMLBlob(code int, b []byte)

#### JSON

sends a JSON response with status code. Maybe panic.

    JSON(code int, i interface{})

#### JSONPretty

sends a pretty-print JSON with status code. Maybe panic.

    JSONPretty(code int, i interface{}, indent string)

#### JSONBlob

sends a JSON blob response with status code. Maybe panic.

    JSONBlob(code int, b []byte)

#### JSONP

sends a JSONP response with status code. It uses `callback`
to construct the JSONP payload. Maybe panic.

    JSONP(code int, callback string, i interface{})

#### JSONPBlob

sends a JSONP blob response with status code. It uses
`callback` to construct the JSONP payload. Maybe panic.

    JSONPBlob(code int, callback string, b []byte)

#### XML

sends an XML response with status code. Maybe panic.

    XML(code int, i interface{})

#### XMLPretty

sends a pretty-print XML with status code. Maybe panic.

    XMLPretty(code int, i interface{}, indent string)

#### XMLBlob

sends an XML blob response with status code. Maybe panic.

    XMLBlob(code int, b []byte)

#### Blob

sends a blob response with status code and content type. Maybe panic.

    Blob(code int, contentType string, b []byte)

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

    Redirect(code int, url string)

#### SSEvent

writes a Server-Sent Event into the body stream. Maybe panic.

    SSEvent(name string, message interface{})

### HandlerFunc

```
type HandlerFunc func(WebContext)
```
