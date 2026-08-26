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

package StarterWebsocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// bindConfig binds a Config from ${spring.websocket} the same way the starter
// wires it, so the value-tag defaults are exercised for real.
func bindConfig(t *testing.T, props map[string]any) Config {
	t.Helper()
	p := flatten.NewPropertiesStorage(flatten.MapProperties(props))
	var c Config
	assert.That(t, conf.Bind(p, &c, "${spring.websocket}")).Nil()
	return c
}

func TestConfigDefaults(t *testing.T) {
	// With no ${spring.websocket.*} properties every field falls back to
	// its documented default.
	c := bindConfig(t, nil)
	assert.That(t, c.HandshakeTimeout).Equal(10 * time.Second)
	assert.That(t, c.ReadBufferSize).Equal(1024)
	assert.That(t, c.WriteBufferSize).Equal(1024)
	assert.That(t, c.EnableCompression).False()
	assert.That(t, len(c.Subprotocols)).Equal(0)
	assert.That(t, len(c.AllowedOrigins)).Equal(0)
}

func TestConfigExplicitValues(t *testing.T) {
	// Explicit properties override every default.
	c := bindConfig(t, map[string]any{
		"spring.websocket.handshakeTimeout":  "5s",
		"spring.websocket.readBufferSize":    2048,
		"spring.websocket.writeBufferSize":   4096,
		"spring.websocket.enableCompression": true,
		"spring.websocket.subprotocols":      []any{"chat", "json"},
		"spring.websocket.allowedOrigins":    []any{"https://a.example", "*"},
	})
	assert.That(t, c.HandshakeTimeout).Equal(5 * time.Second)
	assert.That(t, c.ReadBufferSize).Equal(2048)
	assert.That(t, c.WriteBufferSize).Equal(4096)
	assert.That(t, c.EnableCompression).True()
	assert.That(t, c.Subprotocols[0]).Equal("chat")
	assert.That(t, c.Subprotocols[1]).Equal("json")
	assert.That(t, c.AllowedOrigins[0]).Equal("https://a.example")
	assert.That(t, c.AllowedOrigins[1]).Equal("*")
}

func TestNewUpgraderMapsFields(t *testing.T) {
	u := NewUpgrader(Config{
		HandshakeTimeout:  3 * time.Second,
		ReadBufferSize:    512,
		WriteBufferSize:   256,
		EnableCompression: true,
		Subprotocols:      []string{"chat"},
	})
	assert.That(t, u.HandshakeTimeout).Equal(3 * time.Second)
	assert.That(t, u.ReadBufferSize).Equal(512)
	assert.That(t, u.WriteBufferSize).Equal(256)
	assert.That(t, u.EnableCompression).True()
	assert.That(t, u.Subprotocols[0]).Equal("chat")

	// Without AllowedOrigins the upgrader keeps gorilla's default
	// same-origin policy: CheckOrigin stays nil.
	assert.That(t, u.CheckOrigin).Nil()
}

// echoServer runs an httptest server that upgrades with u and echoes every
// message back.
func echoServer(t *testing.T, u *websocket.Upgrader) string {
	t.Helper()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, msg); err != nil {
				return
			}
		}
	}))
	t.Cleanup(svr.Close)
	return "ws://" + svr.Listener.Addr().String() + "/ws"
}

// dial connects with a gorilla client, optionally announcing subprotocols and
// a foreign Origin header.
func dial(t *testing.T, url string, origin string, subprotocols ...string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	d := &websocket.Dialer{HandshakeTimeout: 2 * time.Second, Subprotocols: subprotocols}
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	return d.Dial(url, h)
}

func TestUpgradeEcho(t *testing.T) {
	// The default upgrader upgrades a same-origin handshake and the
	// connection round-trips messages.
	url := echoServer(t, NewUpgrader(Config{}))
	conn, _, err := dial(t, url, "")
	assert.That(t, err).Nil()
	defer conn.Close()

	assert.That(t, conn.WriteMessage(websocket.TextMessage, []byte("ping"))).Nil()
	_, msg, err := conn.ReadMessage()
	assert.That(t, err).Nil()
	assert.That(t, string(msg)).Equal("ping")
}

func TestUpgradeRejectsForeignOriginByDefault(t *testing.T) {
	// gorilla's default same-origin policy rejects a foreign Origin.
	url := echoServer(t, NewUpgrader(Config{}))
	conn, resp, err := dial(t, url, "http://evil.example")
	assert.That(t, err).NotNil()
	assert.That(t, conn).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusForbidden)
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func TestAllowedOrigins(t *testing.T) {
	// An explicit allowlist admitting the request's Origin lets the
	// handshake through.
	u := NewUpgrader(Config{AllowedOrigins: []string{"http://evil.example"}})
	url := echoServer(t, u)
	conn, _, err := dial(t, url, "http://evil.example")
	assert.That(t, err).Nil()
	_ = conn.Close()

	// A list without the Origin (and without "*") still rejects it.
	u = NewUpgrader(Config{AllowedOrigins: []string{"http://good.example"}})
	url = echoServer(t, u)
	conn, resp, err := dial(t, url, "http://evil.example")
	assert.That(t, err).NotNil()
	assert.That(t, conn).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusForbidden)
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	// The "*" wildcard accepts any origin.
	u = NewUpgrader(Config{AllowedOrigins: []string{"*"}})
	url = echoServer(t, u)
	conn, _, err = dial(t, url, "http://evil.example")
	assert.That(t, err).Nil()
	_ = conn.Close()
}

func TestSubprotocolNegotiation(t *testing.T) {
	// The first client-requested protocol the server supports is selected.
	u := NewUpgrader(Config{Subprotocols: []string{"json", "chat"}})
	url := echoServer(t, u)
	conn, _, err := dial(t, url, "", "chat", "json")
	assert.That(t, err).Nil()
	defer conn.Close()
	assert.That(t, conn.Subprotocol()).Equal("chat")

	// When the client requests none of the server's protocols, no
	// subprotocol is selected but the handshake still succeeds.
	conn, _, err = dial(t, url, "", "binary")
	assert.That(t, err).Nil()
	_ = conn.Close()
}
