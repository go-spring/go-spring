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
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/gs/cond"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/starter-core"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

func init() {
	gs.Provide(createDB).Destroy(closeDB).On(cond.OnMissingBean((*gorm.DB)(nil)))
}

// createDB 从配置文件创建 *gorm.DB 客户端
func createDB(config StarterCore.DBConfig) (*gorm.DB, error) {
	log.Info("open gorm mysql ", config.Url)
	return gorm.Open("mysql", config.Url)
}

// closeDB 关闭 *gorm.DB 客户端
func closeDB(db *gorm.DB) {
	log.Info("close gorm mysql")
	if err := db.Close(); err != nil {
		log.Error(err)
	}
}
