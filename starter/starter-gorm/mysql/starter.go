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
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/database"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/gs/cond"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	gs.Provide(createDB).On(cond.OnMissingBean((*gorm.DB)(nil)))
}

// createDB 从配置文件创建 *gorm.DB 客户端
func createDB(config database.ClientConfig) (*gorm.DB, error) {
	log.Infof("open gorm mysql %s", config.Url)
	return gorm.Open(mysql.Open(config.Url))
}
