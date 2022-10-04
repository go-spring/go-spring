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

package database

//
//import (
//	"context"
//	"database/sql/driver"
//	"encoding/json"
//	"errors"
//	"io"
//	"unsafe"
//
//	"github.com/go-spring/spring-base/clock"
//	"github.com/go-spring/spring-base/net/recorder"
//	"github.com/go-spring/spring-base/net/replayer"
//)
//
//var (
//	errContextDriver = errors.New("please use context driver")
//	errContextMethod = errors.New("please use context method")
//)
//
//type sqlKeyType int
//
//var sqlKey sqlKeyType
//
//func GetSQL(ctx context.Context) (string, bool) {
//	v := ctx.Value(sqlKey)
//	if v == nil {
//		return "", false
//	}
//	return v.(string), true
//}
//
//func SaveSQL(ctx context.Context, sql string) context.Context {
//	return context.WithValue(ctx, sqlKey, sql)
//}
//
//type Driver struct {
//	driver driver.Driver
//}
//
//func NewDriver(driver driver.Driver) driver.Driver {
//	return &Driver{driver: driver}
//}
//
//func (d *Driver) Open(name string) (driver.Conn, error) {
//	connector, err := d.OpenConnector(name)
//	if err != nil {
//		if err == errContextDriver {
//			var conn driver.Conn
//			conn, err = d.driver.Open(name)
//			if err != nil {
//				return nil, err
//			}
//			return &Conn{conn}, nil
//		}
//		return nil, err
//	}
//	return connector.Connect(context.Background())
//}
//
//func (d *Driver) OpenConnector(name string) (driver.Connector, error) {
//	ciCtx, ok := d.driver.(driver.DriverContext)
//	if !ok {
//		return nil, errContextDriver
//	}
//	connector, err := ciCtx.OpenConnector(name)
//	if err != nil {
//		return nil, err
//	}
//	return &Connector{connector: connector}, nil
//}
//
//type Connector struct {
//	connector driver.Connector
//}
//
//func (c *Connector) Driver() driver.Driver {
//	return c.connector.Driver()
//}
//
//func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) {
//	conn, err := c.connector.Connect(ctx)
//	if err != nil {
//		return nil, err
//	}
//	return &Conn{conn}, nil
//}
//
//type Conn struct {
//	conn driver.Conn
//}
//
//func (c *Conn) Close() error {
//	return c.conn.Close()
//}
//
//func (c *Conn) Prepare(query string) (driver.Stmt, error) {
//	stmt, err := c.PrepareContext(context.Background(), query)
//	if err != nil {
//		if err == errContextDriver {
//			stmt, err = c.conn.Prepare(query)
//			if err != nil {
//				return nil, err
//			}
//			return newStmt(stmt), nil
//		}
//		return nil, err
//	}
//	return stmt, nil
//}
//
//func (c *Conn) Begin() (driver.Tx, error) {
//	tx, err := c.BeginTx(context.Background(), driver.TxOptions{})
//	if err != nil {
//		if err == errContextDriver {
//			return c.conn.Begin()
//		}
//		return nil, err
//	}
//	return tx, nil
//}
//
//func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
//	ciCtx, ok := c.conn.(driver.ConnPrepareContext)
//	if !ok {
//		return nil, errContextDriver
//	}
//	stmt, err := ciCtx.PrepareContext(ctx, query)
//	if err != nil {
//		return nil, err
//	}
//	return newStmt(stmt), nil
//}
//
//func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
//	ciCtx, ok := c.conn.(driver.ConnBeginTx)
//	if !ok {
//		return nil, errContextDriver
//	}
//	return ciCtx.BeginTx(ctx, opts)
//}
//
//func newStmt(stmt driver.Stmt) driver.Stmt {
//	if replayer.ReplayMode() {
//		stmt = &replayStmt{baseStmt{stmt}}
//	}
//	if recorder.RecordMode() {
//		stmt = &recordStmt{baseStmt{stmt}}
//	}
//	return stmt
//}
//
//type baseStmt struct {
//	stmt driver.Stmt
//}
//
//func (c *baseStmt) Close() error {
//	return c.stmt.Close()
//}
//
//func (c *baseStmt) NumInput() int {
//	return c.stmt.NumInput()
//}
//
//func (c *baseStmt) Exec(args []driver.Value) (driver.Result, error) {
//	if recorder.RecordMode() || replayer.ReplayMode() {
//		return nil, errContextMethod
//	}
//	return c.stmt.Exec(args)
//}
//
//func (c *baseStmt) Query(args []driver.Value) (driver.Rows, error) {
//	if recorder.RecordMode() || replayer.ReplayMode() {
//		return nil, errContextMethod
//	}
//	return c.stmt.Query(args)
//}
//
//type recordStmt struct {
//	baseStmt
//}
//
//func (c *recordStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
//	siCtx, ok := c.stmt.(driver.StmtExecContext)
//	if !ok {
//		return nil, errContextDriver
//	}
//	result, err := siCtx.ExecContext(ctx, args)
//	if err != nil {
//		return nil, err
//	}
//	if recorder.RecordMode() {
//		var sql string
//		sql, ok = GetSQL(ctx)
//		if !ok {
//			return nil, errors.New("no sql found")
//		}
//		lastInsertId, _ := result.LastInsertId()
//		rowsAffected, _ := result.RowsAffected()
//		recorder.RecordAction(ctx, &recorder.Action{
//			Protocol: recorder.SQL,
//			Request:  sql,
//			Response: Result{
//				TheLastInsertId: lastInsertId,
//				TheRowsAffected: rowsAffected,
//			},
//			Timestamp: clock.Now(ctx).UnixNano(),
//		})
//	}
//	return result, nil
//}
//
//func (c *recordStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
//	siCtx, ok := c.stmt.(driver.StmtQueryContext)
//	if !ok {
//		return nil, errContextDriver
//	}
//	rows, err := siCtx.QueryContext(ctx, args)
//	if err != nil {
//		return nil, err
//	}
//	return &recordRows{ctx: ctx, rows: rows}, nil
//}
//
//type replayStmt struct {
//	baseStmt
//}
//
//func (c *replayStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
//	sql, ok := GetSQL(ctx)
//	if !ok {
//		return nil, errors.New("no sql found")
//	}
//	action := &recorder.Action{
//		Protocol: recorder.SQL,
//		Request:  sql,
//	}
//	ok, err := recorder.ReplayAction(ctx, action, func(r1, r2 interface{}) bool {
//		return r1 == r2
//	})
//	if err != nil {
//		return nil, err
//	}
//	data, ok := action.Response.(json.RawMessage)
//	if !ok {
//		return nil, errors.New("error response type")
//	}
//	var r Result
//	if err = json.Unmarshal(data, &r); err != nil {
//		return nil, err
//	}
//	return r, nil
//}
//
//func (c *replayStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
//	siCtx, ok := c.stmt.(driver.StmtQueryContext)
//	if !ok {
//		return nil, errContextDriver
//	}
//	rows, err := siCtx.QueryContext(ctx, args)
//	if err != nil {
//		return nil, err
//	}
//	rrs := &replayRows{ctx: ctx, rows: rows}
//	if err = rrs.init(); err != nil {
//		return nil, err
//	}
//	return rrs, nil
//}
//
//type Result struct {
//	TheLastInsertId int64 `json:"last_insert_id"`
//	TheRowsAffected int64 `json:"rows_affected"`
//}
//
//func (r Result) LastInsertId() (int64, error) {
//	return r.TheLastInsertId, nil
//}
//
//func (r Result) RowsAffected() (int64, error) {
//	return r.TheRowsAffected, nil
//}
//
//type recordRows struct {
//	ctx  context.Context
//	rows driver.Rows
//	data struct {
//		Cols []string
//		Rows [][]driver.Value
//	}
//}
//
//func (r *recordRows) Columns() []string {
//	return r.rows.Columns()
//}
//
//func (r *recordRows) Close() error {
//	return r.rows.Close()
//}
//
//func (r *recordRows) Next(dest []driver.Value) error {
//	if err := r.rows.Next(dest); err != nil {
//		if err == io.EOF && recorder.RecordMode() {
//			sql, ok := GetSQL(r.ctx)
//			if !ok {
//				return errors.New("no sql found")
//			}
//			r.data.Cols = r.Columns()
//			recorder.RecordAction(r.ctx, &recorder.Action{
//				Protocol:  recorder.SQL,
//				Request:   sql,
//				Response:  r.data,
//				Timestamp: clock.Now(r.ctx).UnixNano(),
//			})
//		}
//		return err
//	}
//	if recorder.RecordMode() {
//		cols := make([]driver.Value, len(dest))
//		for i := range dest {
//			err := convertAssign(&cols[i], dest[i])
//			if err != nil {
//				return err
//			}
//			switch v := cols[i].(type) {
//			case []byte:
//				v = append([]byte("[]byte@@"), v...)
//				cols[i] = bytesToString(v)
//			}
//		}
//		r.data.Rows = append(r.data.Rows, cols)
//	}
//	return nil
//}
//
////func stringToBytes(s string) []byte {
////	return *(*[]byte)(unsafe.Pointer(
////		&struct {
////			s   string
////			cap int
////		}{
////			s, len(s),
////		},
////	))
////}
//
//func bytesToString(b []byte) string {
//	return *(*string)(unsafe.Pointer(&b))
//}
//
//type replayRows struct {
//	ctx  context.Context
//	rows driver.Rows
//	data struct {
//		Curr int
//		Cols []string
//		Rows [][]driver.Value
//	}
//}
//
//func (r *replayRows) init() error {
//	sql, ok := GetSQL(r.ctx)
//	if !ok {
//		return errors.New("no sql found")
//	}
//	action := &recorder.Action{
//		Protocol: recorder.SQL,
//		Request:  sql,
//	}
//	ok, err := recorder.ReplayAction(r.ctx, action, func(r1, r2 interface{}) bool {
//		return r1 == r2
//	})
//	if err != nil {
//		return err
//	}
//	data, ok := action.Response.(json.RawMessage)
//	if !ok {
//		return errors.New("error response type")
//	}
//	return json.Unmarshal(data, &r.data)
//}
//
//func (r *replayRows) Columns() []string {
//	return r.data.Cols
//}
//
//func (r *replayRows) Close() error {
//	return r.rows.Close()
//}
//
//func (r *replayRows) Next(dest []driver.Value) error {
//	if r.data.Curr >= len(r.data.Rows) {
//		return io.EOF
//	}
//	data := r.data.Rows[r.data.Curr]
//	for i, v := range data {
//		dest[i] = v
//	}
//	return nil
//}
