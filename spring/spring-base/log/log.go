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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-spring/spring-base/atomic"
)

const (
	RootLoggerName      = "Root"
	AsyncRootLoggerName = "AsyncRoot"
)

var (
	loggers = map[string]*Logger{}
)

// Filter 日志过滤器。
type Filter interface {
	Filter(level Level, e Entry, msg Message) bool
}

// Appender 日志输出器。不要试图实现异步的 Appender，因为我们会提供异步的 Logger。
type Appender interface {
	Append(e *Event)
}

// LifeCycle 生命周期。
type LifeCycle interface {
	Start() error
	Stop(ctx context.Context)
}

type Node struct {
	Label      string
	Attributes map[string]string
	Children   []*Node
}

func (node *Node) child(label string) *Node {
	for _, c := range node.Children {
		if c.Label == label {
			return c
		}
	}
	return nil
}

// GetLogger 获取名字为声明位置的包名的 *Logger 对象。
func GetLogger(level ...Level) *Logger {
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	i := strings.LastIndex(funcName, "/")
	j := strings.Index(funcName[i:], ".")
	return getLogger(funcName[:i+j], level...)
}

// getLogger 获取名字为 name 的 *Logger 对象。
func getLogger(name string, level ...Level) *Logger {
	if l, ok := loggers[name]; ok {
		return l
	}
	if len(level) == 0 {
		level = append(level, InfoLevel)
	}
	l := NewLogger(name, level[0])
	loggers[name] = l
	return l
}

type Configuration struct {
	root      PrivateConfig
	appenders map[string]Appender
	loggers   map[string]PrivateConfig
}

// Refresh 加载日志配置文件。
func Refresh(fileName string) error {

	ext := filepath.Ext(fileName)
	r, ok := readers[ext]
	if !ok {
		return fmt.Errorf("unsupported file type %s", ext)
	}

	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	rootNode, err := r.Read(b)
	if err != nil {
		return err
	}

	if rootNode.Label != "Configuration" {
		return errors.New("no Configuration root")
	}

	c := &Configuration{
		appenders: make(map[string]Appender),
		loggers:   make(map[string]PrivateConfig),
	}

	if n := rootNode.child("Appenders"); n != nil {
		for _, node := range n.Children {
			p := getPluginByName(node.Label)
			if p == nil {
				return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxx")
			}
			name, ok := node.Attributes["name"]
			if !ok {
				return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxx")
			}
			v := reflect.New(p.Class)
			err = inject(v.Elem(), v.Type().Elem(), node)
			if err != nil {
				return err
			}
			c.appenders[name] = v.Interface().(Appender)
		}
	}

	if n := rootNode.child("Loggers"); n != nil {
		for _, node := range n.Children {
			isRootLogger := false
			if node.Label == RootLoggerName {
				isRootLogger = true
				node.Attributes["name"] = RootLoggerName
			} else if node.Label == AsyncRootLoggerName {
				isRootLogger = true
				node.Attributes["name"] = AsyncRootLoggerName
			}
			p := getPluginByName(node.Label)
			if p == nil {
				return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxx")
			}
			name, ok := node.Attributes["name"]
			if !ok {
				return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxx")
			}
			v := reflect.New(p.Class)
			err = inject(v.Elem(), v.Type().Elem(), node)
			if err != nil {
				return err
			}
			config := v.Interface().(PrivateConfig)
			if isRootLogger {
				if c.root != nil {
					return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxx")
				}
				c.root = config
			}
			c.loggers[name] = config
		}
	}

	for name, config := range c.loggers {

		var base *BaseLoggerConfig
		switch v := config.(type) {
		case *LoggerConfig:
			base = &v.BaseLoggerConfig
		case *AsyncLoggerConfig:
			base = &v.BaseLoggerConfig
		}

		if name != c.root.GetName() {
			base.parent = c.root
		}

		for {
			n := strings.LastIndex(name, "/")
			if n < 0 {
				break
			}
			name = name[:n]
			if parent, ok := c.loggers[name]; ok {
				base.parent = parent
			}
		}

		for _, ref := range base.AppenderRefs {
			appender, ok := c.appenders[ref.Ref]
			if !ok {
				return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxx")
			}
			ref.appender = appender
		}

		if err = config.start(); err != nil {
			return err
		}
	}

	for name, l := range loggers {
		l.reconfigure(c.loggers[name])
	}

	return nil
}

