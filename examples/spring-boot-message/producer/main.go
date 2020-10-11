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

package main

import (
	"context"
	"time"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-message"
	"github.com/go-spring/spring-utils"
	_ "github.com/go-spring/starter-rabbitmq/producer"
)

func init() {
	SpringBoot.RegisterBean(new(producer))
}

type producer struct {
	_ SpringBoot.CommandLineRunner `export:""`

	Producer SpringMessage.Producer `autowire:""`
}

func (r *producer) Run(appCtx SpringBoot.ApplicationContext) {
	appCtx.SafeGoroutine(func() {
		time.Sleep(time.Millisecond * 100)
		msg := SpringMessage.NewMessage().WithTopic("topic-stream-idle").WithBody([]byte(`{"stream":"live"}`))
		err := r.Producer.SendMessage(context.Background(), msg)
		SpringUtils.Panic(err).When(err != nil)
		SpringBoot.Exit()
	})
}

func main() {
	SpringBoot.RunApplication()
}
