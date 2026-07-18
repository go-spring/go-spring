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

package StarterLuaFilter

import (
	"net/http"
	"os"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Filter is a Lua-script-backed HTTP middleware. The script is compiled once at
// startup; each request borrows a sandboxed *lua.LState from a pool, runs the
// script, and can inspect the request, mutate response headers, short-circuit
// with deny(), or simply fall through to the wrapped handler.
type Filter struct {
	name  string
	proto *lua.FunctionProto
	pool  sync.Pool
}

// newFilter compiles the Lua script referenced by the config into a reusable
// function prototype and prepares a pool of sandboxed VMs.
func newFilter(c Config) (*Filter, error) {
	src, err := os.ReadFile(c.Script)
	if err != nil {
		return nil, errutil.Explain(err, "lua filter: read script %s", c.Script)
	}
	proto, err := compile(string(src), c.Script)
	if err != nil {
		return nil, errutil.Explain(err, "lua filter: compile script %s", c.Script)
	}
	f := &Filter{name: c.Script, proto: proto}
	f.pool.New = func() any { return newSandbox() }
	return f, nil
}

// Wrap returns an http.Handler that runs the Lua script before delegating to
// next. This is the seam: callers hand the wrapped handler to a
// *gs.HttpServeMux, so the filter sits in front of any framework engine.
func (f *Filter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		L := f.pool.Get().(*lua.LState)
		defer func() {
			L.SetTop(0)
			f.pool.Put(L)
		}()

		st := &reqState{r: r, w: w}
		f.install(L, st)

		L.Push(L.NewFunctionFromProto(f.proto))
		if err := L.PCall(0, 0, nil); err != nil {
			http.Error(w, "lua filter error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if st.denied {
			return // deny() already wrote the response
		}
		next.ServeHTTP(w, r)
	})
}

// reqState carries the per-request bridge between Go and the Lua VM.
type reqState struct {
	r      *http.Request
	w      http.ResponseWriter
	denied bool
}

// install registers the host API as globals bound to this request's state.
// Re-binding per request keeps the pooled VM stateless between calls.
func (f *Filter) install(L *lua.LState, st *reqState) {
	req := L.NewTable()
	L.SetField(req, "method", lua.LString(st.r.Method))
	L.SetField(req, "path", lua.LString(st.r.URL.Path))
	L.SetField(req, "header", L.NewFunction(func(l *lua.LState) int {
		l.Push(lua.LString(st.r.Header.Get(l.CheckString(1))))
		return 1
	}))
	L.SetField(req, "query", L.NewFunction(func(l *lua.LState) int {
		l.Push(lua.LString(st.r.URL.Query().Get(l.CheckString(1))))
		return 1
	}))
	L.SetGlobal("req", req)

	resp := L.NewTable()
	L.SetField(resp, "set_header", L.NewFunction(func(l *lua.LState) int {
		st.w.Header().Set(l.CheckString(1), l.CheckString(2))
		return 0
	}))
	L.SetGlobal("resp", resp)

	// deny(status, message) short-circuits the chain. Scripts should return
	// immediately after calling it.
	L.SetGlobal("deny", L.NewFunction(func(l *lua.LState) int {
		status := l.OptInt(1, http.StatusForbidden)
		msg := l.OptString(2, "denied by lua filter")
		st.denied = true
		st.w.WriteHeader(status)
		_, _ = st.w.Write([]byte(msg))
		return 0
	}))

	// log(msg) bridges script logging into the go-spring log pipeline.
	L.SetGlobal("log", L.NewFunction(func(l *lua.LState) int {
		log.Infof(st.r.Context(), log.TagAppDef, "[lua %s] %s", f.name, l.CheckString(1))
		return 0
	}))
}

// compile parses and compiles Lua source into a reusable function prototype.
func compile(source, name string) (*lua.FunctionProto, error) {
	chunk, err := parse.Parse(strings.NewReader(source), name)
	if err != nil {
		return nil, err
	}
	return lua.Compile(chunk, name)
}

// newSandbox builds a restricted Lua VM: only pure-computation libraries are
// opened, and the filesystem/loader escapes that OpenBase would otherwise
// expose are stripped. The request-facing capabilities are limited to the host
// API injected per request in install.
func newSandbox() *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	for _, lib := range []struct {
		name string
		open lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(lib.open))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}
	for _, g := range []string{"dofile", "loadfile", "load", "loadstring", "collectgarbage"} {
		L.SetGlobal(g, lua.LNil)
	}
	return L
}
