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

package StarterMySqlMock

import (
	"database/sql"
	"fmt"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-spring/go-spring/spring-boot"
)

func init() {
	SpringBoot.RegisterNameBeanFn("mock-mysql-db", func(fn func(sqlmock.Sqlmock)) (*sql.DB, error) {
		db, mock, err := sqlmock.New()
		if err == nil {
			fn(mock)
		}
		return db, err
	}).Destroy(func(db *sql.DB) {
		fmt.Println("close sql db")
		db.Close()
	})
}
