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

// Command order is service A of the full-stack reference app and the hub that
// ties the ecosystem together in one request path:
//
//   - It is a JWT resource server: its HTTP mux is wrapped by the "api"
//     authenticator from starter-security-jwt, so POST /orders requires a valid
//     bearer token (the gateway forwards the caller's Authorization header).
//   - POST /orders runs a Saga (starter-transaction-saga): step 1 reserves stock
//     on the inventory service (B), discovered through Consul; step 2 "charges".
//     When the Nacos-backed dynamic flag fullstack.order.charge-fail is flipped
//     to true, step 2 fails and the Saga compensates — calling B's /release —
//     proving cross-service rollback triggered by a live config change.
//   - starter-otel gives every log line the request's trace_id and propagates the
//     trace to B, so one order is one trace across A and B.
//   - starter-registry-consul advertises this instance as "order" so the gateway
//     can route lb://order to it.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	StarterSecurityJWT "go-spring.org/starter-security-jwt"
	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/security"
	"go-spring.org/stdlib/transaction"

	"fullstack/internal/consuldisc"

	StarterOTel "go-spring.org/starter-otel"

	// Contributor + provider starters, enabled by blank import.
	_ "go-spring.org/starter-actuator"
	_ "go-spring.org/starter-config-nacos"
	_ "go-spring.org/starter-registry-consul"
	_ "go-spring.org/starter-transaction-saga"
)

// discoveryName is the stdlib/discovery registry key under which the Consul
// resolver is published; the order->inventory LiveDialer looks it up by this
// name. It matches the gateway's spring.gateway.routes.orders.upstream.discovery.
const discoveryName = "consul"

// consulAddr is the Consul agent this process resolves against. Hard-coded to the
// docker-compose agent to keep the sample self-contained, mirroring the
// starter-registry-consul example.
const consulAddr = "127.0.0.1:8500"

var bizTag = log.RegisterBizTag("order", "")

// Flags binds the dynamic, Nacos-sourced switches this service honours. Binding
// through gs.Dync means a value published to Nacos refreshes the field with no
// restart — the config-center hot-reload leg of the demo.
type Flags struct {
	// ChargeFail, when true, makes the Saga's charge step fail so the reserve
	// step is compensated. Toggle it in Nacos to trigger a rollback on demand.
	ChargeFail gs.Dync[bool] `value:"${fullstack.order.charge-fail:=false}"`
}

func init() {
	// Publish the Consul-backed discovery backend before anything resolves through
	// it (the gateway and this service both use discovery.MustGet(discoveryName)).
	if err := consuldisc.Register(discoveryName, consulAddr); err != nil {
		panic(err)
	}

	gs.Provide(&Flags{})

	// The order mux is the JWT resource server. Arg 1 (the authenticator) is
	// selected by name "api" — the sub-key of spring.security.jwt.api in
	// app.properties. The handler chain is: trace span (outermost) -> JWT auth ->
	// business mux, so authentication happens inside the request's span.
	gs.Provide(newOrderMux, gs.IndexArg(1, gs.TagArg("api")))
}

// newOrderMux assembles the order service's HTTP handler as a *gs.HttpServeMux so
// the built-in Go-Spring HTTP server (${spring.http.server}) serves it.
func newOrderMux(coord transaction.Coordinator, auth *StarterSecurityJWT.Authenticator, flags *Flags) *gs.HttpServeMux {
	app := &orderApp{coord: coord, flags: flags, inv: &inventoryClient{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/orders", app.placeOrder)
	return &gs.HttpServeMux{Handler: traceServer(auth.Wrap(mux))}
}

// orderApp holds the collaborators the /orders handler needs.
type orderApp struct {
	coord transaction.Coordinator
	flags *Flags
	inv   *inventoryClient
}

// placeOrder runs a two-step Saga for one order. On success it returns 200
// committed; when compensation ran (charge failed and the reservation was
// released on B) it returns 409 with the terminal Saga status.
func (a *orderApp) placeOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The saga id is the idempotency key. Prefer a caller-supplied request id so a
	// retried order maps to the same saga; fall back to a time-based id.
	sagaID := r.Header.Get("X-Request-Id")
	if sagaID == "" {
		sagaID = "order-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	if a, ok := security.FromContext(ctx); ok {
		log.Infof(ctx, bizTag, "placing order for subject=%s saga=%s", a.Principal.Subject, sagaID)
	}

	steps := []transaction.Step{
		{
			Name: "reserve",
			Action: func(ctx context.Context) (any, error) {
				return a.inv.reserve(ctx)
			},
			Compensate: func(ctx context.Context, result any) error {
				token, _ := result.(string)
				return a.inv.release(ctx, token)
			},
		},
		{
			Name: "charge",
			Action: func(ctx context.Context) (any, error) {
				if a.flags.ChargeFail.Value() {
					return nil, errors.New("charge declined (fullstack.order.charge-fail=true)")
				}
				return "charged", nil
			},
		},
	}

	res, err := a.coord.Execute(ctx, transaction.Saga{ID: sagaID, Steps: steps})
	switch res.Status {
	case transaction.StatusCommitted:
		writeJSON(w, http.StatusOK, `{"status":"committed","saga":"`+sagaID+`"}`)
	case transaction.StatusCompensated:
		log.Warnf(ctx, bizTag, "saga %s compensated: %v", sagaID, err)
		writeJSON(w, http.StatusConflict, `{"status":"compensated","saga":"`+sagaID+`"}`)
	default:
		log.Errorf(ctx, bizTag, "saga %s failed: status=%s err=%v", sagaID, res.Status, err)
		writeJSON(w, http.StatusInternalServerError, `{"status":"`+res.Status.String()+`","saga":"`+sagaID+`"}`)
	}
}

func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body)
}