type Logger struct {
	name  string
	entry SimpleEntry
	value atomic.Value
}

func NewLogger(name string, level Level) *Logger {
	l := &Logger{
		name: name,
		entry: SimpleEntry{
			pub: &pubAppender{
				level:    level,
				appender: &ConsoleAppender{},
			},
		},
	}
	l.reconfigure(nil)
	return l
}

// Name ...
func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) config() PrivateConfig {
	v := l.value.Load()
	if v == empty {
		return nil
	}
	return v.(PrivateConfig)
}

func (l *Logger) reconfigure(config PrivateConfig) {
	if config == nil {
		l.value.Store(empty)
	} else {
		l.value.Store(config)
	}
}

func (l *Logger) Level() Level {
	if c := l.config(); c != nil {
		return c.GetLevel()
	}
	return l.entry.pub.(*pubAppender).level
}

func (l *Logger) GetFilters() []Filter {
	if c := l.config(); c != nil {
		return c.GetFilters()
	}
	return nil
}

func (l *Logger) GetAppenders() []*AppenderRef {
	if c := l.config(); c != nil {
		return c.GetAppenders()
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
		return SimpleEntry{pub: c, tag: tag}
	}
	return l.entry.WithTag(tag)
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func (l *Logger) WithContext(ctx context.Context) ContextEntry {
	if c := l.config(); c != nil {
		return ContextEntry{pub: c, ctx: ctx}
	}
	return l.entry.WithContext(ctx)
}

// Trace outputs log with level TraceLevel.
func (l *Logger) Trace(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Trace(args...)
	}
	return l.entry.WithSkip(1).Trace(args...)
}

// Tracef outputs log with level TraceLevel.
func (l *Logger) Tracef(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Tracef(format, args...)
	}
	return l.entry.WithSkip(1).Tracef(format, args...)
}

// Debug outputs log with level DebugLevel.
func (l *Logger) Debug(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Debug(args...)
	}
	return l.entry.WithSkip(1).Debug(args...)
}

// Debugf outputs log with level DebugLevel.
func (l *Logger) Debugf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Debugf(format, args...)
	}
	return l.entry.WithSkip(1).Debugf(format, args...)
}

// Info outputs log with level InfoLevel.
func (l *Logger) Info(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Info(args...)
	}
	return l.entry.WithSkip(1).Info(args...)
}

// Infof outputs log with level InfoLevel.
func (l *Logger) Infof(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Infof(format, args...)
	}
	return l.entry.WithSkip(1).Infof(format, args...)
}

// Warn outputs log with level WarnLevel.
func (l *Logger) Warn(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Warn(args...)
	}
	return l.entry.WithSkip(1).Warn(args...)
}

// Warnf outputs log with level WarnLevel.
func (l *Logger) Warnf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Warnf(format, args...)
	}
	return l.entry.WithSkip(1).Warnf(format, args...)
}

// Error outputs log with level ErrorLevel.
func (l *Logger) Error(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Error(args...)
	}
	return l.entry.WithSkip(1).Error(args...)
}

// Errorf outputs log with level ErrorLevel.
func (l *Logger) Errorf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Errorf(format, args...)
	}
	return l.entry.WithSkip(1).Errorf(format, args...)
}

// Panic outputs log with level PanicLevel.
func (l *Logger) Panic(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Panic(args...)
	}
	return l.entry.WithSkip(1).Panic(args...)
}

// Panicf outputs log with level PanicLevel.
func (l *Logger) Panicf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Panicf(format, args...)
	}
	return l.entry.WithSkip(1).Panicf(format, args...)
}

// Fatal outputs log with level FatalLevel.
func (l *Logger) Fatal(args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Fatal(args...)
	}
	return l.entry.WithSkip(1).Fatal(args...)
}

// Fatalf outputs log with level FatalLevel.
func (l *Logger) Fatalf(format string, args ...interface{}) *Event {
	if c := l.config(); c != nil {
		return c.GetEntry().WithSkip(1).Fatalf(format, args...)
	}
	return l.entry.WithSkip(1).Fatalf(format, args...)
}
