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

package StarterOAuth2Server

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Register the authorization server as a single bean, gated on
	// "spring.oauth2.server.enabled=true". Unlike client starters it is not a
	// gs.Group: an application runs one authorization server, and its clients are
	// data (the ${spring.oauth2.server.clients} map), not beans.
	//
	// The bean is a Contributor: it opens no listener. The application injects
	// *AuthServer, sets UserAuthFunc, and mounts Handler() onto the HTTP server it
	// already runs (typically under an /oauth2 prefix). The signer holds no
	// closable resource and the in-memory store owns no goroutine, so no destroy
	// hook is needed.
	gs.Provide(newAuthServer, gs.TagArg("${spring.oauth2.server}")).
		Condition(gs.OnProperty("spring.oauth2.server.enabled").HavingValue("true"))
}
