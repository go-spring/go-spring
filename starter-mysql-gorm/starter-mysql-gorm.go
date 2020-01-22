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
	"database/sql"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/starter-db"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

func init() {

	// 如果没有 fromDB 名称的 *gorm.DB 对象则创建 fromConfig 名称的 *gorm.DB 对象
	SpringBoot.RegisterNameBeanFn("std-gorm-mysql-from-config", fromConfig, "${}").
		ConditionOnMissingBean("github.com/jinzhu/gorm/gorm.DB:").
		Destroy(closeDB)

	// 如果已经有 *sql.DB 对象则创建fromDB 名称的 *gorm.DB 对象
	SpringBoot.RegisterNameBeanFn("std-gorm-mysql-from-db", fromDB).
		ConditionOnBean("database/sql/sql.DB:").
		Destroy(closeDB)
}

// fromConfig 从配置文件创建 *gorm.DB 客户端
func fromConfig(config StarterDB.DBConfig) (*gorm.DB, error) {
	SpringLogger.Info("open gorm mysql", config.Url)
	return gorm.Open("mysql", config.Url)
}

// fromDB 从 *sql.DB 对象创建 *gorm.DB 客户端
func fromDB(db *sql.DB) (*gorm.DB, error) {
	SpringLogger.Info("open gorm mysql")
	return gorm.Open("mysql", db)
}

// closeDB 关闭 *gorm.DB 客户端
func closeDB(db *gorm.DB) {
	SpringLogger.Info("close gorm mysql")
	db.Close()
}
