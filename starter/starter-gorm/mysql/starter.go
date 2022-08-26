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
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/cond"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Factory struct {
	Logger *log.Logger `logger:""`
}

func (factory *Factory) CreateDB(config database.ClientConfig) (*gorm.DB, error) {
	factory.Logger.Infof("open gorm mysql %s", config.URL)
	db, err := gorm.Open(mysql.Open(config.URL))
	if err != nil {
		return nil, err
	}
	return db, nil
}

func init() {
	gs.Object(&Factory{})
	gs.Provide((*Factory).CreateDB, arg.R1("${gorm}")).
		Name("GormDB").
		On(cond.OnMissingBean(gs.BeanID((*gorm.DB)(nil), "GormDB")))
}
