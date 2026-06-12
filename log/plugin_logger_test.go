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

package log

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func TestParseBufferFullPolicy(t *testing.T) {
	_, err := ParseBufferFullPolicy("Block")
	assert.Error(t, err).Matches("invalid BufferFullPolicy Block")

	p, err := ParseBufferFullPolicy("block")
	assert.Error(t, err).Nil()
	assert.That(t, p).Equal(BufferFullPolicyBlock)

	p, err = ParseBufferFullPolicy("discard")
	assert.Error(t, err).Nil()
	assert.That(t, p).Equal(BufferFullPolicyDiscard)

	p, err = ParseBufferFullPolicy("drop-oldest")
	assert.Error(t, err).Nil()
	assert.That(t, p).Equal(BufferFullPolicyDropOldest)
}

type CountAppender struct {
	Appender
	count int
}

func (c *CountAppender) Append(e *Event) {
	c.count++
	c.Appender.Append(e)
}

func TestLoggerConfig(t *testing.T) {

	//t.Run("write", func(t *testing.T) {
	//	a := &CountAppender{
	//		Appender: &DiscardAppender{},
	//	}
	//
	//	err := a.Start()
	//	assert.Error(t, err).Nil()
	//
	//	l := &SyncLogger{
	//		AppenderRefs: AppenderRefs{
	//			AppenderRefs: []*AppenderRef{
	//				{Appender: a},
	//			},
	//		},
	//	}
	//
	//	l.Write(InfoLevel, []byte("test"))
	//	assert.That(t, a.count).Equal(0)
	//
	//	l.Stop()
	//	a.Stop()
	//})

	t.Run("success", func(t *testing.T) {
		a := &CountAppender{
			Appender: &DiscardAppender{},
		}

		err := a.Start()
		assert.Error(t, err).Nil()

		l := &SyncLogger{
			LoggerBase: LoggerBase{
				Level: LevelRange{
					MinLevel: InfoLevel,
					MaxLevel: MaxLevel,
				},
				Tags: []string{"_com_*"},
			},
			AppenderRefs: []*AppenderRef{
				{
					Appender: a,
					Level: LevelRange{
						MinLevel: NoneLevel,
						MaxLevel: MaxLevel,
					},
				},
			},
		}

		err = l.Start()
		assert.Error(t, err).Nil()

		assert.That(t, l.Level.Enable(TraceLevel)).False()
		assert.That(t, l.Level.Enable(DebugLevel)).False()
		assert.That(t, l.Level.Enable(InfoLevel)).True()
		assert.That(t, l.Level.Enable(WarnLevel)).True()
		assert.That(t, l.Level.Enable(ErrorLevel)).True()
		assert.That(t, l.Level.Enable(PanicLevel)).True()
		assert.That(t, l.Level.Enable(FatalLevel)).True()

		for range 5 {
			l.Append(&Event{Level: InfoLevel})
		}

		assert.That(t, a.count).Equal(5)

		l.Stop()
		a.Stop()
	})
}

