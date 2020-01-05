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

package StarterMySqlGorm

import (
	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/starter-db"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

func init() {
	SpringBoot.RegisterBeanFn(NewGorm, "${}").Destroy(func(db *gorm.DB) {
		SpringLogger.Info("close mysql")
		db.Close()
	})
}

// NewGorm 创建 gorm.DB 客户端
func NewGorm(config StarterDB.DBConfig) (*gorm.DB, error) {
	SpringLogger.Info("open mysql", config.Url)
	return gorm.Open("mysql", config.Url)
}
