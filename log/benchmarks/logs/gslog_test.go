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

package benchmarks

import (
	"time"

	"github.com/go-spring/log"
)

func fakeGSAppenders() {
	err := log.RefreshConfig(map[string]string{
		"logger.root.type":  "DiscardLogger",
		"logger.root.level": "warn",
	})

	if err != nil {
		panic(err)
	}
}

func fakeGSlogFields() []log.Field {
	return []log.Field{
		log.Int("int", _tenInts[0]),
		log.Any("ints", _tenInts),
		log.String("string", _tenStrings[0]),
		log.Any("strings", _tenStrings),
		log.String("time", _tenTimes[0].Format(time.RFC3339)),
		log.Any("times", _tenTimes),
		log.Any("user1", _oneUser),
		log.Any("user2", _oneUser),
		log.Any("users", _tenUsers),
		log.Any("error", errExample),
	}
}
