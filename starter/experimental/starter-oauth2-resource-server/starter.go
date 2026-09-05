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

package StarterOauth2ResourceServer

import (
	"context"

	"go-spring.org/cloud/security"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// One Validator bean per entry under spring.security.oauth2.resource.jwt,
	// named after its config sub-key and exported as security.TokenValidator,
	// so an application injects the seam (and composes it with
	// a server family's Authenticate middleware or security.Require) without depending on this package's
	// concrete types. An empty map registers nothing — configuration is the
	// enable switch.
	gs.Module(gs.OnProperty("spring.security.oauth2.resource.jwt"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.security.oauth2.resource.jwt}", func(name string, c Config) error {
			if _, err := c.source(); err != nil {
				return errutil.Explain(err, "oauth2-resource-server: instance %q", name)
			}
			log.Debugf(context.Background(), log.TagAppDef, "creating jwt validator name=%s issuer-uri=%s algorithm=%s", name, c.IssuerURI, c.Algorithm)
			r.Provide(newValidator, gs.ValueArg(c)).
				Name(name).
				Export(gs.As[security.TokenValidator]()).
				Caller(1)
			return nil
		})
	})
}
