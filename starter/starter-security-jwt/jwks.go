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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"go-spring.org/stdlib/errutil"
)

// jwksCache fetches and caches the verification keys published at a JWKS
// endpoint, keyed by "kid". Keys are loaded eagerly at startup (so a broken
// endpoint fails fast) and reloaded either when the refresh interval elapses or
// when a token presents an unknown "kid" — the latter absorbs key rotation
// without waiting for the interval.
type jwksCache struct {
	url     string
	refresh time.Duration
	client  *http.Client

	mu      sync.RWMutex
	keys    map[string]any
	fetched time.Time
}

// newJWKSCache builds the cache and performs the initial fetch, returning an
// error when the endpoint is unreachable or serves no usable key.
func newJWKSCache(url string, refresh, timeout time.Duration) (*jwksCache, error) {
	c := &jwksCache{
		url:     url,
		refresh: refresh,
		client:  &http.Client{Timeout: timeout},
	}
	if err := c.reload(); err != nil {
		return nil, err
	}
	return c, nil
}

// key returns the verification key for kid, reloading once when the kid is
// unknown (to pick up a freshly rotated key) or when the cache is stale.
func (c *jwksCache) key(kid string) (any, error) {
	c.mu.RLock()
	k, ok := c.keys[kid]
	stale := time.Since(c.fetched) > c.refresh
	c.mu.RUnlock()

	if ok && !stale {
		return k, nil
	}
	if err := c.reload(); err != nil {
		if ok {
			return k, nil // serve the cached key if the refresh failed
		}
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if k, ok := c.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("security-jwt: no JWKS key for kid %q", kid)
}

// reload fetches the JWKS document and replaces the cached key set.
func (c *jwksCache) reload() error {
	req, err := http.NewRequest(http.MethodGet, c.url, nil)
	if err != nil {
		return errutil.Explain(err, "security-jwt: build JWKS request")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return errutil.Explain(err, "security-jwt: fetch JWKS %s", c.url)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("security-jwt: fetch JWKS %s: status %d", c.url, resp.StatusCode)
	}

	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return errutil.Explain(err, "security-jwt: decode JWKS %s", c.url)
	}

	keys := make(map[string]any, len(doc.Keys))
	for _, k := range doc.Keys {
		pub, err := k.publicKey()
		if err != nil {
			continue // skip keys we cannot parse (unsupported kty/curve)
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return fmt.Errorf("security-jwt: JWKS %s contains no usable key", c.url)
	}

	c.mu.Lock()
	c.keys = keys
	c.fetched = time.Now()
	c.mu.Unlock()
	return nil
}

// jwk is a single JSON Web Key. Only the RSA and EC members needed to
// reconstruct a public key are modeled.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	// RSA
	N string `json:"n"`
	E string `json:"e"`
	// EC
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// publicKey reconstructs a crypto public key from the JWK, supporting RSA and
// the P-256/384/521 EC curves.
func (k jwk) publicKey() (any, error) {
	switch k.Kty {
	case "RSA":
		return k.rsaKey()
	case "EC":
		return k.ecKey()
	default:
		return nil, fmt.Errorf("security-jwt: unsupported JWK kty %q", k.Kty)
	}
}

func (k jwk) rsaKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, errutil.Explain(err, "security-jwt: decode RSA modulus")
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, errutil.Explain(err, "security-jwt: decode RSA exponent")
	}
	// Left-pad the exponent to 8 bytes so it can be read as a uint64.
	padded := make([]byte, 8)
	copy(padded[8-len(eBytes):], eBytes)
	e := binary.BigEndian.Uint64(padded)
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(e),
	}, nil
}

func (k jwk) ecKey() (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("security-jwt: unsupported EC curve %q", k.Crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, errutil.Explain(err, "security-jwt: decode EC x")
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, errutil.Explain(err, "security-jwt: decode EC y")
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}
