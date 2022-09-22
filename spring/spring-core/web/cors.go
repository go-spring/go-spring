package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	DefaultSchemas   = []string{"http://", "https://"}
	ExtensionSchemas = []string{"chrome-extension://", "safari-extension://", "moz-extension://",
		"ms-browser-extension://",
	}
	FileSchemas = []string{"file://"}
	WSSchemas   = []string{"ws://", "wss://"}
)

// DefaultCorsConfig returns a generic default configuration mapped to localhost.
func DefaultCorsConfig() CorsConfig {
	return CorsConfig{
		AllowAllOrigins: true,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodHead, http.MethodOptions},
		AllowHeaders: []string{HeaderOrigin, HeaderContentLength, HeaderContentType,
			HeaderAccept, HeaderAcceptEncoding, HeaderAuthorization, HeaderXForwardedFor,
			HeaderXRequestedWith, HeaderXRequestID},
		AllowCredentials: false,
		MaxAge:           1 * time.Hour,
	}
}

func DefaultCorsFilter() *Prefilter {
	cors := newCors(DefaultCorsConfig())
	return FuncPrefilter(func(ctx Context, chain FilterChain) {
		applyCors(ctx, chain, cors)
	})
}

func CorsFilter(config CorsConfig) *Prefilter {
	cors := newCors(config)
	return FuncPrefilter(func(ctx Context, chain FilterChain) {
		applyCors(ctx, chain, cors)
	})
}

type CorsConfig struct {
	AllowAllOrigins  bool
	AllowOrigins     []string
	AllowOriginFunc  func(origin string) bool
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	ExposeHeaders    []string
	MaxAge           time.Duration

	// Allows to add origins like http://aaa.com/*, https://a.* or http://b.*.bbb.com
	AllowWildcard bool

	// Allows usage of popular browser extensions schemas
	AllowBrowserExtensions bool

	// Allows usage of WebSocket protocol
	AllowWebSockets bool

	// Allows usage of file:// schema (dangerous!) use it only when you 100% sure it's needed
	AllowFiles bool
}

// AddAllowMethods is allowed to add custom methods
func (c *CorsConfig) AddAllowMethods(methods ...string) {
	c.AllowMethods = append(c.AllowMethods, methods...)
}

// AddAllowHeaders is allowed to add custom headers
func (c *CorsConfig) AddAllowHeaders(headers ...string) {
	c.AllowHeaders = append(c.AllowHeaders, headers...)
}

// AddExposeHeaders is allowed to add custom expose headers
func (c *CorsConfig) AddExposeHeaders(headers ...string) {
	c.ExposeHeaders = append(c.ExposeHeaders, headers...)
}

// Validate config is it right or not
func (c CorsConfig) Validate() error {
	if c.AllowAllOrigins && (c.AllowOriginFunc != nil || len(c.AllowOrigins) > 0) {
		return errors.New("conflict settings: all origins are allowed. AllowOriginFunc or AllowOrigins is not needed")
	}
	if !c.AllowAllOrigins && c.AllowOriginFunc == nil && len(c.AllowOrigins) == 0 {
		return errors.New("conflict settings: all origins disabled")
	}
	for _, origin := range c.AllowOrigins {
		if !strings.Contains(origin, "*") && !c.validateAllowedSchemas(origin) {
			return errors.New("bad origin: origins must contain '*' or include " + strings.Join(c.getAllowedSchemas(), ","))
		}
	}
	return nil
}

func (c CorsConfig) getAllowedSchemas() []string {
	allowedSchemas := DefaultSchemas
	if c.AllowBrowserExtensions {
		allowedSchemas = append(allowedSchemas, ExtensionSchemas...)
	}
	if c.AllowWebSockets {
		allowedSchemas = append(allowedSchemas, WSSchemas...)
	}
	if c.AllowFiles {
		allowedSchemas = append(allowedSchemas, FileSchemas...)
	}
	return allowedSchemas
}

func (c CorsConfig) validateAllowedSchemas(origin string) bool {
	allowedSchemas := c.getAllowedSchemas()
	for _, schema := range allowedSchemas {
		if strings.HasPrefix(origin, schema) {
			return true
		}
	}
	return false
}

func (c CorsConfig) parseWildcardRules() [][]string {
	var wRules [][]string

	if !c.AllowWildcard {
		return wRules
	}

	for _, o := range c.AllowOrigins {
		if !strings.Contains(o, "*") {
			continue
		}

		if c := strings.Count(o, "*"); c > 1 {
			panic(errors.New("only one * is allowed").Error())
		}

		i := strings.Index(o, "*")
		if i == 0 {
			wRules = append(wRules, []string{"*", o[1:]})
			continue
		}
		if i == (len(o) - 1) {
			wRules = append(wRules, []string{o[:i-1], "*"})
			continue
		}

		wRules = append(wRules, []string{o[:i], o[i+1:]})
	}

	return wRules
}

type cors struct {
	allowAllOrigins  bool
	allowCredentials bool
	allowOriginFunc  func(string) bool
	allowOrigins     []string
	normalHeaders    http.Header
	preflightHeaders http.Header
	wildcardOrigins  [][]string
}