func TestAsyncLoggerConfig(t *testing.T) {

	t.Run("enable level", func(t *testing.T) {
		l := &AsyncLogger{
			LoggerBase: LoggerBase{
				Level: LevelRange{
					MinLevel: InfoLevel,
					MaxLevel: MaxLevel,
				},
			},
		}

		assert.That(t, l.Level.Enable(TraceLevel)).False()
		assert.That(t, l.Level.Enable(DebugLevel)).False()
		assert.That(t, l.Level.Enable(InfoLevel)).True()
		assert.That(t, l.Level.Enable(WarnLevel)).True()
		assert.That(t, l.Level.Enable(ErrorLevel)).True()
		assert.That(t, l.Level.Enable(PanicLevel)).True()
		assert.That(t, l.Level.Enable(FatalLevel)).True()
	})

	t.Run("error BufferSize", func(t *testing.T) {
		l := &AsyncLogger{
			LoggerBase: LoggerBase{
				Name: "file",
			},
			BufferSize: 10,
		}

		err := l.Start()
		assert.Error(t, err).Matches("bufferSize is too small")
	})

	t.Run("buffer full - discard", func(t *testing.T) {
		a := &CountAppender{
			Appender: &DiscardAppender{},
		}

		err := a.Start()
		assert.Error(t, err).Nil()

		l := &AsyncLogger{
			LoggerBase: LoggerBase{
				Level: LevelRange{
					MinLevel: InfoLevel,
					MaxLevel: MaxLevel,
				},
				Tags: []string{"_com_*"},
			},
			AppenderRefs: []*AppenderRef{
				{
					Appender: a,
					Level: LevelRange{
						MinLevel: NoneLevel,
						MaxLevel: MaxLevel,
					},
				},
			},
			BufferSize:   100,
			OnBufferFull: BufferFullPolicyDiscard,
		}

		err = l.Start()
		assert.Error(t, err).Nil()

		//go func() {
		//	for range 100 {
		//		l.Write(InfoLevel, []byte("hello"))
		//	}
		//}()

		for range 5000 {
			e := &Event{}
			e.Level = InfoLevel
			l.Append(e)
		}

		time.Sleep(200 * time.Millisecond)

		l.Stop()
		a.Stop()

		assert.That(t, l.GetDiscardCounter() > 0).True()
	})

	t.Run("buffer full - discard oldest", func(t *testing.T) {
		a := &CountAppender{
			Appender: &DiscardAppender{},
		}

		err := a.Start()
		assert.Error(t, err).Nil()

		l := &AsyncLogger{
			LoggerBase: LoggerBase{
				Level: LevelRange{
					MinLevel: InfoLevel,
					MaxLevel: MaxLevel,
				},
				Tags: []string{"_com_*"},
			},
			AppenderRefs: []*AppenderRef{
				{
					Appender: a,
					Level: LevelRange{
						MinLevel: NoneLevel,
						MaxLevel: MaxLevel,
					},
				},
			},
			BufferSize:   100,
			OnBufferFull: BufferFullPolicyDropOldest,
		}

		err = l.Start()
		assert.Error(t, err).Nil()

		//go func() {
		//	for range 100 {
		//		l.Write(InfoLevel, []byte("hello"))
		//	}
		//}()

		for range 5000 {
			e := &Event{}
			e.Level = InfoLevel
			l.Append(e)
		}

		time.Sleep(200 * time.Millisecond)

		l.Stop()
		a.Stop()

		assert.That(t, l.GetDiscardCounter() > 0).True()
	})

	t.Run("buffer full - block", func(t *testing.T) {
		a := &CountAppender{
			Appender: &DiscardAppender{},
		}

		err := a.Start()
		assert.Error(t, err).Nil()

		l := &AsyncLogger{
			LoggerBase: LoggerBase{
				Level: LevelRange{
					MinLevel: InfoLevel,
					MaxLevel: MaxLevel,
				},
				Tags: []string{"_com_*"},
			},
			AppenderRefs: []*AppenderRef{
				{Appender: a},
			},
			BufferSize:   100,
			OnBufferFull: BufferFullPolicyBlock,
		}

		err = l.Start()
		assert.Error(t, err).Nil()

		//go func() {
		//	for range 100 {
		//		l.Write(InfoLevel, []byte("hello"))
		//	}
		//}()

		for range 5000 {
			l.Append(&Event{})
		}

		time.Sleep(200 * time.Millisecond)

		l.Stop()
		a.Stop()

		assert.That(t, l.GetDiscardCounter() == 0).True()
	})

	t.Run("success", func(t *testing.T) {
		a := &CountAppender{
			Appender: &DiscardAppender{},
		}

		err := a.Start()
		assert.Error(t, err).Nil()

		l := &AsyncLogger{
			LoggerBase: LoggerBase{
				Level: LevelRange{
					MinLevel: InfoLevel,
					MaxLevel: MaxLevel,
				},
				Tags: []string{"_com_*"},
			},
			AppenderRefs: []*AppenderRef{
				{
					Appender: a,
					Level: LevelRange{
						MinLevel: NoneLevel,
						MaxLevel: MaxLevel,
					},
				},
			},
			BufferSize: 100,
		}

		err = l.Start()
		assert.Error(t, err).Nil()

		for range 5 {
			e := &Event{}
			e.Level = InfoLevel
			l.Append(e)
		}

		time.Sleep(100 * time.Millisecond)
		assert.That(t, a.count).Equal(5)

		l.Stop()
		a.Stop()
	})

	//t.Run("write with discard policy", func(t *testing.T) {
	//	a := &CountAppender{
	//		Appender: &DiscardAppender{},
	//	}
	//
	//	err := a.Start()
	//	assert.Error(t, err).Nil()
	//
	//	l := &AsyncLogger{
	//		AppenderRefs: AppenderRefs{
	//			AppenderRefs: []*AppenderRef{
	//				{Appender: a},
	//			},
	//		},
	//		BufferSize:       100,
	//		BufferFullPolicy: BufferFullPolicyDiscard,
	//	}
	//
	//	err = l.Start()
	//	assert.Error(t, err).Nil()
	//
	//	// Rapidly write large amount of data to fill the buffer
	//	for range 500 {
	//		l.Write(InfoLevel, []byte("test data"))
	//	}
	//
	//	time.Sleep(100 * time.Millisecond)
	//	l.Stop()
	//	a.Stop()
	//
	//	// Some data should be discarded
	//	assert.That(t, l.GetDiscardCounter() > 0).True()
	//})
}

func TestRollingFileLoggerStartErrorCleansAppenders(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "app.log.")
	t.Cleanup(func() {
		closeOpenFilesWithPrefix(prefix)
	})

	l := &RollingFileLogger{
		LoggerBase: LoggerBase{
			Level: LevelRange{
				MinLevel: InfoLevel,
				MaxLevel: MaxLevel,
			},
		},
		Layout:       &TextLayout{},
		FileDir:      dir,
		FileName:     "app.log",
		Interval:     time.Hour,
		AsyncWrite:   true,
		BufferSize:   10,
		OnBufferFull: BufferFullPolicyDiscard,
	}

	err := l.Start()
	assert.Error(t, err).Matches("bufferSize is too small")
	assert.That(t, countOpenFilesWithPrefix(prefix)).Equal(0)
}

func countOpenFilesWithPrefix(prefix string) int {
	fileManager.mutex.Lock()
	defer fileManager.mutex.Unlock()

	count := 0
	for name := range fileManager.files {
		if strings.HasPrefix(name, prefix) {
			count++
		}
	}
	return count
}

func closeOpenFilesWithPrefix(prefix string) {
	for {
		fileManager.mutex.Lock()
		var f *File
		for name, openFile := range fileManager.files {
			if strings.HasPrefix(name, prefix) {
				f = openFile
				break
			}
		}
		fileManager.mutex.Unlock()

		if f == nil {
			return
		}
		CloseFile(f)
	}
}
