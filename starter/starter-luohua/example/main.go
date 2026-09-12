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

// Command example runs a minimal go-spring service blank-importing
// starter-luohua: luohua provides the company SSO validator, error catalog and
// log baseline, and the app wires routes on top exactly as with any starter.
package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go-spring.org/cloud/security"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/i18n"

	StarterGin "go-spring.org/starter-gin"
	luohua "go-spring.org/starter-luohua"
)

// Controller holds the luohua defaults the container wired (security.TokenValidator
// from luohua.identity, its concrete *LuohuaSSO to mint tokens, and the luohua
// error catalog). Any bean may be replaced by the app — luohua never forces one.
type Controller struct {
	Validator security.TokenValidator `autowire:""`
	SSO       *luohua.LuohuaSSO       `autowire:""`
	Messages  i18n.MessageSource      `autowire:""`
}

func (c *Controller) Router() StarterGin.RouterRegister {
	return func(e *gin.Engine) {
		// Open endpoint: mint a luohua token for the demo.
		e.GET("/token", func(ctx *gin.Context) {
			tok, err := c.SSO.Issue("demo-user", "acme", []string{luohua.AuthorityOrdersRead}, time.Hour)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			ctx.JSON(http.StatusOK, gin.H{"token": tok})
		})

		// Protected group: luohua's validator + the standard Authorize shell.
		auth := e.Group("/api", StarterGin.Authenticate(c.Validator, true),
			StarterGin.Authorize(luohua.AuthorityOrdersRead))
		auth.GET("/orders", func(ctx *gin.Context) {
			id, _ := security.FromContext(ctx.Request.Context())
			subject := ""
			if id != nil {
				subject = id.Principal.Subject
			}
			msg, _ := c.Messages.Message(ctx.Request.Context(), "luohua.orders.denied")
			ctx.JSON(http.StatusOK, gin.H{"subject": subject, "note": msg})
		})
	}
}

func init() {
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) StarterGin.RouterRegister { return c.Router() })
}

func main() {
	gs.Web(true).Configure(func(app gs.App) {
		app.Property("spring.gin.server.addr", "127.0.0.1:18080")
		app.Property("spring.luohua.identity.secret", "example-secret")
		app.Property("spring.luohua.identity.issuer", "luohua")
		app.Property("spring.luohua.i18n.default-locale", "zh")
	}).Run()
}
