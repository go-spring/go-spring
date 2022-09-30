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

package StarterGoMongo

import (
	"context"

	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/cond"
	"github.com/go-spring/spring-core/mongo"
	g "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Factory struct {
	Logger *log.Logger `logger:""`
}

// NewClient 创建 MongoDB 客户端
func (f *Factory) NewClient(config mongo.ClientConfig) (*g.Client, error) {
	f.Logger.Sugar().Infof("open mongo db %s", config.URL)
	client, err := g.Connect(context.Background(), options.Client().ApplyURI(config.URL))
	if err != nil {
		return nil, err
	}
	if config.Ping {
		err = client.Ping(context.Background(), readpref.Primary())
		if err != nil {
			return nil, err
		}
	}
	return client, err
}

// CloseClient 关闭 MongoDB 客户端
func (f *Factory) CloseClient(client *g.Client) {
	f.Logger.Sugar().Info("close mongo db")
	err := client.Disconnect(context.Background())
	if err != nil {
		f.Logger.Sugar().Error(nil, err)
	}
}

func init() {
	var factory *Factory
	gs.Object(new(Factory)).Init(func(f *Factory) { factory = f })
	gs.Provide((*Factory).NewClient, arg.R1("${mongo}")).
		Destroy(func(client *g.Client) { factory.CloseClient(client) }).
		Name("MongoDB").
		On(cond.OnMissingBean(gs.BeanID((*g.Client)(nil), "MongoDB")))
}
