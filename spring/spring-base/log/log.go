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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/go-spring/spring-base/atomic"
)

var (
	// console is used to record events when Logger is not configured.
	console = &ConsoleAppender{
		BaseAppender: BaseAppender{
			Layout: &DefaultLayout{
				LineBreak:  true,
				ColorStyle: ColorStyleNormal,
			},
		},
	}

	// configLoggers 配置文件中的 Logger 对象，is safe for map[string]privateConfig.
	configLoggers atomic.Value

	// usingLoggers 用户代码中的 Logger 对象，is safe for map[string]*Logger.
	usingLoggers sync.Map

	// Status records events that occur in the logging system.
	Status = NewLogger("", ErrorLevel)
)

type Initializer interface {
	Init() error
}

type LifeCycle interface {
	Start() error
	Stop(ctx context.Context)
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
		h.logger = NewLogger(h.name, h.level[0])
		{
			var cLoggers map[string]privateConfig
			if v := configLoggers.Load(); v != nil {
				cLoggers = v.(map[string]privateConfig)
			}
			h.logger.reconfigure(cLoggers[h.name])
		}
	})
	return h.logger
}

func GetLogger(name string, level ...Level) *Logger {

	var cLoggers map[string]privateConfig
	if v := configLoggers.Load(); v != nil {
		cLoggers = v.(map[string]privateConfig)
	}
	if cLoggers == nil {
		Status.WithSkip(1).Fatal("should call refresh first")
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
		data, err := io.ReadAll(input)
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
			v := reflect.New(p.Class)
			ev := v.Elem()
			err := inject(ev, ev.Type(), c)
			if err != nil {
				return err
			}
			i, ok := v.Interface().(Initializer)
			if ok {
				if err = i.Init(); err != nil {
					return err
				}
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
				c.Attributes["name"] = c.Label
			}

			p, ok := plugins[c.Label]
			if !ok || p == nil {
				return fmt.Errorf("plugin %s not found", c.Label)
			}
			name, ok := c.Attributes["name"]
			if !ok {
				return errors.New("attribute 'name' not found")
			}

			v := reflect.New(p.Class)
			ev := v.Elem()
			err := inject(ev, ev.Type(), c)
			if err != nil {
				return err
			}
			i, ok := v.Interface().(Initializer)
			if ok {
				if err = i.Init(); err != nil {
					return err
				}
			}

			config := v.Interface().(privateConfig)
			if isRootLogger {
				cRoot = config
			}
			cLoggers[name] = config
		}
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

	configLoggers.Store(cLoggers)

	// 对用户代码中的 Logger 对象应用最新的配置。
	usingLoggers.Range(func(key, value interface{}) bool {
		l := value.(loggerHolder).Get()
		l.reconfigure(cLoggers[key.(string)])
		return true
	})

	return nil
}

type Logger struct {
	value atomic.Value
	name  string
	entry SimpleEntry
	level Level
}

// wrapperConfig atomic.Value 要求底层数据完全一致。
type wrapperConfig struct {
	config privateConfig
}

func NewLogger(name string, level Level) *Logger {
	l := &Logger{name: name, level: level}
	l.reconfigure(nil)
	l.entry.pub = l
	return l
}

// Name returns the logger's name.
func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) config() privateConfig {
	v := l.value.Load().(*wrapperConfig)
	if v.config == empty {
		return nil
	}
	return v.config
}

func (l *Logger) reconfigure(config privateConfig) {
	if config == nil {
		l.value.Store(&wrapperConfig{empty})
	} else {
		l.value.Store(&wrapperConfig{config})
	}
}

func (l *Logger) filter(level Level, e Entry, msg Message) Result {
	if level >= l.level {
		return ResultAccept
	}
	return ResultDeny
}

func (l *Logger) publish(e *Event) {
	console.Append(e)
}

func (l *Logger) Level() Level {
	if c := l.config(); c != nil {
		return c.getLevel()
	}
	return l.level
}

func (l *Logger) Filter() Filter {
	if c := l.config(); c != nil {
		return c.getFilter()
	}
	return nil
}

func (l *Logger) Appenders() []Appender {
	if c := l.config(); c != nil {
		var appenders []Appender
		for _, ref := range c.getAppenders() {
			appenders = append(appenders, ref.appender)
		}
		return appenders
	}
	return nil
}

// WithSkip 创建包含 skip 信息的 Entry 。
func (l *Logger) WithSkip(n int) SimpleEntry {
	if c := l.config(); c != nil {
		return SimpleEntry{
			pub:  c,
			skip: n,
		}
	}
	return l.entry.WithSkip(n)
}

// WithTag 创建包含 tag 信息的 Entry 。
func (l *Logger) WithTag(tag string) SimpleEntry {
	if c := l.config(); c != nil {
		return SimpleEntry{
			pub: c,
			tag: tag,
		}
	}
	return l.entry.WithTag(tag)
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func (l *Logger) WithContext(ctx context.Context) ContextEntry {
	if c := l.config(); c != nil {
		return ContextEntry{
			pub: c,
			ctx: ctx,
		}
	}
	return l.entry.WithContext(ctx)
}

// Trace outputs log with level TraceLevel.
func (l *Logger) Trace(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Trace(args...)
	}
	return l.entry.WithSkip(1).Trace(args...)
}

// Tracef outputs log with level TraceLevel.
func (l *Logger) Tracef(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Tracef(format, args...)
	}
	return l.entry.WithSkip(1).Tracef(format, args...)
}

// Debug outputs log with level DebugLevel.
func (l *Logger) Debug(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Debug(args...)
	}
	return l.entry.WithSkip(1).Debug(args...)
}

// Debugf outputs log with level DebugLevel.
func (l *Logger) Debugf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Debugf(format, args...)
	}
	return l.entry.WithSkip(1).Debugf(format, args...)
}

// Info outputs log with level InfoLevel.
func (l *Logger) Info(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Info(args...)
	}
	return l.entry.WithSkip(1).Info(args...)
}

// Infof outputs log with level InfoLevel.
func (l *Logger) Infof(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Infof(format, args...)
	}
	return l.entry.WithSkip(1).Infof(format, args...)
}

// Warn outputs log with level WarnLevel.
func (l *Logger) Warn(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Warn(args...)
	}
	return l.entry.WithSkip(1).Warn(args...)
}

// Warnf outputs log with level WarnLevel.
func (l *Logger) Warnf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Warnf(format, args...)
	}
	return l.entry.WithSkip(1).Warnf(format, args...)
}

// Error outputs log with level ErrorLevel.
func (l *Logger) Error(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Error(args...)
	}
	return l.entry.WithSkip(1).Error(args...)
}

// Errorf outputs log with level ErrorLevel.
func (l *Logger) Errorf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Errorf(format, args...)
	}
	return l.entry.WithSkip(1).Errorf(format, args...)
}

// Panic outputs log with level PanicLevel.
func (l *Logger) Panic(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Panic(args...)
	}
	return l.entry.WithSkip(1).Panic(args...)
}

// Panicf outputs log with level PanicLevel.
func (l *Logger) Panicf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Panicf(format, args...)
	}
	return l.entry.WithSkip(1).Panicf(format, args...)
}

// Fatal outputs log with level FatalLevel.
func (l *Logger) Fatal(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Fatal(args...)
	}
	return l.entry.WithSkip(1).Fatal(args...)
}

// Fatalf outputs log with level FatalLevel.
func (l *Logger) Fatalf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.getEntry().WithSkip(1).Fatalf(format, args...)
	}
	return l.entry.WithSkip(1).Fatalf(format, args...)
}
