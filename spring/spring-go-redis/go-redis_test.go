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

package SpringGoRedis_test

import (
	"context"
	"fmt"
	"testing"

	g "github.com/go-redis/redis/v8"
	SpringGoRedis "github.com/go-spring/spring-go-redis"
)

func TestClient(t *testing.T) {
	var reply interface{}
	ctx := context.Background()
	c := SpringGoRedis.NewClient(g.NewClient(&g.Options{}))
	reply, err := c.Set(ctx, "name", "king", 0)
	if err != nil {
		t.Fatal()
	}
	fmt.Println(reply)
	reply, err = c.Get(ctx, "name")
	if err != nil {
		t.Fatal()
	}
	fmt.Println(reply)
}
