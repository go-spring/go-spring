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

// Package recorder 流量录制。
package recorder

import (
	"encoding/json"
	"errors"
)

type Session struct {
	ID      string    `json:"id"`
	Actions []*Action `json:"actions"`
}

type Action struct {
	Protocol string      `json:"protocol"`
	Key      string      `json:"key"`
	Data     interface{} `json:"data"`
}

func (action *Action) UnmarshalJSON(data []byte) error {

	var rawAction struct {
		Protocol string          `json:"protocol"`
		Key      string          `json:"key"`
		Data     json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(data, &rawAction); err != nil {
		return err
	}

	f, ok := factory[rawAction.Protocol]
	if !ok {
		return errors.New("unsupported protocol")
	}

	i := f()
	if err := json.Unmarshal(rawAction.Data, &i); err != nil {
		return err
	}

	action.Protocol = rawAction.Protocol
	action.Key = rawAction.Key
	action.Data = i
	return nil
}

const (
	HTTP  = "http"
	REDIS = "redis"
)

var factory = map[string]func() interface{}{
	HTTP:  func() interface{} { return new(Http) },
	REDIS: func() interface{} { return new(Redis) },
}

type Http struct {
	Method   string        `json:"method"`
	URI      string        `json:"uri"`
	Version  string        `json:"version"`
	Request  *HttpRequest  `json:"request"`
	Response *HttpResponse `json:"response"`
}

type HttpRequest struct {
	Query  map[string][]string `json:"query"`
	Header map[string][]string `json:"header"`
	Body   interface{}         `json:"body"`
}

type HttpResponse struct {
	Status string              `json:"status"`
	Header map[string][]string `json:"header"`
	Body   interface{}         `json:"body"`
}

type Redis struct {
	Request  []interface{} `json:"req"`
	Response interface{}   `json:"resp"`
}
