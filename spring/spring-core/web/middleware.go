/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

// request_id
// echo: https://github.com/labstack/echo/blob/master/middleware/request_id.go
// gin: https://github.com/gin-contrib/requestid/blob/master/requestid.go

// rewrite
// echo: https://github.com/labstack/echo/blob/master/middleware/rewrite.go
// gin:

// basic_auth
// echo: https://github.com/labstack/echo/blob/master/middleware/basic_auth.go
// gin:

// key_auth
// echo: https://github.com/labstack/echo/blob/master/middleware/key_auth.go
// gin:

// body_dump
// echo: https://github.com/labstack/echo/blob/master/middleware/body_dump.go
// gin:

// body_limit (413 - Request Entity Too Large)
// echo: https://github.com/labstack/echo/blob/master/middleware/body_limit.go
// gin: https://github.com/gin-contrib/size/blob/master/size.go

// compress (gzip)
// echo: https://github.com/labstack/echo/blob/master/middleware/compress.go
// gin: https://github.com/gin-contrib/gzip/blob/master/gzip.go

// cors (Cross-Origin Resource Sharing)
// echo: https://github.com/labstack/echo/blob/master/middleware/cors.go
// gin: https://github.com/gin-contrib/cors/blob/master/cors.go

// csrf (Cross-Site Request Forgery)
// echo: https://github.com/labstack/echo/blob/master/middleware/csrf.go
// gin:

// secure (XSS)
// echo: https://github.com/labstack/echo/blob/master/middleware/secure.go
// gin:

// jwt (JSON Web Token)
// echo: https://github.com/labstack/echo/blob/master/middleware/jwt.go
// gin: https://github.com/appleboy/gin-jwt/blob/master/auth_jwt.go

// proxy
// echo: https://github.com/labstack/echo/blob/master/middleware/proxy.go
// gin:

// slash
// echo: https://github.com/labstack/echo/blob/master/middleware/slash.go
// gin:

// static
// echo: https://github.com/labstack/echo/blob/master/middleware/static.go
// gin: https://github.com/gin-contrib/static/blob/master/static.go

// casbin
// echo: https://github.com/labstack/echo-contrib/blob/master/casbin/casbin.go
// gin: https://github.com/gin-contrib/authz/blob/master/authz.go

// tracing
// echo: https://github.com/labstack/echo-contrib/blob/master/jaegertracing/jaegertracing.go
// gin: https://github.com/gin-contrib/opengintracing/blob/master/tracing.go

// prometheus
// echo: https://github.com/labstack/echo-contrib/blob/master/prometheus/prometheus.go
// gin: https://github.com/zsais/go-gin-prometheus/blob/master/middleware.go

// session
// echo: https://github.com/labstack/echo-contrib/blob/master/session/session.go
// gin: https://github.com/gin-contrib/sessions/blob/master/sessions.go

// pprof
// echo:
// gin: https://github.com/gin-contrib/pprof/blob/master/pprof.go

// cache
// echo:
// gin: https://github.com/gin-contrib/cache/blob/master/cache.go

// location
// echo:
// gin: https://github.com/gin-contrib/location/blob/master/location.go

// httpsign
// echo:
// gin: https://github.com/gin-contrib/httpsign/blob/master/signatureheader.go
//      https://github.com/gin-contrib/httpsign/blob/master/authenticator.go

// limiter
// echo:
// gin: https://github.com/ulule/limiter/blob/master/drivers/middleware/gin/middleware.go

// swagger
// echo: https://github.com/swaggo/echo-swagger/blob/master/swagger.go
// gin: https://github.com/swaggo/gin-swagger/blob/master/swagger.go

// oauth2
// echo:
// gin: https://github.com/zalando/gin-oauth2/blob/master/ginoauth2.go
