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
	"go-spring.org/spring/gs"
)

func init() {
	// Register multiple JWT authenticators as a group, one per entry under
	// "${spring.security.jwt}". Each bean is named after its config sub-key
	// (e.g. spring.security.jwt.api -> bean "api"), so an application selects an
	// authenticator with gs.TagArg("api") when wiring its *gs.HttpServeMux via
	// Wrap, or injects it as a security.TokenValidator for non-HTTP transports.
	//
	// An empty "${spring.security.jwt}" map registers nothing, so importing the
	// starter without configuration is inert — configuration is the enable switch.
	//
	// An authenticator holds no closable resource (the JWKS cache refreshes
	// on-demand with no background goroutine), so the destroy hook is nil.
	gs.Group("${spring.security.jwt}", newAuthenticator, nil)
}
