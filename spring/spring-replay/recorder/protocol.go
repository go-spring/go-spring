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

package recorder

import "fmt"

const (
	CACHE = "CACHE"
	HTTP  = "HTTP"
	SQL   = "SQL"
	REDIS = "REDIS"
)

var (
	protocols = map[string]Protocol{}
)

type Protocol interface {
	GetLabel(data string) string
	FlatRequest(data string) (map[string]string, error)
	FlatResponse(data string) (map[string]string, error)
}

func GetProtocol(name string) Protocol {
	return protocols[name]
}

func RegisterProtocol(name string, protocol Protocol) {
	if _, ok := protocols[name]; ok {
		panic(fmt.Errorf("%s: duplicate registration", name))
	}
	protocols[name] = protocol
}
