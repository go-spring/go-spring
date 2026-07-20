/*
 * Copyright 2025 The Go-Spring Authors.
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

package server

import (
	"context"
	"strconv"
	"time"

	"examples/proto"

	"go-spring.org/spring/httpsvr"
)

var _ proto.ManagerService = &ManagerServer{}

type ManagerServer struct{}

func (m *ManagerServer) Assistant(ctx context.Context, req *proto.AssistantReq, resp chan<- *httpsvr.Event[*proto.AssistantEvent]) {
	for i := range 5 {
		event := httpsvr.NewEvent[*proto.AssistantEvent]().
			ID(strconv.Itoa(i)).
			Event("message").
			Data(&proto.AssistantEvent{
				Payload: new(proto.Payload{
					FieldType: proto.PayloadTypeAsString(proto.PayloadType_MessageDelta),
					MessageDelta: &proto.MessageDelta{
						Content: new("hello world"),
						IsFinal: new(false),
					},
				}),
			})
		resp <- event
		time.Sleep(time.Second)
	}
}

func (m *ManagerServer) GetManager(ctx context.Context, req *proto.GetManagerReq) *proto.GetManagerResp {
	level := proto.ManagerLevelAsString(proto.ManagerLevel_JUNIOR)
	return &proto.GetManagerResp{
		Data: &proto.Manager{
			Name:  new("Jim"),
			Level: &level,
		},
	}
}

func (m *ManagerServer) ListManagers(ctx context.Context, req *proto.ListManagersReq) *proto.ListManagersResp {
	return nil
}

func (m *ManagerServer) CreateManager(ctx context.Context, req *proto.CreateManagerReq) *proto.CreateManagerResp {
	return nil
}

func (m *ManagerServer) UpdateManager(ctx context.Context, req *proto.UpdateManagerReq) *proto.UpdateManagerResp {
	return nil
}

func (m *ManagerServer) DeleteManager(ctx context.Context, req *proto.GetManagerReq) *proto.DeleteManagerResp {
	return nil
}
