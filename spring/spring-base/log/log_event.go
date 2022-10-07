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

import "time"

// Event provides contextual information about a log message.
type Event struct {
	entry  Entry
	time   time.Time
	file   string
	line   int
	level  Level
	fields func() []Field
}

func (e *Event) Entry() Entry {
	return e.entry
}

func (e *Event) Time() time.Time {
	return e.time
}

func (e *Event) File() string {
	return e.file
}

func (e *Event) Line() int {
	return e.line
}

func (e *Event) Level() Level {
	return e.level
}

func (e *Event) Fields() []Field {
	return e.fields()
}
