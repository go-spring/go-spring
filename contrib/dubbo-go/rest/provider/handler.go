/*
 * Copyright 2025 The Go-Spring Authors.
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

package main

import (
	"context"

	"dubbo.apache.org/dubbo-go/v3/protocol/rest/config"
	"dubbo.apache.org/dubbo-go/v3/server"
	greet "go-spring.org/dubbo-go/rest/proto"
	"go-spring.org/spring/gs"
	StarterDubbo "go-spring.org/starter-dubbo"
)

// init installs the RestServiceConfig map that maps every exported Go method
// of GreetProvider onto a concrete (HTTP verb, URL path, param source)
// tuple. Without this map the REST protocol logs an error at Export time
// and the provider does not answer any request — this is the piece that
// makes REST different from its siblings (Triple / classic-Dubbo /
// JSON-RPC), which can be driven by method-reflection alone.
//
// The outer key ("GreetProvider") is the value dubbo-go stamps onto the
// URL as the `bean.name` parameter, derived from common.GetReference(handler)
// = the Go struct name. We pin that name to greet.BeanName so the consumer
// side does not have to guess it. The map is a process-wide singleton,
// so it must be populated before server.Serve() runs.
func init() {
	// Empty stub configs for the internal services dubbo-go always registers
	// alongside the user's provider (HealthCheckServer, ReflectionServer).
	// Without an entry the REST protocol's Export path logs
	// "%s service doesn't has provider config" and refuses to start the
	// server. We give them a bare (methodless) RestServiceConfig so Export
	// succeeds; they carry no REST routes, and we never call them.
	empty := func() *config.RestServiceConfig {
		return &config.RestServiceConfig{
			Server:               "go-restful",
			RestMethodConfigsMap: map[string]*config.RestMethodConfig{},
		}
	}
	config.SetRestProviderServiceConfigMap(map[string]*config.RestServiceConfig{
		"HealthCheckServer": empty(),
		"ReflectionServer":  empty(),
		greet.BeanName: {
			// go-restful is the only server implementation shipped with
			// dubbo-go v3; the client counterpart is resty.
			Server: "go-restful",
			// Both sides of the REST wire negotiate JSON.
			Produces: "application/json",
			Consumes: "application/json",
			RestMethodConfigsMap: map[string]*config.RestMethodConfig{
				greet.MethodGreet: {
					InterfaceName: greet.GreetServiceInterface,
					MethodName:    greet.MethodGreet,
					Path:          greet.GreetPath,
					MethodType:    greet.GreetHTTPMethod,
					Produces:      "application/json",
					Consumes:      "application/json",
					// The single string argument (index 0 in Greet's signature,
					// counting from after ctx) is carried as ?name=... on the
					// query string. Body=-1 means "no request body".
					QueryParamsMap: map[int]string{0: greet.GreetQueryName},
					Body:           -1,
				},
			},
		},
	})
}

func init() {
	// Provide a StarterDubbo.ServiceRegister bean that binds the GreetProvider
	// to the Dubbo server. starter-dubbo's SimpleDubboServer depends only on this
	// function type, so the concrete service is wired here without the server
	// ever knowing about the greet interface.
	//
	// server.WithInterface(...) publishes the provider into etcd under the
	// Java-style dotted interface name; the REST server exposes it at the
	// URL layout registered above (`GET /greet?name=...`). Cross-language
	// consumers do not need a Dubbo SDK — any HTTP client works.
	gs.Provide(func() StarterDubbo.ServiceRegister {
		return func(svr *server.Server) error {
			return svr.Register(&GreetProvider{}, nil,
				server.WithInterface(greet.GreetServiceInterface))
		}
	})
}

// GreetProvider implements the REST GreetService. dubbo-go reflects over its
// exported methods to build the invocation dispatch table; the mapping onto
// HTTP verbs and URL params is what the RestServiceConfig above pins.
type GreetProvider struct{}

// Reference pins the bean id under which the RestServiceConfig above is
// keyed. If this were omitted, dubbo-go would fall back to the Go struct
// name ("GreetProvider"), which happens to match — but we make it explicit
// so the coupling is visible from a single place (greet.BeanName).
func (s *GreetProvider) Reference() string {
	return greet.BeanName
}

// Greet echoes the request name back, giving the consumer a deterministic
// value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, name string) (string, error) {
	return name, nil
}
