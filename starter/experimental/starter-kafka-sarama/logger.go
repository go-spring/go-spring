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

package StarterKafkaSarama

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"go-spring.org/log"
)

// saramaLogger bridges sarama's package-level logger (a StdLogger with
// Print/Printf/Println) into go-spring's log so connection events (broker
// connects, metadata refresh, request failures, reconnects) flow through the
// same sinks as application logs. Sarama emits at a single level; everything
// is forwarded as Info.
type saramaLogger struct{}

// newSaramaLogger returns a sarama.StdLogger implementation.
func newSaramaLogger() sarama.StdLogger { return saramaLogger{} }

// Print concatenates the arguments with sprint semantics and logs one line.
func (saramaLogger) Print(v ...any) {
	log.Infof(context.Background(), log.TagAppDef, "kafka: %s", trimNL(fmt.Sprint(v...)))
}

// Printf logs a formatted message.
func (saramaLogger) Printf(format string, v ...any) {
	log.Infof(context.Background(), log.TagAppDef, "kafka: %s", trimNL(fmt.Sprintf(format, v...)))
}

// Println joins its arguments with spaces and logs one line.
func (saramaLogger) Println(v ...any) {
	log.Infof(context.Background(), log.TagAppDef, "kafka: %s", trimNL(fmt.Sprintln(v...)))
}

// trimNL strips the trailing newline that fmt.Sprintln/Sarama append so log
// lines aren't double-spaced in the underlying sink.
func trimNL(s string) string {
	return strings.TrimRight(s, "\r\n")
}
