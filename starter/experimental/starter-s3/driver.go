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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + the bundled DefaultDriver, which owns full client assembly
// (credentials, region, bucket-lookup style, the dynamic transport that Init
// later arms).
package StarterS3

import (
	"context"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create an S3 client (a *minio.Client). It is
// an OPTIONAL CONTAINER BEAN: a company or umbrella starter may provide its own
// Driver bean (its constructor returns StarterS3.Driver); when none is present,
// starter-s3 falls back to the bundled [DefaultDriver] inside client assembly.
// A custom driver is a bean, so it may inject the configuration/beans it needs
// — e.g. company config bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every client under
// ${spring.s3} is built through it, and per-instance differences are expressed
// through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*minio.Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new minio.Client from the provided configuration.
//
// The transport is fixed inside minio.Options at construction and cannot be
// swapped on the client afterwards, and the resilience/observability policy is
// only injected into the wrapper after CreateClient returns. So CreateClient
// installs a thin [dynamicTransport] (an atomic RoundTripper indirection)
// whose behavior Init later swaps in — the observe+resilience transport built
// from the injected policy. The dynamic transport is tracked in
// [dynamicTransports] (keyed by the returned client) so newClient can hand it
// to the wrapper.
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*minio.Client, error) {
	lookup, err := bucketLookupType(c.BucketLookup)
	if err != nil {
		return nil, errutil.Explain(err, "s3: invalid bucket-lookup %q", c.BucketLookup)
	}
	dyn := newDynamicTransport()
	cl, err := minio.New(c.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(c.AccessKeyID, c.SecretAccessKey, c.SessionToken),
		Secure:       c.UseSSL,
		Region:       c.Region,
		BucketLookup: lookup,
		Transport:    dyn,
	})
	if err != nil {
		return nil, errutil.Explain(err, "s3: create client failed for %s", c.Endpoint)
	}
	dynamicTransports.Store(cl, dyn)
	return cl, nil
}

// bucketLookupType maps the config string onto minio's BucketLookupType.
// "virtual-host" is an alias of the DNS lookup (bucket in the host name);
// minio v7.0.74 spells that mode BucketLookupDNS.
func bucketLookupType(s string) (minio.BucketLookupType, error) {
	switch s {
	case "", "auto":
		return minio.BucketLookupAuto, nil
	case "virtual-host", "dns":
		return minio.BucketLookupDNS, nil
	case "path":
		return minio.BucketLookupPath, nil
	default:
		return 0, errutil.Explain(nil, "unknown bucket-lookup %q (want auto|virtual-host|path|dns)", s)
	}
}

// dynamicTransports tracks the dynamic transport DefaultDriver installed for
// each client, so newClient can hand it to the wrapper for Init to arm. The
// key is the *minio.Client value; only clients built by DefaultDriver appear
// here.
var dynamicTransports sync.Map // *minio.Client -> *dynamicTransport
