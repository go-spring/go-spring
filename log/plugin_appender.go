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
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"go-spring.org/stdlib/errutil"
)

var (
	// bufferCap defines maximum buffer capacity eligible for reuse.
	// Buffers with capacity larger than this will be discarded instead of reused.
	bufferCap int

	// bufferPool reuses byte buffers to reduce allocations during log encoding.
	bufferPool sync.Pool

	// Stdout is the standard output stream used by appenders.
	Stdout io.Writer = os.Stdout
)

func init() {

	RegisterPlugin[DiscardAppender]("DiscardAppender")
	RegisterPlugin[ConsoleAppender]("ConsoleAppender")
	RegisterPlugin[FileAppender]("FileAppender")
	RegisterPlugin[RollingFileAppender]("RollingFileAppender")

	bufferCap = 10 * 1024 // 10KB
	if s, ok := os.LookupEnv("GS_LOGGER_BUFFER_CAP"); ok {
		n, err := ParseHumanizeBytes(s)
		if err != nil {
			panic(errutil.Explain(err, "invalid value for GS_LOGGER_BUFFER_CAP: %q", s))
		}
		bufferCap = n
	}
}

// ParseHumanizeBytes parses a size string like "10KB" into bytes.
// Currently, only the "KB" unit is supported.
// Returns an error if the format is invalid, the unit is unsupported,
// or the value exceeds the allowed limit (10MB).
func ParseHumanizeBytes(s string) (int, error) {
	lastDigit := 0
	for _, r := range s {
		if !unicode.IsDigit(r) {
			break
		}
		lastDigit++
	}
	num := s[:lastDigit]
	f, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0, err
	}
	unit := strings.ToUpper(strings.TrimSpace(s[lastDigit:]))
	if unit != "KB" {
		return 0, errutil.Explain(nil, "invalid unit %q", unit)
	}
	f *= 1024
	if f > 10*1024*1024 {
		return 0, errutil.Explain(nil, "value too large: %d", f)
	}
	return int(f), nil
}

// getBuffer retrieves a *bytes.Buffer from the pool.
// If the pool is empty, it allocates a new buffer.
func getBuffer() *bytes.Buffer {
	if v := bufferPool.Get(); v != nil {
		return v.(*bytes.Buffer)
	}
	return bytes.NewBuffer(nil)
}

// putBuffer resets the buffer and returns it to the pool for reuse.
// Buffers with capacity larger than bufferCap are discarded
// to prevent retaining excessively large memory.
func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= bufferCap {
		buf.Reset()
		bufferPool.Put(buf)
	}
}

// WriteEvent writes a log event to the given io.Writer using the specified Layout.
// If e.RawBytes is not nil, it writes the raw bytes directly.
// Otherwise, the event is encoded using the layout into a temporary buffer.
// Any write errors are reported via ReportError.
func WriteEvent(w io.Writer, e *Event, layout Layout) {
	if e.RawBytes != nil {
		if _, err := w.Write(e.RawBytes); err != nil {
			ReportError(err)
		}
		return
	}

	buf := getBuffer()
	defer putBuffer(buf)
	layout.EncodeTo(e, buf)
	if _, err := w.Write(buf.Bytes()); err != nil {
		ReportError(err)
	}
}

// Appender defines components responsible for writing log events.
// Implementations should document whether they are safe for concurrent use.
//
// Append MUST NOT modify or retain references to the Event.
type Appender interface {
	Lifecycle             // Start/Stop methods for resource management
	GetName() string      // Returns the appender's name
	Append(e *Event)      // Handles writing a log event
	ConcurrentSafe() bool // Returns true if the appender is concurrent-safe
}

// AppenderBase provides common configuration fields for all appenders.
type AppenderBase struct {
	Name   string `PluginAttribute:"name"`
	Layout Layout `PluginElement:"layout,default=TextLayout"`
}

// GetName returns the appender's name.
func (c *AppenderBase) GetName() string { return c.Name }

var (
	_ Appender = (*DiscardAppender)(nil)
	_ Appender = (*ConsoleAppender)(nil)
	_ Appender = (*FileAppender)(nil)
	_ Appender = (*RollingFileAppender)(nil)
)

// DiscardAppender ignores all log events (no-op).
type DiscardAppender struct {
	AppenderBase
}

func (c *DiscardAppender) Start() error         { return nil }
func (c *DiscardAppender) Stop()                {}
func (c *DiscardAppender) Append(e *Event)      {}
func (c *DiscardAppender) ConcurrentSafe() bool { return true }

// ConsoleAppender writes formatted log events to standard output.
type ConsoleAppender struct {
	AppenderBase
}

func (c *ConsoleAppender) Start() error { return nil }
func (c *ConsoleAppender) Stop()        {}

