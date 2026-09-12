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

package StarterConfigNacos

import (
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// fakeConfigClient stands in for a nacos server: it serves one document and
// records the installed listener. Unimplemented methods come from the embedded
// interface (nil panics if wrongly touched). getErr/listenErr inject failures
// for error-path tests; listens counts ListenConfig calls so listener-dedup can
// be asserted.
type fakeConfigClient struct {
	config_client.IConfigClient

	mu        sync.Mutex
	data      string
	getErr    error
	listenErr error
	listens   int
	onChange  func(namespace, group, dataId, data string)
}

func (f *fakeConfigClient) GetConfig(vo.ConfigParam) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data, f.getErr
}

func (f *fakeConfigClient) ListenConfig(p vo.ConfigParam) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listenErr != nil {
		return f.listenErr
	}
	f.listens++
	f.onChange = p.OnChange
	return nil
}

func (f *fakeConfigClient) CancelListenConfig(vo.ConfigParam) error { return nil }
