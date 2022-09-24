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

type SugarSimpleEntry struct {
	e SimpleEntry
}

// Trace outputs log with level TraceLevel.
func (s SugarSimpleEntry) Trace(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, TraceLevel, s.e.skip, &s.e, fields)
}

// Tracef outputs log with level TraceLevel.
func (s SugarSimpleEntry) Tracef(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, TraceLevel, s.e.skip, &s.e, fields)
}

// Debug outputs log with level DebugLevel.
func (s SugarSimpleEntry) Debug(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, DebugLevel, s.e.skip, &s.e, fields)
}

// Debugf outputs log with level DebugLevel.
func (s SugarSimpleEntry) Debugf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, DebugLevel, s.e.skip, &s.e, fields)
}

// Info outputs log with level InfoLevel.
func (s SugarSimpleEntry) Info(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, InfoLevel, s.e.skip, &s.e, fields)
}

// Infof outputs log with level InfoLevel.
func (s SugarSimpleEntry) Infof(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, InfoLevel, s.e.skip, &s.e, fields)
}

// Warn outputs log with level WarnLevel.
func (s SugarSimpleEntry) Warn(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, WarnLevel, s.e.skip, &s.e, fields)
}

// Warnf outputs log with level WarnLevel.
func (s SugarSimpleEntry) Warnf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, WarnLevel, s.e.skip, &s.e, fields)
}

// Error outputs log with level ErrorLevel.
func (s SugarSimpleEntry) Error(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, ErrorLevel, s.e.skip, &s.e, fields)
}

// Errorf outputs log with level ErrorLevel.
func (s SugarSimpleEntry) Errorf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, ErrorLevel, s.e.skip, &s.e, fields)
}

// Panic outputs log with level PanicLevel.
func (s SugarSimpleEntry) Panic(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, PanicLevel, s.e.skip, &s.e, fields)
}

// Panicf outputs log with level PanicLevel.
func (s SugarSimpleEntry) Panicf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, PanicLevel, s.e.skip, &s.e, fields)
}

// Fatal outputs log with level FatalLevel.
func (s SugarSimpleEntry) Fatal(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, FatalLevel, s.e.skip, &s.e, fields)
}

// Fatalf outputs log with level FatalLevel.
func (s SugarSimpleEntry) Fatalf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, FatalLevel, s.e.skip, &s.e, fields)
}

type SugarContextEntry struct {
	e ContextEntry
}

// Trace outputs log with level TraceLevel.
func (s SugarContextEntry) Trace(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, TraceLevel, s.e.skip, &s.e, fields)
}

// Tracef outputs log with level TraceLevel.
func (s SugarContextEntry) Tracef(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, TraceLevel, s.e.skip, &s.e, fields)
}

// Debug outputs log with level DebugLevel.
func (s SugarContextEntry) Debug(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, DebugLevel, s.e.skip, &s.e, fields)
}

// Debugf outputs log with level DebugLevel.
func (s SugarContextEntry) Debugf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, DebugLevel, s.e.skip, &s.e, fields)
}

// Info outputs log with level InfoLevel.
func (s SugarContextEntry) Info(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, InfoLevel, s.e.skip, &s.e, fields)
}

// Infof outputs log with level InfoLevel.
func (s SugarContextEntry) Infof(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, InfoLevel, s.e.skip, &s.e, fields)
}

// Warn outputs log with level WarnLevel.
func (s SugarContextEntry) Warn(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, WarnLevel, s.e.skip, &s.e, fields)
}

// Warnf outputs log with level WarnLevel.
func (s SugarContextEntry) Warnf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, WarnLevel, s.e.skip, &s.e, fields)
}

// Error outputs log with level ErrorLevel.
func (s SugarContextEntry) Error(errno Errno, args ...interface{}) *Event {
	s.e.errno = errno
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, ErrorLevel, s.e.skip, &s.e, fields)
}

