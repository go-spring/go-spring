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

package StarterDubbo

import (
	"context"

	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/filter"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"
	"go-spring.org/cloud/governance/traffic/canonical"
)

func init() {
	// Register the load-test identification filter under the dubbo filter name
	// "loadtest". Activate it by adding "loadtest" to a provider's filter chain
	// (ideally first), so the marker is on the context before later filters and
	// the service impl run — letting downstream code branch on
	// traffic.IsLoadTest(ctx).
	extension.SetFilter(loadTestFilterKey, newLoadTestFilter)
}

const loadTestFilterKey = "loadtest"

type loadTestFilter struct{}

func newLoadTestFilter() filter.Filter { return &loadTestFilter{} }

// Invoke tags the call context as load-test traffic when the dubbo attachment
// carried by the invocation has the marker key. It is the dubbo inbound
// companion to cloud/governance/traffic's outbound carrier injection. The attachment value
// decodes as string or []byte depending on the protocol; both are handled.
func (f *loadTestFilter) Invoke(ctx context.Context, invoker base.Invoker, inv base.Invocation) result.Result {
	if att := inv.Attachments(); att != nil {
		if canonical.IsAffirmative(attachmentString(att[canonical.MetaKeyLoadTest])) {
			ctx = canonical.WithLoadTest(ctx, "dubbo-attachment")
		}
	}
	return invoker.Invoke(ctx, inv)
}

// OnResponse is a pass-through; the filter only reads inbound attachments.
func (f *loadTestFilter) OnResponse(_ context.Context, res result.Result, _ base.Invoker, _ base.Invocation) result.Result {
	return res
}

// attachmentString extracts a string from a dubbo attachment value (any), which
// decodes as string under the triple protocol and []byte under some others.
func attachmentString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	}
	return ""
}
