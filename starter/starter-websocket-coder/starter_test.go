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

package StarterWebsocketCoder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
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
	// its documented default: nothing negotiated, verified or compressed.
	c := bindConfig(t, nil)
	assert.That(t, len(c.Subprotocols)).Equal(0)
	assert.That(t, c.InsecureSkipVerify).False()
	assert.That(t, len(c.OriginPatterns)).Equal(0)
	assert.That(t, c.CompressionMode).Equal(0)
	assert.That(t, c.CompressionThreshold).Equal(0)
}

func TestConfigExplicitValues(t *testing.T) {
	// Explicit properties override every default.
	c := bindConfig(t, map[string]any{
		"spring.websocket.subprotocols":         []any{"chat", "json"},
		"spring.websocket.insecureSkipVerify":   true,
		"spring.websocket.originPatterns":       []any{"example.com"},
		"spring.websocket.compressionMode":      1,
		"spring.websocket.compressionThreshold": 4096,
	})
	assert.That(t, c.Subprotocols[0]).Equal("chat")
	assert.That(t, c.Subprotocols[1]).Equal("json")
	assert.That(t, c.InsecureSkipVerify).True()
	assert.That(t, c.OriginPatterns[0]).Equal("example.com")
	assert.That(t, c.CompressionMode).Equal(1)
	assert.That(t, c.CompressionThreshold).Equal(4096)
}

func TestNewAcceptOptionsMapsFields(t *testing.T) {
	// Every config field maps onto the corresponding AcceptOptions field;
	// CompressionMode keeps its symbolic meaning.
	o := NewAcceptOptions(Config{
		Subprotocols:         []string{"chat"},
		InsecureSkipVerify:   true,
		OriginPatterns:       []string{"*.example.com"},
		CompressionMode:      int(websocket.CompressionNoContextTakeover),
		CompressionThreshold: 512,
	})
	assert.That(t, o.Subprotocols[0]).Equal("chat")
	assert.That(t, o.InsecureSkipVerify).True()
	assert.That(t, o.OriginPatterns[0]).Equal("*.example.com")
	assert.That(t, o.CompressionMode).Equal(websocket.CompressionNoContextTakeover)
	assert.That(t, o.CompressionThreshold).Equal(512)
}

// echoServer runs an httptest server that accepts connections with the given
// options and echoes every message back.
func echoServer(t *testing.T, opts *websocket.AcceptOptions) string {
	t.Helper()
	if opts == nil {
		opts = &websocket.AcceptOptions{}
	}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, opts)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		for {
			mt, msg, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, mt, msg); err != nil {
				return
			}
		}
	}))
	t.Cleanup(svr.Close)
	return "ws://" + svr.Listener.Addr().String() + "/ws"
}

// dial connects with a coder client, optionally announcing subprotocols and a
// foreign Origin header.
func dial(t *testing.T, url, origin string, subprotocols ...string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader:   h,
		Subprotocols: subprotocols,
	})
}

func TestAcceptEcho(t *testing.T) {
	// Default options accept a same-origin handshake and the connection
	// round-trips messages.
	url := echoServer(t, nil)
	conn, _, err := dial(t, url, "")
	assert.That(t, err).Nil()
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.That(t, conn.Write(ctx, websocket.MessageText, []byte("ping"))).Nil()
	mt, msg, err := conn.Read(ctx)
	assert.That(t, err).Nil()
	assert.That(t, mt).Equal(websocket.MessageText)
	assert.That(t, string(msg)).Equal("ping")
}

func TestAcceptRejectsForeignOriginByDefault(t *testing.T) {
	// coder/websocket verifies the Origin by default: a foreign origin
	// that matches no OriginPattern is rejected.
	url := echoServer(t, &websocket.AcceptOptions{})
	conn, resp, err := dial(t, url, "http://evil.example")
	assert.That(t, err).NotNil()
	assert.That(t, conn).Nil()
	if resp != nil {
		assert.That(t, resp.StatusCode).Equal(http.StatusForbidden)
		_ = resp.Body.Close()
	}
}

func TestOriginPatternAllowsForeignOrigin(t *testing.T) {
	// An OriginPattern matching the foreign origin lets the handshake
	// through; the options built by the starter behave the same as a
	// hand-written AcceptOptions.
	opts := NewAcceptOptions(Config{OriginPatterns: []string{"evil.example"}})
	url := echoServer(t, opts)
	conn, _, err := dial(t, url, "http://evil.example")
	assert.That(t, err).Nil()
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func TestSubprotocolNegotiation(t *testing.T) {
	// coder/websocket selects the first protocol from the server's
	// Subprotocols list that the client also requested ("json" precedes
	// "chat" in the server list, even though the client listed "chat"
	// first).
	opts := NewAcceptOptions(Config{Subprotocols: []string{"json", "chat"}})
	url := echoServer(t, opts)
	conn, _, err := dial(t, url, "", "chat", "json")
	assert.That(t, err).Nil()
	defer conn.Close(websocket.StatusNormalClosure, "")
	assert.That(t, conn.Subprotocol()).Equal("json")
}