// inventoryClient talks to service B through Consul discovery. It builds one
// LiveDialer lazily (on first use, by which time B has registered) and reuses it;
// requests target the logical host "inventory" and the dialer connects to a live
// instance, ignoring that host.
type inventoryClient struct {
	once sync.Once
	hc   *http.Client
	err  error
}

func (c *inventoryClient) client() (*http.Client, error) {
	c.once.Do(func() {
		dis, err := discovery.MustGet(discoveryName)
		if err != nil {
			c.err = err
			return
		}
		ld, err := discovery.NewLiveDialer(context.Background(), dis, "inventory")
		if err != nil {
			c.err = err
			return
		}
		c.hc = &http.Client{Transport: &http.Transport{DialContext: ld.DialContext}}
	})
	return c.hc, c.err
}

// reserve calls B's /reserve and returns the reservation token. It starts a
// client span and injects the trace context so B continues the same trace.
func (c *inventoryClient) reserve(ctx context.Context) (string, error) {
	ctx, span := otel.Tracer("order").Start(ctx, "inventory.reserve")
	defer span.End()

	body, err := c.call(ctx, "http://inventory/reserve", nil)
	if err != nil {
		return "", err
	}
	// The token is small and quote-delimited: {"token":"res-1"}. Pull it out
	// without a JSON dependency to keep the sample lean.
	token := extractField(body, "token")
	if token == "" {
		return "", fmt.Errorf("reserve: no token in response %q", body)
	}
	log.Infof(ctx, bizTag, "reserved inventory token=%s", token)
	return token, nil
}

// release calls B's /release for the given token. It is the reserve step's
// compensation, so a missing token (nothing reserved) is not an error.
func (c *inventoryClient) release(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	ctx, span := otel.Tracer("order").Start(ctx, "inventory.release")
	defer span.End()

	form := "token=" + token
	if _, err := c.call(ctx, "http://inventory/release", []byte(form)); err != nil {
		return err
	}
	log.Infof(ctx, bizTag, "released inventory token=%s", token)
	return nil
}

// call issues a POST to url (form-encoded body when non-nil), injecting the trace
// context, and returns the response body on 2xx.
func (c *inventoryClient) call(ctx context.Context, url string, body []byte) (string, error) {
	hc, err := c.client()
	if err != nil {
		return "", err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reader)
	if err != nil {
		return "", err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("%s: status %d body %q", url, resp.StatusCode, string(b))
	}
	return string(b), nil
}

func main() {
	gs.Run()
}

// traceServer wraps a handler so each inbound request runs inside a span, seeding
// the request's trace_id into logs (via the hook starter-otel installs). It
// extracts any incoming W3C trace context first so the span joins an existing
// trace when the caller propagated one.
func traceServer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := StarterOTel.StartServerSpan(r.Context(), r.Header, "order", r.Method+" "+r.URL.Path)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractField pulls a string value out of a small flat JSON object like
// {"token":"res-1"} without pulling in a JSON dependency, keeping the sample
// lean. It returns "" when the key is absent.
func extractField(body, key string) string {
	marker := `"` + key + `"`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	rest = rest[j+1:]
	k := strings.Index(rest, `"`)
	if k < 0 {
		return ""
	}
	return rest[:k]
}

// init sets the working directory to this source file's directory so the
// relative conf/app.properties path resolves regardless of the launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	wd, _ := os.Getwd()
	fmt.Println("order workdir:", wd)
}
