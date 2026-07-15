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

// Package handler holds the HTTP entry-point functions that routes.go
// references. routes.go is regenerated from greet.api on every scripts/gen-code.sh run
// and expects a symbol per @handler declared in the IDL; the entry-point
// functions here parse the request, delegate to the Go-Spring-owned logic
// bean exposed via ServiceContext, and render the response.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"greetapi/internal/svc"
	"greetapi/internal/types"
)

// GreetHandler is invoked once per request from the route table in
// routes.go. It intentionally mirrors the shape of what goctl would have
// scaffolded so re-generating the routes file needs no follow-up edits — the
// only substantive change is calling svcCtx.Logic.Greet directly instead of
// constructing a fresh logic.NewGreetLogic per request.
func GreetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GreetReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := svcCtx.Logic.Greet(r.Context(), &req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
