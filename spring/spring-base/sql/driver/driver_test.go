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

package driver_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/sql/driver"
	"github.com/golang/mock/gomock"
)

type testHook struct {
	events []string
}

func (h *testHook) ConnPrepareContext(ctx context.Context, query string, fn driver.ConnPrepareContextFunc) (driver.Stmt, error) {
	h.events = append(h.events, fmt.Sprint("testHook::ConnPrepareContext", " ", query))
	return fn(ctx, query)
}

func (h *testHook) ConnExecContext(ctx context.Context, query string, args []driver.NamedValue, fn driver.ConnExecContextFunc) (driver.Result, error) {
	h.events = append(h.events, fmt.Sprint("testHook::ConnExecContext", " ", query))
	return fn(ctx, query, args)
}

func (h *testHook) ConnQueryContext(ctx context.Context, query string, args []driver.NamedValue, fn driver.ConnQueryContextFunc) (driver.Rows, error) {
	h.events = append(h.events, fmt.Sprint("testHook::ConnQueryContext", " ", query))
	return fn(ctx, query, args)
}

func (h *testHook) StmtExecContext(ctx context.Context, query string, args []driver.NamedValue, fn driver.StmtExecContextFunc) (driver.Result, error) {
	h.events = append(h.events, fmt.Sprint("testHook::StmtExecContext", " ", query))
	return fn(ctx, query, args)
}

func (h *testHook) StmtQueryContext(ctx context.Context, query string, args []driver.NamedValue, fn driver.StmtQueryContextFunc) (driver.Rows, error) {
	h.events = append(h.events, fmt.Sprint("testHook::StmtQueryContext", " ", query))
	return fn(ctx, query, args)
}

func TestDriver(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	hook := &testHook{}
	defer func() {
		assert.Equal(t, hook.events, []string{
			"testHook::ConnExecContext select * from users",
		})
	}()

	driver.SetHook(hook)
	defer driver.SetHook(nil)

	mockResult := driver.NewMockResult(ctrl)
	mockResult.EXPECT().RowsAffected().Return(int64(1), nil)

	mockConn := driver.NewMockConn(ctrl)
	mockConn.EXPECT().ExecContext(context.Background(), "select * from users", gomock.Any()).Return(mockResult, nil)

	conn, err := driver.NewConn(mockConn)
	assert.Nil(t, err)

	mockConnector := driver.NewMockConnector(ctrl)
	mockConnector.EXPECT().Connect(context.Background()).Return(conn, nil)

	db := sql.OpenDB(mockConnector)
	result, err := db.ExecContext(context.Background(), "select * from users")
	assert.Nil(t, err)
	rowsAffected, err := result.RowsAffected()
	assert.Nil(t, err)
	assert.Equal(t, rowsAffected, int64(1))
}
