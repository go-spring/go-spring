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
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/gs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	gs.Provide(mockDB(func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("SELECT VERSION()").WillReturnRows(
			mock.NewRows([]string{"VERSION()"}).AddRow("8.0.27"),
		)
		mock.ExpectQuery("SELECT ENGINE FROM `ENGINES`").WillReturnRows(
			mock.NewRows([]string{"ENGINE"}).AddRow("sql-mock"),
		)
	})).Name("GormDB")
}

// mockDB 创建 gorm.DB Mock 客户端
func mockDB(fn func(sqlmock.Sqlmock)) func() (*gorm.DB, error) {
	return func() (*gorm.DB, error) {
		log.Info("create sqlmock db")
		db, mock, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		fn(mock)
		return gorm.Open(mysql.New(mysql.Config{Conn: db}))
	}
}
