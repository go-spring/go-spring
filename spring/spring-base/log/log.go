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
	"strings"
	"sync"

	"github.com/go-spring/spring-base/atomic"
)

var (
	// configLoggers 配置文件中的 Logger 对象，is safe for map[string]privateConfig.
	configLoggers atomic.Value

	// usingLoggers 用户代码中的 Logger 对象，is safe for map[string]*Logger.
	usingLoggers sync.Map

	// Status records events that occur in the logging system.
	Status = newLogger("", ErrorLevel)
)

type Initializer interface {
	Init() error
}

type LifeCycle interface {
	Start() error
	Stop(ctx context.Context)
}

// T 将可变参数转换成切片形式。
func T(a ...interface{}) []interface{} {
	return a
}

type privateConfigMap struct {
	loggers map[string]privateConfig
}

func (m *privateConfigMap) Get(name string) privateConfig {
	for {
		if v, ok := m.loggers[name]; ok {
			return v
		}
		i := strings.LastIndexByte(name, '/')
		if i < 0 {
			break
		}
		name = name[:i]
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
	level  []Level
	once   sync.Once
	logger *Logger
}

func (h *initLoggerHolder) Get() *Logger {
	h.once.Do(func() {
		if len(h.level) == 0 {
			h.level = append(h.level, InfoLevel)
		}
		h.logger = newLogger(h.name, h.level[0])
		m := configLoggers.Load().(*privateConfigMap)
		h.logger.reconfigure(m.Get(h.name))
	})
	return h.logger
}

func GetLogger(name string, level ...Level) *Logger {

	if configLoggers.Load() == nil {
		Status.WithSkip(1).Sugar().Fatal("should call refresh first")
		os.Exit(-1)
	}

	var h loggerHolder = &initLoggerHolder{name: name, level: level}
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
			base.parent = cRoot
		}

		for _, r := range base.AppenderRefs {
			appender, ok := cAppenders[r.Ref]
			if !ok {
				return fmt.Errorf("appender %s not found", r.Ref)
			}
			r.appender = appender
		}

		for {
			n := strings.LastIndex(name, "/")
			if n < 0 {
				break
			}
			name = name[:n]
			if parent, ok := cLoggers[name]; ok {
				base.parent = parent
			}
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

type Logger struct {
	value atomic.Value
	name  string
	level Level
	sugar *SugarLogger
}

// wrapperConfig atomic.Value 要求底层数据完全一致。
type wrapperConfig struct {
	config privateConfig
}

func newLogger(name string, level Level) *Logger {
	l := &Logger{
		name:  name,
		level: level,
	}
	l.sugar = &SugarLogger{
		l: l,
	}
	return l
}

// Name returns the logger's name.
func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) config() privateConfig {
	return l.value.Load().(*wrapperConfig).config
}

func (l *Logger) reconfigure(config privateConfig) {
	l.value.Store(&wrapperConfig{config})
}

func (l *Logger) Level() Level {
	return l.config().getLevel()
}

func (l *Logger) Filter() Filter {
	return l.config().getFilter()
}

func (l *Logger) Appenders() []Appender {
	c := l.config()
	var appenders []Appender
	for _, ref := range c.getAppenders() {
		appenders = append(appenders, ref.appender)
	}
	return appenders
}

// WithSkip 创建包含 skip 信息的 Entry 。
func (l *Logger) WithSkip(n int) SimpleEntry {
	return SimpleEntry{pub: l.config(), skip: n}
}

// WithTag 创建包含 tag 信息的 Entry 。
func (l *Logger) WithTag(tag string) SimpleEntry {
	return SimpleEntry{pub: l.config(), tag: tag}
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func (l *Logger) WithContext(ctx context.Context) ContextEntry {
	return ContextEntry{pub: l.config(), ctx: ctx}
}

func (l *Logger) enableLog(level Level) (privateConfig, bool) {
	c := l.config()
	s := c.getName()
	if len(s) != len(l.name) || s != l.name {
		if level < l.level {
			return c, false
		}
	}
	return c, true
}

func (l *Logger) Sugar() *SugarLogger {
	return l.sugar
}

// Tracew outputs log with level TraceLevel.
func (l *Logger) Tracew(fields ...Field) *Event {
	c, ok := l.enableLog(TraceLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Tracew(fields...)
}

// Debugw outputs log with level DebugLevel.
func (l *Logger) Debugw(fields ...Field) *Event {
	c, ok := l.enableLog(DebugLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Debugw(fields...)
}

// Infow outputs log with level InfoLevel.
func (l *Logger) Infow(fields ...Field) *Event {
	c, ok := l.enableLog(InfoLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Infow(fields...)
}

// Warnw outputs log with level WarnLevel.
func (l *Logger) Warnw(fields ...Field) *Event {
	c, ok := l.enableLog(WarnLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Warnw(fields...)
}

// Errorw outputs log with level ErrorLevel.
func (l *Logger) Errorw(fields ...Field) *Event {
	c, ok := l.enableLog(ErrorLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Errorw(fields...)
}

// Panicw outputs log with level PanicLevel.
func (l *Logger) Panicw(fields ...Field) *Event {
	c, ok := l.enableLog(PanicLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Panicw(fields...)
}

// Fatalw outputs log with level FatalLevel.
func (l *Logger) Fatalw(fields ...Field) *Event {
	c, ok := l.enableLog(FatalLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Fatalw(fields...)
}
