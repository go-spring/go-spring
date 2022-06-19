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

package queue

import "sync"

var (
	queue *Queue
	once  sync.Once
)

func Instance() *Queue {
	once.Do(func() {
		queue = &Queue{
			ringBuffer: make(chan Item, 100000),
		}
		queue.consume()
	})
	return queue
}

type Item interface {
	OnEvent()
}

type Queue struct {
	ringBuffer chan Item
}

func (q *Queue) Publish(item Item) bool {
	select {
	case q.ringBuffer <- item:
		return true
	default:
		return false
	}
}

func (q *Queue) consume() {
	go func() {
		for {
			v := <-q.ringBuffer
			if v != nil {
				v.OnEvent()
			}
		}
	}()
}
