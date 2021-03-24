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

package GoMongoFactory

import (
	"context"

	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/starter-mongo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// NewClient 创建 MongoDB 客户端
func NewClient(config StarterMongo.Config) (*mongo.Client, error) {
	log.Info("open mongo db ", config.Url)
	ctx := context.Background()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.Url))
	if err != nil {
		return nil, err
	}

	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}
	return client, err
}

// CloseClient 关闭 MongoDB 客户端
func CloseClient(client *mongo.Client) {
	log.Info("close mongo db")
	if err := client.Disconnect(context.Background()); err != nil {
		log.Error(err)
	}
}
