package httpsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/http/proto"
	order "GS_PROJECT_MODULE/src/internal/order/controller"
	user "GS_PROJECT_MODULE/src/internal/user/controller"

	redigo "github.com/gomodule/redigo/redis"
	goredis "github.com/redis/go-redis/v9"
	"go-spring.org/spring/gs"
)

func init() {
	gs.Object(&GS_PROJECT_NAMEController{})
}

type GS_PROJECT_NAMEController struct {
	order.OrderController
	user.UserController

	RedigoPool    *redigo.Pool    `autowire:""`
	GoRedisClient *goredis.Client `autowire:""`
}

func (c *GS_PROJECT_NAMEController) Ping(ctx context.Context, req *proto.PingReq) *proto.PingResp {
	req.Name = "Go-Spring"

	// Use go-redis client to set a key "ping"
	c.GoRedisClient.Set(ctx, "ping", req.Name, 0)

	p := c.RedigoPool.Get()
	defer func() { p.Close() }()

	// Use redigo client to get the value of the "ping" key
	reply, err := p.Do("GET", "ping")
	if err != nil {
		panic(err)
	}
	data, err := redigo.String(reply, err)
	if err != nil {
		panic(err)
	}

	// Return the Ping response with data read from Redis
	return &proto.PingResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   data,
	}
}