func newCors(config CorsConfig) *cors {
	if err := config.Validate(); err != nil {
		panic(err.Error())
	}

	for _, origin := range config.AllowOrigins {
		if origin == "*" {
			config.AllowAllOrigins = true
		}
	}

	return &cors{
		allowOriginFunc:  config.AllowOriginFunc,
		allowAllOrigins:  config.AllowAllOrigins,
		allowCredentials: config.AllowCredentials,
		allowOrigins:     normalize(config.AllowOrigins),
		normalHeaders:    generateNormalHeaders(config),
		preflightHeaders: generatePreflightHeaders(config),
		wildcardOrigins:  config.parseWildcardRules(),
	}
}

func (cors *cors) validateWildcardOrigin(origin string) bool {
	for _, w := range cors.wildcardOrigins {
		if w[0] == "*" && strings.HasSuffix(origin, w[1]) {
			return true
		}
		if w[1] == "*" && strings.HasPrefix(origin, w[0]) {
			return true
		}
		if strings.HasPrefix(origin, w[0]) && strings.HasSuffix(origin, w[1]) {
			return true
		}
	}

	return false
}

func (cors *cors) validateOrigin(origin string) bool {
	if cors.allowAllOrigins {
		return true
	}
	for _, value := range cors.allowOrigins {
		if value == origin {
			return true
		}
	}
	if len(cors.wildcardOrigins) > 0 && cors.validateWildcardOrigin(origin) {
		return true
	}
	if cors.allowOriginFunc != nil {
		return cors.allowOriginFunc(origin)
	}
	return false
}

func (cors *cors) handlePreflight(ctx Context) {
	header := ctx.Response().Header()
	for key, value := range cors.preflightHeaders {
		header.Set(key, strings.Join(value, ","))
	}
}

func (cors *cors) handleNormal(ctx Context) {
	header := ctx.Response().Header()
	for key, value := range cors.normalHeaders {
		header.Set(key, strings.Join(value, ","))
	}
}

func applyCors(ctx Context, chain FilterChain, cors *cors) {
	req := ctx.Request()

	origin := req.Header.Get(HeaderOrigin)
	if len(origin) == 0 {
		chain.Next(ctx)
		return
	}

	if origin == "http://"+req.Host || origin == "https://"+req.Host {
		chain.Next(ctx)
		return
	}

	if !cors.validateOrigin(origin) {
		ctx.SetStatus(http.StatusForbidden)
		return
	}

	if !cors.allowAllOrigins {
		ctx.Response().Header().Set(HeaderAccessControlAllowOrigin, origin)
	}

	if req.Method == http.MethodOptions {
		cors.handlePreflight(ctx)
		ctx.SetStatus(http.StatusNoContent)
		return
	}

	cors.handleNormal(ctx)
	chain.Next(ctx)
}

func normalize(values []string) []string {
	if values == nil {
		return nil
	}
	distinctMap := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		value = strings.ToLower(value)
		if _, seen := distinctMap[value]; !seen {
			normalized = append(normalized, value)
			distinctMap[value] = true
		}
	}
	return normalized
}

func generateNormalHeaders(c CorsConfig) http.Header {
	headers := make(http.Header)
	if c.AllowCredentials {
		headers.Set(HeaderAccessControlAllowCredentials, "true")
	}
	if len(c.ExposeHeaders) > 0 {
		exposeHeaders := headConvert(normalize(c.ExposeHeaders), http.CanonicalHeaderKey)
		headers.Set(HeaderAccessControlExposeHeaders, strings.Join(exposeHeaders, ","))
	}
	if c.AllowAllOrigins {
		headers.Set(HeaderAccessControlAllowOrigin, "*")
	} else {
		headers.Set(HeaderVary, HeaderOrigin)
	}
	return headers
}

func generatePreflightHeaders(c CorsConfig) http.Header {
	headers := make(http.Header)
	if c.AllowCredentials {
		headers.Set(HeaderAccessControlAllowCredentials, "true")
	}
	if len(c.AllowMethods) > 0 {
		allowMethods := headConvert(normalize(c.AllowMethods), strings.ToUpper)
		value := strings.Join(allowMethods, ",")
		headers.Set(HeaderAccessControlAllowMethods, value)
	}
	if len(c.AllowHeaders) > 0 {
		allowHeaders := headConvert(normalize(c.AllowHeaders), http.CanonicalHeaderKey)
		value := strings.Join(allowHeaders, ",")
		headers.Set(HeaderAccessControlAllowHeaders, value)
	}
	if c.MaxAge > time.Duration(0) {
		value := strconv.FormatInt(int64(c.MaxAge/time.Second), 10)
		headers.Set(HeaderAccessControlMaxAge, value)
	}
	if c.AllowAllOrigins {
		headers.Set(HeaderAccessControlAllowOrigin, "*")
	} else {
		headers.Add(HeaderVary, HeaderOrigin)
		headers.Add(HeaderVary, HeaderAccessControlRequestMethod)
		headers.Add(HeaderVary, HeaderAccessControlRequestHeaders)
	}
	return headers
}

type converter func(string) string

func headConvert(s []string, c converter) []string {
	var out []string
	for _, i := range s {
		out = append(out, c(i))
	}
	return out
}
