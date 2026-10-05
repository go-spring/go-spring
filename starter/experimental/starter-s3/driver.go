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
// (credentials, region, bucket-lookup style, and the dynamic transport the
// wrapper's declaration+resilience stack is installed into).

package StarterS3

import (
	"context"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go-spring.org/cloud"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create an S3 client. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterS3.Driver); when none is present, starter-s3
// falls back to the bundled [DefaultDriver] inside client assembly. A custom
// driver is a bean, so it may inject the configuration/beans it needs — e.g.
// company config bound from a properties file at wiring time.
//
// CreateClient returns the module's exported [Client] — the wrapper apps inject
// — not the raw *minio.Client, so a driver takes part in the type the rest of
// the ecosystem sees. It returns the client COMPLETE: cfg fixes the endpoint the
// resilience service label is derived from, the driver hands the wrapper the
// [dynamicTransport] it installed so the declaration layer can be swapped in,
// and params supplies the container's facilities (see [cloud.ClientParams]),
// which [NewClient] applies while building. Nothing patches the client
// afterwards.
//
// params is one struct rather than a parameter per capability so this interface
// — which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its fields simply ignores it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.s3} is built through it, and per-instance differences are expressed
// through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new S3 client from the provided configuration.
//
// The transport is fixed inside minio.Options at construction and cannot be
// swapped on the client afterwards, while the declaration+resilience transport
// is built by the wrapper's constructor ([NewClient]). So CreateClient installs
// a thin [dynamicTransport] (an atomic RoundTripper indirection) as the client's
// transport and returns a wrapper built over it: [NewClient] swaps the
// declaration+resilience stack in, with the indirection keeping it installable
// after minio.New has already captured the transport.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error) {
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
	return NewClient(cl, dyn, c, params), nil
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
