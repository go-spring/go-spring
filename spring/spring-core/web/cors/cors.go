package cors

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-spring/spring-core/web"
)

type Options struct {
	AllowedOrigins         []string
	AllowOriginFunc        func(origin string) bool
	AllowOriginRequestFunc func(r *http.Request, origin string) bool
	AllowedMethods         []string
	AllowedHeaders         []string
	ExposedHeaders         []string
	MaxAge                 int
	AllowCredentials       bool
	AllowPrivateNetwork    bool
	OptionsPassThrough     bool
	OptionsSuccessStatus   int
	Debug                  bool
}

type Logger interface {
	Printf(string, ...interface{})
}

type cors struct {
	Log                    Logger
	allowedOrigins         []string
	allowedWOrigins        []wildcard
	allowOriginFunc        func(origin string) bool
	allowOriginRequestFunc func(r *http.Request, origin string) bool
	allowedHeaders         []string
	allowedMethods         []string
	exposedHeaders         []string
	maxAge                 int
	allowedOriginsAll      bool
	allowedHeadersAll      bool
	optionsSuccessStatus   int
	allowCredentials       bool
	allowPrivateNetwork    bool
	optionPassThrough      bool
}

func DefaultPreFilter() *web.Prefilter {
	return web.NewPrefilter(filter(allowAllConfig()))
}

// PreFilter cors must be handled before routing
func PreFilter(options Options) *web.Prefilter {
	return web.NewPrefilter(filter(options))
}

func newCors(options Options) *cors {
	c := &cors{
		exposedHeaders:         convert(options.ExposedHeaders, http.CanonicalHeaderKey),
		allowOriginFunc:        options.AllowOriginFunc,
		allowOriginRequestFunc: options.AllowOriginRequestFunc,
		allowCredentials:       options.AllowCredentials,
		allowPrivateNetwork:    options.AllowPrivateNetwork,
		maxAge:                 options.MaxAge,
		optionPassThrough:      options.OptionsPassThrough,
	}
	if options.Debug && c.Log == nil {
		c.Log = log.New(os.Stdout, "[cors] ", log.LstdFlags)
	}

	// Allowed Origins
	if len(options.AllowedOrigins) == 0 {
		if options.AllowOriginFunc == nil && options.AllowOriginRequestFunc == nil {
			c.allowedOriginsAll = true
		}
	} else {
		c.allowedOrigins = []string{}
		c.allowedWOrigins = []wildcard{}
		for _, origin := range options.AllowedOrigins {
			origin = strings.ToLower(origin)
			if origin == "*" {
				c.allowedOriginsAll = true
				c.allowedOrigins = nil
				c.allowedWOrigins = nil
				break
			} else if i := strings.IndexByte(origin, '*'); i >= 0 {
				w := wildcard{origin[0:i], origin[i+1:]}
				c.allowedWOrigins = append(c.allowedWOrigins, w)
			} else {
				c.allowedOrigins = append(c.allowedOrigins, origin)
			}
		}
	}

	// Allowed Headers
	if len(options.AllowedHeaders) == 0 {
		c.allowedHeaders = []string{"Origin", "Accept", "Content-Type", "X-Requested-With"}
	} else {
		c.allowedHeaders = convert(append(options.AllowedHeaders, "Origin"), http.CanonicalHeaderKey)
		for _, h := range options.AllowedHeaders {
			if h == "*" {
				c.allowedHeadersAll = true
				c.allowedHeaders = nil
				break
			}
		}
	}

	// Allowed Methods
	if len(options.AllowedMethods) == 0 {
		c.allowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead}
	} else {
		c.allowedMethods = convert(options.AllowedMethods, strings.ToUpper)
	}

	// Options Success Status Code
	if options.OptionsSuccessStatus == 0 {
		c.optionsSuccessStatus = http.StatusNoContent
	} else {
		c.optionsSuccessStatus = options.OptionsSuccessStatus
	}

	return c
}

func filter(options Options) web.Filter {
	cors := newCors(options)

	return web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		req := ctx.Request()
		w := ctx.ResponseWriter()

		if req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != "" {
			cors.logf("ServeHTTP: Preflight request")
			cors.handlePreflight(w, req)
			if cors.optionPassThrough {
				chain.Next(ctx)
			} else {
				w.WriteHeader(cors.optionsSuccessStatus)
			}
		} else {
			cors.logf("ServeHTTP: Actual request")
			cors.handleActualRequest(w, req)
			chain.Next(ctx)
		}
	})
}

func allowAllConfig() Options {
	return Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowCredentials: false,
	}
}

