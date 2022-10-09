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

package log_test

import (
	"context"
	"sync"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/log"
)

type CountingNoOpAppender struct {
	log.BaseAppender
	count atomic.Int64
}

func (c *CountingNoOpAppender) Count() int64        { return c.count.Load() }
func (c *CountingNoOpAppender) Append(e *log.Event) { c.count.Add(1) }

func TestCountingNoOpAppender(t *testing.T) {
	appender := &CountingNoOpAppender{}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			appender.Append(nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 6; i++ {
			appender.Append(nil)
		}
	}()
	wg.Wait()
	assert.Equal(t, appender.Count(), int64(9))
}

func TestNullAppender(t *testing.T) {
	appender := new(log.NullAppender)
	err := appender.Start()
	assert.Nil(t, err)
	appender.Stop(context.Background())
	name := appender.GetName()
	assert.Equal(t, name, "")
	layout := appender.GetLayout()
	assert.Nil(t, layout)
	appender.Append(nil)
}

func TestConsoleAppender(t *testing.T) {
	//appender := &log.ConsoleAppender{
	//	BaseAppender: log.BaseAppender{
	//		Layout: nil,
	//	},
	//}
}
