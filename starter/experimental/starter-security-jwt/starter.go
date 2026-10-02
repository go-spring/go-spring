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

package StarterSecurityJWT

import (
	"go-spring.org/cloud/security"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// One Authenticator bean per entry under "${spring.security.jwt.instances}",
	// named after its config sub-key (e.g. ...instances.api -> bean "api"). Each
	// is also exported as security.TokenValidator, so an application injects the
	// seam — and composes it with a server family's Authenticate middleware — by
	// name, without depending on this package's concrete types, exactly as the
	// sibling validators (oauth2-resource-server, luohua) do. Injecting
	// *Authenticator directly still works when Wrap is needed.
	//
	// An empty map registers nothing, so importing the starter without
	// configuration is inert — configuration is the enable switch.
	//
	// An authenticator holds no closable resource (the JWKS cache refreshes
	// on-demand with no background goroutine), so there is no destroy hook.
	gs.Module(gs.OnProperty("spring.security.jwt.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.security.jwt.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.security.jwt.instances", "spring.security.jwt.default")
		return conf.BindEach(p, "${spring.security.jwt.instances}", func(name string, c Config) error {
			r.Provide(newAuthenticator,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c))).
				Name(name).
				Export(gs.As[security.TokenValidator]()).
				Caller(1)
			return nil
		})
	})
}
