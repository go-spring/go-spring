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
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/starter-gorm/mysql/factory"
	"gorm.io/gorm"
)

func init() {
	gs.Provide(func() (*gorm.DB, error) {
		mockDB, mock, err := factory.MockDB()
		if err != nil {
			return nil, err
		}
		mock.ExpectQuery("SELECT ENGINE FROM `ENGINES`").WillReturnRows(
			mock.NewRows([]string{"ENGINE"}).AddRow("sql-mock"),
		)
		return mockDB, nil
	}).Name("GormDB")
}