// Errorf outputs log with level ErrorLevel.
func (s SugarContextEntry) Errorf(errno Errno, format string, args ...interface{}) *Event {
	s.e.errno = errno
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, ErrorLevel, s.e.skip, &s.e, fields)
}

// Panic outputs log with level PanicLevel.
func (s SugarContextEntry) Panic(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, PanicLevel, s.e.skip, &s.e, fields)
}

// Panicf outputs log with level PanicLevel.
func (s SugarContextEntry) Panicf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, PanicLevel, s.e.skip, &s.e, fields)
}

// Fatal outputs log with level FatalLevel.
func (s SugarContextEntry) Fatal(args ...interface{}) *Event {
	fields := []Field{Message("", args...)}
	return publish(s.e.pub, FatalLevel, s.e.skip, &s.e, fields)
}

// Fatalf outputs log with level FatalLevel.
func (s SugarContextEntry) Fatalf(format string, args ...interface{}) *Event {
	fields := []Field{Message(format, args...)}
	return publish(s.e.pub, FatalLevel, s.e.skip, &s.e, fields)
}

type SugarLogger struct {
	l *Logger
}

// Trace outputs log with level TraceLevel.
func (s *SugarLogger) Trace(args ...interface{}) *Event {
	c, ok := s.l.enableLog(TraceLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Trace(args...)
}

// Tracef outputs log with level TraceLevel.
func (s *SugarLogger) Tracef(format string, args ...interface{}) *Event {
	c, ok := s.l.enableLog(TraceLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Tracef(format, args...)
}

// Debug outputs log with level DebugLevel.
func (s *SugarLogger) Debug(args ...interface{}) *Event {
	c, ok := s.l.enableLog(DebugLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Debug(args...)
}

// Debugf outputs log with level DebugLevel.
func (s *SugarLogger) Debugf(format string, args ...interface{}) *Event {
	c, ok := s.l.enableLog(DebugLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Debugf(format, args...)
}

// Info outputs log with level InfoLevel.
func (s *SugarLogger) Info(args ...interface{}) *Event {
	c, ok := s.l.enableLog(InfoLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Info(args...)
}

// Infof outputs log with level InfoLevel.
func (s *SugarLogger) Infof(format string, args ...interface{}) *Event {
	c, ok := s.l.enableLog(InfoLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Infof(format, args...)
}

// Warn outputs log with level WarnLevel.
func (s *SugarLogger) Warn(args ...interface{}) *Event {
	c, ok := s.l.enableLog(WarnLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Warn(args...)
}

// Warnf outputs log with level WarnLevel.
func (s *SugarLogger) Warnf(format string, args ...interface{}) *Event {
	c, ok := s.l.enableLog(WarnLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Warnf(format, args...)
}

// Error outputs log with level ErrorLevel.
func (s *SugarLogger) Error(args ...interface{}) *Event {
	c, ok := s.l.enableLog(ErrorLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Error(args...)
}

// Errorf outputs log with level ErrorLevel.
func (s *SugarLogger) Errorf(format string, args ...interface{}) *Event {
	c, ok := s.l.enableLog(ErrorLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Errorf(format, args...)
}

// Panic outputs log with level PanicLevel.
func (s *SugarLogger) Panic(args ...interface{}) *Event {
	c, ok := s.l.enableLog(PanicLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Panic(args...)
}

// Panicf outputs log with level PanicLevel.
func (s *SugarLogger) Panicf(format string, args ...interface{}) *Event {
	c, ok := s.l.enableLog(PanicLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Panicf(format, args...)
}

// Fatal outputs log with level FatalLevel.
func (s *SugarLogger) Fatal(args ...interface{}) *Event {
	c, ok := s.l.enableLog(FatalLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Fatal(args...)
}

// Fatalf outputs log with level FatalLevel.
func (s *SugarLogger) Fatalf(format string, args ...interface{}) *Event {
	c, ok := s.l.enableLog(FatalLevel)
	if !ok {
		return nil
	}
	return c.getEntry().WithSkip(1).Sugar().Fatalf(format, args...)
}
