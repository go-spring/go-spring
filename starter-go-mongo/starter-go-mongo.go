package StarterGoMongo

import (
	"context"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring/spring-boot"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoConfig struct {
	Url string `value:"${mongo.url}"`
}

func init() {
	SpringBoot.RegisterNameBeanFn("std-go-mongo-client", NewGoMongoClient).
		ConditionOnMissingBean((*mongo.Client)(nil)).
		Destroy(CloseMongo)
}

func NewGoMongoClient(config MongoConfig) (*mongo.Client, error) {
	SpringLogger.Info("open mongo ", config.Url)
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(config.Url))
	if err := client.Ping(nil, readpref.Primary()); err != nil {
		return nil, err
	}
	return client, err
}

func CloseMongo(client *mongo.Client) {
	SpringLogger.Info("close mongo")
	if err := client.Disconnect(context.Background()); err != nil {
		SpringLogger.Error(err)
	}
}