func (c *cors) handlePreflight(w http.ResponseWriter, r *http.Request) {
	headers := w.Header()
	origin := r.Header.Get("Origin")

	if r.Method != http.MethodOptions {
		c.logf("Preflight aborted: %s!=OPTIONS", r.Method)
		return
	}

	headers.Add("Vary", "Origin")
	headers.Add("Vary", "Access-Control-Request-Method")
	headers.Add("Vary", "Access-Control-Request-Headers")

	if origin == "" {
		c.logf("Preflight aborted: empty origin")
		return
	}
	if !c.isOriginAllowed(r, origin) {
		c.logf("Preflight aborted: origin '%s' not allowed", origin)
		return
	}

	reqMethod := r.Header.Get("Access-Control-Request-Method")
	if !c.isMethodAllowed(reqMethod) {
		c.logf("Preflight aborted: method '%s' not allowed", reqMethod)
		return
	}

	reqHeaders := parseHeaderList(r.Header.Get("Access-Control-Request-Headers"))
	if !c.areHeadersAllowed(reqHeaders) {
		c.logf("Preflight aborted: headers '%v' not allowed", reqHeaders)
		return
	}

	if c.allowedOriginsAll {
		headers.Set("Access-Control-Allow-Origin", "*")
	} else {
		headers.Set("Access-Control-Allow-Origin", origin)
	}

	headers.Set("Access-Control-Allow-Methods", strings.ToUpper(reqMethod))

	if len(reqHeaders) > 0 {
		headers.Set("Access-Control-Allow-Headers", strings.Join(reqHeaders, ", "))
	}
	if c.allowCredentials {
		headers.Set("Access-Control-Allow-Credentials", "true")
	}
	if c.allowPrivateNetwork && r.Header.Get("Access-Control-Request-Private-Network") == "true" {
		headers.Set("Access-Control-Allow-Private-Network", "true")
	}
	if c.maxAge > 0 {
		headers.Set("Access-Control-Max-Age", strconv.Itoa(c.maxAge))
	}

	c.logf("Preflight response headers: %v", headers)
}

func (c *cors) handleActualRequest(w http.ResponseWriter, req *http.Request) {
	headers := w.Header()
	origin := req.Header.Get("Origin")
	if origin == "" {
		c.logf("Actual request no headers added: missing origin")
		return
	}

	headers.Add("Vary", "Origin")

	if !c.isOriginAllowed(req, origin) {
		c.logf("Actual request no headers added: origin '%s' not allowed", origin)
		return
	}

	if !c.isMethodAllowed(req.Method) {
		c.logf("Actual request no headers added: method '%s' not allowed", req.Method)
		return
	}

	if c.allowedOriginsAll {
		headers.Set("Access-Control-Allow-Origin", "*")
	} else {
		headers.Set("Access-Control-Allow-Origin", origin)
	}

	if len(c.exposedHeaders) > 0 {
		headers.Set("Access-Control-Expose-Headers", strings.Join(c.exposedHeaders, ", "))
	}

	if c.allowCredentials {
		headers.Set("Access-Control-Allow-Credentials", "true")
	}

	c.logf("  Actual response added headers: %v", headers)
}

// logf
func (c *cors) logf(format string, a ...interface{}) {
	if c.Log != nil {
		c.Log.Printf(format, a...)
	}
}

func (c *cors) isOriginAllowed(r *http.Request, origin string) bool {
	if c.allowedOriginsAll {
		return true
	}

	if c.allowOriginRequestFunc != nil {
		return c.allowOriginRequestFunc(r, origin)
	}

	if c.allowOriginFunc != nil {
		return c.allowOriginFunc(origin)
	}

	origin = strings.ToLower(origin)
	for _, o := range c.allowedOrigins {
		if o == origin {
			return true
		}
	}

	for _, w := range c.allowedWOrigins {
		if w.match(origin) {
			return true
		}
	}
	return false
}

func (c *cors) isMethodAllowed(method string) bool {
	if len(c.allowedMethods) == 0 {
		return false
	}
	method = strings.ToUpper(method)
	if method == http.MethodOptions {
		return true
	}
	for _, m := range c.allowedMethods {
		if m == method {
			return true
		}
	}
	return false
}

// areHeadersAllowed checks if a given list of headers are allowed to used within
// a cross-domain request.
func (c *cors) areHeadersAllowed(requestedHeaders []string) bool {
	if c.allowedHeadersAll || len(requestedHeaders) == 0 {
		return true
	}
	for _, header := range requestedHeaders {
		header = http.CanonicalHeaderKey(header)
		found := false
		for _, h := range c.allowedHeaders {
			if h == header {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
