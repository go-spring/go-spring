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

package mock

import (
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/elliotchance/redismock"
	"github.com/go-redis/redis"
	"github.com/go-spring/spring-boot"
	"github.com/go-spring/starter-db/go-sqlmock-factory"
	"github.com/go-spring/starter-go-redis/go-redis-mock-factory"
)

func init() {

	SpringBoot.RegisterBeanFn(GoSqlMockFactory.MockDB(func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("SELECT ENGINE FROM `ENGINES`").WillReturnRows(
			mock.NewRows([]string{"ENGINE"}).AddRow("sql-mock"),
		)
	}))

	SpringBoot.RegisterBeanFn(GoRedisMockFactory.MockRedisClient(func(mock *redismock.ClientMock) {
		mock.On("Set", "key", "ok", time.Second*10).Return(redis.NewStatusResult("", nil))
		mock.On("Get", "key").Return(redis.NewStringResult("ok", nil))
	}))
}
