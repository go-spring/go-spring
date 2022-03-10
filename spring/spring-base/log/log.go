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
	"encoding/xml"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/go-spring/spring-base/atomic"
)

const RootLoggerName = "Root"

var (
	usingLoggers      = map[string]*Logger{}
	appenderFactories = map[string]AppenderFactory{}
)

type AppenderConfig interface {
	GetName() string
}

// AppenderFactory 定义 Appender 工厂。
type AppenderFactory interface {
	NewAppenderConfig() AppenderConfig
	NewAppender(config AppenderConfig) (Appender, error)
}

// RegisterAppenderFactory 注册 Appender 工厂。
func RegisterAppenderFactory(appender string, factory AppenderFactory) {
	appenderFactories[appender] = factory
}

type LoggerConfig struct {
	Level     Level
	Appenders []Appender
}

type Logger struct {
	name   string
	entry  BaseEntry
	config atomic.Value
}

// GetRootLogger 获取根 *Logger 对象。
func GetRootLogger() *Logger {
	return GetLogger(RootLoggerName)
}

// GetLogger 获取名为 name 的 *Logger 对象。
func GetLogger(name ...string) *Logger {
	if len(name) == 0 {
		if pc, _, _, ok := runtime.Caller(1); ok {
			funcName := runtime.FuncForPC(pc).Name()
			i := strings.LastIndex(funcName, "/")
			j := strings.Index(funcName[i:], ".")
			name = append(name, funcName[:i+j])
		} else {
			name = append(name, RootLoggerName)
		}
	}
	l, ok := usingLoggers[name[0]]
	if ok {
		return l
	}
	l = &Logger{name: name[0]}
	l.entry.logger = l
	usingLoggers[l.name] = l
	return l
}

func NewLogger(config *LoggerConfig) *Logger {
	l := &Logger{}
	l.entry.logger = l
	l.config.Store(config)
	return l
}

func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) getConfig() *LoggerConfig {
	config, _ := l.config.Load().(*LoggerConfig)
	return config
}

// WithSkip 创建包含 skip 信息的 Entry 。
func (l *Logger) WithSkip(n int) BaseEntry {
	return l.entry.WithSkip(n)
}

// WithTag 创建包含 tag 信息的 Entry 。
func (l *Logger) WithTag(tag string) BaseEntry {
	return l.entry.WithTag(tag)
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func (l *Logger) WithContext(ctx context.Context) CtxEntry {
	return l.entry.WithContext(ctx)
}

// Trace 输出 TRACE 级别的日志。
func (l *Logger) Trace(args ...interface{}) {
	printf(TraceLevel, &l.entry, "", args)
}

// Tracef 输出 TRACE 级别的日志。
func (l *Logger) Tracef(format string, args ...interface{}) {
	printf(TraceLevel, &l.entry, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func (l *Logger) Debug(args ...interface{}) {
	printf(DebugLevel, &l.entry, "", args)
}

// Debugf 输出 DEBUG 级别的日志。
func (l *Logger) Debugf(format string, args ...interface{}) {
	printf(DebugLevel, &l.entry, format, args)
}

// Info 输出 INFO 级别的日志。
func (l *Logger) Info(args ...interface{}) {
	printf(InfoLevel, &l.entry, "", args)
}

// Infof 输出 INFO 级别的日志。
func (l *Logger) Infof(format string, args ...interface{}) {
	printf(InfoLevel, &l.entry, format, args)
}

// Warn 输出 WARN 级别的日志。
func (l *Logger) Warn(args ...interface{}) {
	printf(WarnLevel, &l.entry, "", args)
}

// Warnf 输出 WARN 级别的日志。
func (l *Logger) Warnf(format string, args ...interface{}) {
	printf(WarnLevel, &l.entry, format, args)
}

// Error 输出 ERROR 级别的日志。
func (l *Logger) Error(args ...interface{}) {
	printf(ErrorLevel, &l.entry, "", args)
}

// Errorf 输出 ERROR 级别的日志。
func (l *Logger) Errorf(format string, args ...interface{}) {
	printf(ErrorLevel, &l.entry, format, args)
}

// Panic 输出 PANIC 级别的日志。
func (l *Logger) Panic(args ...interface{}) {
	printf(PanicLevel, &l.entry, "", args)
}

// Panicf 输出 PANIC 级别的日志。
func (l *Logger) Panicf(format string, args ...interface{}) {
	printf(PanicLevel, &l.entry, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func (l *Logger) Fatal(args ...interface{}) {
	printf(FatalLevel, &l.entry, "", args)
}

// Fatalf 输出 FATAL 级别的日志。
func (l *Logger) Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, &l.entry, format, args)
}

// Load 加载日志配置文件。
func Load(configFile string) error {

	const (
		EnterConfiguration = 1
		ExitConfiguration  = 2
		EnterAppenders     = 3
		ExitAppenders      = 4
		EnterLoggers       = 5
		ExitLoggers        = 6
	)

	state := 0
	configLoggers := map[string]*Logger{}
	configAppenders := map[string]Appender{}
	d := xml.NewDecoder(strings.NewReader(configFile))
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Configuration":
				state = EnterConfiguration
				continue
			case "Appenders":
				state = EnterAppenders
				continue
			case "Loggers":
				state = EnterLoggers
				continue
			}
			if state == EnterAppenders {
				factory, ok := appenderFactories[t.Name.Local]
				if !ok {
					return fmt.Errorf("no appender factory `%s` found", t.Name.Local)
				}
				config := factory.NewAppenderConfig()
				err = d.DecodeElement(&config, &t)
				if err != nil {
					return err
				}
				var appender Appender
				appender, err = factory.NewAppender(config)
				if err != nil {
					return err
				}
				configAppenders[config.GetName()] = appender
				continue
			}
			if state == EnterLoggers {
				var config struct {
					Name         string `xml:"name,attr"`
					Level        string `xml:"level,attr"`
					AppenderRefs []struct {
						Ref string `xml:"ref,attr"`
					} `xml:"AppenderRef"`
				}
				err = d.DecodeElement(&config, &t)
				if err != nil {
					return err
				}
				if t.Name.Local == RootLoggerName {
					config.Name = RootLoggerName
				}
				level := StringToLevel(config.Level)
				if level == NoneLevel {
					return fmt.Errorf("error level `%s` for logger `%s`", config.Level, config.Name)
				}
				var appenders []Appender
				for _, ref := range config.AppenderRefs {
					v, ok := configAppenders[ref.Ref]
					if !ok {
						return fmt.Errorf("no appender ref `%s` found", ref.Ref)
					}
					appenders = append(appenders, v)
				}
				l := &Logger{}
				l.entry.logger = l
				l.config.Store(&LoggerConfig{
					Level:     level,
					Appenders: appenders,
				})
				configLoggers[config.Name] = l
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "Configuration":
				state = ExitConfiguration
				continue
			case "Appenders":
				state = ExitAppenders
				continue
			case "Loggers":
				state = ExitLoggers
				continue
			}
		}
	}

	rootLogger, ok := configLoggers[RootLoggerName]
	if !ok {
		return fmt.Errorf("no logger `%s` found", RootLoggerName)
	}

	for name, usingLogger := range usingLoggers {
		if l, ok := configLoggers[name]; ok {
			usingLogger.config.Store(l.config.Load())
		} else {
			usingLogger.config.Store(rootLogger.config.Load())
		}
	}
	return nil
}
