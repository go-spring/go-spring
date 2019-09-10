/*
 * Copyright 2012-2019 the original author or authors.
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

package SpringRpcClient

type RpcProtocol interface {
	Send() (string, error)
}

type RpcProtocolHttpJson struct {
}

func (p *RpcProtocolHttpJson) Send() (string, error) {
	return "", nil
}

type RpcProtocolHttpForm struct {
}

func (p *RpcProtocolHttpForm) Send() (string, error) {
	return "", nil
}

type RpcServiceInstance struct {
	Address string
}

type RpcService struct {
	Name      string
	Instances []RpcServiceInstance
	Protocol  RpcProtocol
}

func (service *RpcService) Call() (string, error) {
	if service.Protocol != nil {
		return service.Protocol.Send()
	}
	return "", nil
}

type RpcServiceMap struct {
	Data map[string]RpcService
}

func (sm *RpcServiceMap) AddService(service RpcService) {
	sm.Data[service.Name] = service
}

func (sm *RpcServiceMap) GetService(name string) (RpcService, bool) {
	service, ok := sm.Data[name]
	return service, ok
}
