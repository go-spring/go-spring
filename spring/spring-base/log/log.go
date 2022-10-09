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

package log

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-spring/spring-base/atomic"
)

// configLoggers 配置文件中的 Logger 对象，is safe for map[string]privateConfig.
var configLoggers atomic.Value

// usingLoggers 用户代码中的 Logger 对象，is safe for map[string]*Logger.
var usingLoggers sync.Map

type Initializer interface {
	Init() error
}

type LifeCycle interface {
	Start() error
	Stop(ctx context.Context)
}

type privateConfigMap struct {
	loggers map[string]privateConfig
}

func (m *privateConfigMap) Get(name string) privateConfig {
	if v, ok := m.loggers[name]; ok {
		return v
	}
	return m.loggers["<<ROOT>>"]
}

type loggerHolder interface {
	Get() *Logger
}

type simLoggerHolder struct {
	logger *Logger
}

func (h *simLoggerHolder) Get() *Logger {
	return h.logger
}

type initLoggerHolder struct {
	name   string
	once   sync.Once
	logger *Logger
}

func (h *initLoggerHolder) Get() *Logger {
	h.once.Do(func() {
		h.logger = newLogger(h.name)
		m := configLoggers.Load().(*privateConfigMap)
		h.logger.reconfigure(m.Get(h.name))
	})
	return h.logger
}

func GetLogger(name string) *Logger {

	if configLoggers.Load() == nil {
		panic(errors.New("should call refresh first"))
	}

	var h loggerHolder = &initLoggerHolder{name: name}
	actual, loaded := usingLoggers.LoadOrStore(name, h)
	if loaded {
		return actual.(loggerHolder).Get()
	}

	h = &simLoggerHolder{logger: h.Get()}
	usingLoggers.LoadOrStore(name, h)
	return h.Get()
}

// Refresh 加载日志配置文件。
func Refresh(fileName string) error {
	ext := filepath.Ext(fileName)
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	return RefreshReader(file, ext)
}

// RefreshBuffer 加载日志配置文件。
func RefreshBuffer(buffer string, ext string) error {
	input := bytes.NewBufferString(buffer)
	return RefreshReader(input, ext)
}

// RefreshReader 加载日志配置文件。
func RefreshReader(input io.Reader, ext string) error {

	var rootNode *Node
	{
		r, ok := readers[ext]
		if !ok {
			return fmt.Errorf("unsupported file type %s", ext)
		}
		data, err := ioutil.ReadAll(input)
		if err != nil {
			return err
		}
		rootNode, err = r.Read(data)
		if err != nil {
			return err
		}
	}

	if rootNode.Label != "Configuration" {
		return errors.New("the Configuration root not found")
	}

	var (
		cRoot      privateConfig
		cAppenders = make(map[string]Appender)
		cLoggers   = make(map[string]privateConfig)
	)

	if node := rootNode.child("Appenders"); node != nil {
		for _, c := range node.Children {
			p, ok := plugins[c.Label]
			if !ok {
				return fmt.Errorf("plugin %s not found", c.Label)
			}
			name, ok := c.Attributes["name"]
			if !ok {
				return errors.New("attribute 'name' not found")
			}
			v, err := newPlugin(p.Class, c)
			if err != nil {
				return err
			}
			cAppenders[name] = v.Interface().(Appender)
		}
	}

	if node := rootNode.child("Loggers"); node != nil {
		for _, c := range node.Children {

			isRootLogger := c.Label == "Root" || c.Label == "AsyncRoot"
			if isRootLogger {
				if cRoot != nil {
					return errors.New("found more than one root loggers")
				}
				c.Attributes["name"] = "<<ROOT>>"
			}

			p, ok := plugins[c.Label]
			if !ok || p == nil {
				return fmt.Errorf("plugin %s not found", c.Label)
			}
			name, ok := c.Attributes["name"]
			if !ok {
				return errors.New("attribute 'name' not found")
			}

			v, err := newPlugin(p.Class, c)
			if err != nil {
				return err
			}
			config := v.Interface().(privateConfig)
			if isRootLogger {
				cRoot = config
			}
			cLoggers[name] = config
		}
	}

	if cRoot == nil {
		return errors.New("found no root logger")
	}

	for name, config := range cLoggers {

		var base *baseLoggerConfig
		switch v := config.(type) {
		case *loggerConfig:
			base = &v.baseLoggerConfig
		case *asyncLoggerConfig:
			base = &v.baseLoggerConfig
		}

		if name != cRoot.getName() {
			base.root = cRoot
		}

		for _, r := range base.AppenderRefs {
			appender, ok := cAppenders[r.Ref]
			if !ok {
				return fmt.Errorf("appender %s not found", r.Ref)
			}
			r.appender = appender
		}
	}

	m := &privateConfigMap{cLoggers}
	configLoggers.Store(m)

	// 对用户代码中的 Logger 对象应用最新的配置。
	usingLoggers.Range(func(key, value interface{}) bool {
		l := value.(loggerHolder).Get()
		l.reconfigure(m.Get(key.(string)))
		return true
	})

	return nil
}