// Append formats the event and writes it to standard output.
func (c *ConsoleAppender) Append(e *Event) {
	WriteEvent(Stdout, e, c.Layout)
}

func (c *ConsoleAppender) ConcurrentSafe() bool { return true }

// FileAppender writes formatted log events to a file in append mode.
type FileAppender struct {
	AppenderBase

	FileDir  string `PluginAttribute:"dir,default=./logs"`
	FileName string `PluginAttribute:"file"`

	file *File
}

// Start opens the log file for appending.
func (c *FileAppender) Start() error {
	filePath := filepath.Join(c.FileDir, c.FileName)
	f, err := OpenFile(filePath)
	if err != nil {
		return err
	}
	c.file = f
	return nil
}

// Stop flushes and closes the file.
func (c *FileAppender) Stop() {
	if c.file != nil {
		CloseFile(c.file)
	}
}

// Append formats the log event and writes it to the file.
func (c *FileAppender) Append(e *Event) {
	WriteEvent(c.file, e, c.Layout)
}

func (c *FileAppender) ConcurrentSafe() bool { return true }

// RollingFileAppender writes log events to files that rotate at fixed time intervals.
// It is safe for concurrent use only when Lock is true.
// If Lock is false, callers must ensure serialized access (e.g., via an async logger).
type RollingFileAppender struct {
	AppenderBase

	FileDir  string        `PluginAttribute:"dir,default=./logs"`
	FileName string        `PluginAttribute:"file"`
	Interval time.Duration `PluginAttribute:"interval,default=1h"`
	MaxAge   time.Duration `PluginAttribute:"maxAge,default=168h"`
	SyncLock bool          `PluginAttribute:"syncLock,default=false"`

	writer *RollingFileWriter
	mutex  sync.Mutex
}

// Start opens the initial log file and prepares for rotation.
func (c *RollingFileAppender) Start() error {
	c.writer = &RollingFileWriter{
		fileDir:  c.FileDir,
		fileName: c.FileName,
		interval: c.Interval,
		maxAge:   c.MaxAge,
	}
	_, err := c.writer.Rotate()
	return err
}

// Stop flushes and closes the current file.
func (c *RollingFileAppender) Stop() {
	c.writer.Close()
}

// Append formats the log event and writes it to the current file.
func (c *RollingFileAppender) Append(e *Event) {
	var (
		file *File
		err  error
	)
	if c.SyncLock { // for sync logger or multi-threaded usage
		c.mutex.Lock()
		file, err = c.writer.Rotate()
		c.mutex.Unlock()
	} else { // for async logger that ensures serialization
		file, err = c.writer.Rotate()
	}
	if err != nil {
		ReportError(err)
	}
	if file != nil {
		WriteEvent(file, e, c.Layout)
	}
}

func (c *RollingFileAppender) ConcurrentSafe() bool { return c.SyncLock }

// RollingFileWriter is the low-level sequential writer.
// It is NOT safe for concurrent use;
// synchronization is the responsibility of the caller/appender.
type RollingFileWriter struct {
	fileDir  string
	fileName string
	interval time.Duration
	currFile *File
	currTime int64
	maxAge   time.Duration
}

// Rotate creates a new log file if the current time exceeds the rotation interval.
// It returns the active file for writing.
// The previous file is closed asynchronously after a delay.
// This method is not concurrency-safe.
func (w *RollingFileWriter) Rotate() (*File, error) {
	now := time.Now()
	newTime := now.Truncate(w.interval).Unix()
	if newTime <= w.currTime {
		return w.currFile, nil
	}

	formatTime := now.Format("20060102150405")
	fileName := w.fileName + "." + formatTime
	filePath := filepath.Join(w.fileDir, fileName)
	file, err := OpenFile(filePath)
	if err != nil {
		return w.currFile, err
	}

	if w.currFile != nil {
		oldFile := w.currFile
		go func() {
			// Delay closing old file. Some logs may be lost.
			time.Sleep(5 * time.Minute)
			CloseFile(oldFile)
			w.clearExpiredFiles()
		}()
	}

	w.currFile = file
	w.currTime = newTime
	return w.currFile, nil
}

// clearExpiredFiles deletes log files matching the configured filename prefix
// that are older than MaxAge. Errors during deletion are ignored.
func (w *RollingFileWriter) clearExpiredFiles() {
	expiration := time.Now().Add(-w.maxAge)
	entries, _ := os.ReadDir(w.fileDir)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), w.fileName+".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(expiration) {
			_ = os.Remove(filepath.Join(w.fileDir, entry.Name()))
		}
	}
}

// Close closes the current file.
func (w *RollingFileWriter) Close() {
	if w.currFile != nil {
		CloseFile(w.currFile)
	}
}
