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
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/stdlib/errutil"
)

var (
	errNoSigningKey  = errors.New("oauth2-server: no signing key configured (set one of secret, private-key/private-key-file)")
	errBothSigning   = errors.New("oauth2-server: both HMAC secret and PEM private key configured (set exactly one)")
	errBadAlgForKey  = errors.New("oauth2-server: configured algorithm is not compatible with the key source")
	errUnsupportKind = errors.New("oauth2-server: unsupported private key type (want RSA or ECDSA)")
)

// signer mints signed JWT access tokens and publishes its verification key set.
// It is built once at startup: an ambiguous or unparsable key fails fast, so a
// running server can always sign.
type signer struct {
	method jwt.SigningMethod
	key    any    // []byte (HMAC), *rsa.PrivateKey, or *ecdsa.PrivateKey
	kid    string
	jwks   []byte // precomputed {"keys":[...]} document; an empty set for HMAC
}

// newSigner resolves the single configured key source and signing algorithm and
// precomputes the JWKS document.
func newSigner(c Config) (*signer, error) {
	hasSecret := c.Secret != ""
	hasPEM := c.PrivateKey != "" || c.PrivateKeyFile != ""
	switch {
	case !hasSecret && !hasPEM:
		return nil, errNoSigningKey
	case hasSecret && hasPEM:
		return nil, errBothSigning
	}

	s := &signer{kid: c.KeyID}
	if s.kid == "" {
		s.kid = "default"
	}

	if hasSecret {
		method, err := hmacMethod(c.Algorithm)
		if err != nil {
			return nil, err
		}
		s.method = method
		s.key = []byte(c.Secret)
		s.jwks = emptyJWKS()
		return s, nil
	}

	priv, err := parsePEMPrivateKey(c)
	if err != nil {
		return nil, err
	}
	method, jwksDoc, err := asymmetricSetup(c.Algorithm, s.kid, priv)
	if err != nil {
		return nil, err
	}
	s.method = method
	s.key = priv
	s.jwks = jwksDoc
	return s, nil
}

// sign builds and signs a token carrying the given claims.
func (s *signer) sign(claims jwt.MapClaims) (string, error) {
	tok := jwt.NewWithClaims(s.method, claims)
	tok.Header["kid"] = s.kid
	return tok.SignedString(s.key)
}

// hmacMethod maps an optional algorithm string onto an HMAC signing method,
// defaulting to HS256 and rejecting an asymmetric algorithm.
func hmacMethod(alg string) (jwt.SigningMethod, error) {
	switch strings.ToUpper(alg) {
	case "", "HS256":
		return jwt.SigningMethodHS256, nil
	case "HS384":
		return jwt.SigningMethodHS384, nil
	case "HS512":
		return jwt.SigningMethodHS512, nil
	default:
		return nil, errBadAlgForKey
	}
}

// asymmetricSetup maps an optional algorithm onto an RSA/EC signing method
// compatible with the key and returns the method plus the published JWKS.
func asymmetricSetup(alg, kid string, priv any) (jwt.SigningMethod, []byte, error) {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		method, err := rsaMethod(alg)
		if err != nil {
			return nil, nil, err
		}
		doc, err := rsaJWKS(kid, method.Alg(), &k.PublicKey)
		return method, doc, err
	case *ecdsa.PrivateKey:
		method, err := ecMethod(alg, k)
		if err != nil {
			return nil, nil, err
		}
		doc, err := ecJWKS(kid, method.Alg(), &k.PublicKey)
		return method, doc, err
	default:
		return nil, nil, errUnsupportKind
	}
}

func rsaMethod(alg string) (jwt.SigningMethod, error) {
	switch strings.ToUpper(alg) {
	case "", "RS256":
		return jwt.SigningMethodRS256, nil
	case "RS384":
		return jwt.SigningMethodRS384, nil
	case "RS512":
		return jwt.SigningMethodRS512, nil
	case "PS256":
		return jwt.SigningMethodPS256, nil
	case "PS384":
		return jwt.SigningMethodPS384, nil
	case "PS512":
		return jwt.SigningMethodPS512, nil
	default:
		return nil, errBadAlgForKey
	}
}

// ecMethod picks the ES* method for the key's curve, honoring an explicit
// (compatible) algorithm override.
func ecMethod(alg string, k *ecdsa.PrivateKey) (jwt.SigningMethod, error) {
	byCurve := map[int]jwt.SigningMethod{
		256: jwt.SigningMethodES256,
		384: jwt.SigningMethodES384,
		521: jwt.SigningMethodES512,
	}
	def, ok := byCurve[k.Curve.Params().BitSize]
	if !ok {
		return nil, errUnsupportKind
	}
	if alg == "" {
		return def, nil
	}
	switch strings.ToUpper(alg) {
	case "ES256", "ES384", "ES512":
		if !strings.EqualFold(alg, def.Alg()) {
			return nil, errBadAlgForKey
		}
		return def, nil
	default:
		return nil, errBadAlgForKey
	}
}

// parsePEMPrivateKey loads the PEM private key from the inline value or the
// file, trying PKCS#8, PKCS#1 (RSA), then SEC1 (EC).
func parsePEMPrivateKey(c Config) (any, error) {
	pem := []byte(c.PrivateKey)
	if c.PrivateKeyFile != "" {
		b, err := os.ReadFile(c.PrivateKeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "oauth2-server: read private-key-file %s", c.PrivateKeyFile)
		}
		pem = b
	}
	if k, err := jwt.ParseRSAPrivateKeyFromPEM(pem); err == nil {
		return k, nil
	}
	if k, err := jwt.ParseECPrivateKeyFromPEM(pem); err == nil {
		return k, nil
	}
	return nil, errors.New("oauth2-server: private key is neither a valid RSA nor ECDSA PEM")
}

// jwk mirrors a single JSON Web Key entry; only the members needed to publish an
// RSA or EC public key are modeled.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid,omitempty"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
	// RSA
	N string `json:"n,omitempty"`
	E string `json:"e,omitempty"`
	// EC
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

func emptyJWKS() []byte {
	b, _ := json.Marshal(struct {
		Keys []jwk `json:"keys"`
	}{Keys: []jwk{}})
	return b
}

func rsaJWKS(kid, alg string, pub *rsa.PublicKey) ([]byte, error) {
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	k := jwk{
		Kty: "RSA",
		Kid: kid,
		Use: "sig",
		Alg: alg,
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	return json.Marshal(struct {
		Keys []jwk `json:"keys"`
	}{Keys: []jwk{k}})
}

func ecJWKS(kid, alg string, pub *ecdsa.PublicKey) ([]byte, error) {
	crv := ""
	switch pub.Curve.Params().BitSize {
	case 256:
		crv = "P-256"
	case 384:
		crv = "P-384"
	case 521:
		crv = "P-521"
	default:
		return nil, errUnsupportKind
	}
	// Fixed-width big-endian coordinates per RFC 7518 §6.2.1.2.
	size := (pub.Curve.Params().BitSize + 7) / 8
	k := jwk{
		Kty: "EC",
		Kid: kid,
		Use: "sig",
		Alg: alg,
		Crv: crv,
		X:   base64.RawURLEncoding.EncodeToString(leftPad(pub.X.Bytes(), size)),
		Y:   base64.RawURLEncoding.EncodeToString(leftPad(pub.Y.Bytes(), size)),
	}
	return json.Marshal(struct {
		Keys []jwk `json:"keys"`
	}{Keys: []jwk{k}})
}

// leftPad returns b left-padded with zero bytes to exactly size bytes.
func leftPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}

// now is overridable in tests; production uses the wall clock.
var now = time.Now
