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

package secure

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-spring/spring-core/web"
)

const (
	stsHeader           = "Strict-Transport-Security"
	stsSubdomainString  = "; includeSubdomains"
	frameOptionsHeader  = "X-Frame-Options"
	frameOptionsValue   = "DENY"
	contentTypeHeader   = "X-Content-Type-Options"
	contentTypeValue    = "nosniff"
	xssProtectionHeader = "X-XSS-Protection"
	xssProtectionValue  = "1; mode=block"
	cspHeader           = "Content-Security-Policy"
)

func defaultBadHostHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Bad Host", http.StatusInternalServerError)
}

type Options struct {
	// AllowedHosts is a list of fully qualified domain names that are allowed
	AllowedHosts []string
	// SSLRedirect set to true, then only allow https requests. Default is false
	SSLRedirect bool
	// SSLTemporaryRedirect set to true, 302 will be used while redirecting. Default is false (301)
	SSLTemporaryRedirect bool
	// SSLHost is the host name that is used to redirect http requests to https
	SSLHost string
	// SSLProxyHeaders is set of header keys with associated values that would indicate a valid https request
	SSLProxyHeaders map[string]string
	// STSSeconds is the max-age of the Strict-Transport-Security header
	STSSeconds int64
	// STSIncludeSubdomains set to true, `includeSubdomains` will be appended to the Strict-Transport-Security header
	STSIncludeSubdomains bool
	// FrameDeny set to true, adds the X-Frame-Options header with the value of `DENY`
	FrameDeny bool
	// CustomFrameOptionsValue allows the X-Frame-Options header value to be set with a custom value.
	// this overrides the FrameDeny option.
	CustomFrameOptionsValue string
	// ContentTypeNosniff set rto true, adds the X-Content-Type-Options header with the value `nosniff`
	ContentTypeNosniff bool
	// BrowserXssFilter set to true, adds the X-XSS-Protection header with the value `1; mode=block`
	BrowserXssFilter bool
	// ContentSecurityPolicy allows the Content-Security-Policy header value to be set with a custom value
	ContentSecurityPolicy string
	// BadHostHandler when an error occurs
	BadHostHandler http.Handler
	IsDevelopment  bool
}

type secure struct {
	opt Options
	Log Logger
}

type Logger interface {
	Printf(string, ...interface{})
}

func PreFilter(o Options) *web.Prefilter {
	return web.NewPrefilter(Filter(o))
}

func Filter(o Options) web.Filter {
	s := newSecure(o)

	return web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		err := s.process(ctx.ResponseWriter(), ctx.Request())
		if err != nil {
			s.Log.Printf("error: %s", err.Error())
			return
		}

		chain.Next(ctx)
	})
}

func newSecure(options Options) *secure {
	s := secure{}

	if options.BadHostHandler == nil {
		options.BadHostHandler = http.HandlerFunc(defaultBadHostHandler)
	}

	s.opt = options

	if options.IsDevelopment {
		s.Log = log.New(os.Stdout, "[secure] ", log.LstdFlags)
	}

	return &s
}

func (s *secure) process(w http.ResponseWriter, r *http.Request) error {
	if len(s.opt.AllowedHosts) > 0 && !s.opt.IsDevelopment {
		isGoodHost := false
		for _, allowedHost := range s.opt.AllowedHosts {
			if strings.EqualFold(allowedHost, r.Host) {
				isGoodHost = true
				break
			}
		}

		if !isGoodHost {
			s.opt.BadHostHandler.ServeHTTP(w, r)
			return errors.New("bad host name: " + r.Host)
		}
	}

	if s.opt.SSLRedirect && !s.opt.IsDevelopment {
		isSSL := false
		if strings.EqualFold(r.URL.Scheme, "https") || r.TLS != nil {
			isSSL = true
		} else {
			for k, v := range s.opt.SSLProxyHeaders {
				if r.Header.Get(k) == v {
					isSSL = true
					break
				}
			}
		}

		if !isSSL {
			url := r.URL
			url.Scheme = "https"
			url.Host = r.Host

			if len(s.opt.SSLHost) > 0 {
				url.Host = s.opt.SSLHost
			}

			status := http.StatusMovedPermanently
			if s.opt.SSLTemporaryRedirect {
				status = http.StatusTemporaryRedirect
			}

			http.Redirect(w, r, url.String(), status)
			return errors.New("redirecting to HTTPS")
		}
	}

	// Strict Transport Security header.
	if s.opt.STSSeconds != 0 && !s.opt.IsDevelopment {
		stsSub := ""
		if s.opt.STSIncludeSubdomains {
			stsSub = stsSubdomainString
		}

		w.Header().Add(stsHeader, fmt.Sprintf("max-age=%d%s", s.opt.STSSeconds, stsSub))
	}

	// Frame Options header.
	if len(s.opt.CustomFrameOptionsValue) > 0 {
		w.Header().Add(frameOptionsHeader, s.opt.CustomFrameOptionsValue)
	} else if s.opt.FrameDeny {
		w.Header().Add(frameOptionsHeader, frameOptionsValue)
	}

	if s.opt.ContentTypeNosniff {
		w.Header().Add(contentTypeHeader, contentTypeValue)
	}

	if s.opt.BrowserXssFilter {
		w.Header().Add(xssProtectionHeader, xssProtectionValue)
	}

	if len(s.opt.ContentSecurityPolicy) > 0 {
		w.Header().Add(cspHeader, s.opt.ContentSecurityPolicy)
	}

	if s.opt.IsDevelopment {
		s.Log.Printf("req head: %v", w.Header())
	}

	return nil
}
